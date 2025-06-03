// Copyright 2021 gorse Project Authors
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

package data

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"time"

	mapset "github.com/deckarep/golang-set/v2"
	"github.com/juju/errors"
	"github.com/zhenghaoz/gorse/storage"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func feedbackKeyFromString(s string) (*FeedbackKey, error) {
	var feedbackKey FeedbackKey
	err := json.Unmarshal([]byte(s), &feedbackKey)
	return &feedbackKey, err
}

func (k *FeedbackKey) toString() (string, error) {
	b, err := json.Marshal(k)
	return string(b), err
}

func unpack(o any) any {
	if o == nil {
		return nil
	}
	switch p := o.(type) {
	case primitive.A:
		return []any(p)
	case primitive.D:
		m := make(map[string]any)
		for _, e := range p {
			m[e.Key] = unpack(e.Value)
		}
		return m
	default:
		return p
	}
}

// MongoDB is the data storage based on MongoDB.
type MongoDB struct {
	storage.TablePrefix
	client *mongo.Client
	dbName string
}

// Optimize is used by ClickHouse only.
func (db *MongoDB) Optimize() error {
	return nil
}

// Init collections and indices in MongoDB.
func (db *MongoDB) Init() error {
	ctx := context.Background()
	d := db.client.Database(db.dbName)
	// list collections
	var hasUsers, hasItems, hasFeedback bool
	collections, err := d.ListCollectionNames(ctx, bson.M{})
	if err != nil {
		return errors.Trace(err)
	}
	for _, collectionName := range collections {
		switch collectionName {
		case db.UsersTable():
			hasUsers = true
		case db.ItemsTable():
			hasItems = true
		case db.FeedbackTable():
			hasFeedback = true
		}
	}
	// create collections
	if !hasUsers {
		if err = d.CreateCollection(ctx, db.UsersTable()); err != nil {
			return errors.Trace(err)
		}
	}
	if !hasItems {
		if err = d.CreateCollection(ctx, db.ItemsTable()); err != nil {
			return errors.Trace(err)
		}
	}
	if !hasFeedback {
		if err = d.CreateCollection(ctx, db.FeedbackTable()); err != nil {
			return errors.Trace(err)
		}
	}
	// create index
	_, err = d.Collection(db.UsersTable()).Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys: bson.M{
			"userid": 1,
		},
		Options: options.Index().SetUnique(true),
	})
	if err != nil {
		return errors.Trace(err)
	}
	_, err = d.Collection(db.ItemsTable()).Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys: bson.M{
			"itemid": 1,
		},
		Options: options.Index().SetUnique(true),
	})
	if err != nil {
		return errors.Trace(err)
	}
	_, err = d.Collection(db.FeedbackTable()).Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys: bson.M{
			"feedbackkey": 1,
		},
		Options: options.Index().SetUnique(true),
	})
	if err != nil {
		return errors.Trace(err)
	}
	_, err = d.Collection(db.FeedbackTable()).Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys: bson.M{
			"feedbackkey.userid": 1,
		},
	})
	if err != nil {
		return errors.Trace(err)
	}
	_, err = d.Collection(db.FeedbackTable()).Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys: bson.M{
			"feedbackkey.itemid": 1,
		},
	})
	if err != nil {
		return errors.Trace(err)
	}
	return nil
}

func (db *MongoDB) Ping() error {
	return db.client.Ping(context.Background(), nil)
}

// Close connection to MongoDB.
func (db *MongoDB) Close() error {
	return db.client.Disconnect(context.Background())
}

func (db *MongoDB) Purge() error {
	tables := []string{db.ItemsTable(), db.FeedbackTable(), db.UsersTable()}
	for _, tableName := range tables {
		c := db.client.Database(db.dbName).Collection(tableName)
		_, err := c.DeleteMany(context.Background(), bson.D{})
		if err != nil {
			return errors.Trace(err)
		}
	}
	return nil
}

