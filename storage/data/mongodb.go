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

// Init collections and indices in MongoDB.
func (db *MongoDB) Init() error {
	ctx := context.Background()
	d := db.client.Database(db.dbName)
	// list collections
	var hasUsers, hasItems, hasFeedback, hasMeasurements bool
	collections, err := d.ListCollectionNames(ctx, bson.M{})
	if err != nil {
		return err
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
			return err
		}
	}
	if !hasItems {
		if err = d.CreateCollection(ctx, "items"); err != nil {
			return err
		}
	}
	if !hasFeedback {
		if err = d.CreateCollection(ctx, "feedback"); err != nil {
			return err
		}
	}
	if !hasMeasurements {
		if err = d.CreateCollection(ctx, "measurements"); err != nil {
			return err
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
		return err
	}
	_, err = d.Collection("items").Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys: bson.M{
			"itemid": 1,
		},
		Options: options.Index().SetUnique(true),
	})
	if err != nil {
		return err
	}
	_, err = d.Collection("feedback").Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys: bson.M{
			"feedbackkey": 1,
		},
		Options: options.Index().SetUnique(true),
	})
	if err != nil {
		return err
	}
	_, err = d.Collection("feedback").Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys: bson.M{
			"feedbackkey.userid": 1,
		},
	})
	return err
}

// Close connection to MongoDB.
func (db *MongoDB) Close() error {
	return db.client.Disconnect(context.Background())
}

// InsertMeasurement insert a measurement into MongoDB.
func (db *MongoDB) InsertMeasurement(measurement Measurement) error {
	ctx := context.Background()
	c := db.client.Database(db.dbName).Collection("measurements")
	_, err := c.InsertOne(ctx, measurement)
	return err
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
		return measurements, err
	}
	for r.Next(ctx) {
		var measurement Measurement
		if err = r.Decode(&measurement); err != nil {
			return measurements, err
		}
		measurements = append(measurements, measurement)
	}
	return measurements, nil
}

// InsertItem inserts a item into MongoDB.
func (db *MongoDB) InsertItem(item Item) error {
	ctx := context.Background()
	c := db.client.Database(db.dbName).Collection("items")
	opt := options.Update()
	opt.SetUpsert(true)
	_, err := c.UpdateOne(ctx, bson.M{"itemid": bson.M{"$eq": item.ItemId}}, bson.M{"$set": item}, opt)
	return err
}

// BatchInsertItem insert items into MongoDB.
func (db *MongoDB) BatchInsertItem(items []Item) error {
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
	return err
}

