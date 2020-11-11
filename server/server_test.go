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
package server

import (
	"github.com/emicklei/go-restful"
	"github.com/steinfletcher/apitest"
	"github.com/thanhpk/randstr"
	"github.com/zhenghaoz/gorse/storage"
	"log"
	"net/http"
	"os"
	"path"
	"testing"
)

func setupServer() storage.Database {
	dir := path.Join(os.TempDir(), randstr.String(16))
	err := os.Mkdir(dir, 0777)
	if err != nil {
		log.Fatal(err)
	}
	db, err := storage.Open(dir)
	if err != nil {
		log.Fatal(err)
	}
	server := Server{DB: db}
	server.SetupWebService()
	return db
}

func TestServer_Users(t *testing.T) {
	// Setup
	db := setupServer()
	defer db.Close()
	// Run server using httptest
	apitest.New().
		Handler(restful.DefaultContainer).
		Post("/user").
		JSON(storage.User{UserId: "0"}).
		Expect(t).
		Status(http.StatusOK).
		Body(`{"RowAffected":1}`).
		End()
}

func TestServer_Items(t *testing.T) {

}

func TestServer_Feedback(t *testing.T) {

}
