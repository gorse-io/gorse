package core

import (
	"crypto/md5"
	"fmt"
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

//func TestDownloadFromUrl(t *testing.T) {
//	// Download
//	fileName, err := downloadFromUrl("https://cdn.sine-x.com/datasets/movielens/ml-100k.zip", downloadDir)
//	if err != nil {
//		t.Fatal(err)
//	}
//	// Checksum
//	if sum := md5Sum(fileName); sum != "0e33842e24a9c977be4e0107933c0723" {
//		t.Logf("MD5 sum doesn't match (%s != 0e33842e24a9c977be4e0107933c0723)", sum)
//	}
//}
//
//func TestUnzip(t *testing.T) {
//	// Download
//	zipName, err := downloadFromUrl("https://cdn.sine-x.com/datasets/movielens/ml-100k.zip", downloadDir)
//	if err != nil {
//		t.Fatal("download file failed ", err)
//	}
//	// Extract files
//	fileNames, err := unzip(zipName, dataSetDir)
//	// Check
//	if err != nil {
//		t.Fatal("unzip file failed ", err)
//	}
//	if len(fileNames) != 24 {
//		t.Fatal("Number of file doesn't match")
//	}
//}

func TestLoadDataFromBuiltIn(t *testing.T) {
	data := LoadDataFromBuiltIn("filmtrust")
	assert.Equal(t, 35497, data.Count())
}

func TestLoadDataFromCSV_Implicit(t *testing.T) {
	data := LoadDataFromCSV("../example/file_data/feedback_implicit.csv", ",", false)
	assert.Equal(t, 5, data.Count())
	for i := 0; i < data.Count(); i++ {
		userId, itemId, value := data.Get(i)
		userIndex, itemIndex, _ := data.GetWithIndex(i)
		assert.Equal(t, i, userId)
		assert.Equal(t, 2*i, itemId)
		assert.Equal(t, 0.0, value)
		assert.Equal(t, i, userIndex)
		assert.Equal(t, i, itemIndex)
	}
}

func TestLoadDataFromCSV_Explicit(t *testing.T) {
	data := LoadDataFromCSV("../example/file_data/feedback_explicit.csv", ",", false)
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

func TestLoadDataFromCSV_Explicit_Header(t *testing.T) {
	data := LoadDataFromCSV("../example/file_data/feedback_explicit_header.csv", ",", true)
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
	data := LoadDataFromNetflix("../example/file_data/feedback_netflix.txt", ",", true)
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

func TestLoadEntityFromCSV(t *testing.T) {
	entities := LoadEntityFromCSV("../example/file_data/items.csv", "::", "|", false,
		[]string{"ItemId", "Title", "Genres"}, 0)
	// 1::Toy Story (1995)::Animation|Children's|Comedy
	// 2::Jumanji (1995)::Adventure|Children's|Fantasy
	// 3::Grumpier Old Men (1995)::Comedy|Romance
	// 4::Waiting to Exhale (1995)::Comedy|Drama
	// 5::Father of the Bride Part II (1995)::Comedy
	expected := []map[string]interface{}{
		{"ItemId": 1, "Title": "Toy Story (1995)", "Genres": []string{"Animation", "Children's", "Comedy"}},
		{"ItemId": 2, "Title": "Jumanji (1995)", "Genres": []string{"Adventure", "Children's", "Fantasy"}},
		{"ItemId": 3, "Title": "Grumpier Old Men (1995)", "Genres": []string{"Comedy", "Romance"}},
		{"ItemId": 4, "Title": "Waiting to Exhale (1995)", "Genres": []string{"Comedy", "Drama"}},
		{"ItemId": 5, "Title": "Father of the Bride Part II (1995)", "Genres": "Comedy"},
	}
	assert.Equal(t, expected, entities)
}

func TestLoadEntityFromCSV_Header(t *testing.T) {
	entities := LoadEntityFromCSV("../example/file_data/items_header.csv", "::", "|", true, nil, 0)
	// 1::Toy Story (1995)::Animation|Children's|Comedy
	// 2::Jumanji (1995)::Adventure|Children's|Fantasy
	// 3::Grumpier Old Men (1995)::Comedy|Romance
	// 4::Waiting to Exhale (1995)::Comedy|Drama
	// 5::Father of the Bride Part II (1995)::Comedy
	expected := []map[string]interface{}{
		{"ItemId": 1, "Title": "Toy Story (1995)", "Genres": []string{"Animation", "Children's", "Comedy"}},
		{"ItemId": 2, "Title": "Jumanji (1995)", "Genres": []string{"Adventure", "Children's", "Fantasy"}},
		{"ItemId": 3, "Title": "Grumpier Old Men (1995)", "Genres": []string{"Comedy", "Romance"}},
		{"ItemId": 4, "Title": "Waiting to Exhale (1995)", "Genres": []string{"Comedy", "Drama"}},
		{"ItemId": 5, "Title": "Father of the Bride Part II (1995)", "Genres": "Comedy"},
	}
	assert.Equal(t, expected, entities)
}
