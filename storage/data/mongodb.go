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
	"encoding/json"
	"github.com/juju/errors"
	"time"

	"github.com/scylladb/go-set/strset"
	"go.mongodb.org/mongo-driver/bson"
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

// MongoDB is the data storage based on MongoDB.
type MongoDB struct {
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
	var hasUsers, hasItems, hasFeedback, hasMeasurements bool
	collections, err := d.ListCollectionNames(ctx, bson.M{})
	if err != nil {
		return errors.Trace(err)
	}
	for _, collectionName := range collections {
		switch collectionName {
		case "users":
			hasUsers = true
		case "items":
			hasItems = true
		case "feedback":
			hasFeedback = true
		case "measurements":
			hasMeasurements = true
		}
	}
	// create collections
	if !hasUsers {
		if err = d.CreateCollection(ctx, "users"); err != nil {
			return errors.Trace(err)
		}
	}
	if !hasItems {
		if err = d.CreateCollection(ctx, "items"); err != nil {
			return errors.Trace(err)
		}
	}
	if !hasFeedback {
		if err = d.CreateCollection(ctx, "feedback"); err != nil {
			return errors.Trace(err)
		}
	}
	if !hasMeasurements {
		if err = d.CreateCollection(ctx, "measurements"); err != nil {
			return errors.Trace(err)
		}
	}
	// create index
	_, err = d.Collection("users").Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys: bson.M{
			"userid": 1,
		},
		Options: options.Index().SetUnique(true),
	})
	if err != nil {
		return errors.Trace(err)
	}
	_, err = d.Collection("items").Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys: bson.M{
			"itemid": 1,
		},
		Options: options.Index().SetUnique(true),
	})
	if err != nil {
		return errors.Trace(err)
	}
	_, err = d.Collection("feedback").Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys: bson.M{
			"feedbackkey": 1,
		},
		Options: options.Index().SetUnique(true),
	})
	if err != nil {
		return errors.Trace(err)
	}
	_, err = d.Collection("feedback").Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys: bson.M{
			"feedbackkey.userid": 1,
		},
	})
	return errors.Trace(err)
}

// Close connection to MongoDB.
func (db *MongoDB) Close() error {
	return db.client.Disconnect(context.Background())
}

// InsertMeasurement insert a measurement into MongoDB.
func (db *MongoDB) InsertMeasurement(measurement Measurement) error {
	ctx := context.Background()
	c := db.client.Database(db.dbName).Collection("measurements")
	opt := options.Update()
	opt.SetUpsert(true)
	_, err := c.UpdateOne(ctx,
		bson.M{"name": bson.M{"$eq": measurement.Name}, "timestamp": bson.M{"$eq": measurement.Timestamp}},
		bson.M{"$set": measurement}, opt)
	return errors.Trace(err)
}

// GetMeasurements get recent measurements from MongoDB.
func (db *MongoDB) GetMeasurements(name string, n int) ([]Measurement, error) {
	ctx := context.Background()
	c := db.client.Database(db.dbName).Collection("measurements")
	opt := options.Find()
	opt.SetLimit(int64(n))
	opt.SetSort(bson.D{{"timestamp", -1}})
	r, err := c.Find(ctx, bson.M{"name": bson.M{"$eq": name}}, opt)
	measurements := make([]Measurement, 0)
	if err != nil {
		return nil, err
	}
	defer r.Close(ctx)
	for r.Next(ctx) {
		var measurement Measurement
		if err = r.Decode(&measurement); err != nil {
			return measurements, err
		}
		measurements = append(measurements, measurement)
	}
	return measurements, nil
}

// BatchInsertItems insert items into MongoDB.
func (db *MongoDB) BatchInsertItems(items []Item) error {
	ctx := context.Background()
	c := db.client.Database(db.dbName).Collection("items")
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

// DeleteItem deletes a item from MongoDB.
func (db *MongoDB) DeleteItem(itemId string) error {
	ctx := context.Background()
	c := db.client.Database(db.dbName).Collection("items")
	_, err := c.DeleteOne(ctx, bson.M{"itemid": itemId})
	if err != nil {
		return errors.Trace(err)
	}
	c = db.client.Database(db.dbName).Collection("feedback")
	_, err = c.DeleteMany(ctx, bson.M{
		"feedbackkey.itemid": bson.M{"$eq": itemId},
	})
	return errors.Trace(err)
}

// GetItem returns a item from MongoDB.
func (db *MongoDB) GetItem(itemId string) (item Item, err error) {
	ctx := context.Background()
	c := db.client.Database(db.dbName).Collection("items")
	r := c.FindOne(ctx, bson.M{"itemid": itemId})
	if r.Err() == mongo.ErrNoDocuments {
		err = ErrItemNotExist
		return
	}
	err = r.Decode(&item)
	return
}

// GetItems returns items from MongoDB.
func (db *MongoDB) GetItems(cursor string, n int, timeLimit *time.Time) (string, []Item, error) {
	ctx := context.Background()
	c := db.client.Database(db.dbName).Collection("items")
	opt := options.Find()
	opt.SetLimit(int64(n))
	opt.SetSort(bson.D{{"itemid", 1}})
	filter := bson.M{"itemid": bson.M{"$gt": cursor}}
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
		items = append(items, item)
	}
	if len(items) == n {
		cursor = items[n-1].ItemId
	} else {
		cursor = ""
	}
	return cursor, items, nil
}

