// Copyright 2025 gorse Project Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package logics

import (
	"context"
	"slices"
	"sync"
	"time"

	mapset "github.com/deckarep/golang-set/v2"
	"github.com/expr-lang/expr"
	"github.com/expr-lang/expr/vm"
	"github.com/gorse-io/gorse/common/bfloats"
	"github.com/gorse-io/gorse/common/log"
	"github.com/gorse-io/gorse/config"
	"github.com/gorse-io/gorse/dataset"
	"github.com/gorse-io/gorse/storage/cache"
	"github.com/gorse-io/gorse/storage/data"
	"github.com/gorse-io/gorse/storage/vectors"
	"github.com/pkg/errors"
	"github.com/samber/lo"
	"go.uber.org/zap"
)

type UserToUserOptions struct {
	Context      context.Context
	VectorClient vectors.Database
	VectorConfig vectors.VectorConfig
	BatchSize    int
	TagsIDF      []float32
	ItemsIDF     []float32
}

type UserToUser interface {
	Users() []*data.User
	Push(user *data.User, feedback []int32)
	Finish() error
	PopAll(i int) []cache.Score
	Timestamp() time.Time
}

func NewUserToUser(cfg config.UserToUserConfig, n int, timestamp time.Time, opts *UserToUserOptions) (UserToUser, error) {
	if opts == nil || opts.VectorClient == nil {
		return nil, errors.New("vector database is required for user-to-user")
	}
	switch cfg.Type {
	case "embedding":
		return newEmbeddingUserToUser(cfg, n, timestamp, opts)
	case "tags":
		if opts.TagsIDF == nil {
			return nil, errors.New("tags IDF is required for tags user-to-user")
		}
		return newTagsUserToUser(cfg, n, timestamp, opts, opts.TagsIDF)
	case "items":
		if opts.ItemsIDF == nil {
			return nil, errors.New("items IDF is required for items user-to-user")
		}
		return newItemsUserToUser(cfg, n, timestamp, opts, opts.ItemsIDF)
	case "auto":
		if opts.TagsIDF == nil || opts.ItemsIDF == nil {
			return nil, errors.New("tags IDF and items IDF are required for auto user-to-user")
		}
		return newAutoUserToUser(cfg, n, timestamp, opts, opts.TagsIDF, opts.ItemsIDF)
	default:
		return nil, errors.New("unknown user-to-user method")
	}
}

type baseUserToUser struct {
	ctx          context.Context
	n            int
	timestamp    time.Time
	collection   string
	distance     vectors.Distance
	scoreScale   float64
	vectorClient vectors.Database
	writer       *similarityVectorWriter

	mu      sync.Mutex
	users   []*data.User
	queries []vectors.Vector
}

func newBaseUserToUser(cfg config.UserToUserConfig, n int, timestamp time.Time, opts *UserToUserOptions, sparse bool) *baseUserToUser {
	distance := vectors.Euclidean
	if sparse {
		distance = vectors.Dot
	}
	collection := vectors.UserToUserCollection(cfg.Name, cfg.Type)
	return &baseUserToUser{
		ctx:          opts.Context,
		n:            n,
		timestamp:    timestamp,
		collection:   collection,
		distance:     distance,
		scoreScale:   1,
		vectorClient: opts.VectorClient,
		writer: newSimilarityVectorWriter(opts.Context, opts.VectorClient, collection, distance,
			opts.VectorConfig, timestamp, opts.BatchSize, sparse),
	}
}

func (b *baseUserToUser) Users() []*data.User {
	b.mu.Lock()
	defer b.mu.Unlock()
	return slices.Clone(b.users)
}

func (b *baseUserToUser) Timestamp() time.Time {
	return b.timestamp
}

func (b *baseUserToUser) push(user *data.User, query vectors.Vector) {
	stored := query
	stored.Id = user.UserId
	written := b.writer.Push(stored)
	if !written && (!b.writer.sparse || len(query.Indices) > 0) {
		return
	}
	b.mu.Lock()
	b.users = append(b.users, user)
	b.queries = append(b.queries, query)
	b.mu.Unlock()
}

func (b *baseUserToUser) Finish() error {
	return b.writer.Finish()
}

