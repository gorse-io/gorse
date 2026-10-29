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
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/gorse-io/gorse/storage"
	"github.com/pkg/errors"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/x/mongo/driver/connstring"
	"go.opentelemetry.io/contrib/instrumentation/go.mongodb.org/mongo-driver/mongo/otelmongo"
)

func init() {
	Register([]string{storage.MongoPrefix, storage.MongoSrvPrefix}, func(path, tablePrefix string, opts ...storage.Option) (Database, error) {
		// connect to database
		database := new(MongoDB)
		clientOpts := options.Client()
		clientOpts.Monitor = otelmongo.NewMonitor()
		clientOpts.ApplyURI(path)
		var err error
		if database.client, err = mongo.Connect(context.Background(), clientOpts); err != nil {
			return nil, errors.WithStack(err)
		}
		// parse DSN and extract database name
		if cs, err := connstring.ParseAndValidate(path); err != nil {
			return nil, errors.WithStack(err)
		} else {
			database.dbName = cs.Database
			database.TablePrefix = storage.TablePrefix(tablePrefix)
		}
		return database, nil
	})
}

type MongoDB struct {
	storage.TablePrefix
	client *mongo.Client
	dbName string
}

func (m MongoDB) Init() error {
	ctx := context.Background()
	if err := m.initGeoIndexes(ctx); err != nil {
		return err
	}
	d := m.client.Database(m.dbName)
	// list collections
	var hasValues bool
	collections, err := d.ListCollectionNames(ctx, bson.M{})
	if err != nil {
		return errors.WithStack(err)
	}
	for _, collectionName := range collections {
		switch collectionName {
		case m.ValuesTable():
			hasValues = true
		}
	}
	// create collections
	if !hasValues {
		if err = d.CreateCollection(ctx, m.ValuesTable()); err != nil {
			return errors.WithStack(err)
		}
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
		return errors.WithStack(err)
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
		return errors.WithStack(err)
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
		return errors.WithStack(err)
	}
	defer valuesIterator.Close(ctx)
	for valuesIterator.Next(ctx) {
		var row bson.Raw
		if err = valuesIterator.Decode(&row); err != nil {
			return errors.WithStack(err)
		}
		if err = work(row.Lookup("_id").StringValue()); err != nil {
			return errors.WithStack(err)
		}
	}

	return nil
}