// BatchInsertItems insert items into MongoDB.
func (db *MongoDB) BatchInsertItems(ctx context.Context, items []Item) error {
	if len(items) == 0 {
		return nil
	}
	c := db.client.Database(db.dbName).Collection(db.ItemsTable())
	var models []mongo.WriteModel
	for _, item := range items {
		models = append(models, mongo.NewUpdateOneModel().
			SetUpsert(true).
			SetFilter(bson.M{"itemid": bson.M{"$eq": item.ItemId}}).
			SetUpdate(bson.M{"$set": item}))
	}
	_, err := c.BulkWrite(ctx, models)
	return errors.Trace(err)
}

func (db *MongoDB) BatchGetItems(ctx context.Context, itemIds []string) ([]Item, error) {
	if len(itemIds) == 0 {
		return nil, nil
	}
	c := db.client.Database(db.dbName).Collection(db.ItemsTable())
	r, err := c.Find(ctx, bson.M{"itemid": bson.M{"$in": itemIds}})
	if err != nil {
		return nil, errors.Trace(err)
	}
	items := make([]Item, 0)
	defer r.Close(ctx)
	for r.Next(ctx) {
		var item Item
		if err = r.Decode(&item); err != nil {
			return nil, errors.Trace(err)
		}
		item.Labels = unpack(item.Labels)
		items = append(items, item)
	}
	return items, nil
}

// ModifyItem modify an item in MongoDB.
func (db *MongoDB) ModifyItem(ctx context.Context, itemId string, patch ItemPatch) error {
	// create update
	update := bson.M{}
	if patch.IsHidden != nil {
		update["ishidden"] = patch.IsHidden
	}
	if patch.Categories != nil {
		update["categories"] = patch.Categories
	}
	if patch.Comment != nil {
		update["comment"] = patch.Comment
	}
	if patch.Labels != nil {
		update["labels"] = patch.Labels
	}
	if patch.Timestamp != nil {
		update["timestamp"] = patch.Timestamp
	}
	// execute
	c := db.client.Database(db.dbName).Collection(db.ItemsTable())
	_, err := c.UpdateOne(ctx, bson.M{"itemid": bson.M{"$eq": itemId}}, bson.M{"$set": update})
	return errors.Trace(err)
}

// DeleteItem deletes a item from MongoDB.
func (db *MongoDB) DeleteItem(ctx context.Context, itemId string) error {
	c := db.client.Database(db.dbName).Collection(db.ItemsTable())
	_, err := c.DeleteOne(ctx, bson.M{"itemid": itemId})
	if err != nil {
		return errors.Trace(err)
	}
	c = db.client.Database(db.dbName).Collection(db.FeedbackTable())
	_, err = c.DeleteMany(ctx, bson.M{
		"feedbackkey.itemid": bson.M{"$eq": itemId},
	})
	return errors.Trace(err)
}

// GetItem returns a item from MongoDB.
func (db *MongoDB) GetItem(ctx context.Context, itemId string) (item Item, err error) {
	c := db.client.Database(db.dbName).Collection(db.ItemsTable())
	r := c.FindOne(ctx, bson.M{"itemid": itemId})
	if r.Err() == mongo.ErrNoDocuments {
		err = errors.Annotate(ErrItemNotExist, itemId)
		return
	}
	err = r.Decode(&item)
	item.Labels = unpack(item.Labels)
	return
}

