// Copyright 2020 gorse Project Authors
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
)

type MongoDB struct {
	client *mongo.Client
	dbName string
}

func (db *MongoDB) Init() error {
	ctx := context.Background()
	d := db.client.Database(db.dbName)
	if err := d.CreateCollection(ctx, "users"); err != nil {
		return err
	}
	if err := d.CreateCollection(ctx, "items"); err != nil {
		return err
	}
	return d.CreateCollection(ctx, "feedback")
}

func (db *MongoDB) Close() error {
	return db.client.Disconnect(context.Background())
}

func (db *MongoDB) InsertItem(item Item) error {
	ctx := context.Background()
	c := db.client.Database(db.dbName).Collection("items")
	_, err := c.InsertOne(ctx, item)
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
	_, err := c.DeleteOne(ctx, bson.M{"_id": itemId})
	c = db.client.Database(db.dbName).Collection("feedback")
	_, err = c.DeleteMany(ctx, bson.M{
		"_id.itemid": bson.M{"$eq": itemId},
	})
	return err
}

func (db *MongoDB) GetItem(itemId string) (item Item, err error) {
	ctx := context.Background()
	c := db.client.Database(db.dbName).Collection("items")
	r := c.FindOne(ctx, bson.M{"_id": itemId})
	err = r.Decode(&item)
	return
}

func (db *MongoDB) GetItems(cursor string, n int) (string, []Item, error) {
	ctx := context.Background()
	c := db.client.Database(db.dbName).Collection("items")
	opt := options.Find()
	opt.SetLimit(int64(n))
	r, err := c.Find(ctx, bson.M{"_id": bson.M{"$gt": cursor}}, opt)
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

func (db *MongoDB) GetItemFeedback(feedbackType, itemId string) ([]Feedback, error) {
	ctx := context.Background()
	c := db.client.Database(db.dbName).Collection("feedback")
	r, err := c.Find(ctx, bson.M{
		"_id.feedbacktype": bson.M{"$eq": feedbackType},
		"_id.itemid":       bson.M{"$eq": itemId},
	})
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
	return feedbacks, nil
}

func (db *MongoDB) InsertUser(user User) error {
	ctx := context.Background()
	c := db.client.Database(db.dbName).Collection("users")
	_, err := c.InsertOne(ctx, user)
	return err
}

func (db *MongoDB) DeleteUser(userId string) error {
	ctx := context.Background()
	c := db.client.Database(db.dbName).Collection("users")
	_, err := c.DeleteOne(ctx, bson.M{"_id": userId})
	c = db.client.Database(db.dbName).Collection("feedback")
	_, err = c.DeleteMany(ctx, bson.M{
		"_id.userid": bson.M{"$eq": userId},
	})
	return err
}

func (db *MongoDB) GetUser(userId string) (user User, err error) {
	ctx := context.Background()
	c := db.client.Database(db.dbName).Collection("users")
	r := c.FindOne(ctx, bson.M{"_id": userId})
	err = r.Decode(&user)
	return
}

func (db *MongoDB) GetUsers(cursor string, n int) (string, []User, error) {
	ctx := context.Background()
	c := db.client.Database(db.dbName).Collection("users")
	opt := options.Find()
	opt.SetLimit(int64(n))
	r, err := c.Find(ctx, bson.M{"_id": bson.M{"$gt": cursor}}, opt)
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

func (db *MongoDB) GetUserFeedback(feedbackType, userId string) ([]Feedback, error) {
	ctx := context.Background()
	c := db.client.Database(db.dbName).Collection("feedback")
	r, err := c.Find(ctx, bson.M{
		"_id.feedbacktype": bson.M{"$eq": feedbackType},
		"_id.userid":       bson.M{"$eq": userId},
	})
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
	return feedbacks, nil
}

func (db *MongoDB) InsertFeedback(feedback Feedback, insertUser, insertItem bool) error {
	ctx := context.Background()
	opt := options.Update()
	opt.SetUpsert(true)
	// insert feedback
	c := db.client.Database(db.dbName).Collection("feedback")
	_, err := c.UpdateOne(ctx, bson.M{"_id": feedback.FeedbackKey}, bson.M{"$set": feedback}, opt)
	if err != nil {
		return err
	}
	// insert user
	if insertUser {
		c = db.client.Database(db.dbName).Collection("users")
		_, err = c.UpdateOne(ctx, bson.M{"_id": feedback.UserId}, bson.M{"$set": bson.M{"_id": feedback.UserId}}, opt)
		if err != nil {
			return err
		}
	}
	// insert item
	if insertItem {
		c = db.client.Database(db.dbName).Collection("items")
		_, err = c.UpdateOne(ctx, bson.M{"_id": feedback.ItemId}, bson.M{"$set": bson.M{"_id": feedback.ItemId}}, opt)
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

func (db *MongoDB) GetFeedback(feedbackType, cursor string, n int) (string, []Feedback, error) {
	ctx := context.Background()
	c := db.client.Database(db.dbName).Collection("feedback")
	opt := options.Find()
	opt.SetLimit(int64(n))
	var filter bson.M
	if len(cursor) == 0 {
		filter = bson.M{
			"_id.feedbacktype": bson.M{"$eq": feedbackType},
		}
	} else {
		feedbackKey, err := FeedbackKeyFromString(cursor)
		if err != nil {
			return "", nil, err
		}
		filter = bson.M{
			"_id.feedbacktype": bson.M{"$eq": feedbackType},
			"_id":              bson.M{"$gt": feedbackKey},
		}
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
