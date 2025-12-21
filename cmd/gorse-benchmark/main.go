package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"runtime"
	"sort"
	"strings"

	mapset "github.com/deckarep/golang-set/v2"
	"github.com/gorse-io/gorse/config"
	"github.com/gorse-io/gorse/dataset"
	"github.com/gorse-io/gorse/logics"
	"github.com/gorse-io/gorse/master"
	"github.com/gorse-io/gorse/model/ctr"
	"github.com/gorse-io/gorse/storage"
	"github.com/gorse-io/gorse/storage/data"
	"github.com/samber/lo"
	"github.com/spf13/cobra"
	"golang.org/x/term"
	"modernc.org/sortutil"
)

var rootCmd = &cobra.Command{
	Use:   "gorse-benchmark",
	Short: "Gorse Benchmarking Tool",
}

var llmCmd = &cobra.Command{
	Use:   "llm",
	Short: "Benchmark LLM models",
	Run: func(cmd *cobra.Command, args []string) {
		// Load configuration
		configPath, _ := cmd.Flags().GetString("config")
		cfg, err := config.LoadConfig(configPath)
		if err != nil {
			log.Fatalf("failed to load config: %v", err)
		}
		// Load dataset
		m := master.NewMaster(cfg, os.TempDir(), false)
		m.DataClient, err = data.Open(m.Config.Database.DataStore, m.Config.Database.DataTablePrefix,
			storage.WithIsolationLevel(m.Config.Database.MySQL.IsolationLevel))
		if err != nil {
			log.Fatalf("failed to open data client: %v", err)
		}
		evaluator := master.NewOnlineEvaluator(
			m.Config.Recommend.DataSource.PositiveFeedbackTypes,
			m.Config.Recommend.DataSource.ReadFeedbackTypes)
		dataset, aux, err := m.LoadDataFromDatabase(context.Background(), m.DataClient,
			m.Config.Recommend.DataSource.PositiveFeedbackTypes,
			m.Config.Recommend.DataSource.ReadFeedbackTypes,
			m.Config.Recommend.DataSource.ItemTTL,
			m.Config.Recommend.DataSource.PositiveFeedbackTTL,
			evaluator,
			nil)
		if err != nil {
			log.Fatalf("failed to load dataset: %v", err)
		}
		fmt.Println("Dataset loaded:")
		fmt.Printf("  Users: %d\n", dataset.CountUsers())
		fmt.Printf("  Items: %d\n", dataset.CountItems())
		fmt.Printf("  Positive Feedbacks: %d\n", dataset.CountPositive())
		fmt.Printf("  Negative Feedbacks: %d\n", dataset.CountNegative())
		// Split dataset
		train, test := dataset.Split(0.8, 42)
		EvaluateLLM(cfg, train, test, aux.GetItems())
		// EvaluateFM(train, test)
	},
}

func EvaluateLLM(cfg *config.Config, train, test dataset.CTRSplit, items []data.Item) float32 {
	PrintHorizontalLine("-")
	fmt.Println("Evaluating LLM...")
	chat, err := logics.NewChatRanker(cfg.OpenAI, cfg.Recommend.Ranker.Prompt)
	if err != nil {
		log.Fatalf("failed to create chat ranker: %v", err)
	}

	userTrain := make(map[int32][]int32, train.CountUsers())
	for i := 0; i < train.Count(); i++ {
		indices, _, target := train.Get(i)
		userId := indices[0]
		itemId := indices[1] - int32(train.CountUsers())
		if target > 0 {
			userTrain[userId] = append(userTrain[userId], itemId)
		}
	}

	userTest := make(map[int32][]int32, test.CountUsers())
	userPositive := make(map[int32]mapset.Set[int32])
	userNegative := make(map[int32]mapset.Set[int32])
	for i := 0; i < test.Count(); i++ {
		indices, _, target := test.Get(i)
		userId := indices[0]
		itemId := indices[1] - int32(test.CountUsers())
		userTest[userId] = append(userTest[userId], itemId)
		if target > 0 {
			if _, ok := userPositive[userId]; !ok {
				userPositive[userId] = mapset.NewSet[int32]()
			}
			userPositive[userId].Add(itemId)
		} else {
			if _, ok := userNegative[userId]; !ok {
				userNegative[userId] = mapset.NewSet[int32]()
			}
			userNegative[userId].Add(itemId)
		}
	}

	var sumAUC float32
	var validUsers float32
	for userId, testItems := range userTest {
		if _, ok := userPositive[userId]; !ok {
			continue
		}
		if _, ok := userNegative[userId]; !ok {
			continue
		}
		candidates := make([]*data.Item, 0, len(testItems))
		for _, itemId := range testItems {
			candidates = append(candidates, &items[itemId])
		}
		feedback := make([]*logics.FeedbackItem, 0, len(testItems))
		for _, itemId := range userTrain[userId] {
			feedback = append(feedback, &logics.FeedbackItem{
				Item: items[itemId],
			})
		}
		result, err := chat.Rank(&data.User{}, feedback, candidates)
		if err != nil {
			log.Fatalf("failed to rank items for user %d: %v", userId, err)
		}
		var posPredictions, negPredictions []float32
		for i, name := range result {
			itemId := test.GetIndex().EncodeItem(name) - int32(test.CountUsers())
			if userPositive[userId].Contains(itemId) {
				posPredictions = append(posPredictions, float32(len(result)-i))
			} else if userNegative[userId].Contains(itemId) {
				negPredictions = append(negPredictions, float32(len(result)-i))
			} else {
				log.Fatalf("item %s not found in test set for user %d", name, userId)
			}
		}
		sumAUC += AUC(posPredictions, negPredictions) * float32(len(posPredictions))
		validUsers += float32(len(posPredictions))
		fmt.Println("User", userId, "AUC:", AUC(posPredictions, negPredictions))
		if validUsers >= 100 {
			break
		}
	}
	if validUsers == 0 {
		return 0
	}

	score := sumAUC / validUsers
	fmt.Println("LLM GAUC:", score)
	return score
}

