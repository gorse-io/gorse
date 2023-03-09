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

package message

import "go.mongodb.org/mongo-driver/mongo"

type MongoDB struct {
	client *mongo.Client
	dbName string
}

func (db MongoDB) Push(name string, message Message) error {
	db.client.Database(db.dbName).Collection("message").InsertOne(nil, message)
	return nil
}

func (db MongoDB) Pop(name string) (Message, error) {
	return Message{}, nil
}
