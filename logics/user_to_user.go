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

package logics

import (
	"sort"
	"time"

	mapset "github.com/deckarep/golang-set/v2"
	"github.com/expr-lang/expr"
	"github.com/expr-lang/expr/vm"
	"github.com/gorse-io/gorse/base/log"
	"github.com/gorse-io/gorse/common/ann"
	"github.com/gorse-io/gorse/common/floats"
	"github.com/gorse-io/gorse/config"
	"github.com/gorse-io/gorse/dataset"
	"github.com/gorse-io/gorse/storage/cache"
	"github.com/gorse-io/gorse/storage/data"
	"github.com/juju/errors"
	"github.com/samber/lo"
	"go.uber.org/zap"
)

type UserToUserOptions struct {
	TagsIDF  []float32
	ItemsIDF []float32
}

type UserToUser interface {
	Users() []*data.User
	Push(user *data.User, feedback []int32)
	PopAll(i int) []cache.Score
	Timestamp() time.Time
}

func NewUserToUser(cfg config.UserToUserConfig, n int, timestamp time.Time, opts *UserToUserOptions) (UserToUser, error) {
	switch cfg.Type {
	case "embedding":
		return newEmbeddingUserToUser(cfg, n, timestamp)
	case "tags":
		if opts == nil || opts.TagsIDF == nil {
			return nil, errors.New("tags IDF is required for tags user-to-user")
		}
		return newTagsUserToUser(cfg, n, timestamp, opts.TagsIDF)
	case "items":
		if opts == nil || opts.ItemsIDF == nil {
			return nil, errors.New("items IDF is required for items user-to-user")
		}
		return newItemsUserToUser(cfg, n, timestamp, opts.ItemsIDF)
	case "auto":
		if opts == nil || opts.TagsIDF == nil || opts.ItemsIDF == nil {
			return nil, errors.New("tags IDF and items IDF are required for auto user-to-user")
		}
		return newAutoUserToUser(cfg, n, timestamp, opts.TagsIDF, opts.ItemsIDF)
	}
	return nil, errors.New("unknown user-to-user method")
}

type baseUserToUser[T any] struct {
	name       string
	n          int
	timestamp  time.Time
	columnFunc *vm.Program
	index      *ann.HNSW[T]
	users      []*data.User
}

func (b *baseUserToUser[T]) Users() []*data.User {
	return b.users
}

func (b *baseUserToUser[T]) Timestamp() time.Time {
	return b.timestamp
}

func (b *baseUserToUser[T]) PopAll(i int) []cache.Score {
	scores, err := b.index.SearchIndex(i, b.n+1, true)
	if err != nil {
		log.Logger().Error("failed to search index", zap.Error(err))
		return nil
	}
	return lo.Map(scores, func(v lo.Tuple2[int, float32], _ int) cache.Score {
		return cache.Score{
			Id:        b.users[v.A].UserId,
			Score:     -float64(v.B),
			Timestamp: b.timestamp,
		}
	})
}

type embeddingUserToUser struct {
	baseUserToUser[[]float32]
	dimension int
}

func newEmbeddingUserToUser(cfg config.UserToUserConfig, n int, timestamp time.Time) (UserToUser, error) {
	// Compile column expression
	columnFunc, err := expr.Compile(cfg.Column, expr.Env(map[string]any{
		"user": data.User{},
	}))
	if err != nil {
		return nil, err
	}
	return &embeddingUserToUser{baseUserToUser: baseUserToUser[[]float32]{
		name:       cfg.Name,
		n:          n,
		timestamp:  timestamp,
		columnFunc: columnFunc,
		index:      ann.NewHNSW[[]float32](floats.Euclidean),
		users:      []*data.User{},
	}}, nil
}

func (e *embeddingUserToUser) Push(user *data.User, _ []int32) {
	// Evaluate filter function
	result, err := expr.Run(e.columnFunc, map[string]any{
		"user": user,
	})
	if err != nil {
		log.Logger().Error("failed to evaluate column expression", zap.Error(err))
		return
	}
	// Check column type
	v, ok := result.([]float32)
	if !ok {
		log.Logger().Error("invalid column type", zap.Any("column", result))
		return
	}
	// Check dimension
	if e.dimension == 0 && len(v) > 0 {
		e.dimension = len(v)
	} else if e.dimension != len(v) {
		log.Logger().Error("invalid dimension", zap.Int("expected", e.dimension), zap.Int("actual", len(v)))
		return
	}
	// Push user
	e.users = append(e.users, user)
	_ = e.index.Add(v)
}

