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

import (
	"database/sql"
	"github.com/google/uuid"
	"github.com/juju/errors"
	"gorm.io/gorm"
	"io"
	"time"
)

type Row struct {
	Name      string
	Data      string
	Timestamp time.Time `gorm:"index:timestamp"`
	Id        string    `gorm:"index:timestamp"`
}

type SQLite struct {
	client *sql.DB
	gormDB *gorm.DB
}

func (db SQLite) Init() error {
	err := db.gormDB.AutoMigrate(&Row{})
	return errors.Trace(err)
}

func (db SQLite) Push(name string, message Message) error {
	return db.gormDB.Create(Row{
		Name:      name,
		Data:      message.Data,
		Timestamp: message.Timestamp,
		Id:        uuid.New().String(),
	}).Error
}

func (db SQLite) Pop(name string) (message Message, err error) {
	err = db.gormDB.Transaction(func(tx *gorm.DB) error {
		var row Row
		if err = tx.Where("name = ?", name).Order("timestamp").First(&row).Error; err != nil {
			return err
		}
		if err = tx.Where("name = ? and id = ?", name, row.Id).Delete(&row).Error; err != nil {
			return err
		}
		message = Message{
			Data:      row.Data,
			Timestamp: row.Timestamp,
		}
		return nil
	})
	if err == gorm.ErrRecordNotFound {
		err = io.EOF
	}
	return
}
