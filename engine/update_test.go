package engine

import (
	"github.com/stretchr/testify/assert"
	"github.com/zhenghaoz/gorse/base"
	"github.com/zhenghaoz/gorse/core"
	"os"
	"path"
	"runtime"
	"testing"
)

const rankingEpsilon = 0.008

func TestUpdateItemPop(t *testing.T) {
	// Create database
	db, err := Open(path.Join(core.TempDir, "/test_update_item_pop.db"))
	if err != nil {
		t.Fatal(err)
	}
	// Update popular items
	users := []int{1, 1, 2, 1, 2, 3, 1, 2, 3, 4, 1, 2, 3, 4, 5}
	items := []int{1, 2, 2, 3, 3, 3, 4, 4, 4, 4, 5, 5, 5, 5, 5}
	ratings := []float64{1, 2, 2, 3, 3, 3, 4, 4, 4, 4, 5, 5, 5, 5, 5}
	dataSet := core.NewDataSet(users, items, ratings)
	if err = UpdateItemPop(3, dataSet, db); err != nil {
		t.Fatal(err)
	}
	// Check popular items
	recommends, err := db.GetPopular(0)
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, []RecommendedItem{
		{5, 5}, {4, 4}, {3, 3},
	}, recommends)
	// Clean database
	if err = os.Remove(path.Join(core.TempDir, "/test_update_item_pop.db")); err != nil {
		t.Fatal(err)
	}
}

func TestUpdateNeighbors(t *testing.T) {
	// Create database
	db, err := Open(path.Join(core.TempDir, "/test_update_neighbors.db"))
	if err != nil {
		t.Fatal(err)
	}
	// Update neighbor
	users := make([]int, 0, 81)
	items := make([]int, 0, 81)
	ratings := make([]float64, 0, 81)
	for i := 1; i < 10; i++ {
		for j := 1; j < 10; j++ {
			users = append(users, i)
			items = append(items, j)
			if i+j < 9 {
				ratings = append(ratings, 1)
			} else {
				ratings = append(ratings, 0)
			}
		}
	}
	dataSet := core.NewDataSet(users, items, ratings)
	if err = UpdateNeighbors("msd", 5, dataSet, db); err != nil {
		t.Fatal(err)
	}
	// Find N nearest neighbors
	recommends, err := db.GetNeighbors(1, 0)
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, 2, recommends[0].ItemId)
	assert.Equal(t, 3, recommends[1].ItemId)
	assert.Equal(t, 4, recommends[2].ItemId)
	assert.Equal(t, 5, recommends[3].ItemId)
	assert.Equal(t, 6, recommends[4].ItemId)
	// Clean database
	if err = os.Remove(path.Join(core.TempDir, "/test_update_neighbors.db")); err != nil {
		t.Fatal(err)
	}
}

func TestUpdateRecommends(t *testing.T) {
	// Create database
	db, err := Open(path.Join(core.TempDir, "/test_update_recommends.db"))
	if err != nil {
		t.Fatal(err)
	}
	// Update recommends
	dataSet := core.LoadDataFromBuiltIn("ml-100k")
	trainSet, testSet := core.Split(dataSet, 0.2)
	params := base.Params{
		base.NFactors:   10,
		base.Reg:        0.01,
		base.Lr:         0.05,
		base.NEpochs:    100,
		base.InitMean:   0,
		base.InitStdDev: 0.001,
	}
	if err = UpdateRecommends("bpr", params, 10, runtime.NumCPU(), trainSet, db); err != nil {
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
			rankList, err := db.GetRecommends(userId, 10)
			if err != nil {
				t.Fatal(err)
			}
			count++
			items := make([]int, len(rankList))
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
	// Clean database
	if err = os.Remove(path.Join(core.TempDir, "/test_update_recommends.db")); err != nil {
		t.Fatal(err)
	}
}
