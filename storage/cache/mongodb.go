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
	"github.com/juju/errors"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
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

func (m MongoDB) Scan(work func(string) error) error {
	ctx := context.Background()

	// scan values
	valuesCollection := m.client.Database(m.dbName).Collection("values")
	valuesIterator, err := valuesCollection.Find(ctx, bson.M{})
	if err != nil {
		return errors.Trace(err)
	}
	defer valuesIterator.Close(ctx)
	for valuesIterator.Next(ctx) {
		var row bson.Raw
		if err = valuesIterator.Decode(&row); err != nil {
			return errors.Trace(err)
		}
		if err = work(row.Lookup("_id").StringValue()); err != nil {
			return errors.Trace(err)
		}
	}

	// scan sets
	setCollection := m.client.Database(m.dbName).Collection("sets")
	setIterator, err := setCollection.Find(ctx, bson.M{})
	if err != nil {
		return errors.Trace(err)
	}
	defer setIterator.Close(ctx)
	prevKey := ""
	for setIterator.Next(ctx) {
		var row bson.Raw
		if err = setIterator.Decode(&row); err != nil {
			return errors.Trace(err)
		}
		key := row.Lookup("name").StringValue()
		if key != prevKey {
			if err = work(key); err != nil {
				return errors.Trace(err)
			}
			prevKey = key
		}
	}

	// scan sorted sets
	sortedSetCollection := m.client.Database(m.dbName).Collection("sorted_sets")
	sortedSetIterator, err := sortedSetCollection.Find(ctx, bson.M{})
	if err != nil {
		return errors.Trace(err)
	}
	defer sortedSetIterator.Close(ctx)
	prevKey = ""
	for sortedSetIterator.Next(ctx) {
		var row bson.Raw
		if err = sortedSetIterator.Decode(&row); err != nil {
			return errors.Trace(err)
		}
		key := row.Lookup("name").StringValue()
		if key != prevKey {
			if err = work(key); err != nil {
				return errors.Trace(err)
			}
			prevKey = key
		}
	}
	return nil
}

func (m MongoDB) Set(values ...Value) error {
	if len(values) == 0 {
		return nil
	}
	ctx := context.Background()
	c := m.client.Database(m.dbName).Collection("values")
	var models []mongo.WriteModel
	for _, value := range values {
		models = append(models, mongo.NewUpdateOneModel().
			SetUpsert(true).
			SetFilter(bson.M{"_id": value.name}).
			SetUpdate(bson.M{"$set": bson.M{"_id": value.name, "value": value.value}}))
	}
	_, err := c.BulkWrite(ctx, models)
	return errors.Trace(err)
}

func (m MongoDB) Get(name string) *ReturnValue {
	ctx := context.Background()
	c := m.client.Database(m.dbName).Collection("values")
	r := c.FindOne(ctx, bson.M{"_id": bson.M{"$eq": name}})
	if err := r.Err(); err == mongo.ErrNoDocuments {
		return &ReturnValue{err: errors.Annotate(ErrObjectNotExist, name)}
	} else if err != nil {
		return &ReturnValue{err: errors.Trace(err)}
	}
	if raw, err := r.DecodeBytes(); err != nil {
		return &ReturnValue{err: errors.Trace(err)}
	} else {
		return &ReturnValue{value: raw.Lookup("value").StringValue()}
	}
}

func (m MongoDB) Delete(name string) error {
	ctx := context.Background()
	c := m.client.Database(m.dbName).Collection("values")
	_, err := c.DeleteOne(ctx, bson.M{"_id": bson.M{"$eq": name}})
	return errors.Trace(err)
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
	if len(members) == 0 {
		return nil
	}
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
				SetFilter(bson.M{"name": sorted.name, "member": score.Id}).
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

func (m MongoDB) RemSorted(members ...SetMember) error {
	if len(members) == 0 {
		return nil
	}
	ctx := context.Background()
	c := m.client.Database(m.dbName).Collection("sorted_sets")
	var models []mongo.WriteModel
	for _, member := range members {
		models = append(models, mongo.NewDeleteOneModel().SetFilter(bson.M{"name": member.name, "member": member.member}))
	}
	_, err := c.BulkWrite(ctx, models)
	return errors.Trace(err)
}
