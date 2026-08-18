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
	Push(user *data.User, feedback []int32)
	Finish() error
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

type baseUserToUser struct {
	writer *similarityVectorWriter
}

func newBaseUserToUser(cfg config.UserToUserConfig, timestamp time.Time, opts *UserToUserOptions, sparse bool) *baseUserToUser {
	distance := vectors.Euclidean
	if sparse {
		distance = vectors.Dot
	}
	collection := vectors.UserToUserCollection(cfg.Name)
	return &baseUserToUser{
		writer: newSimilarityVectorWriter(opts.Context, opts.VectorClient, collection, distance,
			opts.VectorConfig, timestamp, opts.BatchSize, sparse),
	}
}

func (b *baseUserToUser) push(user *data.User, query vectors.Vector) {
	query.Id = user.UserId
	b.writer.Push(query)
}

func (b *baseUserToUser) Finish() error {
	return b.writer.Finish()
}

type embeddingUserToUser struct {
	*baseUserToUser
	columnFunc *vm.Program
}

func newEmbeddingUserToUser(cfg config.UserToUserConfig, timestamp time.Time, opts *UserToUserOptions) (UserToUser, error) {
	columnFunc, err := expr.Compile(cfg.Column, expr.Env(map[string]any{"user": data.User{}}))
	if err != nil {
		return nil, err
	}
	return &embeddingUserToUser{baseUserToUser: newBaseUserToUser(cfg, timestamp, opts, false), columnFunc: columnFunc}, nil
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

func newTagsUserToUser(cfg config.UserToUserConfig, timestamp time.Time, opts *UserToUserOptions, idf []float32) (UserToUser, error) {
	columnFunc, err := expr.Compile(cfg.Column, expr.Env(map[string]any{"user": data.User{}}))
	if err != nil {
		return nil, err
	}
	return &tagsUserToUser{baseUserToUser: newBaseUserToUser(cfg, timestamp, opts, true), columnFunc: columnFunc, idf: idf}, nil
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

func newItemsUserToUser(cfg config.UserToUserConfig, timestamp time.Time, opts *UserToUserOptions, idf []float32) (UserToUser, error) {
	if cfg.Column != "" {
		return nil, errors.New("column is not supported in items user-to-user")
	}
	return &itemsUserToUser{baseUserToUser: newBaseUserToUser(cfg, timestamp, opts, true), idf: idf}, nil
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

func newAutoUserToUser(cfg config.UserToUserConfig, timestamp time.Time, opts *UserToUserOptions, tagsIDF, itemsIDF []float32) (UserToUser, error) {
	return &autoUserToUser{
		baseUserToUser: newBaseUserToUser(cfg, timestamp, opts, true),
		tagsIDF:        tagsIDF,
		itemsIDF:       itemsIDF,
	}, nil
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
