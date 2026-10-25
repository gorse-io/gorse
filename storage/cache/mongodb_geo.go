// Copyright 2026 gorse Project Authors
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
	"sync"

	"github.com/gorse-io/gorse/storage"
	"github.com/pkg/errors"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

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
	if err != nil && !(errors.As(err, &commandError) && commandError.HasErrorCode(48)) {
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
	if err != nil && !(errors.As(err, &commandError) && commandError.HasErrorCode(48)) {
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
