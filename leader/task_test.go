package leader

import (
	"github.com/stretchr/testify/assert"
	"github.com/thanhpk/randstr"
	"github.com/zhenghaoz/gorse/database"
	"github.com/zhenghaoz/gorse/model"
	"github.com/zhenghaoz/gorse/worker"
	"log"
	"math"
	"os"
	"path"
	"runtime"
	"strconv"
	"testing"
	"time"
)

const (
	rankingEpsilon = 0.008
)

func createTempDir() string {
	dir := path.Join(os.TempDir(), randstr.String(16))
	err := os.Mkdir(dir, 0777)
	if err != nil {
		log.Fatal(err)
	}
	return dir
}

func TestRefreshPopItem(t *testing.T) {
	// Create database
	db, err := database.Open(createTempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	// Insert data
	userIds := []string{"1", "1", "2", "1", "2", "3", "1", "2", "3", "4", "1", "2", "3", "4", "5"}
	itemIds := []string{"1", "2", "2", "3", "3", "3", "4", "4", "4", "4", "5", "5", "5", "5", "5"}
	ratings := []float64{1, 2, 2, 3, 3, 3, 4, 4, 4, 4, 5, 5, 5, 5, 5}
	for i := range userIds {
		if err := db.InsertFeedback(database.Feedback{userIds[i], itemIds[i], ratings[i]}); err != nil {
			t.Fatal(err)
		}
	}
	items := []database.Item{
		{ItemId: "1", Labels: []string{"a"}},
		{ItemId: "2", Labels: []string{"b"}},
		{ItemId: "3", Labels: []string{"a", "b"}},
		{ItemId: "4", Labels: []string{"a"}},
		{ItemId: "5", Labels: []string{"b"}},
	}
	if err = db.InsertItems(items, true); err != nil {
		t.Fatal(err)
	}
	// Refresh
	if err = worker.UpdatePopItem(db, 3); err != nil {
		t.Fatal(err)
	}
	// Check popular itemIds
	recommends, err := db.GetPop("", 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, []database.RecommendedItem{
		{database.Item{ItemId: "5", Popularity: 5, Labels: items[4].Labels}, 5},
		{database.Item{ItemId: "4", Popularity: 4, Labels: items[3].Labels}, 4},
		{database.Item{ItemId: "3", Popularity: 3, Labels: items[2].Labels}, 3},
	}, recommends)
	recommends, err = db.GetPop("a", 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, []database.RecommendedItem{
		{database.Item{ItemId: "4", Popularity: 4, Labels: items[3].Labels}, 4},
		{database.Item{ItemId: "3", Popularity: 3, Labels: items[2].Labels}, 3},
		{database.Item{ItemId: "1", Popularity: 1, Labels: items[0].Labels}, 1},
	}, recommends)
	recommends, err = db.GetPop("b", 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, []database.RecommendedItem{
		{database.Item{ItemId: "5", Popularity: 5, Labels: items[4].Labels}, 5},
		{database.Item{ItemId: "3", Popularity: 3, Labels: items[2].Labels}, 3},
		{database.Item{ItemId: "2", Popularity: 2, Labels: items[1].Labels}, 2},
	}, recommends)
}

func TestRefreshLatest(t *testing.T) {
	// Create database
	db, err := database.Open(createTempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	// Insert data
	itemIds := []string{"1", "2", "3", "4", "5", "6"}
	timestamps := []time.Time{
		time.Date(2010, 1, 1, 1, 1, 1, 1, time.UTC),
		time.Date(2020, 1, 1, 1, 1, 1, 1, time.UTC),
		time.Date(2030, 1, 1, 1, 1, 1, 1, time.UTC),
		time.Date(2025, 1, 1, 1, 1, 1, 1, time.UTC),
		time.Date(2015, 1, 1, 1, 1, 1, 1, time.UTC),
		time.Date(2005, 1, 1, 1, 1, 1, 1, time.UTC),
	}
	labels := [][]string{
		{"a"},
		{"b"},
		{"a"},
		{"a", "b"},
		{"a", "b"},
		{"b"},
	}
	for i := range itemIds {
		if err = db.InsertItem(database.Item{ItemId: itemIds[i], Timestamp: timestamps[i], Labels: labels[i]}, true); err != nil {
			t.Fatal(err)
		}
	}
	// Update latest
	if err = worker.RefreshLatest(db, 3); err != nil {
		t.Fatal(err)
	}
	// Check popular items
	recommends, err := db.GetLatest("", 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, []database.RecommendedItem{
		{database.Item{ItemId: itemIds[2], Timestamp: timestamps[2], Labels: labels[2]}, float64(timestamps[2].Unix())},
		{database.Item{ItemId: itemIds[3], Timestamp: timestamps[3], Labels: labels[3]}, float64(timestamps[3].Unix())},
		{database.Item{ItemId: itemIds[1], Timestamp: timestamps[1], Labels: labels[1]}, float64(timestamps[1].Unix())},
	}, recommends)
	recommends, err = db.GetLatest("a", 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, []database.RecommendedItem{
		{database.Item{ItemId: itemIds[2], Timestamp: timestamps[2], Labels: labels[2]}, float64(timestamps[2].Unix())},
		{database.Item{ItemId: itemIds[3], Timestamp: timestamps[3], Labels: labels[3]}, float64(timestamps[3].Unix())},
		{database.Item{ItemId: itemIds[4], Timestamp: timestamps[4], Labels: labels[4]}, float64(timestamps[4].Unix())},
	}, recommends)
	recommends, err = db.GetLatest("b", 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, []database.RecommendedItem{
		{database.Item{ItemId: itemIds[3], Timestamp: timestamps[3], Labels: labels[3]}, float64(timestamps[3].Unix())},
		{database.Item{ItemId: itemIds[1], Timestamp: timestamps[1], Labels: labels[1]}, float64(timestamps[1].Unix())},
		{database.Item{ItemId: itemIds[4], Timestamp: timestamps[4], Labels: labels[4]}, float64(timestamps[4].Unix())},
	}, recommends)
}

func TestLabelCosine(t *testing.T) {
	// Create database
	db, err := database.Open(createTempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	// Insert items
	items := []database.Item{
		{ItemId: "1", Labels: []string{"a", "b"}},
		{ItemId: "2", Labels: []string{"c", "d"}},
		{ItemId: "3", Labels: []string{"a", "c"}},
	}
	if err = db.InsertItems(items, true); err != nil {
		t.Fatal(err)
	}
	// Check
	score, err := worker.LabelCosine(db, "1", "2")
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, 0.0, score)
	score, err = worker.LabelCosine(db, "1", "3")
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, 1.0/math.Sqrt(2.0)/math.Sqrt(2.0), score)
}

func TestFeedbackCosine(t *testing.T) {
	// Create database
	db, err := database.Open(createTempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	// Insert data
	feedback := []database.Feedback{
		{ItemId: "1", UserId: "1"},
		{ItemId: "1", UserId: "2"},
		{ItemId: "2", UserId: "3"},
		{ItemId: "2", UserId: "4"},
		{ItemId: "3", UserId: "1"},
		{ItemId: "3", UserId: "3"},
	}
	if err := db.InsertFeedbacks(feedback); err != nil {
		t.Fatal(err)
	}
	// Check
	score, err := worker.FeedbackCosine(db, "1", "2")
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, 0.0, score)
	score, err = worker.FeedbackCosine(db, "1", "3")
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, 1.0/math.Sqrt(2.0)/math.Sqrt(2.0), score)
}

