// Copyright 2022 gorse Project Authors
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
	"github.com/araddon/dateparse"
	"github.com/juju/errors"
	"github.com/samber/lo"
	"github.com/scylladb/go-set/strset"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"strconv"
	"time"
)

type MongoDB struct {
	client *mongo.Client
	dbName string
}

func (m MongoDB) Init() error {
	ctx := context.Background()
	d := m.client.Database(m.dbName)
	// list collections
	var hasValues, hasSets, hasSortedSets bool
	collections, err := d.ListCollectionNames(ctx, bson.M{})
	if err != nil {
		return errors.Trace(err)
	}
	for _, collectionName := range collections {
		switch collectionName {
		case "values":
			hasValues = true
		case "sets":
			hasSets = true
		case "sorted_sets":
			hasSortedSets = true
		}
	}
	// create collections
	if !hasValues {
		if err = d.CreateCollection(ctx, "values"); err != nil {
			return errors.Trace(err)
		}
	}
	if !hasSets {
		if err = d.CreateCollection(ctx, "sets"); err != nil {
			return errors.Trace(err)
		}
	}
	if !hasSortedSets {
		if err = d.CreateCollection(ctx, "sorted_sets"); err != nil {
			return errors.Trace(err)
		}
	}
	// create index
	_, err = d.Collection("sets").Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys: bson.D{
			{"name", 1},
			{"member", 1},
		},
		Options: options.Index().SetUnique(true),
	})
	if err != nil {
		return errors.Trace(err)
	}
	_, err = d.Collection("sorted_sets").Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys: bson.D{
			{"name", 1},
			{"member", 1},
		},
		Options: options.Index().SetUnique(true),
	})
	if err != nil {
		return errors.Trace(err)
	}
	_, err = d.Collection("sorted_sets").Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys: bson.D{
			{"name", 1},
			{"score", 1},
		},
	})
	if err != nil {
		return errors.Trace(err)
	}
	return nil
}

func (m MongoDB) Close() error {
	return m.client.Disconnect(context.Background())
}

func (m MongoDB) Set(values ...Value) error {
	ctx := context.Background()
	c := m.client.Database(m.dbName).Collection("values")
	var models []mongo.WriteModel
	for _, value := range values {
		models = append(models, mongo.NewUpdateOneModel().
			SetUpsert(true).
			SetFilter(bson.M{"_id": bson.M{"$eq": value.name}}).
			SetUpdate(bson.M{"$set": bson.M{"_id": value.name, "value": value.value}}))
	}
	_, err := c.BulkWrite(ctx, models)
	return errors.Trace(err)
}

func (m MongoDB) GetString(name string) (string, error) {
	ctx := context.Background()
	c := m.client.Database(m.dbName).Collection("values")
	r := c.FindOne(ctx, bson.M{"_id": bson.M{"$eq": name}})
	if err := r.Err(); err == mongo.ErrNoDocuments {
		return "", errors.Annotate(ErrObjectNotExist, name)
	} else if err != nil {
		return "", errors.Trace(err)
	}
	if raw, err := r.DecodeBytes(); err != nil {
		return "", errors.Trace(err)
	} else {
		return raw.Lookup("value").StringValue(), nil
	}
}

func (m MongoDB) GetInt(name string) (int, error) {
	val, err := m.GetString(name)
	if err != nil {
		return 0, nil
	}
	buf, err := strconv.Atoi(val)
	if err != nil {
		return 0, errors.Trace(err)
	}
	return buf, err
}

func (m MongoDB) GetTime(name string) (time.Time, error) {
	val, err := m.GetString(name)
	if err != nil {
		return time.Time{}, nil
	}
	tm, err := dateparse.ParseAny(val)
	if err != nil {
		return time.Time{}, nil
	}
	return tm, nil
}

func (m MongoDB) Delete(name string) error {
	ctx := context.Background()
	c := m.client.Database(m.dbName).Collection("values")
	_, err := c.DeleteOne(ctx, bson.M{"_id": bson.M{"$eq": name}})
	return errors.Trace(err)
}