// GetItems returns items from MongoDB.
func (db *MongoDB) GetItems(ctx context.Context, cursor string, n int, timeLimit *time.Time) (string, []Item, error) {
	buf, err := base64.StdEncoding.DecodeString(cursor)
	if err != nil {
		return "", nil, errors.Trace(err)
	}
	cursorItem := string(buf)
	c := db.client.Database(db.dbName).Collection(db.ItemsTable())
	opt := options.Find()
	opt.SetLimit(int64(n))
	opt.SetSort(bson.D{{"itemid", 1}})
	filter := bson.M{"itemid": bson.M{"$gt": cursorItem}}
	if timeLimit != nil {
		filter["timestamp"] = bson.M{"$gt": *timeLimit}
	}
	r, err := c.Find(ctx, filter, opt)
	if err != nil {
		return "", nil, err
	}
	items := make([]Item, 0)
	defer r.Close(ctx)
	for r.Next(ctx) {
		var item Item
		if err = r.Decode(&item); err != nil {
			return "", nil, err
		}
		item.Labels = unpack(item.Labels)
		items = append(items, item)
	}
	if len(items) == n {
		cursor = items[n-1].ItemId
	} else {
		cursor = ""
	}
	return base64.StdEncoding.EncodeToString([]byte(cursor)), items, nil
}

// GetItemStream read items from MongoDB by stream.
func (db *MongoDB) GetItemStream(ctx context.Context, batchSize int, timeLimit *time.Time) (chan []Item, chan error) {
	itemChan := make(chan []Item, bufSize)
	errChan := make(chan error, 1)
	go func() {
		defer close(itemChan)
		defer close(errChan)
		// send query
		ctx := context.Background()
		c := db.client.Database(db.dbName).Collection(db.ItemsTable())
		opt := options.Find()
		filter := bson.M{}
		if timeLimit != nil {
			filter["timestamp"] = bson.M{"$gt": *timeLimit}
		}
		r, err := c.Find(ctx, filter, opt)
		if err != nil {
			errChan <- errors.Trace(err)
			return
		}
		// fetch result
		items := make([]Item, 0, batchSize)
		defer r.Close(ctx)
		for r.Next(ctx) {
			var item Item
			if err = r.Decode(&item); err != nil {
				errChan <- errors.Trace(err)
				return
			}
			item.Labels = unpack(item.Labels)
			items = append(items, item)
			if len(items) == batchSize {
				itemChan <- items
				items = make([]Item, 0, batchSize)
			}
		}
		if len(items) > 0 {
			itemChan <- items
		}
		errChan <- nil
	}()
	return itemChan, errChan
}

// GetItemFeedback returns feedback of a item from MongoDB.
func (db *MongoDB) GetItemFeedback(ctx context.Context, itemId string, feedbackTypes ...string) ([]Feedback, error) {
	c := db.client.Database(db.dbName).Collection(db.FeedbackTable())
	var r *mongo.Cursor
	var err error
	filter := bson.M{
		"feedbackkey.itemid": bson.M{"$eq": itemId},
		"timestamp":          bson.M{"$lte": time.Now()},
	}
	if len(feedbackTypes) > 0 {
		filter["feedbackkey.feedbacktype"] = bson.M{"$in": feedbackTypes}
	}
	r, err = c.Find(ctx, filter)
	if err != nil {
		return nil, err
	}
	feedbacks := make([]Feedback, 0)
	defer r.Close(ctx)
	for r.Next(ctx) {
		var feedback Feedback
		if err = r.Decode(&feedback); err != nil {
			return nil, err
		}
		feedbacks = append(feedbacks, feedback)
	}
	return feedbacks, nil
}

// BatchInsertUsers inserts a user into MongoDB.
func (db *MongoDB) BatchInsertUsers(ctx context.Context, users []User) error {
	if len(users) == 0 {
		return nil
	}
	c := db.client.Database(db.dbName).Collection(db.UsersTable())
	var models []mongo.WriteModel
	for _, user := range users {
		models = append(models, mongo.NewUpdateOneModel().
			SetUpsert(true).
			SetFilter(bson.M{"userid": bson.M{"$eq": user.UserId}}).
			SetUpdate(bson.M{"$set": user}))
	}
	_, err := c.BulkWrite(ctx, models)
	return errors.Trace(err)
}