func (m MongoDB) Purge() error {
	if err := m.purgeGeoIndexes(context.Background()); err != nil {
		return err
	}
	tables := []string{m.ValuesTable(), m.DocumentTable()}
	for _, tableName := range tables {
		c := m.client.Database(m.dbName).Collection(tableName)
		_, err := c.DeleteMany(context.Background(), bson.D{})
		if err != nil {
			return errors.WithStack(err)
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
	return errors.WithStack(err)
}

func (m MongoDB) Get(ctx context.Context, name string) *ReturnValue {
	c := m.client.Database(m.dbName).Collection(m.ValuesTable())
	r := c.FindOne(ctx, bson.M{"_id": bson.M{"$eq": name}})
	if err := r.Err(); err == mongo.ErrNoDocuments {
		return &ReturnValue{value: "", exists: false}
	} else if err != nil {
		return &ReturnValue{err: errors.WithStack(err), exists: false}
	}
	if raw, err := r.DecodeBytes(); err != nil {
		return &ReturnValue{err: errors.WithStack(err), exists: false}
	} else {
		return &ReturnValue{value: raw.Lookup("value").StringValue(), exists: true}
	}
}

func (m MongoDB) Delete(ctx context.Context, name string) error {
	c := m.client.Database(m.dbName).Collection(m.ValuesTable())
	_, err := c.DeleteOne(ctx, bson.M{"_id": bson.M{"$eq": name}})
	return errors.WithStack(err)
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
	return errors.WithStack(err)
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
		return nil, errors.WithStack(err)
	}
	documents := make([]Score, 0)
	for cur.Next(ctx) {
		var document Score
		if err = cur.Decode(&document); err != nil {
			return nil, errors.WithStack(err)
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
	return errors.WithStack(err)
}

func (m MongoDB) DeleteScores(ctx context.Context, collections []string, condition ScoreCondition) error {
	if err := condition.Check(); err != nil {
		return errors.WithStack(err)
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
	return errors.WithStack(err)
}

func (m MongoDB) ScanScores(ctx context.Context, callback func(collection string, id string, subset string, timestamp time.Time) error) error {
	cursor, err := m.client.Database(m.dbName).Collection(m.DocumentTable()).Find(ctx, bson.M{})
	if err != nil {
		return errors.WithStack(err)
	}
	defer cursor.Close(ctx)
	for cursor.Next(ctx) {
		// check context cancellation
		select {
		case <-ctx.Done():
			return errors.WithStack(ctx.Err())
		default:
		}
		// decode document
		collection := cursor.Current.Lookup("collection").StringValue()
		subset := cursor.Current.Lookup("subset").StringValue()
		id := cursor.Current.Lookup("id").StringValue()
		timestamp := cursor.Current.Lookup("timestamp").Time()
		if err = callback(collection, id, subset, timestamp); err != nil {
			return errors.WithStack(err)
		}
	}
	return nil
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
	return errors.WithStack(err)
}

func (m MongoDB) GetTimeSeriesPoints(ctx context.Context, name string, begin, end time.Time, duration time.Duration) ([]TimeSeriesPoint, error) {
	cursor, err := m.client.Database(m.dbName).Collection(m.PointsTable()).Aggregate(ctx, []bson.M{
		{"$match": bson.M{
			"name":      name,
			"timestamp": bson.M{"$gte": begin, "$lte": end},
		}},
		{"$group": bson.M{
			"_id": bson.M{
				"$multiply": bson.A{
					bson.M{"$floor": bson.M{"$divide": bson.A{bson.M{"$toLong": "$timestamp"}, duration.Milliseconds()}}},
					duration.Milliseconds(),
				},
			},
			"name":  bson.M{"$top": bson.M{"sortBy": bson.M{"timestamp": -1}, "output": "$name"}},
			"value": bson.M{"$top": bson.M{"sortBy": bson.M{"timestamp": -1}, "output": "$value"}},
		}},
		{"$project": bson.M{
			"_id":       0,
			"timestamp": bson.M{"$toDate": "$_id"},
			"name":      1,
			"value":     1,
		}},
		{"$sort": bson.M{"timestamp": 1}},
	})
	if err != nil {
		return nil, errors.WithStack(err)
	}
	defer cursor.Close(ctx)
	points := []TimeSeriesPoint{}
	for cursor.Next(ctx) {
		var point TimeSeriesPoint
		if err = cursor.Decode(&point); err != nil {
			return nil, errors.WithStack(err)
		}
		points = append(points, point)
	}
	if err = cursor.Err(); err != nil {
		return nil, errors.WithStack(err)
	}
	return points, nil
}

const mongoGeoVersion = 1

// Serialize lifecycle changes against point operations across MongoDB handles in
// this process, so a delayed upsert cannot recreate a dropped collection. Callers
// must coordinate lifecycle changes across processes; concurrent cross-process
// lifecycle operations are deliberately outside this design's guarantees.
var mongoGeoMu sync.RWMutex

// The catalog owns the physical collections, including collections whose creation
// or deletion was interrupted. Names and IDs always use binary comparison.
type mongoGeoIndex struct {
	Name         string `bson:"name"`
	PhysicalName string `bson:"physical_name"`
	Version      int    `bson:"version"`
	State        string `bson:"state"`
}

type mongoGeoPoint struct {
	ID        string  `bson:"id"`
	Longitude float64 `bson:"longitude"`
	Latitude  float64 `bson:"latitude"`
	Cell      int64   `bson:"cell"`
}

func (m MongoDB) geoCatalog() *mongo.Collection {
	return m.client.Database(m.dbName).Collection(geoCatalogName(string(m.TablePrefix)))
}

func mongoGeoSimpleCollation() *options.Collation {
	return &options.Collation{Locale: "simple"}
}

func (m MongoDB) createGeoCatalog(ctx context.Context) error {
	db := m.client.Database(m.dbName)
	err := db.CreateCollection(ctx, geoCatalogName(string(m.TablePrefix)),
		options.CreateCollection().SetCollation(mongoGeoSimpleCollation()))
	var commandError mongo.CommandError
	if err != nil && (!errors.As(err, &commandError) || !commandError.HasErrorCode(48)) {
		return errors.WithStack(err)
	}
	_, err = m.geoCatalog().Indexes().CreateMany(ctx, []mongo.IndexModel{
		{Keys: bson.D{{Key: "name", Value: 1}}, Options: options.Index().SetUnique(true).SetCollation(mongoGeoSimpleCollation())},
		{Keys: bson.D{{Key: "physical_name", Value: 1}}, Options: options.Index().SetUnique(true).SetCollation(mongoGeoSimpleCollation())},
	})
	return errors.WithStack(err)
}

func (m MongoDB) validateGeoIndex(index mongoGeoIndex) error {
	if index.Version != mongoGeoVersion || index.PhysicalName != geoPhysicalName(string(m.TablePrefix), index.Name) {
		return fmt.Errorf("invalid MongoDB geo catalog entry %q", index.Name)
	}
	return validateGeoName(index.Name)
}

func (m MongoDB) geoIndexFilter(index mongoGeoIndex) bson.M {
	return bson.M{"name": index.Name, "physical_name": index.PhysicalName, "version": index.Version, "state": index.State}
}

func (m MongoDB) finishGeoIndexCreation(ctx context.Context, index mongoGeoIndex) error {
	if err := m.validateGeoIndex(index); err != nil {
		return err
	}
	db := m.client.Database(m.dbName)
	err := db.CreateCollection(ctx, index.PhysicalName,
		options.CreateCollection().SetCollation(mongoGeoSimpleCollation()))
	var commandError mongo.CommandError
	if err != nil && (!errors.As(err, &commandError) || !commandError.HasErrorCode(48)) {
		return errors.WithStack(err)
	}
	_, err = db.Collection(index.PhysicalName).Indexes().CreateMany(ctx, []mongo.IndexModel{
		{Keys: bson.D{{Key: "id", Value: 1}}, Options: options.Index().SetUnique(true).SetCollation(mongoGeoSimpleCollation())},
		{Keys: bson.D{{Key: "cell", Value: 1}, {Key: "id", Value: 1}}, Options: options.Index().SetCollation(mongoGeoSimpleCollation())},
	})
	if err != nil {
		return errors.WithStack(err)
	}
	result, err := m.geoCatalog().UpdateOne(ctx, m.geoIndexFilter(index), bson.M{"$set": bson.M{"state": "ready"}},
		options.Update().SetCollation(mongoGeoSimpleCollation()))
	if err != nil {
		return errors.WithStack(err)
	}
	if result.MatchedCount == 0 {
		return fmt.Errorf("geo index %q %w", index.Name, storage.ErrNotFound)
	}
	return nil
}

func (m MongoDB) finishGeoIndexDeletion(ctx context.Context, index mongoGeoIndex) error {
	if err := m.validateGeoIndex(index); err != nil {
		return err
	}
	if err := m.client.Database(m.dbName).Collection(index.PhysicalName).Drop(ctx); err != nil {
		return errors.WithStack(err)
	}
	_, err := m.geoCatalog().DeleteOne(ctx, m.geoIndexFilter(index),
		options.Delete().SetCollation(mongoGeoSimpleCollation()))
	return errors.WithStack(err)
}

func (m MongoDB) initGeoIndexes(ctx context.Context) error {
	mongoGeoMu.Lock()
	defer mongoGeoMu.Unlock()
	if err := m.createGeoCatalog(ctx); err != nil {
		return err
	}
	cursor, err := m.geoCatalog().Find(ctx, bson.M{}, options.Find().SetBatchSize(512))
	if err != nil {
		return errors.WithStack(err)
	}
	defer cursor.Close(ctx)
	for cursor.Next(ctx) {
		var index mongoGeoIndex
		if err = cursor.Decode(&index); err != nil {
			return errors.WithStack(err)
		}
		if err = m.validateGeoIndex(index); err != nil {
			return err
		}
		switch index.State {
		case "creating":
			err = m.finishGeoIndexCreation(ctx, index)
		case "dropping":
			err = m.finishGeoIndexDeletion(ctx, index)
		case "ready":
		default:
			return fmt.Errorf("invalid MongoDB geo index state %q", index.State)
		}
		if err != nil {
			return err
		}
	}
	return errors.WithStack(cursor.Err())
}

func (m MongoDB) ListGeoIndex(ctx context.Context) ([]string, error) {
	mongoGeoMu.RLock()
	defer mongoGeoMu.RUnlock()
	cursor, err := m.geoCatalog().Find(ctx, bson.M{"state": "ready"},
		options.Find().SetBatchSize(512).SetCollation(mongoGeoSimpleCollation()))
	if err != nil {
		return nil, errors.WithStack(err)
	}
	defer cursor.Close(ctx)
	names := make([]string, 0)
	for cursor.Next(ctx) {
		var index mongoGeoIndex
		if err = cursor.Decode(&index); err != nil {
			return nil, errors.WithStack(err)
		}
		if err = m.validateGeoIndex(index); err != nil {
			return nil, err
		}
		names = append(names, index.Name)
	}
	if err = cursor.Err(); err != nil {
		return nil, errors.WithStack(err)
	}
	sort.Strings(names)
	return names, nil
}

func (m MongoDB) AddGeoIndex(ctx context.Context, name string) error {
	if err := validateGeoName(name); err != nil {
		return err
	}
	mongoGeoMu.Lock()
	defer mongoGeoMu.Unlock()
	if err := m.createGeoCatalog(ctx); err != nil {
		return err
	}
	var existing mongoGeoIndex
	err := m.geoCatalog().FindOne(ctx, bson.M{"name": name},
		options.FindOne().SetCollation(mongoGeoSimpleCollation())).Decode(&existing)
	if err == nil {
		if err = m.validateGeoIndex(existing); err != nil {
			return err
		}
		switch existing.State {
		case "ready":
			return fmt.Errorf("geo index %q %w", name, storage.ErrAlreadyExists)
		case "creating":
			// The catalog owns this collection, so an interrupted create is
			// safe to finish without adopting an uncataloged collection.
			return m.finishGeoIndexCreation(ctx, existing)
		case "dropping":
			if err = m.deleteGeoIndex(ctx, name); err != nil {
				return err
			}
		default:
			return fmt.Errorf("invalid MongoDB geo index state %q", existing.State)
		}
	} else if !errors.Is(err, mongo.ErrNoDocuments) {
		return errors.WithStack(err)
	}
	index := mongoGeoIndex{Name: name, PhysicalName: geoPhysicalName(string(m.TablePrefix), name), Version: mongoGeoVersion, State: "creating"}
	if _, err := m.geoCatalog().InsertOne(ctx, index); err != nil {
		return errors.WithStack(err)
	}
	return m.finishGeoIndexCreation(ctx, index)
}

func (m MongoDB) readyGeoIndex(ctx context.Context, name string) (*mongo.Collection, error) {
	if err := validateGeoName(name); err != nil {
		return nil, err
	}
	var index mongoGeoIndex
	err := m.geoCatalog().FindOne(ctx, bson.M{"name": name, "state": "ready"},
		options.FindOne().SetCollation(mongoGeoSimpleCollation())).Decode(&index)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return nil, fmt.Errorf("geo index %q %w", name, storage.ErrNotFound)
	}
	if err != nil {
		return nil, errors.WithStack(err)
	}
	if err = m.validateGeoIndex(index); err != nil {
		return nil, err
	}
	return m.client.Database(m.dbName).Collection(index.PhysicalName), nil
}

func (m MongoDB) AddGeoPoints(ctx context.Context, name string, points []GeoPoint) error {
	records, err := prepareGeoPoints(name, points)
	if err != nil {
		return err
	}
	mongoGeoMu.RLock()
	defer mongoGeoMu.RUnlock()
	collection, err := m.readyGeoIndex(ctx, name)
	if err != nil {
		return err
	}
	for _, record := range records {
		if err = ctx.Err(); err != nil {
			return errors.WithStack(err)
		}
		filter := bson.M{"id": record.Id}
		update := bson.M{"$set": bson.M{"longitude": record.Longitude, "latitude": record.Latitude, "cell": record.Cell}}
		// A concurrent first upsert can cause a duplicate key error, and a
		// point delete may remove its winner before we retry. Keep upsert
		// enabled and bound retries rather than accepting a zero-match update.
		for attempt := 0; attempt < 3; attempt++ {
			var result *mongo.UpdateResult
			result, err = collection.UpdateOne(ctx, filter, update, options.Update().SetUpsert(true).SetCollation(mongoGeoSimpleCollation()))
			if !mongo.IsDuplicateKeyError(err) {
				if err == nil && result.MatchedCount == 0 && result.UpsertedCount == 0 {
					err = fmt.Errorf("geo point %q was neither matched nor inserted", record.Id)
				}
				break
			}
		}
		if err != nil {
			return errors.WithStack(err)
		}
	}
	return nil
}

func (m MongoDB) SearchGeoPoints(ctx context.Context, name string, query GeoQuery) ([]GeoResult, error) {
	query, err := normalizeGeoQuery(name, query)
	if err != nil {
		return nil, err
	}
	mongoGeoMu.RLock()
	defer mongoGeoMu.RUnlock()
	collection, err := m.readyGeoIndex(ctx, name)
	if err != nil {
		return nil, err
	}
	return searchGeoPoints(ctx, query, func(ctx context.Context, ranges []geoRange, visit func(geoRecord) error) error {
		for _, cellRange := range ranges {
			if err := ctx.Err(); err != nil {
				return errors.WithStack(err)
			}
			cursor, err := collection.Find(ctx, bson.M{"cell": bson.M{"$gte": cellRange.Min, "$lte": cellRange.Max}},
				options.Find().SetSort(bson.D{{Key: "cell", Value: 1}, {Key: "id", Value: 1}}).
					SetBatchSize(512).SetCollation(mongoGeoSimpleCollation()))
			if err != nil {
				return errors.WithStack(err)
			}
			err = func() error {
				defer cursor.Close(ctx)
				for cursor.Next(ctx) {
					if err := ctx.Err(); err != nil {
						return errors.WithStack(err)
					}
					var point mongoGeoPoint
					if err := cursor.Decode(&point); err != nil {
						return errors.WithStack(err)
					}
					if err := visit(geoRecord{GeoPoint: GeoPoint{Id: point.ID, Longitude: point.Longitude, Latitude: point.Latitude}, Cell: point.Cell}); err != nil {
						return err
					}
				}
				return errors.WithStack(cursor.Err())
			}()
			if err != nil {
				return err
			}
		}
		return nil
	})
}

func (m MongoDB) DeleteGeoPoints(ctx context.Context, name string, ids ...string) error {
	ids, err := prepareGeoIDs(name, ids)
	if err != nil {
		return err
	}
	mongoGeoMu.RLock()
	defer mongoGeoMu.RUnlock()
	collection, err := m.readyGeoIndex(ctx, name)
	if err != nil {
		return err
	}
	for begin := 0; begin < len(ids); begin += 512 {
		end := begin + 512
		if end > len(ids) {
			end = len(ids)
		}
		if _, err = collection.DeleteMany(ctx, bson.M{"id": bson.M{"$in": ids[begin:end]}},
			options.Delete().SetCollation(mongoGeoSimpleCollation())); err != nil {
			return errors.WithStack(err)
		}
	}
	return errors.WithStack(ctx.Err())
}

func (m MongoDB) DeleteGeoIndex(ctx context.Context, name string) error {
	mongoGeoMu.Lock()
	defer mongoGeoMu.Unlock()
	return m.deleteGeoIndex(ctx, name)
}

// deleteGeoIndex requires the caller to hold mongoGeoMu exclusively.
func (m MongoDB) deleteGeoIndex(ctx context.Context, name string) error {
	if err := validateGeoName(name); err != nil {
		return err
	}
	var index mongoGeoIndex
	err := m.geoCatalog().FindOneAndUpdate(ctx, bson.M{"name": name}, bson.M{"$set": bson.M{"state": "dropping"}},
		options.FindOneAndUpdate().SetReturnDocument(options.After).SetCollation(mongoGeoSimpleCollation())).Decode(&index)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return nil
	}
	if err != nil {
		return errors.WithStack(err)
	}
	return m.finishGeoIndexDeletion(ctx, index)
}

func (m MongoDB) purgeGeoIndexes(ctx context.Context) error {
	mongoGeoMu.Lock()
	defer mongoGeoMu.Unlock()
	cursor, err := m.geoCatalog().Find(ctx, bson.M{}, options.Find().SetBatchSize(512))
	if err != nil {
		return errors.WithStack(err)
	}
	defer cursor.Close(ctx)
	for cursor.Next(ctx) {
		var index mongoGeoIndex
		if err = cursor.Decode(&index); err != nil {
			return errors.WithStack(err)
		}
		if err = m.deleteGeoIndex(ctx, index.Name); err != nil {
			return err
		}
	}
	return errors.WithStack(cursor.Err())
}
