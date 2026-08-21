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
	Add(user *data.User, feedback []int32) error
	Clean() error
}

func QueryUserToUser(ctx context.Context, vectorClient vectors.Database, userToUserConfig config.UserToUserConfig, userId string, n int) ([]cache.Score, error) {
	collection := vectors.UserToUserCollection(userToUserConfig.Name)
	queries, err := vectorClient.GetVectors(ctx, collection, []string{userId})
	if err != nil {
		return nil, errors.WithStack(err)
	}
	if len(queries) == 0 {
		return nil, nil
	}
	neighbors, err := vectorClient.QueryVectors(ctx, collection, queries[0], nil, n+1)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	distance := vectors.Dot
	if userToUserConfig.Type == "embedding" {
		distance = vectors.Euclidean
	}
	scoreScale := 1.0
	if userToUserConfig.Type == "auto" {
		scoreScale = .5
	}
	scores := make([]cache.Score, 0, min(n, len(neighbors)))
	for _, neighbor := range neighbors {
		if neighbor.Id == userId || distance == vectors.Dot && neighbor.Score <= 0 {
			continue
		}
		score := float64(neighbor.Score) * scoreScale
		if distance == vectors.Euclidean {
			score = 1 / (1 - score)
		}
		scores = append(scores, cache.Score{Id: neighbor.Id, Score: score})
		if len(scores) == n {
			break
		}
	}
	return scores, nil
}

func NewUserToUser(cfg config.UserToUserConfig, timestamp time.Time, opts *UserToUserOptions) (UserToUser, error) {
	if opts == nil || opts.VectorClient == nil {
		return nil, errors.New("vector database is required for user-to-user")
	}
	switch cfg.Type {
	case "embedding":
		return newEmbeddingUserToUser(cfg, timestamp, opts)
	case "tags":
		if opts.TagsIDF == nil {
			return nil, errors.New("tags IDF is required for tags user-to-user")
		}
		return newTagsUserToUser(cfg, timestamp, opts, opts.TagsIDF)
	case "items":
		if opts.ItemsIDF == nil {
			return nil, errors.New("items IDF is required for items user-to-user")
		}
		return newItemsUserToUser(cfg, timestamp, opts, opts.ItemsIDF)
	case "auto":
		if opts.TagsIDF == nil || opts.ItemsIDF == nil {
			return nil, errors.New("tags IDF and items IDF are required for auto user-to-user")
		}
		return newAutoUserToUser(cfg, timestamp, opts, opts.TagsIDF, opts.ItemsIDF)
	default:
		return nil, errors.New("unknown user-to-user method")
	}
}

func newUserToUserVectorWriter(cfg config.UserToUserConfig, timestamp time.Time, opts *UserToUserOptions, sparse bool) *VectorWriter {
	distance := vectors.Euclidean
	if sparse {
		distance = vectors.Dot
	}
	collection := vectors.UserToUserCollection(cfg.Name)
	return newSimilarityVectorWriter(opts.Context, opts.VectorClient, collection, distance,
		opts.VectorConfig, timestamp, opts.BatchSize, sparse)
}

type embeddingUserToUser struct {
	*VectorWriter
	columnFunc *vm.Program
}

func newEmbeddingUserToUser(cfg config.UserToUserConfig, timestamp time.Time, opts *UserToUserOptions) (UserToUser, error) {
	columnFunc, err := expr.Compile(cfg.Column, expr.Env(map[string]any{"user": data.User{}}))
	if err != nil {
		return nil, err
	}
	return &embeddingUserToUser{
		VectorWriter: newUserToUserVectorWriter(cfg, timestamp, opts, false),
		columnFunc:   columnFunc,
	}, nil
}

func (e *embeddingUserToUser) Add(user *data.User, _ []int32) error {
	result, err := expr.Run(e.columnFunc, map[string]any{"user": user})
	if err != nil {
		log.Logger().Error("failed to evaluate column expression", zap.Error(err))
		return nil
	}
	value, ok := bfloats.FromAny(result)
	if !ok || len(value) == 0 {
		log.Logger().Error("invalid embedding column type", zap.Any("column", result))
		return nil
	}
	return e.VectorWriter.Add(vectors.Vector{Id: user.UserId, Values: bfloats.ToFloat32(value), Timestamp: e.timestamp})
}

type tagsUserToUser struct {
	*VectorWriter
	columnFunc *vm.Program
	idf        []float32
}

func newTagsUserToUser(cfg config.UserToUserConfig, timestamp time.Time, opts *UserToUserOptions, idf []float32) (UserToUser, error) {
	columnFunc, err := expr.Compile(cfg.Column, expr.Env(map[string]any{"user": data.User{}}))
	if err != nil {
		return nil, err
	}
	return &tagsUserToUser{
		VectorWriter: newUserToUserVectorWriter(cfg, timestamp, opts, true),
		columnFunc:   columnFunc,
		idf:          idf,
	}, nil
}

func (t *tagsUserToUser) Add(user *data.User, _ []int32) error {
	result, err := expr.Run(t.columnFunc, map[string]any{"user": user})
	if err != nil {
		log.Logger().Error("failed to evaluate column expression", zap.Error(err))
		return nil
	}
	tags := mapset.NewSet[dataset.ID]()
	flatten(result, tags)
	ids := tags.ToSlice()
	slices.Sort(ids)
	vector := newSparseVector(ids, t.idf, 0)
	vector.Id = user.UserId
	vector.Timestamp = t.timestamp
	return t.VectorWriter.Add(vector)
}

type itemsUserToUser struct {
	*VectorWriter
	idf []float32
}

func newItemsUserToUser(cfg config.UserToUserConfig, timestamp time.Time, opts *UserToUserOptions, idf []float32) (UserToUser, error) {
	if cfg.Column != "" {
		return nil, errors.New("column is not supported in items user-to-user")
	}
	return &itemsUserToUser{
		VectorWriter: newUserToUserVectorWriter(cfg, timestamp, opts, true),
		idf:          idf,
	}, nil
}

func (i *itemsUserToUser) Add(user *data.User, feedback []int32) error {
	slices.Sort(feedback)
	vector := newSparseVector(feedback, i.idf, 0)
	vector.Id = user.UserId
	vector.Timestamp = i.timestamp
	return i.VectorWriter.Add(vector)
}

type autoUserToUser struct {
	*VectorWriter
	tagsIDF  []float32
	itemsIDF []float32
}

func newAutoUserToUser(cfg config.UserToUserConfig, timestamp time.Time, opts *UserToUserOptions, tagsIDF, itemsIDF []float32) (UserToUser, error) {
	return &autoUserToUser{
		VectorWriter: newUserToUserVectorWriter(cfg, timestamp, opts, true),
		tagsIDF:      tagsIDF,
		itemsIDF:     itemsIDF,
	}, nil
}

func (a *autoUserToUser) Add(user *data.User, feedback []int32) error {
	tags := mapset.NewSet[dataset.ID]()
	flatten(user.Labels, tags)
	tagIDs := tags.ToSlice()
	slices.Sort(tagIDs)
	slices.Sort(feedback)
	vector := newSparseVector(tagIDs, a.tagsIDF, 0)
	vector = appendSparseVector(vector, feedback, a.itemsIDF, uint32(len(a.tagsIDF)))
	vector.Id = user.UserId
	vector.Timestamp = a.timestamp
	return a.VectorWriter.Add(vector)
}