// ModifyUser modify a user in MongoDB.
func (db *MongoDB) ModifyUser(ctx context.Context, userId string, patch UserPatch) error {
	// create patch
	update := bson.M{}
	if patch.Labels != nil {
		update["labels"] = patch.Labels
	}
	if patch.Comment != nil {
		update["comment"] = patch.Comment
	}
	if patch.Subscribe != nil {
		update["subscribe"] = patch.Subscribe
	}
	// execute
	c := db.client.Database(db.dbName).Collection(db.UsersTable())
	_, err := c.UpdateOne(ctx, bson.M{"userid": bson.M{"$eq": userId}}, bson.M{"$set": update})
	return errors.Trace(err)
}

// DeleteUser deletes a user from MongoDB.
func (db *MongoDB) DeleteUser(ctx context.Context, userId string) error {
	c := db.client.Database(db.dbName).Collection(db.UsersTable())
	_, err := c.DeleteOne(ctx, bson.M{"userid": userId})
	if err != nil {
		return errors.Trace(err)
	}
	c = db.client.Database(db.dbName).Collection(db.FeedbackTable())
	_, err = c.DeleteMany(ctx, bson.M{
		"feedbackkey.userid": bson.M{"$eq": userId},
	})
	return errors.Trace(err)
}

// GetUser returns a user from MongoDB.
func (db *MongoDB) GetUser(ctx context.Context, userId string) (user User, err error) {
	c := db.client.Database(db.dbName).Collection(db.UsersTable())
	r := c.FindOne(ctx, bson.M{"userid": userId})
	if r.Err() == mongo.ErrNoDocuments {
		err = errors.Annotate(ErrUserNotExist, userId)
		return
	}
	err = r.Decode(&user)
	user.Labels = unpack(user.Labels)
	return
}

// GetUsers returns users from MongoDB.
func (db *MongoDB) GetUsers(ctx context.Context, cursor string, n int) (string, []User, error) {
	buf, err := base64.StdEncoding.DecodeString(cursor)
	if err != nil {
		return "", nil, errors.Trace(err)
	}
	cursorUser := string(buf)
	c := db.client.Database(db.dbName).Collection(db.UsersTable())
	opt := options.Find()
	opt.SetLimit(int64(n))
	opt.SetSort(bson.D{{"userid", 1}})
	r, err := c.Find(ctx, bson.M{"userid": bson.M{"$gt": cursorUser}}, opt)
	if err != nil {
		return "", nil, err
	}
	users := make([]User, 0)
	defer r.Close(ctx)
	for r.Next(ctx) {
		var user User
		if err = r.Decode(&user); err != nil {
			return "", nil, err
		}
		user.Labels = unpack(user.Labels)
		users = append(users, user)
	}
	if len(users) == n {
		cursor = users[n-1].UserId
	} else {
		cursor = ""
	}
	return base64.StdEncoding.EncodeToString([]byte(cursor)), users, nil
}

// GetUserStream reads users from MongoDB by stream.
func (db *MongoDB) GetUserStream(ctx context.Context, batchSize int) (chan []User, chan error) {
	userChan := make(chan []User, bufSize)
	errChan := make(chan error, 1)
	go func() {
		defer close(userChan)
		defer close(errChan)
		// send query
		ctx := context.Background()
		c := db.client.Database(db.dbName).Collection(db.UsersTable())
		opt := options.Find()
		r, err := c.Find(ctx, bson.M{}, opt)
		if err != nil {
			errChan <- errors.Trace(err)
			return
		}
		users := make([]User, 0, batchSize)
		defer r.Close(ctx)
		for r.Next(ctx) {
			var user User
			if err = r.Decode(&user); err != nil {
				errChan <- errors.Trace(err)
				return
			}
			user.Labels = unpack(user.Labels)
			users = append(users, user)
			if len(users) == batchSize {
				userChan <- users
				users = make([]User, 0, batchSize)
			}
		}
		if len(users) > 0 {
			userChan <- users
		}
		errChan <- nil
	}()
	return userChan, errChan
}

