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
	"io"
	"time"

	"github.com/juju/errors"
	"github.com/zhenghaoz/gorse/storage"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type MongoDB struct {
	storage.TablePrefix
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
		case m.ValuesTable():
			hasValues = true
		case m.SetsTable():
			hasSets = true
		case m.SortedSetsTable():
			hasSortedSets = true
		}
	}
	// create collections
	if !hasValues {
		if err = d.CreateCollection(ctx, m.ValuesTable()); err != nil {
			return errors.Trace(err)
		}
	}
	if !hasSets {
		if err = d.CreateCollection(ctx, m.SetsTable()); err != nil {
			return errors.Trace(err)
		}
	}
	if !hasSortedSets {
		if err = d.CreateCollection(ctx, m.SortedSetsTable()); err != nil {
			return errors.Trace(err)
		}
	}
	// create index
	_, err = d.Collection(m.SetsTable()).Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys: bson.D{
			{"name", 1},
			{"member", 1},
		},
		Options: options.Index().SetUnique(true),
	})
	if err != nil {
		return errors.Trace(err)
	}
	_, err = d.Collection(m.SortedSetsTable()).Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys: bson.D{
			{"name", 1},
			{"member", 1},
		},
		Options: options.Index().SetUnique(true),
	})
	if err != nil {
		return errors.Trace(err)
	}
	_, err = d.Collection(m.SortedSetsTable()).Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys: bson.D{
			{"name", 1},
			{"score", 1},
		},
	})
	if err != nil {
		return errors.Trace(err)
	}
	_, err = d.Collection(m.MessageTable()).Indexes().CreateMany(ctx, []mongo.IndexModel{
		{
			// update set ... where name = ? and value = ?
			Keys: bson.D{
				{"name", 1},
				{"value", 1},
			},
		},
		{
			// select * from messages where name = ? order by timestamp asc limit 1
			Keys: bson.D{
				{"name", 1},
				{"timestamp", 1},
			},
		},
	})
	if err != nil {
		return errors.Trace(err)
	}
	_, err = d.Collection(m.DocumentTable()).Indexes().CreateMany(ctx, []mongo.IndexModel{
		{
			// update set ... where name = ? and value = ?
			Keys: bson.D{
				{"name", 1},
				{"value", 1},
			},
		},
		{
			// select * from documents where name = ? and ? <@ categories order by score asc
			Keys: bson.D{
				{"name", 1},
				{"categories", 1},
				{"score", -1},
			},
		},
	})
	return errors.Trace(err)
}

func (m MongoDB) Close() error {
	return m.client.Disconnect(context.Background())
}

func (m MongoDB) Ping() error {
	return m.client.Ping(context.Background(), nil)
}