func (m MongoDB) Exists(names ...string) ([]int, error) {
	ctx := context.Background()
	c := m.client.Database(m.dbName).Collection("values")
	opt := options.Find()
	opt.SetProjection(bson.M{"_id": 1})
	r, err := c.Find(ctx, bson.M{"_id": bson.M{"$in": names}}, opt)
	if err != nil {
		return nil, err
	}
	existedNames := strset.New()
	for r.Next(ctx) {
		var row bson.Raw
		if err = r.Decode(&row); err != nil {
			return nil, err
		}
		existedNames.Add(row.Lookup("_id").StringValue())
	}
	result := make([]int, len(names))
	for i, name := range names {
		if existedNames.Has(name) {
			result[i] = 1
		}
	}
	return result, nil
}

func (m MongoDB) GetSet(name string) ([]string, error) {
	ctx := context.Background()
	c := m.client.Database(m.dbName).Collection("sets")
	r, err := c.Find(ctx, bson.M{"name": name})
	if err != nil {
		return nil, errors.Trace(err)
	}
	var members []string
	for r.Next(ctx) {
		var doc bson.Raw
		if err = r.Decode(&doc); err != nil {
			return nil, err
		}
		members = append(members, doc.Lookup("member").StringValue())
	}
	return members, nil
}

func (m MongoDB) SetSet(name string, members ...string) error {
	ctx := context.Background()
	c := m.client.Database(m.dbName).Collection("sets")
	var models []mongo.WriteModel
	models = append(models, mongo.NewDeleteManyModel().SetFilter(bson.M{"name": bson.M{"$eq": name}}))
	for _, member := range members {
		models = append(models, mongo.NewUpdateOneModel().
			SetUpsert(true).
			SetFilter(bson.M{"name": bson.M{"$eq": name}, "member": bson.M{"$eq": member}}).
			SetUpdate(bson.M{"$set": bson.M{"name": name, "member": member}}))
	}
	_, err := c.BulkWrite(ctx, models)
	return errors.Trace(err)
}

func (m MongoDB) AddSet(name string, members ...string) error {
	if len(members) == 0 {
		return nil
	}
	ctx := context.Background()
	c := m.client.Database(m.dbName).Collection("sets")
	var models []mongo.WriteModel
	for _, member := range members {
		models = append(models, mongo.NewUpdateOneModel().
			SetUpsert(true).
			SetFilter(bson.M{"name": bson.M{"$eq": name}, "member": bson.M{"$eq": member}}).
			SetUpdate(bson.M{"$set": bson.M{"name": name, "member": member}}))
	}
	_, err := c.BulkWrite(ctx, models)
	return errors.Trace(err)
}

func (m MongoDB) RemSet(name string, members ...string) error {
	ctx := context.Background()
	c := m.client.Database(m.dbName).Collection("sets")
	var models []mongo.WriteModel
	for _, member := range members {
		models = append(models, mongo.NewDeleteOneModel().
			SetFilter(bson.M{"name": bson.M{"$eq": name}, "member": bson.M{"$eq": member}}))
	}
	_, err := c.BulkWrite(ctx, models)
	return errors.Trace(err)
}

func (m MongoDB) GetSortedScores(members ...SetMember) ([]float64, error) {
	ctx := context.Background()
	c := m.client.Database(m.dbName).Collection("sorted_sets")
	conditions := lo.Map(members, func(member SetMember, _ int) bson.D {
		return bson.D{
			{"name", member.name},
			{"member", member.member},
		}
	})
	r, err := c.Find(ctx, bson.D{{"$or", conditions}})
	if err != nil {
		return nil, errors.Trace(err)
	}
	resultsMap := make(map[SetMember]float64)
	for r.Next(ctx) {
		var doc bson.Raw
		if err = r.Decode(&doc); err != nil {
			return nil, err
		}
		member := SetMember{
			name:   doc.Lookup("name").StringValue(),
			member: doc.Lookup("member").StringValue(),
		}
		resultsMap[member] = doc.Lookup("score").Double()
	}
	return lo.Map(members, func(member SetMember, _ int) float64 {
		return resultsMap[member]
	}), nil
}