// GetUserFeedback returns feedback of a user from MongoDB.
func (db *MongoDB) GetUserFeedback(ctx context.Context, userId string, endTime *time.Time, feedbackTypes ...string) ([]Feedback, error) {
	c := db.client.Database(db.dbName).Collection(db.FeedbackTable())
	var r *mongo.Cursor
	var err error
	filter := bson.M{
		"feedbackkey.userid": bson.M{"$eq": userId},
	}
	if endTime != nil {
		filter["timestamp"] = bson.M{"$lte": endTime}
	}
	if len(feedbackTypes) > 0 {
		filter["feedbackkey.feedbacktype"] = bson.M{"$in": feedbackTypes}
	}
	r, err = c.Find(ctx, filter)
	if err != nil {
		return nil, err
	}
	feedbacks := make([]Feedback, 0)
	defer r.Close(ctx)
	for r.Next(ctx) {
		var feedback Feedback
		if err = r.Decode(&feedback); err != nil {
			return nil, err
		}
		feedbacks = append(feedbacks, feedback)
	}
	return feedbacks, nil
}

// BatchInsertFeedback returns multiple feedback into MongoDB.
func (db *MongoDB) BatchInsertFeedback(ctx context.Context, feedback []Feedback, insertUser, insertItem, overwrite bool) error {
	// skip empty list
	if len(feedback) == 0 {
		return nil
	}
	// collect users and items
	users := mapset.NewSet[string]()
	items := mapset.NewSet[string]()
	for _, v := range feedback {
		users.Add(v.UserId)
		items.Add(v.ItemId)
	}
	// insert users
	userList := users.ToSlice()
	if insertUser {
		var models []mongo.WriteModel
		for _, userId := range userList {
			models = append(models, mongo.NewUpdateOneModel().
				SetUpsert(true).
				SetFilter(bson.M{"userid": bson.M{"$eq": userId}}).
				SetUpdate(bson.M{"$setOnInsert": User{UserId: userId}}))
		}
		c := db.client.Database(db.dbName).Collection(db.UsersTable())
		_, err := c.BulkWrite(ctx, models)
		if err != nil {
			return errors.Trace(err)
		}
	} else {
		for _, userId := range userList {
			_, err := db.GetUser(ctx, userId)
			if err != nil {
				if errors.Is(err, errors.NotFound) {
					users.Remove(userId)
					continue
				}
				return errors.Trace(err)
			}
		}
	}
	// insert items
	itemList := items.ToSlice()
	if insertItem {
		var models []mongo.WriteModel
		for _, itemId := range itemList {
			models = append(models, mongo.NewUpdateOneModel().
				SetUpsert(true).
				SetFilter(bson.M{"itemid": bson.M{"$eq": itemId}}).
				SetUpdate(bson.M{"$setOnInsert": Item{ItemId: itemId}}))
		}
		c := db.client.Database(db.dbName).Collection(db.ItemsTable())
		_, err := c.BulkWrite(ctx, models)
		if err != nil {
			return errors.Trace(err)
		}
	} else {
		for _, itemId := range itemList {
			_, err := db.GetItem(ctx, itemId)
			if err != nil {
				if errors.Is(err, errors.NotFound) {
					items.Remove(itemId)
					continue
				}
				return errors.Trace(err)
			}
		}
	}
	// insert feedback
	c := db.client.Database(db.dbName).Collection(db.FeedbackTable())
	var models []mongo.WriteModel
	for _, f := range feedback {
		if users.Contains(f.UserId) && items.Contains(f.ItemId) {
			model := mongo.NewUpdateOneModel().
				SetUpsert(true).
				SetFilter(bson.M{
					"feedbackkey": f.FeedbackKey,
				})
			if overwrite {
				model.SetUpdate(bson.M{"$set": f})
			} else {
				model.SetUpdate(bson.M{
					"$setOnInsert": bson.M{
						"feedbackkey": f.FeedbackKey,
						"timestamp":   f.Timestamp,
						"comment":     f.Comment,
					},
					"$inc": bson.M{
						"value": f.Value,
					},
				})
			}
			models = append(models, model)
		}
	}
	if len(models) == 0 {
		return nil
	}
	_, err := c.BulkWrite(ctx, models)
	return errors.Trace(err)
}

