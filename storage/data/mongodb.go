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
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"time"
)

type MongoDB struct {
	client *mongo.Client
	dbName string
}

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
	return nil
}

func (db *MongoDB) Close() error {
	return db.client.Disconnect(context.Background())
}

func (db *MongoDB) InsertMeasurement(measurement Measurement) error {
	ctx := context.Background()
	c := db.client.Database(db.dbName).Collection("measurements")
	_, err := c.InsertOne(ctx, measurement)
	return err
}

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

func (db *MongoDB) InsertItem(item Item) error {
	ctx := context.Background()
	c := db.client.Database(db.dbName).Collection("items")
	opt := options.Update()
	opt.SetUpsert(true)
	_, err := c.UpdateOne(ctx, bson.M{"itemid": bson.M{"$eq": item.ItemId}}, bson.M{"$set": item}, opt)
	return err
}

func (db *MongoDB) BatchInsertItem(items []Item) error {
	for _, item := range items {
		if err := db.InsertItem(item); err != nil {
			return err
		}
	}
	return nil
}

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

func (db *MongoDB) GetItem(itemId string) (item Item, err error) {
	ctx := context.Background()
	c := db.client.Database(db.dbName).Collection("items")
	r := c.FindOne(ctx, bson.M{"itemid": itemId})
	err = r.Decode(&item)
	return
}

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

func (db *MongoDB) GetItemFeedback(itemId string, feedbackType *string) ([]Feedback, error) {
	startTime := time.Now()
	ctx := context.Background()
	c := db.client.Database(db.dbName).Collection("feedback")
	var r *mongo.Cursor
	var err error
	if feedbackType != nil {
		r, err = c.Find(ctx, bson.M{
			"feedbackkey.feedbacktype": bson.M{"$eq": *feedbackType},
			"feedbackkey.itemid":       bson.M{"$eq": itemId},
		})
	} else {
		r, err = c.Find(ctx, bson.M{
			"feedbackkey.itemid": bson.M{"$eq": itemId},
		})
	}
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

func (db *MongoDB) InsertUser(user User) error {
	ctx := context.Background()
	c := db.client.Database(db.dbName).Collection("users")
	opt := options.Update()
	opt.SetUpsert(true)
	_, err := c.UpdateOne(ctx, bson.M{"userid": bson.M{"$eq": user.UserId}}, bson.M{"$set": user}, opt)
	return err
}

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

func (db *MongoDB) GetUser(userId string) (user User, err error) {
	ctx := context.Background()
	c := db.client.Database(db.dbName).Collection("users")
	r := c.FindOne(ctx, bson.M{"userid": userId})
	err = r.Decode(&user)
	return
}

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

func (db *MongoDB) GetUserFeedback(userId string, feedbackType *string) ([]Feedback, error) {
	startTime := time.Now()
	ctx := context.Background()
	c := db.client.Database(db.dbName).Collection("feedback")
	var r *mongo.Cursor
	var err error
	if feedbackType != nil {
		r, err = c.Find(ctx, bson.M{
			"feedbackkey.feedbacktype": bson.M{"$eq": *feedbackType},
			"feedbackkey.userid":       bson.M{"$eq": userId},
		})
	} else {
		r, err = c.Find(ctx, bson.M{
			"feedbackkey.userid": bson.M{"$eq": userId},
		})
	}
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

func (db *MongoDB) InsertFeedback(feedback Feedback, insertUser, insertItem bool) error {
	ctx := context.Background()
	opt := options.Update()
	opt.SetUpsert(true)
	// insert feedback
	c := db.client.Database(db.dbName).Collection("feedback")
	_, err := c.UpdateOne(ctx, bson.M{
		"feedbackkey.feedbacktype": feedback.FeedbackType,
		"feedbackkey.userid":       feedback.UserId,
		"feedbackkey.itemid":       feedback.ItemId,
	}, bson.M{"$set": feedback}, opt)
	if err != nil {
		return err
	}
	// insert user
	if insertUser {
		c = db.client.Database(db.dbName).Collection("users")
		_, err = c.UpdateOne(ctx, bson.M{"userid": feedback.UserId}, bson.M{"$set": bson.M{"userid": feedback.UserId}}, opt)
		if err != nil {
			return err
		}
	}
	// insert item
	if insertItem {
		c = db.client.Database(db.dbName).Collection("items")
		_, err = c.UpdateOne(ctx, bson.M{"itemid": feedback.ItemId}, bson.M{"$set": bson.M{"itemid": feedback.ItemId}}, opt)
		if err != nil {
			return err
		}
	}
	return nil
}

func (db *MongoDB) BatchInsertFeedback(feedback []Feedback, insertUser, insertItem bool) error {
	for _, f := range feedback {
		if err := db.InsertFeedback(f, insertUser, insertItem); err != nil {
			return err
		}
	}
	return nil
}

func (db *MongoDB) GetFeedback(cursor string, n int, feedbackType *string, timeLimit *time.Time) (string, []Feedback, error) {
	ctx := context.Background()
	c := db.client.Database(db.dbName).Collection("feedback")
	opt := options.Find()
	opt.SetLimit(int64(n))
	opt.SetSort(bson.D{{"feedbackkey", 1}})
	filter := make(bson.M)
	// pass cursor to filter
	if cursor != "" {
		feedbackKey, err := FeedbackKeyFromString(cursor)
		if err != nil {
			return "", nil, err
		}
		filter["feedbackkey"] = bson.M{"$gt": feedbackKey}
	}
	// pass feedback type to filter
	if feedbackType != nil {
		filter["feedbackkey.feedbacktype"] = bson.M{"$eq": *feedbackType}
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
		cursor, err = feedbacks[n-1].ToString()
		if err != nil {
			return "", nil, err
		}
	} else {
		cursor = ""
	}
	return cursor, feedbacks, nil
}

func (db *MongoDB) GetUserItemFeedback(userId, itemId string, feedbackType *string) ([]Feedback, error) {
	startTime := time.Now()
	ctx := context.Background()
	c := db.client.Database(db.dbName).Collection("feedback")
	var filter bson.M
	if feedbackType != nil {
		filter = bson.M{
			"feedbackkey.feedbacktype": bson.M{"$eq": *feedbackType},
			"feedbackkey.userid":       bson.M{"$eq": userId},
			"feedbackkey.itemid":       bson.M{"$eq": itemId},
		}
	} else {
		filter = bson.M{
			"feedbackkey.userid": bson.M{"$eq": userId},
			"feedbackkey.itemid": bson.M{"$eq": itemId},
		}
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

func (db *MongoDB) DeleteUserItemFeedback(userId, itemId string, feedbackType *string) (int, error) {
	ctx := context.Background()
	c := db.client.Database(db.dbName).Collection("feedback")
	var filter bson.M
	if feedbackType != nil {
		filter = bson.M{
			"feedbackkey.feedbacktype": bson.M{"$eq": *feedbackType},
			"feedbackkey.userid":       bson.M{"$eq": userId},
			"feedbackkey.itemid":       bson.M{"$eq": itemId},
		}
	} else {
		filter = bson.M{
			"feedbackkey.userid": bson.M{"$eq": userId},
			"feedbackkey.itemid": bson.M{"$eq": itemId},
		}
	}
	r, err := c.DeleteMany(ctx, filter)
	if err != nil {
		return 0, err
	}
	return int(r.DeletedCount), nil
}