// DeleteItem deletes a item from MongoDB.
func (db *MongoDB) DeleteItem(itemId string) error {
	ctx := context.Background()
	c := db.client.Database(db.dbName).Collection("items")
	_, err := c.DeleteOne(ctx, bson.M{"itemid": itemId})
	if err != nil {
		return err
	}
	c = db.client.Database(db.dbName).Collection("feedback")
	_, err = c.DeleteMany(ctx, bson.M{
		"feedbackkey.itemid": bson.M{"$eq": itemId},
	})
	return err
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

// InsertUser inserts a user into MongoDB.
func (db *MongoDB) InsertUser(user User) error {
	ctx := context.Background()
	c := db.client.Database(db.dbName).Collection("users")
	opt := options.Update()
	opt.SetUpsert(true)
	_, err := c.UpdateOne(ctx, bson.M{"userid": bson.M{"$eq": user.UserId}}, bson.M{"$set": user}, opt)
	return err
}

// DeleteUser deletes a user from MongoDB.
func (db *MongoDB) DeleteUser(userId string) error {
	ctx := context.Background()
	c := db.client.Database(db.dbName).Collection("users")
	_, err := c.DeleteOne(ctx, bson.M{"userid": userId})
	if err != nil {
		return err
	}
	c = db.client.Database(db.dbName).Collection("feedback")
	_, err = c.DeleteMany(ctx, bson.M{
		"feedbackkey.userid": bson.M{"$eq": userId},
	})
	return err
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

// InsertFeedback insert a feedback into MongoDB.
func (db *MongoDB) InsertFeedback(feedback Feedback, insertUser, insertItem bool) error {
	ctx := context.Background()
	opt := options.Update()
	opt.SetUpsert(true)
	// insert user
	if insertUser {
		c := db.client.Database(db.dbName).Collection("users")
		_, err := c.UpdateOne(ctx, bson.M{"userid": feedback.UserId}, bson.M{"$set": bson.M{"userid": feedback.UserId}}, opt)
		if err != nil {
			return err
		}
	} else {
		_, err := db.GetUser(feedback.UserId)
		if err != nil {
			if err == ErrUserNotExist {
				return nil
			}
			return err
		}
	}
	// insert item
	if insertItem {
		c := db.client.Database(db.dbName).Collection("items")
		_, err := c.UpdateOne(ctx, bson.M{"itemid": feedback.ItemId}, bson.M{"$set": bson.M{"itemid": feedback.ItemId}}, opt)
		if err != nil {
			return err
		}
	} else {
		_, err := db.GetItem(feedback.ItemId)
		if err != nil {
			if err == ErrItemNotExist {
				return nil
			}
			return err
		}
	}
	// insert feedback
	c := db.client.Database(db.dbName).Collection("feedback")
	_, err := c.UpdateOne(ctx, bson.M{
		"feedbackkey.feedbacktype": feedback.FeedbackType,
		"feedbackkey.userid":       feedback.UserId,
		"feedbackkey.itemid":       feedback.ItemId,
	}, bson.M{"$set": feedback}, opt)
	return err
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
				SetUpdate(bson.M{"$set": User{UserId: userId}}))
		}
		c := db.client.Database(db.dbName).Collection("users")
		_, err := c.BulkWrite(ctx, models)
		if err != nil {
			return err
		}
	} else {
		for _, userId := range userList {
			_, err := db.GetUser(userId)
			if err != nil {
				if err == ErrUserNotExist {
					users.Remove(userId)
					continue
				}
				return err
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
				SetUpdate(bson.M{"$set": Item{ItemId: itemId}}))
		}
		c := db.client.Database(db.dbName).Collection("items")
		_, err := c.BulkWrite(ctx, models)
		if err != nil {
			return err
		}
	} else {
		for _, itemId := range itemList {
			_, err := db.GetItem(itemId)
			if err != nil {
				if err == ErrItemNotExist {
					items.Remove(itemId)
					continue
				}
				return err
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
	return err
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
	// count read feedbacks
	readCountAgg, err := c.Aggregate(ctx, mongo.Pipeline{
		{{"$match", bson.M{
			"timestamp": bson.M{
				"$gte": time.Date(date.Year(), date.Month(), date.Day(), 0, 0, 0, 0, time.UTC),
				"$lt":  time.Date(date.Year(), date.Month(), date.Day()+1, 0, 0, 0, 0, time.UTC),
			},
			"feedbackkey.feedbacktype": bson.M{
				"$in": append([]string{readType}, positiveTypes...),
			},
		}}},
		{{"$project", bson.M{
			"feedbackkey.userid": 1,
			"feedbackkey.itemid": 1,
		}}},
		{{"$group", bson.M{
			"_id": "$feedbackkey",
		}}},
		{{"$group", bson.M{
			"_id":        "$_id.userid",
			"read_count": bson.M{"$sum": 1},
		}}},
	})
	if err != nil {
		return 0, err
	}
	readCount := make(map[string]int32)
	for readCountAgg.Next(ctx) {
		var ret bson.D
		err = readCountAgg.Decode(&ret)
		if err != nil {
			return 0, err
		}
		readCount[ret.Map()["_id"].(string)] = ret.Map()["read_count"].(int32)
	}
	// count positive feedbacks
	feedbackCountAgg, err := c.Aggregate(ctx, mongo.Pipeline{
		{{"$match", bson.M{
			"timestamp": bson.M{
				"$gte": time.Date(date.Year(), date.Month(), date.Day(), 0, 0, 0, 0, time.UTC),
				"$lt":  time.Date(date.Year(), date.Month(), date.Day()+1, 0, 0, 0, 0, time.UTC),
			},
			"feedbackkey.feedbacktype": bson.M{
				"$in": positiveTypes,
			},
		}}},
		{{"$project", bson.M{
			"feedbackkey.userid": 1,
			"feedbackkey.itemid": 1,
		}}},
		{{"$group", bson.M{
			"_id": "$feedbackkey",
		}}},
		{{"$group", bson.M{
			"_id":            "$_id.userid",
			"positive_count": bson.M{"$sum": 1},
		}}},
	})
	if err != nil {
		return 0, err
	}
	sum, count := 0.0, 0.0
	for feedbackCountAgg.Next(ctx) {
		var ret bson.D
		err = feedbackCountAgg.Decode(&ret)
		if err != nil {
			return 0, err
		}
		count++
		sum += float64(ret.Map()["positive_count"].(int32)) / float64(readCount[ret.Map()["_id"].(string)])
	}
	if count > 0 {
		sum /= count
	}
	return sum, err
}
