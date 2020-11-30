// Copyright 2020 gorse Project Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
package model

import (
	"crypto/md5"
	"fmt"
	"io"
	"log"
	"os"
	"testing"
)

func TestNewDataSet(t *testing.T) {
	//users := []storage.User{{UserId: "0"}, {UserId: "1"}, {UserId: "2"}}
	//items := []storage.Item{{ItemId: "0"}, {ItemId: "1"}, {ItemId: "2"}}
	//feedback := []storage.Feedback{
	//	{UserId: "0", ItemId: "0"},
	//	{UserId: "0", ItemId: "1"},
	//	{UserId: "0", ItemId: "2"},
	//	{UserId: "1", ItemId: "0"},
	//	{UserId: "1", ItemId: "1"},
	//	{UserId: "2", ItemId: "0"},
	//	{UserId: "3", ItemId: "0"},
	//	{UserId: "0", ItemId: "3"},
	//}
	//dataSet := NewMapIndexDataset(feedback, users, items)
	//assert.Equal(t, 6, dataSet.Count())
	//assert.Equal(t, 3, dataSet.UserCount())
	//assert.Equal(t, 3, dataSet.ItemCount())
	//userId, itemId := dataSet.GetID(3)
	//assert.Equal(t, "1", userId)
	//assert.Equal(t, "0", itemId)
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

func TestUnzip(t *testing.T) {
	// Download
	zipName, err := downloadFromUrl("https://cdn.sine-x.com/datasets/movielens/ml-100k.zip", os.TempDir())
	if err != nil {
		t.Fatal("download file failed ", err)
	}
	// Extract files
	fileNames, err := unzip(zipName, DataSetDir)
	// Check
	if err != nil {
		t.Fatal("unzip file failed ", err)
	}
	if len(fileNames) != 24 {
		t.Fatal("Number of file doesn't match")
	}
}

func TestLoadDataFromBuiltIn(t *testing.T) {
	//data := LoadDataFromBuiltIn("filmtrust")
	//assert.Equal(t, 35497, data.Count())
}

//func TestLoadDataFromCSV_Implicit(t *testing.T) {
//	data := LoadDataFromCSV("../example/data/feedback_implicit.csv", ",", false)
//	assert.Equal(t, 5, data.Count())
//	for i := 0; i < data.Count(); i++ {
//		userId, itemId, value := data.GetID(i)
//		userIndex, itemIndex, _ := data.GetIndex(i)
//		assert.Equal(t, strconv.Itoa(i), userId)
//		assert.Equal(t, strconv.Itoa(2*i), itemId)
//		assert.Equal(t, 0.0, value)
//		assert.Equal(t, i, userIndex)
//		assert.Equal(t, i, itemIndex)
//	}
//}
//
//func TestLoadDataFromCSV_Explicit(t *testing.T) {
//	data := LoadDataFromCSV("../example/data/feedback_explicit.csv", ",", false)
//	assert.Equal(t, 5, data.Count())
//	for i := 0; i < data.Count(); i++ {
//		userId, itemId, value := data.GetID(i)
//		userIndex, itemIndex, _ := data.GetIndex(i)
//		assert.Equal(t, strconv.Itoa(i), userId)
//		assert.Equal(t, strconv.Itoa(2*i), itemId)
//		assert.Equal(t, 3*i, int(value))
//		assert.Equal(t, i, userIndex)
//		assert.Equal(t, i, itemIndex)
//	}
//}
//
//func TestLoadDataFromCSV_Explicit_Header(t *testing.T) {
//	data := LoadDataFromCSV("../example/data/feedback_explicit_header.csv", ",", true)
//	assert.Equal(t, 5, data.Count())
//	for i := 0; i < data.Count(); i++ {
//		userId, itemId, value := data.GetID(i)
//		userIndex, itemIndex, _ := data.GetIndex(i)
//		assert.Equal(t, strconv.Itoa(i), userId)
//		assert.Equal(t, strconv.Itoa(2*i), itemId)
//		assert.Equal(t, 3*i, int(value))
//		assert.Equal(t, i, userIndex)
//		assert.Equal(t, i, itemIndex)
//	}
//}
//
//func TestLoadEntityFromCSV(t *testing.T) {
//	entities := LoadEntityFromCSV("../example/data/ItemFeedback.csv", "::", "|", false,
//		[]string{"ItemId", "Title", "Genres"}, 0)
//	// 1::Toy Story (1995)::Animation|Children's|Comedy
//	// 2::Jumanji (1995)::Adventure|Children's|Fantasy
//	// 3::Grumpier Old Men (1995)::Comedy|Romance
//	// 4::Waiting to Exhale (1995)::Comedy|Drama
//	// 5::Father of the Bride Part II (1995)::Comedy
//	expected := []map[string]interface{}{
//		{"ItemId": "1", "Title": "Toy Story (1995)", "Genres": []string{"Animation", "Children's", "Comedy"}},
//		{"ItemId": "2", "Title": "Jumanji (1995)", "Genres": []string{"Adventure", "Children's", "Fantasy"}},
//		{"ItemId": "3", "Title": "Grumpier Old Men (1995)", "Genres": []string{"Comedy", "Romance"}},
//		{"ItemId": "4", "Title": "Waiting to Exhale (1995)", "Genres": []string{"Comedy", "Drama"}},
//		{"ItemId": "5", "Title": "Father of the Bride Part II (1995)", "Genres": "Comedy"},
//	}
//	assert.Equal(t, expected, entities)
//}
//
//func TestLoadEntityFromCSV_Header(t *testing.T) {
//	entities := LoadEntityFromCSV("../example/data/items_header.csv", "::", "|", true, nil, 0)
//	// 1::Toy Story (1995)::Animation|Children's|Comedy
//	// 2::Jumanji (1995)::Adventure|Children's|Fantasy
//	// 3::Grumpier Old Men (1995)::Comedy|Romance
//	// 4::Waiting to Exhale (1995)::Comedy|Drama
//	// 5::Father of the Bride Part II (1995)::Comedy
//	expected := []map[string]interface{}{
//		{"ItemId": "1", "Title": "Toy Story (1995)", "Genres": []string{"Animation", "Children's", "Comedy"}},
//		{"ItemId": "2", "Title": "Jumanji (1995)", "Genres": []string{"Adventure", "Children's", "Fantasy"}},
//		{"ItemId": "3", "Title": "Grumpier Old Men (1995)", "Genres": []string{"Comedy", "Romance"}},
//		{"ItemId": "4", "Title": "Waiting to Exhale (1995)", "Genres": []string{"Comedy", "Drama"}},
//		{"ItemId": "5", "Title": "Father of the Bride Part II (1995)", "Genres": "Comedy"},
//	}
//	assert.Equal(t, expected, entities)
//}