// GetFeedback returns multiple feedback from MongoDB.
func (db *MongoDB) GetFeedback(ctx context.Context, cursor string, n int, beginTime, endTime *time.Time, feedbackTypes ...string) (string, []Feedback, error) {
	buf, err := base64.StdEncoding.DecodeString(cursor)
	if err != nil {
		return "", nil, errors.Trace(err)
	}
	c := db.client.Database(db.dbName).Collection(db.FeedbackTable())
	opt := options.Find()
	opt.SetLimit(int64(n))
	opt.SetSort(bson.D{{"feedbackkey", 1}})
	filter := make(bson.M)
	// pass cursor to filter
	if len(buf) > 0 {
		feedbackKey, err := feedbackKeyFromString(string(buf))
		if err != nil {
			return "", nil, err
		}
		filter["feedbackkey"] = bson.M{"$gt": feedbackKey}
	}
	// pass feedback type to filter
	if len(feedbackTypes) > 0 {
		filter["feedbackkey.feedbacktype"] = bson.M{"$in": feedbackTypes}
	}
	// pass time limit to filter
	timestampConditions := bson.M{}
	if beginTime != nil {
		timestampConditions["$gt"] = *beginTime
	}
	if endTime != nil {
		timestampConditions["$lte"] = *endTime
	}
	filter["timestamp"] = timestampConditions
	r, err := c.Find(ctx, filter, opt)
	if err != nil {
		return "", nil, err
	}
	feedbacks := make([]Feedback, 0)
	defer r.Close(ctx)
	for r.Next(ctx) {
		var feedback Feedback
		if err = r.Decode(&feedback); err != nil {
			return "", nil, err
		}
		feedbacks = append(feedbacks, feedback)
	}
	if len(feedbacks) == n {
		cursor, err = feedbacks[n-1].toString()
		if err != nil {
			return "", nil, err
		}
	} else {
		cursor = ""
	}
	return base64.StdEncoding.EncodeToString([]byte(cursor)), feedbacks, nil
}

// GetFeedbackStream reads feedback from MongoDB by stream.
func (db *MongoDB) GetFeedbackStream(ctx context.Context, batchSize int, scanOptions ...ScanOption) (chan []Feedback, chan error) {
	scan := NewScanOptions(scanOptions...)
	feedbackChan := make(chan []Feedback, bufSize)
	errChan := make(chan error, 1)
	go func() {
		defer close(feedbackChan)
		defer close(errChan)
		// send query
		ctx := context.Background()
		c := db.client.Database(db.dbName).Collection(db.FeedbackTable())
		opt := options.Find()
		filter := make(bson.M)
		// pass feedback type to filter
		if len(scan.FeedbackTypes) > 0 {
			filter["feedbackkey.feedbacktype"] = bson.M{"$in": scan.FeedbackTypes}
		}
		// pass time limit to filter
		if scan.BeginTime != nil || scan.EndTime != nil {
			timestampConditions := bson.M{}
			if scan.BeginTime != nil {
				timestampConditions["$gt"] = *scan.BeginTime
			}
			if scan.EndTime != nil {
				timestampConditions["$lte"] = *scan.EndTime
			}
			filter["timestamp"] = timestampConditions
		}
		// pass user id to filter
		if scan.BeginUserId != nil || scan.EndUserId != nil {
			userIdConditions := bson.M{}
			if scan.BeginUserId != nil {
				userIdConditions["$gte"] = *scan.BeginUserId
			}
			if scan.EndUserId != nil {
				userIdConditions["$lte"] = *scan.EndUserId
			}
			filter["feedbackkey.userid"] = userIdConditions
		}
		if scan.BeginItemId != nil || scan.EndItemId != nil {
			itemIdConditions := bson.M{}
			if scan.BeginItemId != nil {
				itemIdConditions["$gte"] = *scan.BeginItemId
			}
			if scan.EndItemId != nil {
				itemIdConditions["$lte"] = *scan.EndItemId
			}
			filter["feedbackkey.itemid"] = itemIdConditions
		}
		if scan.OrderByItemId {
			opt.SetSort(bson.D{{"feedbackkey.itemid", 1}})
		}

		r, err := c.Find(ctx, filter, opt)
		if err != nil {
			errChan <- errors.Trace(err)
			return
		}
		feedbacks := make([]Feedback, 0, batchSize)
		defer r.Close(ctx)
		for r.Next(ctx) {
			var feedback Feedback
			if err = r.Decode(&feedback); err != nil {
				errChan <- errors.Trace(err)
				return
			}
			feedbacks = append(feedbacks, feedback)
			if len(feedbacks) == batchSize {
				feedbackChan <- feedbacks
				feedbacks = make([]Feedback, 0, batchSize)
			}
		}
		if len(feedbacks) > 0 {
			feedbackChan <- feedbacks
		}
		errChan <- nil
	}()
	return feedbackChan, errChan
}