func (m MongoDB) GetSorted(name string, begin, end int) ([]Scored, error) {
	ctx := context.Background()
	c := m.client.Database(m.dbName).Collection("sorted_sets")
	opt := options.Find()
	opt.SetSort(bson.M{"score": -1})
	if end >= 0 {
		opt.SetLimit(int64(end + 1))
	}
	r, err := c.Find(ctx, bson.M{"name": name}, opt)
	if err != nil {
		return nil, errors.Trace(err)
	}
	var scores []Scored
	for r.Next(ctx) {
		var doc bson.Raw
		if err = r.Decode(&doc); err != nil {
			return nil, errors.Trace(err)
		}
		scores = append(scores, Scored{
			Id:    doc.Lookup("member").StringValue(),
			Score: doc.Lookup("score").Double(),
		})
	}
	if len(scores) >= begin {
		scores = scores[begin:]
	}
	return scores, nil
}

func (m MongoDB) GetSortedByScore(name string, begin, end float64) ([]Scored, error) {
	ctx := context.Background()
	c := m.client.Database(m.dbName).Collection("sorted_sets")
	opt := options.Find()
	opt.SetSort(bson.M{"score": 1})
	r, err := c.Find(ctx, bson.D{
		{"name", name},
		{"score", bson.M{"$gte": begin}},
		{"score", bson.M{"$lte": end}},
	}, opt)
	if err != nil {
		return nil, errors.Trace(err)
	}
	var scores []Scored
	for r.Next(ctx) {
		var doc bson.Raw
		if err = r.Decode(&doc); err != nil {
			return nil, err
		}
		scores = append(scores, Scored{
			Id:    doc.Lookup("member").StringValue(),
			Score: doc.Lookup("score").Double(),
		})
	}
	return scores, nil
}

func (m MongoDB) RemSortedByScore(name string, begin, end float64) error {
	ctx := context.Background()
	c := m.client.Database(m.dbName).Collection("sorted_sets")
	_, err := c.DeleteMany(ctx, bson.D{
		{"name", name},
		{"score", bson.M{"$gte": begin}},
		{"score", bson.M{"$lte": end}},
	})
	return errors.Trace(err)
}

func (m MongoDB) AddSorted(sortedSets ...SortedSet) error {
	ctx := context.Background()
	c := m.client.Database(m.dbName).Collection("sorted_sets")
	var models []mongo.WriteModel
	for _, sorted := range sortedSets {
		for _, score := range sorted.scores {
			models = append(models, mongo.NewUpdateOneModel().
				SetUpsert(true).
				SetFilter(bson.M{"name": bson.M{"$eq": sorted.name}, "member": bson.M{"$eq": score.Id}}).
				SetUpdate(bson.M{"$set": bson.M{"name": sorted.name, "member": score.Id, "score": score.Score}}))
		}
	}
	if len(models) == 0 {
		return nil
	}
	_, err := c.BulkWrite(ctx, models)
	return errors.Trace(err)
}

func (m MongoDB) SetSorted(name string, scores []Scored) error {
	ctx := context.Background()
	c := m.client.Database(m.dbName).Collection("sorted_sets")
	var models []mongo.WriteModel
	models = append(models, mongo.NewDeleteManyModel().SetFilter(bson.M{"name": bson.M{"$eq": name}}))
	for _, score := range scores {
		models = append(models, mongo.NewUpdateOneModel().
			SetUpsert(true).
			SetFilter(bson.M{"name": bson.M{"$eq": name}, "member": bson.M{"$eq": score.Id}}).
			SetUpdate(bson.M{"$set": bson.M{"name": name, "member": score.Id, "score": score.Score}}))
	}
	_, err := c.BulkWrite(ctx, models)
	return errors.Trace(err)
}

func (m MongoDB) RemSorted(name, member string) error {
	ctx := context.Background()
	c := m.client.Database(m.dbName).Collection("sorted_sets")
	_, err := c.DeleteOne(ctx, bson.M{"name": name, "member": member})
	return errors.Trace(err)
}
