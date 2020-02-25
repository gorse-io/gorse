package engine

import (
	"github.com/stretchr/testify/assert"
	"github.com/thanhpk/randstr"
	"github.com/zhenghaoz/gorse/base"
	"github.com/zhenghaoz/gorse/core"
	"os"
	"path"
	"runtime"
	"strconv"
	"testing"
	"time"
)

const rankingEpsilon = 0.008

func TestUpdateItemPop(t *testing.T) {
	// Create database
	fileName := path.Join(core.TempDir, randstr.String(16))
	db, err := Open(fileName)
	if err != nil {
		t.Fatal(err)
	}
	// Update popular items
	users := []string{"1", "1", "2", "1", "2", "3", "1", "2", "3", "4", "1", "2", "3", "4", "5"}
	items := []string{"1", "2", "2", "3", "3", "3", "4", "4", "4", "4", "5", "5", "5", "5", "5"}
	ratings := []float64{1, 2, 2, 3, 3, 3, 4, 4, 4, 4, 5, 5, 5, 5, 5}
	dataSet := core.NewDataSet(users, items, ratings)
	if err = UpdatePopularity(dataSet, db); err != nil {
		t.Fatal(err)
	}
	if err = UpdatePopItem(3, db); err != nil {
		t.Fatal(err)
	}
	// Check popular items
	recommends, err := db.GetList(ListPop, 0)
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, []RecommendedItem{
		{Item{ItemId: "5", Popularity: 5}, 5},
		{Item{ItemId: "4", Popularity: 4}, 4},
		{Item{ItemId: "3", Popularity: 3}, 3},
	}, recommends)
	// Close database
	if err = db.Close(); err != nil {
		t.Fatal(err)
	}
	// Clean database
	if err = os.Remove(fileName); err != nil {
		t.Fatal(err)
	}
}

func TestUpdateLatest(t *testing.T) {
	// Create database
	fileName := path.Join(core.TempDir, randstr.String(16))
	db, err := Open(fileName)
	if err != nil {
		t.Fatal(err)
	}
	// Insert items
	itemIds := []string{"1", "2", "3", "4", "5", "6"}
	timestamps := []time.Time{
		time.Date(2010, 1, 1, 1, 1, 1, 1, time.UTC),
		time.Date(2020, 1, 1, 1, 1, 1, 1, time.UTC),
		time.Date(2030, 1, 1, 1, 1, 1, 1, time.UTC),
		time.Date(2025, 1, 1, 1, 1, 1, 1, time.UTC),
		time.Date(2015, 1, 1, 1, 1, 1, 1, time.UTC),
		time.Date(2005, 1, 1, 1, 1, 1, 1, time.UTC),
	}
	if err = db.InsertItems(itemIds, timestamps); err != nil {
		t.Fatal(err)
	}
	// Update latest
	if err = UpdateLatest(3, db); err != nil {
		t.Fatal(err)
	}
	// Check popular items
	recommends, err := db.GetList(ListLatest, 0)
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, []RecommendedItem{
		{Item{ItemId: itemIds[2], Timestamp: timestamps[2]}, float64(timestamps[2].Unix())},
		{Item{ItemId: itemIds[3], Timestamp: timestamps[3]}, float64(timestamps[3].Unix())},
		{Item{ItemId: itemIds[1], Timestamp: timestamps[1]}, float64(timestamps[1].Unix())},
	}, recommends)
	// Close database
	if err = db.Close(); err != nil {
		t.Fatal(err)
	}
	// Clean database
	if err = os.Remove(fileName); err != nil {
		t.Fatal(err)
	}
}

