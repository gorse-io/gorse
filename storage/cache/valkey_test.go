// Copyright 2025 gorse Project Authors
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

package cache

import (
	"context"
	"fmt"
	"math"
	"os"
	"testing"
	"time"

	"github.com/gorse-io/gorse/common/log"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

var (
	valkeyDSN string
)

func init() {
	env := func(key, defaultValue string) string {
		if value := os.Getenv(key); value != "" {
			return value
		}
		return defaultValue
	}
	valkeyDSN = env("VALKEY_URI", "valkey://127.0.0.1:6380/")
}

type ValkeyTestSuite struct {
	baseTestSuite
}

func (suite *ValkeyTestSuite) SetupSuite() {
	var err error
	suite.Database, err = Open(valkeyDSN, "gorse_")
	suite.NoError(err)
	// flush db
	valkeyClient, ok := suite.Database.(*Valkey)
	suite.True(ok)
	if !valkeyClient.isCluster {
		_, err = valkeyClient.standaloneClient.CustomCommand(suite.T().Context(), []string{"FLUSHDB"})
		suite.NoError(err)
	}
	// create schema
	err = suite.Database.Init()
	suite.NoError(err)
}

func (suite *ValkeyTestSuite) TestEscapeCharacters() {
	ts := time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC)
	ctx := suite.T().Context()
	for _, c := range []string{"-", ":", ".", "/"} {
		suite.Run(c, func() {
			collection := fmt.Sprintf("a%s1", c)
			subset := fmt.Sprintf("b%s2", c)
			id := fmt.Sprintf("c%s3", c)
			err := suite.AddScores(ctx, collection, subset, []Score{{
				Id:         id,
				Score:      math.MaxFloat64,
				Categories: []string{"a", "b"},
				Timestamp:  ts,
			}})
			suite.NoError(err)
			documents, err := suite.SearchScores(ctx, collection, subset, []string{"b"}, 0, -1)
			suite.NoError(err)
			suite.Equal([]Score{{Id: id, Score: math.MaxFloat64, Categories: []string{"a", "b"}, Timestamp: ts}}, documents)

			err = suite.UpdateScores(ctx, []string{collection}, nil, id, ScorePatch{Score: new(float64(1))})
			suite.NoError(err)
			documents, err = suite.SearchScores(ctx, collection, subset, []string{"b"}, 0, -1)
			suite.NoError(err)
			suite.Equal([]Score{{Id: id, Score: 1, Categories: []string{"a", "b"}, Timestamp: ts}}, documents)

			err = suite.DeleteScores(ctx, []string{collection}, ScoreCondition{
				Subset: new(subset),
				Id:     new(id),
			})
			suite.NoError(err)
			documents, err = suite.SearchScores(ctx, collection, subset, []string{"b"}, 0, -1)
			suite.NoError(err)
			suite.Empty(documents)
		})
	}
}

func (suite *ValkeyTestSuite) TestUpdateScoresWithPagination() {
	ctx := suite.T().Context()
	db, ok := suite.Database.(*Valkey)
	suite.True(ok)
	limit := db.maxSearchResults
	db.maxSearchResults = 2
	defer func() {
		db.maxSearchResults = limit
	}()

	for i := range 5 {
		subset := fmt.Sprintf("subset-%d", i)
		err := suite.AddScores(ctx, "collection-a", subset, []Score{{
			Id:         "shared-item",
			Score:      float64(i),
			Categories: []string{"old"},
			Timestamp:  time.Now().UTC(),
		}})
		suite.NoError(err)
	}

	err := suite.UpdateScores(ctx, []string{"collection-a"}, nil, "shared-item", ScorePatch{
		Categories: []string{"new"},
	})
	suite.NoError(err)

	for i := range 5 {
		subset := fmt.Sprintf("subset-%d", i)
		docs, err := suite.SearchScores(ctx, "collection-a", subset, []string{"new"}, 0, -1)
		suite.NoError(err)
		suite.Require().Len(docs, 1)
		suite.Equal("shared-item", docs[0].Id)
	}
}

func (suite *ValkeyTestSuite) TestUpdateScoresWithPaginationAndScorePatch() {
	ctx := suite.T().Context()
	db, ok := suite.Database.(*Valkey)
	suite.True(ok)
	limit := db.maxSearchResults
	db.maxSearchResults = 1
	defer func() {
		db.maxSearchResults = limit
	}()

	initialScores := []float64{3, 2, 1}
	for i, score := range initialScores {
		subset := fmt.Sprintf("score-subset-%d", i)
		err := suite.AddScores(ctx, "collection-b", subset, []Score{{
			Id:         "shared-item",
			Score:      score,
			Categories: []string{"score-old"},
			Timestamp:  time.Now().UTC(),
		}})
		suite.NoError(err)
	}

	targetScore := float64(0)
	err := suite.UpdateScores(ctx, []string{"collection-b"}, nil, "shared-item", ScorePatch{
		Score: &targetScore,
	})
	suite.NoError(err)

	for i := range initialScores {
		subset := fmt.Sprintf("score-subset-%d", i)
		docs, err := suite.SearchScores(ctx, "collection-b", subset, nil, 0, -1)
		suite.NoError(err)
		suite.Require().Len(docs, 1)
		suite.Equal(targetScore, docs[0].Score)
	}
}