func EvaluateFM(train, test dataset.CTRSplit) float32 {
	PrintHorizontalLine("-")
	fmt.Println("Training FM...")
	ml := ctr.NewFMV2(nil)
	ml.Fit(context.Background(), train, test,
		ctr.NewFitConfig().
			SetVerbose(10).
			SetJobs(runtime.NumCPU()).
			SetPatience(10))

	var posFeatures, negFeatures []lo.Tuple2[[]int32, []float32]
	var posUsers, negUsers []int32
	for i := 0; i < test.Count(); i++ {
		indices, values, target := test.Get(i)
		userId := indices[0]
		if target > 0 {
			posFeatures = append(posFeatures, lo.Tuple2[[]int32, []float32]{A: indices, B: values})
			posUsers = append(posUsers, userId)
		} else {
			negFeatures = append(negFeatures, lo.Tuple2[[]int32, []float32]{A: indices, B: values})
			negUsers = append(negUsers, userId)
		}
	}
	posPrediction := ml.BatchInternalPredict(posFeatures, runtime.NumCPU())
	negPrediction := ml.BatchInternalPredict(negFeatures, runtime.NumCPU())

	userPosPrediction := make(map[int32][]float32)
	userNegPrediction := make(map[int32][]float32)
	for i, p := range posPrediction {
		userPosPrediction[posUsers[i]] = append(userPosPrediction[posUsers[i]], p)
	}
	for i, p := range negPrediction {
		userNegPrediction[negUsers[i]] = append(userNegPrediction[negUsers[i]], p)
	}
	var sumAUC float32
	var validUsers float32
	for user, pos := range userPosPrediction {
		if neg, ok := userNegPrediction[user]; ok {
			sumAUC += AUC(pos, neg) * float32(len(pos))
			validUsers += float32(len(pos))
		}
	}
	if validUsers == 0 {
		return 0
	}
	score := sumAUC / validUsers

	fmt.Println("FM GAUC:", score)
	return score
}

func AUC(posPrediction, negPrediction []float32) float32 {
	sort.Sort(sortutil.Float32Slice(posPrediction))
	sort.Sort(sortutil.Float32Slice(negPrediction))
	var sum float32
	var nPos int
	for pPos := range posPrediction {
		// find the negative sample with the greatest prediction less than current positive sample
		for nPos < len(negPrediction) && negPrediction[nPos] < posPrediction[pPos] {
			nPos++
		}
		// add the number of negative samples have less prediction than current positive sample
		sum += float32(nPos)
	}
	if len(posPrediction)*len(negPrediction) == 0 {
		return 0
	}
	return sum / float32(len(posPrediction)*len(negPrediction))
}

func PrintHorizontalLine(char string) {
	width, _, err := term.GetSize(int(os.Stdout.Fd()))
	if err != nil {
		width = 80
	}
	line := strings.Repeat(char, width)
	fmt.Println(line)
}

func init() {
	rootCmd.PersistentFlags().StringP("config", "c", "", "Path to configuration file")
	rootCmd.AddCommand(llmCmd)
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		log.Fatal(err)
	}
}