func TestRefreshNeighbors(t *testing.T) {
	// Create database
	db, err := database.Open(createTempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	// Insert feedback
	users := make([]string, 0, 81)
	items := make([]string, 0, 81)
	ratings := make([]float64, 0, 81)
	for i := 1; i < 10; i++ {
		for j := 1; j < 10; j++ {
			if i+j < 9 {
				users = append(users, strconv.Itoa(i))
				items = append(items, strconv.Itoa(j))
				ratings = append(ratings, 1)
			}
		}
	}
	for i := range items {
		if err = db.InsertFeedback(database.Feedback{ItemId: items[i], UserId: users[i]}); err != nil {
			t.Fatal(err)
		}
	}
	if err = worker.RefreshNeighbors(db, 5, runtime.NumCPU()); err != nil {
		t.Fatal(err)
	}
	// Find N nearest neighbors
	recommends, err := db.GetNeighbors("1", 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, 5, len(recommends))
	assert.Equal(t, "2", recommends[0].ItemId)
	assert.Equal(t, "3", recommends[1].ItemId)
	assert.Equal(t, "4", recommends[2].ItemId)
	assert.Equal(t, "5", recommends[3].ItemId)
	assert.Equal(t, "6", recommends[4].ItemId)
}

func TestRefreshRecommends(t *testing.T) {
	// Create database
	db, err := database.Open(createTempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	// Update recommends
	dataSet := model.LoadDataFromBuiltIn("ml-100k")
	trainSets, testSets := model.NewRatioSplitter(1, 0.2)(dataSet, 0)
	trainSet, testSet := trainSets[0], testSets[0]
	feedback := make([]database.Feedback, 0, trainSet.Count())
	for i := 0; i < trainSet.Count(); i++ {
		userId, itemId, rating := trainSet.Get(i)
		feedback = append(feedback, database.Feedback{userId, itemId, rating})
	}
	if err = db.InsertFeedbacks(feedback); err != nil {
		t.Fatal(err)
	}
	ranker := model.NewItemPop(nil)
	ranker.Fit(trainSet, nil)
	if err = worker.UpdatePopItem(db, 100); err != nil {
		t.Fatal(err)
	}
	if err = worker.RefreshRecommends(db, ranker, 10, runtime.NumCPU(), worker.CollectPop); err != nil {
		t.Fatal(err)
	}
	// Check result
	precision, recall, count := 0.0, 0.0, 0.0
	for userIndex := 0; userIndex < testSet.UserCount(); userIndex++ {
		userId := trainSet.UserIndexer().ToID(userIndex)
		// Find top-n items in test set
		targetSet := testSet.UserByIndex(userIndex)
		if targetSet.Len() > 0 {
			// Find top-n items in database
			rankList, err := db.GetRecommend(userId, 10, 0)
			if err != nil {
				t.Fatal(err)
			}
			count++
			items := make([]string, len(rankList))
			for i := range rankList {
				items[i] = rankList[i].ItemId
			}
			precision += model.Precision(targetSet, items)
			recall += model.Recall(targetSet, items)
		}
	}
	precision /= count
	recall /= count
	t.Logf("Precision@10 = %v, Recall@10 = %v", precision, recall)
	if precision-0.190 < -rankingEpsilon {
		t.Fatal("unexpected Precision@10")
	}
	if recall-0.116 < -rankingEpsilon {
		t.Fatal("unexpected Recall@10")
	}
}
