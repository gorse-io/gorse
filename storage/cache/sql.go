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
	"database/sql"
	_ "github.com/go-sql-driver/mysql"
	"github.com/juju/errors"
	_ "github.com/lib/pq"
	"github.com/samber/lo"
	"github.com/scylladb/go-set/strset"
	_ "github.com/sijms/go-ora/v2"
	"github.com/zhenghaoz/gorse/base/log"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"math"
	"moul.io/zapgorm2"
)

type SQLDriver int

const (
	MySQL SQLDriver = iota
	Postgres
	SQLite
	Oracle
)

var gormConfig = &gorm.Config{
	Logger:          zapgorm2.New(log.Logger()),
	CreateBatchSize: 1000,
}

type SQLValue struct {
	Name  string `gorm:"type:varchar(256);primaryKey"`
	Value string `gorm:"type:varchar(256);not null"`
}

func (*SQLValue) TableName() string {
	return "values"
}

type SQLSet struct {
	Name   string `gorm:"type:varchar(256);primaryKey"`
	Member string `gorm:"type:varchar(256);primaryKey"`
}

func (*SQLSet) TableName() string {
	return "sets"
}

type SQLSortedSet struct {
	Name   string  `gorm:"type:varchar(256);primaryKey;index:name"`
	Member string  `gorm:"type:varchar(256);primaryKey"`
	Score  float64 `gorm:"type:double precision;not null;index:name"`
}

func (*SQLSortedSet) TableName() string {
	return "sorted_sets"
}

type SQLDatabase struct {
	gormDB *gorm.DB
	client *sql.DB
	driver SQLDriver
}

func (db *SQLDatabase) Close() error {
	return db.client.Close()
}

func (db *SQLDatabase) Init() error {
	err := db.gormDB.AutoMigrate(&SQLValue{}, &SQLSet{}, &SQLSortedSet{})
	return errors.Trace(err)
}

func (db *SQLDatabase) Scan(work func(string) error) error {
	var (
		valuerRows *sql.Rows
		setRows    *sql.Rows
		sortedRows *sql.Rows
		err        error
	)

	// scan values
	valuerRows, err = db.gormDB.Table("values").Select("name").Rows()
	if err != nil {
		return errors.Trace(err)
	}
	defer valuerRows.Close()
	for valuerRows.Next() {
		var key string
		if err = valuerRows.Scan(&key); err != nil {
			return errors.Trace(err)
		}
		if err = work(key); err != nil {
			return errors.Trace(err)
		}
	}

	// scan sets
	setRows, err = db.gormDB.Table("sets").Select("name").Rows()
	if err != nil {
		return errors.Trace(err)
	}
	defer setRows.Close()
	var prevKey string
	for setRows.Next() {
		var key string
		if err = setRows.Scan(&key); err != nil {
			return errors.Trace(err)
		}
		if key != prevKey {
			if err = work(key); err != nil {
				return errors.Trace(err)
			}
			prevKey = key
		}
	}

	// scan sorted sets
	sortedRows, err = db.gormDB.Table("sorted_sets").Select("name").Rows()
	if err != nil {
		return errors.Trace(err)
	}
	defer sortedRows.Close()
	prevKey = ""
	for sortedRows.Next() {
		var key string
		if err = sortedRows.Scan(&key); err != nil {
			return errors.Trace(err)
		}
		if key != prevKey {
			if err = work(key); err != nil {
				return errors.Trace(err)
			}
			prevKey = key
		}
	}
	return nil
}

func (db *SQLDatabase) Set(values ...Value) error {
	if len(values) == 0 {
		return nil
	}
	valueSet := strset.New()
	rows := make([]SQLValue, 0, len(values))
	for _, value := range values {
		if !valueSet.Has(value.name) {
			rows = append(rows, SQLValue{
				Name:  value.name,
				Value: value.value,
			})
			valueSet.Add(value.name)
		}
	}
	err := db.gormDB.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "name"}},
		DoUpdates: clause.AssignmentColumns([]string{"value"}),
	}).Create(rows).Error
	return errors.Trace(err)
}

func (db *SQLDatabase) Get(name string) *ReturnValue {
	rs, err := db.gormDB.Table("values").Where("name = ?", name).Select("value").Rows()
	if err != nil {
		return &ReturnValue{err: errors.Trace(err)}
	}
	defer rs.Close()
	if rs.Next() {
		var value string
		err := rs.Scan(&value)
		if err != nil {
			return &ReturnValue{err: errors.Trace(err)}
		}
		return &ReturnValue{value: value}
	}
	return &ReturnValue{err: errors.Annotate(ErrObjectNotExist, name)}
}

func (db *SQLDatabase) Delete(name string) error {
	err := db.gormDB.Delete(&SQLValue{Name: name}).Error
	return errors.Trace(err)
}

func (db *SQLDatabase) GetSet(key string) ([]string, error) {
	rs, err := db.gormDB.Table("sets").Select("member").Where("name = ?", key).Rows()
	if err != nil {
		return nil, errors.Trace(err)
	}
	defer rs.Close()
	var members []string
	for rs.Next() {
		var member string
		if err = rs.Scan(&member); err != nil {
			return nil, errors.Trace(err)
		}
		members = append(members, member)
	}
	return members, nil
}