type tagsUserToUser struct {
	baseUserToUser[[]dataset.ID]
	IDF[dataset.ID]
}

func newTagsUserToUser(cfg config.UserToUserConfig, n int, timestamp time.Time, idf []float32) (UserToUser, error) {
	// Compile column expression
	columnFunc, err := expr.Compile(cfg.Column, expr.Env(map[string]any{
		"user": data.User{},
	}))
	if err != nil {
		return nil, err
	}
	t := &tagsUserToUser{IDF: idf}
	t.baseUserToUser = baseUserToUser[[]dataset.ID]{
		name:       cfg.Name,
		n:          n,
		timestamp:  timestamp,
		columnFunc: columnFunc,
		index:      ann.NewHNSW[[]dataset.ID](t.distance),
	}
	return t, nil
}

func (t *tagsUserToUser) Push(user *data.User, _ []int32) {
	// Evaluate filter function
	result, err := expr.Run(t.columnFunc, map[string]any{
		"user": user,
	})
	if err != nil {
		log.Logger().Error("failed to evaluate column expression", zap.Error(err))
		return
	}
	// Extract tags
	tSet := mapset.NewSet[dataset.ID]()
	flatten(result, tSet)
	v := tSet.ToSlice()
	sort.Slice(v, func(i, j int) bool {
		return v[i] < v[j]
	})
	// Push user
	t.users = append(t.users, user)
	_ = t.index.Add(v)
}

type itemsUserToUser struct {
	baseUserToUser[[]int32]
	IDF[int32]
}

func newItemsUserToUser(cfg config.UserToUserConfig, n int, timestamp time.Time, idf []float32) (UserToUser, error) {
	if cfg.Column != "" {
		return nil, errors.New("column is not supported in items user-to-user")
	}
	i := &itemsUserToUser{IDF: idf}
	i.baseUserToUser = baseUserToUser[[]int32]{
		name:      cfg.Name,
		n:         n,
		timestamp: timestamp,
		index:     ann.NewHNSW[[]int32](i.distance),
	}
	return i, nil
}

func (i *itemsUserToUser) Push(user *data.User, feedback []int32) {
	// Sort feedback
	sort.Slice(feedback, func(i, j int) bool {
		return feedback[i] < feedback[j]
	})
	// Push user
	i.users = append(i.users, user)
	_ = i.index.Add(feedback)
}

type autoUserToUser struct {
	baseUserToUser[lo.Tuple2[[]dataset.ID, []int32]]
	tIDF IDF[dataset.ID]
	iIDF IDF[int32]
}

func newAutoUserToUser(cfg config.UserToUserConfig, n int, timestamp time.Time, tIDF, iIDF []float32) (UserToUser, error) {
	a := &autoUserToUser{
		tIDF: tIDF,
		iIDF: iIDF,
	}
	a.baseUserToUser = baseUserToUser[lo.Tuple2[[]dataset.ID, []int32]]{
		name:      cfg.Name,
		n:         n,
		timestamp: timestamp,
		index:     ann.NewHNSW[lo.Tuple2[[]dataset.ID, []int32]](a.distance),
	}
	return a, nil
}

func (a *autoUserToUser) Push(user *data.User, feedback []int32) {
	// Extract tags
	tSet := mapset.NewSet[dataset.ID]()
	flatten(user.Labels, tSet)
	t := tSet.ToSlice()
	sort.Slice(t, func(i, j int) bool {
		return t[i] < t[j]
	})
	// Sort feedback
	sort.Slice(feedback, func(i, j int) bool {
		return feedback[i] < feedback[j]
	})
	// Push user
	a.users = append(a.users, user)
	_ = a.index.Add(lo.Tuple2[[]dataset.ID, []int32]{A: t, B: feedback})
}

func (a *autoUserToUser) distance(u, v lo.Tuple2[[]dataset.ID, []int32]) float32 {
	return (a.tIDF.distance(u.A, v.A) + a.iIDF.distance(u.B, v.B)) / 2
}