// GetUserItemFeedback returns a feedback return the user id and item id from MongoDB.
func (db *MongoDB) GetUserItemFeedback(ctx context.Context, userId, itemId string, feedbackTypes ...string) ([]Feedback, error) {
	c := db.client.Database(db.dbName).Collection(db.FeedbackTable())
	var filter = bson.M{
		"feedbackkey.userid": bson.M{"$eq": userId},
		"feedbackkey.itemid": bson.M{"$eq": itemId},
	}
	if len(feedbackTypes) > 0 {
		filter["feedbackkey.feedbacktype"] = bson.M{"$in": feedbackTypes}
	}
	r, err := c.Find(ctx, filter)
	if err != nil {
		return nil, err
	}
	feedbacks := make([]Feedback, 0)
	defer r.Close(ctx)
	for r.Next(ctx) {
		var feedback Feedback
		if err = r.Decode(&feedback); err != nil {
			return nil, err
		}
		feedbacks = append(feedbacks, feedback)
	}
	return feedbacks, nil
}

// DeleteUserItemFeedback deletes a feedback return the user id and item id from MongoDB.
func (db *MongoDB) DeleteUserItemFeedback(ctx context.Context, userId, itemId string, feedbackTypes ...string) (int, error) {
	c := db.client.Database(db.dbName).Collection(db.FeedbackTable())
	var filter = bson.M{
		"feedbackkey.userid": bson.M{"$eq": userId},
		"feedbackkey.itemid": bson.M{"$eq": itemId},
	}
	if len(feedbackTypes) > 0 {
		filter["feedbackkey.feedbacktype"] = bson.M{"$in": feedbackTypes}
	}
	r, err := c.DeleteMany(ctx, filter)
	if err != nil {
		return 0, err
	}
	return int(r.DeletedCount), nil
}

func (db *MongoDB) CountUsers(ctx context.Context) (int, error) {
	n, err := db.client.Database(db.dbName).Collection(db.UsersTable()).EstimatedDocumentCount(ctx)
	return int(n), err
}

func (db *MongoDB) CountItems(ctx context.Context) (int, error) {
	n, err := db.client.Database(db.dbName).Collection(db.ItemsTable()).EstimatedDocumentCount(ctx)
	return int(n), err
}

func (db *MongoDB) CountFeedback(ctx context.Context) (int, error) {
	n, err := db.client.Database(db.dbName).Collection(db.FeedbackTable()).EstimatedDocumentCount(ctx)
	return int(n), err
}