func (b *baseUserToUser) PopAll(i int) []cache.Score {
	b.mu.Lock()
	user := b.users[i]
	query := b.queries[i]
	b.mu.Unlock()
	if len(query.Values) == 0 {
		return []cache.Score{}
	}
	neighbors, err := b.vectorClient.QueryVectors(b.ctx, b.collection, query, nil, b.n+1)
	if err != nil {
		log.Logger().Error("failed to query user-to-user vectors",
			zap.String("collection", b.collection), zap.String("user_id", user.UserId), zap.Error(err))
		return nil
	}
	scores := make([]cache.Score, 0, min(b.n, len(neighbors)))
	for _, neighbor := range neighbors {
		if neighbor.Id == user.UserId || b.distance == vectors.Dot && neighbor.Score <= 0 {
			continue
		}
		score := float64(neighbor.Score) * b.scoreScale
		if b.distance == vectors.Euclidean {
			score = 1 / (1 - float64(neighbor.Score))
		}
		scores = append(scores, cache.Score{Id: neighbor.Id, Score: score, Timestamp: b.timestamp})
		if len(scores) == b.n {
			break
		}
	}
	return scores
}

type embeddingUserToUser struct {
	*baseUserToUser
	columnFunc *vm.Program
}

func newEmbeddingUserToUser(cfg config.UserToUserConfig, n int, timestamp time.Time, opts *UserToUserOptions) (UserToUser, error) {
	columnFunc, err := expr.Compile(cfg.Column, expr.Env(map[string]any{"user": data.User{}}))
	if err != nil {
		return nil, err
	}
	return &embeddingUserToUser{baseUserToUser: newBaseUserToUser(cfg, n, timestamp, opts, false), columnFunc: columnFunc}, nil
}

func (e *embeddingUserToUser) Push(user *data.User, _ []int32) {
	result, err := expr.Run(e.columnFunc, map[string]any{"user": user})
	if err != nil {
		log.Logger().Error("failed to evaluate column expression", zap.Error(err))
		return
	}
	value, ok := bfloats.FromAny(result)
	if !ok || len(value) == 0 {
		log.Logger().Error("invalid embedding column type", zap.Any("column", result))
		return
	}
	e.push(user, vectors.Vector{Values: bfloats.ToFloat32(value)})
}

type tagsUserToUser struct {
	*baseUserToUser
	columnFunc *vm.Program
	idf        []float32
}

func newTagsUserToUser(cfg config.UserToUserConfig, n int, timestamp time.Time, opts *UserToUserOptions, idf []float32) (UserToUser, error) {
	columnFunc, err := expr.Compile(cfg.Column, expr.Env(map[string]any{"user": data.User{}}))
	if err != nil {
		return nil, err
	}
	return &tagsUserToUser{baseUserToUser: newBaseUserToUser(cfg, n, timestamp, opts, true), columnFunc: columnFunc, idf: idf}, nil
}

func (t *tagsUserToUser) Push(user *data.User, _ []int32) {
	result, err := expr.Run(t.columnFunc, map[string]any{"user": user})
	if err != nil {
		log.Logger().Error("failed to evaluate column expression", zap.Error(err))
		return
	}
	tags := mapset.NewSet[dataset.ID]()
	flatten(result, tags)
	ids := tags.ToSlice()
	slices.Sort(ids)
	t.push(user, newSparseVector(ids, t.idf, 0))
}

type itemsUserToUser struct {
	*baseUserToUser
	idf []float32
}

func newItemsUserToUser(cfg config.UserToUserConfig, n int, timestamp time.Time, opts *UserToUserOptions, idf []float32) (UserToUser, error) {
	if cfg.Column != "" {
		return nil, errors.New("column is not supported in items user-to-user")
	}
	return &itemsUserToUser{baseUserToUser: newBaseUserToUser(cfg, n, timestamp, opts, true), idf: idf}, nil
}

func (i *itemsUserToUser) Push(user *data.User, feedback []int32) {
	slices.Sort(feedback)
	i.push(user, newSparseVector(feedback, i.idf, 0))
}

type autoUserToUser struct {
	*baseUserToUser
	tagsIDF  []float32
	itemsIDF []float32
}

func newAutoUserToUser(cfg config.UserToUserConfig, n int, timestamp time.Time, opts *UserToUserOptions, tagsIDF, itemsIDF []float32) (UserToUser, error) {
	base := newBaseUserToUser(cfg, n, timestamp, opts, true)
	base.scoreScale = .5
	return &autoUserToUser{baseUserToUser: base, tagsIDF: tagsIDF, itemsIDF: itemsIDF}, nil
}

func (a *autoUserToUser) Push(user *data.User, feedback []int32) {
	tags := mapset.NewSet[dataset.ID]()
	flatten(user.Labels, tags)
	tagIDs := tags.ToSlice()
	slices.Sort(tagIDs)
	slices.Sort(feedback)
	vector := newSparseVector(tagIDs, a.tagsIDF, 0)
	vector = appendSparseVector(vector, newSparseVector(feedback, a.itemsIDF, uint32(len(a.tagsIDF))))
	a.push(user, vector)
}