func TestUpdateNeighbors(t *testing.T) {
	// Create database
	fileName := path.Join(core.TempDir, randstr.String(16))
	db, err := Open(fileName)
	if err != nil {
		t.Fatal(err)
	}
	// Update neighbor
	users := make([]string, 0, 81)
	items := make([]string, 0, 81)
	ratings := make([]float64, 0, 81)
	for i := 1; i < 10; i++ {
		for j := 1; j < 10; j++ {
			users = append(users, strconv.Itoa(i))
			items = append(items, strconv.Itoa(j))
			if i+j < 9 {
				ratings = append(ratings, 1)
			} else {
				ratings = append(ratings, 0)
			}
		}
	}
	dataSet := core.NewDataSet(users, items, ratings)
	if err = db.InsertItems(items, nil); err != nil {
		t.Fatal(err)
	}
	if err = UpdateNeighbors("msd", 5, dataSet, db); err != nil {
		t.Fatal(err)
	}
	// Find N nearest neighbors
	recommends, err := db.GetIdentList(BucketNeighbors, "1", 0)
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, "2", recommends[0].ItemId)
	assert.Equal(t, "3", recommends[1].ItemId)
	assert.Equal(t, "4", recommends[2].ItemId)
	assert.Equal(t, "5", recommends[3].ItemId)
	assert.Equal(t, "6", recommends[4].ItemId)
	// Close database
	if err = db.Close(); err != nil {
		t.Fatal(err)
	}
	// Clean database
	if err = os.Remove(fileName); err != nil {
		t.Fatal(err)
	}
}

func TestUpdateRecommends(t *testing.T) {
	// Create database
	fileName := path.Join(core.TempDir, randstr.String(16))
	db, err := Open(fileName)
	if err != nil {
		t.Fatal(err)
	}
	// Update recommends
	dataSet := core.LoadDataFromBuiltIn("ml-100k")
	itemId := make([]string, dataSet.ItemCount())
	for itemIndex := 0; itemIndex < dataSet.ItemCount(); itemIndex++ {
		itemId[itemIndex] = dataSet.ItemIndexer().ToID(itemIndex)
	}
	if err = db.InsertItems(itemId, nil); err != nil {
		t.Fatal(err)
	}
	trainSet, testSet := core.Split(dataSet, 0.2)
	params := base.Params{
		base.NFactors:   10,
		base.Reg:        0.01,
		base.Lr:         0.05,
		base.NEpochs:    100,
		base.InitMean:   0,
		base.InitStdDev: 0.001,
	}
	if err = UpdateRecommends("bpr", params, 10, runtime.NumCPU(),false, trainSet, db); err != nil {
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
			rankList, err := db.GetIdentList(BucketRecommends, userId, 10)
			if err != nil {
				t.Fatal(err)
			}
			count++
			items := make([]string, len(rankList))
			for i := range rankList {
				items[i] = rankList[i].ItemId
			}
			precision += core.Precision(targetSet, items)
			recall += core.Recall(targetSet, items)
		}
	}
	precision /= count
	recall /= count
	t.Logf("Precision@10 = %v, Recall@10 = %v", precision, recall)
	if precision-0.321 < -rankingEpsilon {
		t.Log("unexpected Precision@10")
	}
	if precision-0.209 < -rankingEpsilon {
		t.Log("unexpected Recall@10")
	}
	// Close database
	if err = db.Close(); err != nil {
		t.Fatal(err)
	}
	// Clean database
	if err = os.Remove(fileName); err != nil {
		t.Fatal(err)
	}
}

func TestUpdateRecommendsInvalidModel(t *testing.T) {
	// Create database
	fileName := path.Join(core.TempDir, randstr.String(16))
	db, err := Open(fileName)
	if err != nil {
		t.Fatal(err)
	}
	// Update recommends
	dataSet := core.LoadDataFromBuiltIn("ml-100k")
	itemId := make([]string, dataSet.ItemCount())
	for itemIndex := 0; itemIndex < dataSet.ItemCount(); itemIndex++ {
		itemId[itemIndex] = dataSet.ItemIndexer().ToID(itemIndex)
	}
	if err = db.InsertItems(itemId, nil); err != nil {
		t.Fatal(err)
	}
	if err = UpdateRecommends("invalid-model", nil, 10, runtime.NumCPU(), false,dataSet, db); err == nil {
		t.Fatal("function should return an error")
	}
	// Close database
	if err = db.Close(); err != nil {
		t.Fatal(err)
	}
	// Clean database
	if err = os.Remove(fileName); err != nil {
		t.Fatal(err)
	}
}