func (db *SQLDatabase) SetSet(key string, members ...string) error {
	err := db.gormDB.Delete(&SQLSet{}, "name = ?", key).Error
	if err != nil {
		return errors.Trace(err)
	}
	if len(members) == 0 {
		return nil
	}
	rows := lo.Map(members, func(member string, _ int) SQLSet {
		return SQLSet{
			Name:   key,
			Member: member,
		}
	})
	err = db.gormDB.Clauses(clause.OnConflict{DoNothing: true}).Create(rows).Error
	return errors.Trace(err)
}

func (db *SQLDatabase) AddSet(key string, members ...string) error {
	if len(members) == 0 {
		return nil
	}
	rows := lo.Map(members, func(member string, _ int) SQLSet {
		return SQLSet{
			Name:   key,
			Member: member,
		}
	})
	err := db.gormDB.Clauses(clause.OnConflict{DoNothing: true}).Create(rows).Error
	return errors.Trace(err)
}

func (db *SQLDatabase) RemSet(key string, members ...string) error {
	if len(members) == 0 {
		return nil
	}
	rows := lo.Map(members, func(member string, _ int) SQLSet {
		return SQLSet{
			Name:   key,
			Member: member,
		}
	})
	err := db.gormDB.Delete(rows).Error
	return errors.Trace(err)
}

func (db *SQLDatabase) AddSorted(sortedSets ...SortedSet) error {
	rows := make([]SQLSortedSet, 0, len(sortedSets))
	memberSets := make(map[lo.Tuple2[string, string]]struct{})
	for _, sortedSet := range sortedSets {
		for _, member := range sortedSet.scores {
			if _, exist := memberSets[lo.Tuple2[string, string]{sortedSet.name, member.Id}]; !exist {
				rows = append(rows, SQLSortedSet{
					Name:   sortedSet.name,
					Member: member.Id,
					Score:  member.Score,
				})
				memberSets[lo.Tuple2[string, string]{sortedSet.name, member.Id}] = struct{}{}
			}
		}
	}
	if len(rows) == 0 {
		return nil
	}
	if err := db.gormDB.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "name"}, {Name: "member"}},
		DoUpdates: clause.AssignmentColumns([]string{"score"}),
	}).Create(rows).Error; err != nil {
		return errors.Trace(err)
	}
	return nil
}

func (db *SQLDatabase) GetSorted(key string, begin, end int) ([]Scored, error) {
	tx := db.gormDB.Table("sorted_sets").Select("member, score").Where("name = ?", key).Order("score DESC")
	if end < begin {
		tx.Offset(begin).Limit(math.MaxInt64)
	} else {
		tx.Offset(begin).Limit(end - begin + 1)
	}
	rs, err := tx.Rows()
	if err != nil {
		return nil, errors.Trace(err)
	}
	defer rs.Close()
	var members []Scored
	for rs.Next() {
		var member Scored
		if err = rs.Scan(&member.Id, &member.Score); err != nil {
			return nil, errors.Trace(err)
		}
		members = append(members, member)
	}
	return members, nil
}

func (db *SQLDatabase) GetSortedByScore(key string, begin, end float64) ([]Scored, error) {
	rs, err := db.gormDB.Table("sorted_sets").
		Select("member, score").
		Where("name = ? AND score >= ? AND score <= ?", key, begin, end).
		Order("score").Rows()
	if err != nil {
		return nil, errors.Trace(err)
	}
	defer rs.Close()
	var members []Scored
	for rs.Next() {
		var member Scored
		if err = rs.Scan(&member.Id, &member.Score); err != nil {
			return nil, errors.Trace(err)
		}
		members = append(members, member)
	}
	return members, nil
}

func (db *SQLDatabase) RemSortedByScore(key string, begin, end float64) error {
	err := db.gormDB.Delete(&SortedSet{}, "name = ? AND ? <= score AND score <= ?", key, begin, end).Error
	return errors.Trace(err)
}

func (db *SQLDatabase) SetSorted(key string, scores []Scored) error {
	err := db.gormDB.Delete(&SortedSet{}, "name = ?", key).Error
	if err != nil {
		return errors.Trace(err)
	}
	if len(scores) > 0 {
		memberSets := make(map[lo.Tuple2[string, string]]struct{})
		rows := make([]SQLSortedSet, 0, len(scores))
		for _, member := range scores {
			if _, exist := memberSets[lo.Tuple2[string, string]{key, member.Id}]; !exist {
				rows = append(rows, SQLSortedSet{
					Name:   key,
					Member: member.Id,
					Score:  member.Score,
				})
				memberSets[lo.Tuple2[string, string]{key, member.Id}] = struct{}{}
			}
		}
		err = db.gormDB.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "name"}, {Name: "member"}},
			DoUpdates: clause.AssignmentColumns([]string{"score"}),
		}).Create(&rows).Error
		if err != nil {
			return errors.Trace(err)
		}
	}
	return nil
}

func (db *SQLDatabase) RemSorted(members ...SetMember) error {
	if len(members) == 0 {
		return nil
	}
	rows := lo.Map(members, func(member SetMember, _ int) SQLSortedSet {
		return SQLSortedSet{
			Name:   member.name,
			Member: member.member,
		}
	})
	err := db.gormDB.Delete(rows).Error
	return errors.Trace(err)
}
