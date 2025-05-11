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
	var hasValues, hasSets bool
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
			Keys: bson.D{
				{"collection", 1},
				{"subset", 1},
				{"id", 1},
			},
		},
		{
			Keys: bson.D{
				{"collection", 1},
				{"subset", 1},
				{"categories", 1},
				{"is_hidden", 1},
				{"score", -1},
			},
		},
	})
	if err != nil {
		return errors.Trace(err)
	}
	_, err = d.Collection(m.PointsTable()).Indexes().CreateMany(ctx, []mongo.IndexModel{
		{
			// update set ... where name = ? and timestammp = ?
			Keys: bson.D{
				{"name", 1},
				{"timestamp", 1},
			},
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
	return nil
}

func (m MongoDB) Purge() error {
	tables := []string{m.ValuesTable(), m.SetsTable(), m.DocumentTable()}
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

func (m MongoDB) AddScores(ctx context.Context, collection, subset string, documents []Score) error {
	if len(documents) == 0 {
		return nil
	}
	var models []mongo.WriteModel
	for _, document := range documents {
		models = append(models, mongo.NewUpdateOneModel().
			SetUpsert(true).
			SetFilter(bson.M{
				"collection": collection,
				"subset":     subset,
				"id":         document.Id,
			}).
			SetUpdate(bson.M{"$set": bson.M{
				"score":      document.Score,
				"is_hidden":  document.IsHidden,
				"categories": document.Categories,
				"timestamp":  document.Timestamp,
			}}))
	}
	_, err := m.client.Database(m.dbName).Collection(m.DocumentTable()).BulkWrite(ctx, models)
	return errors.Trace(err)
}

func (m MongoDB) SearchScores(ctx context.Context, collection, subset string, query []string, begin, end int) ([]Score, error) {
	opt := options.Find().SetSkip(int64(begin)).SetSort(bson.M{"score": -1})
	if end != -1 {
		opt.SetLimit(int64(end - begin))
	}
	filter := bson.M{
		"collection": collection,
		"subset":     subset,
		"is_hidden":  false,
	}
	if len(query) > 0 {
		filter["categories"] = bson.M{"$all": query}
	}
	cur, err := m.client.Database(m.dbName).Collection(m.DocumentTable()).Find(ctx, filter, opt)
	if err != nil {
		return nil, errors.Trace(err)
	}
	documents := make([]Score, 0)
	for cur.Next(ctx) {
		var document Score
		if err = cur.Decode(&document); err != nil {
			return nil, errors.Trace(err)
		}
		documents = append(documents, document)
	}
	return documents, nil
}

func (m MongoDB) UpdateScores(ctx context.Context, collections []string, subset *string, id string, patch ScorePatch) error {
	if len(collections) == 0 {
		return nil
	}
	if patch.IsHidden == nil && patch.Categories == nil && patch.Score == nil {
		return nil
	}
	filter := bson.M{
		"collection": bson.M{"$in": collections},
		"id":         id,
	}
	if subset != nil {
		filter["subset"] = *subset
	}
	update := bson.D{}
	if patch.IsHidden != nil {
		update = append(update, bson.E{Key: "$set", Value: bson.M{"is_hidden": *patch.IsHidden}})
	}
	if patch.Categories != nil {
		update = append(update, bson.E{Key: "$set", Value: bson.M{"categories": patch.Categories}})
	}
	if patch.Score != nil {
		update = append(update, bson.E{Key: "$set", Value: bson.M{"score": *patch.Score}})
	}
	_, err := m.client.Database(m.dbName).Collection(m.DocumentTable()).UpdateMany(ctx, filter, update)
	return errors.Trace(err)
}

func (m MongoDB) DeleteScores(ctx context.Context, collections []string, condition ScoreCondition) error {
	if err := condition.Check(); err != nil {
		return errors.Trace(err)
	}
	filter := bson.M{"collection": bson.M{"$in": collections}}
	if condition.Subset != nil {
		filter["subset"] = condition.Subset
	}
	if condition.Id != nil {
		filter["id"] = condition.Id
	}
	if condition.Before != nil {
		filter["timestamp"] = bson.M{"$lt": condition.Before}
	}
	_, err := m.client.Database(m.dbName).Collection(m.DocumentTable()).DeleteMany(ctx, filter)
	return errors.Trace(err)
}

func (m MongoDB) AddTimeSeriesPoints(ctx context.Context, points []TimeSeriesPoint) error {
	var models []mongo.WriteModel
	for _, point := range points {
		models = append(models, mongo.NewUpdateOneModel().
			SetUpsert(true).
			SetFilter(bson.M{
				"name":      point.Name,
				"timestamp": point.Timestamp,
			}).
			SetUpdate(bson.M{"$set": bson.M{
				"value": point.Value,
			}}))
	}
	_, err := m.client.Database(m.dbName).Collection(m.PointsTable()).BulkWrite(ctx, models)
	return errors.Trace(err)
}

func (m MongoDB) GetTimeSeriesPoints(ctx context.Context, name string, begin, end time.Time, duration time.Duration) ([]TimeSeriesPoint, error) {
	cursor, err := m.client.Database(m.dbName).Collection(m.PointsTable()).Aggregate(ctx, []bson.M{
		{"$match": bson.M{
			"name":      name,
			"timestamp": bson.M{"$gte": begin, "$lte": end},
		}},
		{"$sort": bson.M{"timestamp": -1}},
		{"$group": bson.M{
			"_id": bson.M{
				"$multiply": bson.A{
					bson.M{"$floor": bson.M{"$divide": bson.A{bson.M{"$toLong": "$timestamp"}, duration.Milliseconds()}}},
					duration.Milliseconds(),
				},
			},
			"name":  bson.M{"$first": "$name"},
			"value": bson.M{"$first": "$value"},
		}},
		{"$project": bson.M{
			"_id":       0,
			"timestamp": bson.M{"$toDate": "$_id"},
			"name":      1,
			"value":     1,
		}},
	})
	if err != nil {
		return nil, errors.Trace(err)
	}
	defer cursor.Close(ctx)
	var points []TimeSeriesPoint
	for cursor.Next(ctx) {
		var point TimeSeriesPoint
		if err = cursor.Decode(&point); err != nil {
			return nil, errors.Trace(err)
		}
		points = append(points, point)
	}
	if err = cursor.Err(); err != nil {
		return nil, errors.Trace(err)
	}
	return points, nil
}