func (suite *ValkeyTestSuite) TestUpdateScoresWithPaginationAndTiedScores() {
	ctx := suite.T().Context()
	db, ok := suite.Database.(*Valkey)
	suite.True(ok)
	limit := db.maxSearchResults
	db.maxSearchResults = 2
	defer func() {
		db.maxSearchResults = limit
	}()

	for i := range 5 {
		subset := fmt.Sprintf("tie-subset-%d", i)
		err := suite.AddScores(ctx, "collection-c", subset, []Score{{
			Id:         "shared-item",
			Score:      1,
			Categories: []string{"tie-old"},
			Timestamp:  time.Now().UTC(),
		}})
		suite.NoError(err)
	}

	err := suite.UpdateScores(ctx, []string{"collection-c"}, nil, "shared-item", ScorePatch{
		Categories: []string{"tie-new"},
	})
	suite.NoError(err)

	for i := range 5 {
		subset := fmt.Sprintf("tie-subset-%d", i)
		docs, err := suite.SearchScores(ctx, "collection-c", subset, []string{"tie-new"}, 0, -1)
		suite.NoError(err)
		suite.Require().Len(docs, 1)
		suite.Equal("shared-item", docs[0].Id)
	}
}

func TestValkey(t *testing.T) {
	suite.Run(t, new(ValkeyTestSuite))
}

func TestParseValkeyURL(t *testing.T) {
	// Basic URL
	addr, username, password, db, useTLS, err := parseValkeyURL("valkey://127.0.0.1:6380/")
	assert.NoError(t, err)
	assert.Equal(t, "127.0.0.1", addr.Host)
	assert.Equal(t, 6380, addr.Port)
	assert.Equal(t, "", username)
	assert.Equal(t, "", password)
	assert.Equal(t, 0, db)
	assert.False(t, useTLS)

	// URL with password only
	addr, username, password, db, useTLS, err = parseValkeyURL("valkey://:secret@localhost:6379/2")
	assert.NoError(t, err)
	assert.Equal(t, "localhost", addr.Host)
	assert.Equal(t, 6379, addr.Port)
	assert.Equal(t, "", username)
	assert.Equal(t, "secret", password)
	assert.Equal(t, 2, db)
	assert.False(t, useTLS)

	// URL with username and password
	addr, username, password, db, useTLS, err = parseValkeyURL("valkey://myuser:mypass@host.example.com:6380/3")
	assert.NoError(t, err)
	assert.Equal(t, "host.example.com", addr.Host)
	assert.Equal(t, 6380, addr.Port)
	assert.Equal(t, "myuser", username)
	assert.Equal(t, "mypass", password)
	assert.Equal(t, 3, db)
	assert.False(t, useTLS)

	// TLS URL
	addr, _, _, _, useTLS, err = parseValkeyURL("valkeys://localhost:6379/0")
	assert.NoError(t, err)
	assert.True(t, useTLS)

	// Default port
	addr, _, _, _, _, err = parseValkeyURL("valkey://localhost/")
	assert.NoError(t, err)
	assert.Equal(t, 6379, addr.Port)
}

func TestParseValkeyClusterURL(t *testing.T) {
	// Basic cluster URL
	addresses, username, password, useTLS, err := parseValkeyClusterURL("valkey+cluster://:password@192.168.1.11:6379?addr=192.168.0.5:6379&addr=192.168.0.7:6379")
	assert.NoError(t, err)
	assert.Len(t, addresses, 3)
	assert.Equal(t, "192.168.1.11", addresses[0].Host)
	assert.Equal(t, 6379, addresses[0].Port)
	assert.Equal(t, "192.168.0.5", addresses[1].Host)
	assert.Equal(t, "192.168.0.7", addresses[2].Host)
	assert.Equal(t, "", username)
	assert.Equal(t, "password", password)
	assert.False(t, useTLS)

	// Cluster URL with username
	addresses, username, password, useTLS, err = parseValkeyClusterURL("valkeys+cluster://admin:secret@node1:6380?addr=node2:6380")
	assert.NoError(t, err)
	assert.Len(t, addresses, 2)
	assert.Equal(t, "node1", addresses[0].Host)
	assert.Equal(t, 6380, addresses[0].Port)
	assert.Equal(t, "node2", addresses[1].Host)
	assert.Equal(t, "admin", username)
	assert.Equal(t, "secret", password)
	assert.True(t, useTLS)
}

func BenchmarkValkey(b *testing.B) {
	log.CloseLogger()
	database, err := Open(valkeyDSN, "gorse_")
	assert.NoError(b, err)
	// flush db
	valkeyClient := database.(*Valkey)
	if !valkeyClient.isCluster {
		_, err = valkeyClient.standaloneClient.CustomCommand(context.Background(), []string{"FLUSHDB"})
		assert.NoError(b, err)
	}
	// create schema
	err = database.Init()
	assert.NoError(b, err)
	// benchmark
	benchmark(b, database)
}
