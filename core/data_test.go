package core

import (
	"crypto/md5"
	"fmt"
	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"io"
	"log"
	"os"
	"testing"
)

func TestNewDataSet(t *testing.T) {
	userIDs := []int{1, 2, 3, 4, 5}
	itemIDs := []int{2, 4, 6, 8, 10}
	ratings := []float64{3, 6, 9, 12, 15}
	dataSet := NewDataSet(userIDs, itemIDs, ratings)
	// Check Count()
	assert.Equal(t, 5, dataSet.Count())
	var nilSet *DataSet
	assert.Equal(t, 0, nilSet.Count())
	// Check Get()
	userId, itemId, rating := dataSet.Get(0)
	assert.Equal(t, 1, userId)
	assert.Equal(t, 2, itemId)
	assert.Equal(t, float64(3), rating)
	// Check GetWithIndex()
	userIndex, itemIndex, rating := dataSet.GetWithIndex(0)
	assert.Equal(t, 0, userIndex)
	assert.Equal(t, 0, itemIndex)
	assert.Equal(t, float64(3), rating)
	// Check GlobalMean()
	assert.Equal(t, float64(9), dataSet.GlobalMean())
	// Check UserCount()
	assert.Equal(t, 5, dataSet.UserCount())
	// Check ItemCount()
	assert.Equal(t, 5, dataSet.ItemCount())
}

func TestNewSubSet(t *testing.T) {

}

func md5Sum(fileName string) string {
	// Open file
	f, err := os.Open(fileName)
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()
	// Generate check sum
	h := md5.New()
	if _, err := io.Copy(h, f); err != nil {
		log.Fatal(err)
	}
	return fmt.Sprintf("%x", h.Sum(nil))
}

func TestDownloadFromUrl(t *testing.T) {
	// Download
	fileName, err := downloadFromUrl("https://cdn.sine-x.com/datasets/movielens/ml-100k.zip", downloadDir)
	if err != nil {
		t.Fatal(err)
	}
	// Checksum
	if md5 := md5Sum(fileName); md5 != "0e33842e24a9c977be4e0107933c0723" {
		t.Logf("MD5 sum doesn't match (%s != 0e33842e24a9c977be4e0107933c0723)", md5)
	}
}

func TestUnzip(t *testing.T) {
	// Download
	zipName, err := downloadFromUrl("https://cdn.sine-x.com/datasets/movielens/ml-100k.zip", downloadDir)
	if err != nil {
		t.Fatal("download file failed ", err)
	}
	// Extract files
	fileNames, err := unzip(zipName, dataSetDir)
	// Check
	if err != nil {
		t.Fatal("unzip file failed ", err)
	}
	if len(fileNames) != 24 {
		t.Fatal("Number of file doesn't match")
	}
}

func TestLoadDataFromBuiltIn(t *testing.T) {
	data := LoadDataFromBuiltIn("ml-100k")
	assert.Equal(t, 100000, data.Count())
}

func TestLoadDataFromCSV_Explicit(t *testing.T) {
	data := LoadDataFromCSV("../example/data/implicit.csv", ",", true)
	assert.Equal(t, 5, data.Count())
	for i := 0; i < data.Count(); i++ {
		userId, itemId, value := data.Get(i)
		userIndex, itemIndex, _ := data.GetWithIndex(i)
		assert.Equal(t, i, userId)
		assert.Equal(t, 2*i, itemId)
		assert.Equal(t, 3*i, int(value))
		assert.Equal(t, i, userIndex)
		assert.Equal(t, i, itemIndex)
	}
}

func TestLoadDataFromNetflix(t *testing.T) {
	data := LoadDataFromNetflix("../example/data/netflix.txt", ",", true)
	assert.Equal(t, 5, data.Count())
	for i := 0; i < data.Count(); i++ {
		userId, itemId, value := data.Get(i)
		userIndex, itemIndex, _ := data.GetWithIndex(i)
		assert.Equal(t, 2*i, userId)
		assert.Equal(t, i, itemId)
		assert.Equal(t, 3*i, int(value))
		assert.Equal(t, i, userIndex)
		assert.Equal(t, i, itemIndex)
	}
}

func TestLoadDataFromSQL(t *testing.T) {
	// Create mock database
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
	}
	defer db.Close()
	// Create expect query
	expectRows := sqlmock.NewRows([]string{"user_id", "item_id", "rating"})
	expectRows.AddRow(0, 0, 0)
	expectRows.AddRow(1, 2, 3)
	expectRows.AddRow(2, 4, 6)
	expectRows.AddRow(3, 6, 9)
	expectRows.AddRow(4, 8, 12)
	mock.ExpectQuery("SELECT user_id, item_id, rating FROM ratings;").WillReturnRows(expectRows)
	// Load data from SQL
	data, err := LoadDataFromSQL(db, "ratings", "user_id", "item_id", "rating")
	if err != nil {
		t.Fatalf("error was not expected while query: %s", err)
	}
	// Check data
	assert.Equal(t, 5, data.Count())
	for i := 0; i < data.Count(); i++ {
		userId, itemId, value := data.Get(i)
		denseUserId, denseItemId, _ := data.GetWithIndex(i)
		assert.Equal(t, i, userId)
		assert.Equal(t, 2*i, itemId)
		assert.Equal(t, 3*i, int(value))
		assert.Equal(t, i, denseUserId)
		assert.Equal(t, i, denseItemId)
	}
	// we make sure that all expectations were met
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("there were unfulfilled expectations: %s", err)
	}
}

func TestDataSet_GetUserRatingsSet(t *testing.T) {
	//data := DataSet{
	//	itemIndexer: &base.Indexer{
	//		Indices: map[int]int{0: 0, 2: 1, 4: 2, 6: 3},
	//		IDs:     []int{0, 2, 4, 6},
	//	},
	//	userIndexer: &base.Indexer{
	//		Indices: map[int]int{2: 0},
	//		IDs:     []int{2},
	//	},
	//	denseUserRatings: []*base.SparseVector{
	//		{
	//			Indices: []int{1, 2},
	//			Values:  []float64{10.0, 20.0},
	//		},
	//	},
	//}
	//set := data.GetUserRatingsSet(2)
	//assert.Equal(t, map[int]float64{2: 10, 4: 20}, set)
}