// GetItemStream read items from MongoDB by stream.
func (db *MongoDB) GetItemStream(batchSize int, timeLimit *time.Time) (chan []Item, chan error) {
	itemChan := make(chan []Item, bufSize)
	errChan := make(chan error, 1)
	go func() {
		defer close(itemChan)
		defer close(errChan)
		// send query
		ctx := context.Background()
		c := db.client.Database(db.dbName).Collection("items")
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
func (db *MongoDB) GetItemFeedback(itemId string, feedbackTypes ...string) ([]Feedback, error) {
	startTime := time.Now()
	ctx := context.Background()
	c := db.client.Database(db.dbName).Collection("feedback")
	var r *mongo.Cursor
	var err error
	filter := bson.M{
		"feedbackkey.itemid": bson.M{"$eq": itemId},
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
	GetItemFeedbackLatency.Observe(time.Since(startTime).Seconds())
	return feedbacks, nil
}

// BatchInsertUsers inserts a user into MongoDB.
func (db *MongoDB) BatchInsertUsers(users []User) error {
	ctx := context.Background()
	c := db.client.Database(db.dbName).Collection("users")
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

// DeleteUser deletes a user from MongoDB.
func (db *MongoDB) DeleteUser(userId string) error {
	ctx := context.Background()
	c := db.client.Database(db.dbName).Collection("users")
	_, err := c.DeleteOne(ctx, bson.M{"userid": userId})
	if err != nil {
		return errors.Trace(err)
	}
	c = db.client.Database(db.dbName).Collection("feedback")
	_, err = c.DeleteMany(ctx, bson.M{
		"feedbackkey.userid": bson.M{"$eq": userId},
	})
	return errors.Trace(err)
}

// GetUser returns a user from MongoDB.
func (db *MongoDB) GetUser(userId string) (user User, err error) {
	ctx := context.Background()
	c := db.client.Database(db.dbName).Collection("users")
	r := c.FindOne(ctx, bson.M{"userid": userId})
	if r.Err() == mongo.ErrNoDocuments {
		err = ErrUserNotExist
		return
	}
	err = r.Decode(&user)
	return
}

// GetUsers returns users from MongoDB.
func (db *MongoDB) GetUsers(cursor string, n int) (string, []User, error) {
	ctx := context.Background()
	c := db.client.Database(db.dbName).Collection("users")
	opt := options.Find()
	opt.SetLimit(int64(n))
	opt.SetSort(bson.D{{"userid", 1}})
	r, err := c.Find(ctx, bson.M{"userid": bson.M{"$gt": cursor}}, opt)
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
		users = append(users, user)
	}
	if len(users) == n {
		cursor = users[n-1].UserId
	} else {
		cursor = ""
	}
	return cursor, users, nil
}

// GetUserStream reads users from MongoDB by stream.
func (db *MongoDB) GetUserStream(batchSize int) (chan []User, chan error) {
	userChan := make(chan []User, bufSize)
	errChan := make(chan error, 1)
	go func() {
		defer close(userChan)
		defer close(errChan)
		// send query
		ctx := context.Background()
		c := db.client.Database(db.dbName).Collection("users")
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
func (db *MongoDB) GetUserFeedback(userId string, feedbackTypes ...string) ([]Feedback, error) {
	startTime := time.Now()
	ctx := context.Background()
	c := db.client.Database(db.dbName).Collection("feedback")
	var r *mongo.Cursor
	var err error
	filter := bson.M{
		"feedbackkey.userid": bson.M{"$eq": userId},
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
	GetUserFeedbackLatency.Observe(time.Since(startTime).Seconds())
	return feedbacks, nil
}

// BatchInsertFeedback returns multiple feedback into MongoDB.
func (db *MongoDB) BatchInsertFeedback(feedback []Feedback, insertUser, insertItem bool) error {
	ctx := context.Background()
	// collect users and items
	users := strset.New()
	items := strset.New()
	for _, v := range feedback {
		users.Add(v.UserId)
		items.Add(v.ItemId)
	}
	// insert users
	userList := users.List()
	if insertUser {
		var models []mongo.WriteModel
		for _, userId := range userList {
			models = append(models, mongo.NewUpdateOneModel().
				SetUpsert(true).
				SetFilter(bson.M{"userid": bson.M{"$eq": userId}}).
				SetUpdate(bson.M{"$setOnInsert": User{UserId: userId}}))
		}
		c := db.client.Database(db.dbName).Collection("users")
		_, err := c.BulkWrite(ctx, models)
		if err != nil {
			return errors.Trace(err)
		}
	} else {
		for _, userId := range userList {
			_, err := db.GetUser(userId)
			if err != nil {
				if err == ErrUserNotExist {
					users.Remove(userId)
					continue
				}
				return errors.Trace(err)
			}
		}
	}
	// insert items
	itemList := items.List()
	if insertItem {
		var models []mongo.WriteModel
		for _, itemId := range itemList {
			models = append(models, mongo.NewUpdateOneModel().
				SetUpsert(true).
				SetFilter(bson.M{"itemid": bson.M{"$eq": itemId}}).
				SetUpdate(bson.M{"$setOnInsert": Item{ItemId: itemId}}))
		}
		c := db.client.Database(db.dbName).Collection("items")
		_, err := c.BulkWrite(ctx, models)
		if err != nil {
			return errors.Trace(err)
		}
	} else {
		for _, itemId := range itemList {
			_, err := db.GetItem(itemId)
			if err != nil {
				if err == ErrItemNotExist {
					items.Remove(itemId)
					continue
				}
				return errors.Trace(err)
			}
		}
	}
	// insert feedback
	c := db.client.Database(db.dbName).Collection("feedback")
	var models []mongo.WriteModel
	for _, f := range feedback {
		models = append(models, mongo.NewUpdateOneModel().
			SetUpsert(true).
			SetFilter(bson.M{
				"feedbackkey": f.FeedbackKey,
			}).SetUpdate(bson.M{"$set": f}))
	}
	_, err := c.BulkWrite(ctx, models)
	return errors.Trace(err)
}

// GetFeedback returns multiple feedback from MongoDB.
func (db *MongoDB) GetFeedback(cursor string, n int, timeLimit *time.Time, feedbackTypes ...string) (string, []Feedback, error) {
	ctx := context.Background()
	c := db.client.Database(db.dbName).Collection("feedback")
	opt := options.Find()
	opt.SetLimit(int64(n))
	opt.SetSort(bson.D{{"feedbackkey", 1}})
	filter := make(bson.M)
	// pass cursor to filter
	if cursor != "" {
		feedbackKey, err := feedbackKeyFromString(cursor)
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
	if timeLimit != nil {
		filter["timestamp"] = bson.M{"$gt": *timeLimit}
	}
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
	return cursor, feedbacks, nil
}

// GetFeedbackStream reads feedback from MongoDB by stream.
func (db *MongoDB) GetFeedbackStream(batchSize int, timeLimit *time.Time, feedbackTypes ...string) (chan []Feedback, chan error) {
	feedbackChan := make(chan []Feedback, bufSize)
	errChan := make(chan error, 1)
	go func() {
		defer close(feedbackChan)
		defer close(errChan)
		// send query
		ctx := context.Background()
		c := db.client.Database(db.dbName).Collection("feedback")
		opt := options.Find()
		filter := make(bson.M)
		// pass feedback type to filter
		if len(feedbackTypes) > 0 {
			filter["feedbackkey.feedbacktype"] = bson.M{"$in": feedbackTypes}
		}
		// pass time limit to filter
		if timeLimit != nil {
			filter["timestamp"] = bson.M{"$gt": *timeLimit}
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
func (db *MongoDB) GetUserItemFeedback(userId, itemId string, feedbackTypes ...string) ([]Feedback, error) {
	startTime := time.Now()
	ctx := context.Background()
	c := db.client.Database(db.dbName).Collection("feedback")
	var filter bson.M = bson.M{
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
	GetUserItemFeedbackLatency.Observe(time.Since(startTime).Seconds())
	return feedbacks, nil
}

// DeleteUserItemFeedback deletes a feedback return the user id and item id from MongoDB.
func (db *MongoDB) DeleteUserItemFeedback(userId, itemId string, feedbackTypes ...string) (int, error) {
	ctx := context.Background()
	c := db.client.Database(db.dbName).Collection("feedback")
	var filter bson.M = bson.M{
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

// CountActiveUsers returns the number active users starting from a specified date.
func (db *MongoDB) CountActiveUsers(date time.Time) (int, error) {
	ctx := context.Background()
	c := db.client.Database(db.dbName).Collection("feedback")
	distinct, err := c.Distinct(ctx, "feedbackkey.userid", bson.M{
		"timestamp": bson.M{
			"$gte": time.Date(date.Year(), date.Month(), date.Day(), 0, 0, 0, 0, time.UTC),
			"$lt":  time.Date(date.Year(), date.Month(), date.Day()+1, 0, 0, 0, 0, time.UTC),
		},
	})
	if err != nil {
		return 0, err
	}
	return len(distinct), nil
}

// GetClickThroughRate computes the click-through-rate of a specified date.
func (db *MongoDB) GetClickThroughRate(date time.Time, positiveTypes []string, readType string) (float64, error) {
	ctx := context.Background()
	c := db.client.Database(db.dbName).Collection("feedback")
	readCountAgg, err := c.Aggregate(ctx, mongo.Pipeline{
		// collect read feedbacks
		{{"$match", bson.M{
			"timestamp": bson.M{
				"$gte": time.Date(date.Year(), date.Month(), date.Day(), 0, 0, 0, 0, time.UTC),
				"$lt":  time.Date(date.Year(), date.Month(), date.Day()+1, 0, 0, 0, 0, time.UTC),
			},
			"feedbackkey.feedbacktype": bson.M{
				"$eq": readType,
			},
		}}},
		{{"$project", bson.M{
			"feedbackkey.userid": 1,
			"feedbackkey.itemid": 1,
		}}},
		{{"$group", bson.M{
			"_id": "$feedbackkey",
		}}},
		// collect positive feedback
		{{"$lookup", bson.M{
			"from": "feedback",
			"let":  bson.M{"feedbackkey": "$_id"},
			"pipeline": mongo.Pipeline{
				{{"$match", bson.M{
					"$expr": bson.M{
						"$and": []bson.M{
							{"$gte": []interface{}{"$timestamp", time.Date(date.Year(), date.Month(), date.Day(), 0, 0, 0, 0, time.UTC)}},
							{"$lt": []interface{}{"$timestamp", time.Date(date.Year(), date.Month(), date.Day()+1, 0, 0, 0, 0, time.UTC)}},
							{"$in": []interface{}{"$feedbackkey.feedbacktype", positiveTypes}},
							{"$eq": []string{"$feedbackkey.userid", "$$feedbackkey.userid"}},
							{"$eq": []string{"$feedbackkey.itemid", "$$feedbackkey.itemid"}},
						},
					},
				}}},
				{{"$project", bson.M{
					"feedbackkey.userid": 1,
					"feedbackkey.itemid": 1,
				}}},
				{{"$group", bson.M{
					"_id": "$feedbackkey",
				}}},
			},
			"as": "positive_feedback",
		}}},
		{{"$unwind", bson.M{
			"path":                       "$positive_feedback",
			"preserveNullAndEmptyArrays": true,
		}}},
		{{"$project", bson.M{
			"is_positive": bson.M{"$cond": []interface{}{bson.M{"$not": []string{"$positive_feedback"}}, 0, 1}},
		}}},
		{{"$group", bson.M{
			"_id":            "$_id.userid",
			"positive_count": bson.M{"$sum": "$is_positive"},
			"read_count":     bson.M{"$sum": 1},
		}}},
		// users must have at least one positive feedback
		{{"$match", bson.M{
			"positive_count": bson.M{
				"$gt": 0,
			},
		}}},
		// get click-through rates
		{{"$project", bson.M{
			"ctr": bson.M{"$divide": []interface{}{"$positive_count", "$read_count"}},
		}}},
		// get the average of click-through rates
		{{"$group", bson.M{
			"_id":     nil,
			"avg_ctr": bson.M{"$avg": "$ctr"},
		}}},
	})
	if err != nil {
		return 0, err
	}
	var result float64
	if readCountAgg.Next(ctx) {
		var ret bson.D
		err = readCountAgg.Decode(&ret)
		if err != nil {
			return 0, err
		}
		result = ret.Map()["avg_ctr"].(float64)
	}
	return result, err
}