func (m MongoDB) Scan(work func(string) error) error {
	ctx := context.Background()

	// scan values
	valuesCollection := m.client.Database(m.dbName).Collection(m.ValuesTable())
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
	setCollection := m.client.Database(m.dbName).Collection(m.SetsTable())
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
	sortedSetCollection := m.client.Database(m.dbName).Collection(m.SortedSetsTable())
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

func (m MongoDB) Purge() error {
	tables := []string{m.ValuesTable(), m.SortedSetsTable(), m.SetsTable()}
	for _, tableName := range tables {
		c := m.client.Database(m.dbName).Collection(tableName)
		_, err := c.DeleteMany(context.Background(), bson.D{})
		if err != nil {
			return errors.Trace(err)
		}
	}
	return nil
}

func (m MongoDB) Set(ctx context.Context, values ...Value) error {
	if len(values) == 0 {
		return nil
	}
	c := m.client.Database(m.dbName).Collection(m.ValuesTable())
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

func (m MongoDB) Get(ctx context.Context, name string) *ReturnValue {
	c := m.client.Database(m.dbName).Collection(m.ValuesTable())
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

func (m MongoDB) Delete(ctx context.Context, name string) error {
	c := m.client.Database(m.dbName).Collection(m.ValuesTable())
	_, err := c.DeleteOne(ctx, bson.M{"_id": bson.M{"$eq": name}})
	return errors.Trace(err)
}

func (m MongoDB) GetSet(ctx context.Context, name string) ([]string, error) {
	c := m.client.Database(m.dbName).Collection(m.SetsTable())
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

func (m MongoDB) SetSet(ctx context.Context, name string, members ...string) error {
	c := m.client.Database(m.dbName).Collection(m.SetsTable())
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

func (m MongoDB) AddSet(ctx context.Context, name string, members ...string) error {
	if len(members) == 0 {
		return nil
	}
	c := m.client.Database(m.dbName).Collection(m.SetsTable())
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

func (m MongoDB) RemSet(ctx context.Context, name string, members ...string) error {
	if len(members) == 0 {
		return nil
	}
	c := m.client.Database(m.dbName).Collection(m.SetsTable())
	var models []mongo.WriteModel
	for _, member := range members {
		models = append(models, mongo.NewDeleteOneModel().
			SetFilter(bson.M{"name": bson.M{"$eq": name}, "member": bson.M{"$eq": member}}))
	}
	_, err := c.BulkWrite(ctx, models)
	return errors.Trace(err)
}

func (m MongoDB) GetSorted(ctx context.Context, name string, begin, end int) ([]Scored, error) {
	c := m.client.Database(m.dbName).Collection(m.SortedSetsTable())
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

func (m MongoDB) GetSortedByScore(ctx context.Context, name string, begin, end float64) ([]Scored, error) {
	c := m.client.Database(m.dbName).Collection(m.SortedSetsTable())
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

func (m MongoDB) RemSortedByScore(ctx context.Context, name string, begin, end float64) error {
	c := m.client.Database(m.dbName).Collection(m.SortedSetsTable())
	_, err := c.DeleteMany(ctx, bson.D{
		{"name", name},
		{"score", bson.M{"$gte": begin}},
		{"score", bson.M{"$lte": end}},
	})
	return errors.Trace(err)
}

func (m MongoDB) AddSorted(ctx context.Context, sortedSets ...SortedSet) error {
	c := m.client.Database(m.dbName).Collection(m.SortedSetsTable())
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

func (m MongoDB) SetSorted(ctx context.Context, name string, scores []Scored) error {
	c := m.client.Database(m.dbName).Collection(m.SortedSetsTable())
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

func (m MongoDB) RemSorted(ctx context.Context, members ...SetMember) error {
	if len(members) == 0 {
		return nil
	}
	c := m.client.Database(m.dbName).Collection(m.SortedSetsTable())
	var models []mongo.WriteModel
	for _, member := range members {
		models = append(models, mongo.NewDeleteOneModel().SetFilter(bson.M{"name": member.name, "member": member.member}))
	}
	_, err := c.BulkWrite(ctx, models)
	return errors.Trace(err)
}

func (m MongoDB) Push(ctx context.Context, name, value string) error {
	_, err := m.client.Database(m.dbName).Collection(m.MessageTable()).UpdateOne(ctx,
		bson.M{"name": name, "value": value},
		bson.M{"$set": bson.M{"name": name, "value": value, "timestamp": time.Now().UnixNano()}},
		options.Update().SetUpsert(true))
	return err
}

func (m MongoDB) Pop(ctx context.Context, name string) (string, error) {
	result := m.client.Database(m.dbName).Collection(m.MessageTable()).FindOneAndDelete(ctx,
		bson.M{"name": name}, options.FindOneAndDelete().SetSort(bson.M{"timestamp": 1}))
	if err := result.Err(); err == mongo.ErrNoDocuments {
		return "", io.EOF
	} else if err != nil {
		return "", errors.Trace(err)
	}
	var b bson.M
	if err := result.Decode(&b); err != nil {
		return "", errors.Trace(err)
	}
	return b["value"].(string), nil
}

func (m MongoDB) Remain(ctx context.Context, name string) (int64, error) {
	return m.client.Database(m.dbName).Collection(m.MessageTable()).CountDocuments(ctx, bson.M{
		"name": name,
	})
}

func (m MongoDB) AddDocuments(ctx context.Context, name string, documents ...Document) error {
	var models []mongo.WriteModel
	for _, document := range documents {
		models = append(models, mongo.NewUpdateOneModel().
			SetUpsert(true).
			SetFilter(bson.M{
				"name":  name,
				"value": document.Value,
			}).
			SetUpdate(bson.M{"$set": bson.M{
				"score":      document.Score,
				"categories": document.Categories,
			}}))
	}
	_, err := m.client.Database(m.dbName).Collection(m.DocumentTable()).BulkWrite(ctx, models)
	return errors.Trace(err)
}

func (m MongoDB) SearchDocuments(ctx context.Context, name string, query []string, begin, end int) ([]Document, error) {
	opt := options.Find().SetSkip(int64(begin)).SetSort(bson.M{"score": -1})
	if end != -1 {
		opt.SetLimit(int64(end - begin))
	}
	cur, err := m.client.Database(m.dbName).Collection(m.DocumentTable()).Find(ctx, bson.M{
		"name":       name,
		"categories": bson.M{"$all": query},
	}, opt)
	if err != nil {
		return nil, errors.Trace(err)
	}
	documents := make([]Document, 0)
	for cur.Next(ctx) {
		var document Document
		if err = cur.Decode(&document); err != nil {
			return nil, errors.Trace(err)
		}
		documents = append(documents, document)
	}
	return documents, nil
}
