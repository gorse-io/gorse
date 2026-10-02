package data

import (
	"reflect"
	"time"

	"github.com/gorse-io/gorse/config"
)

// Each backend may truncate metadata (MongoDB to milliseconds, SQL to
// microseconds); compare write bounds rather than application clock equality.
func (suite *baseTestSuite) TestUpdateAt() {
	ctx := suite.T().Context()
	field, ok := reflect.TypeFor[Item]().FieldByName("UpdateAt")
	suite.Require().True(ok, "Item must expose storage-owned UpdateAt metadata")
	suite.Equal("update_at", field.Tag.Get("mapstructure"))
	_, ok = reflect.TypeFor[User]().FieldByName("UpdateAt")
	suite.Require().True(ok, "User must expose storage-owned UpdateAt metadata")
	metadata := func(value any) time.Time {
		return reflect.ValueOf(value).FieldByName("UpdateAt").Interface().(time.Time)
	}
	setMetadata := func(value any, timestamp time.Time) {
		reflect.ValueOf(value).Elem().FieldByName("UpdateAt").Set(reflect.ValueOf(timestamp))
	}
	checkBounds := func(value any, begin time.Time) {
		updated := metadata(value)
		suite.False(updated.IsZero())
		suite.False(updated.Before(begin.Truncate(time.Second)))
		suite.False(updated.After(time.Now().UTC()))
		_, offset := updated.Zone()
		suite.Zero(offset)
	}
	businessTime := time.Date(2020, 1, 2, 3, 4, 5, 0, time.UTC)
	item := Item{ItemId: "metadata-item", Timestamp: businessTime, Comment: "searchable"}
	user := User{UserId: "metadata-user", Comment: "original"}
	callerTime := time.Date(1999, 1, 1, 0, 0, 0, 0, time.UTC)
	setMetadata(&item, callerTime)
	setMetadata(&user, callerTime)
	begin := time.Now().UTC()
	suite.Require().NoError(suite.BatchInsertItems(ctx, []Item{item}))
	suite.Require().NoError(suite.BatchInsertUsers(ctx, []User{user}))
	suite.Require().NoError(suite.Optimize())
	storedItem, err := suite.GetItem(ctx, item.ItemId)
	suite.Require().NoError(err)
	storedUser, err := suite.GetUser(ctx, user.UserId)
	suite.Require().NoError(err)
	checkBounds(storedItem, begin)
	checkBounds(storedUser, begin)
	suite.True(storedItem.Timestamp.Equal(businessTime))
	// Upserts replace caller metadata, even when all business fields are unchanged.
	previousItemTime, previousUserTime := metadata(storedItem), metadata(storedUser)
	time.Sleep(time.Second)
	begin = time.Now().UTC()
	suite.Require().NoError(suite.BatchInsertItems(ctx, []Item{item}))
	suite.Require().NoError(suite.BatchInsertUsers(ctx, []User{user}))
	suite.Require().NoError(suite.Optimize())
	storedItem, err = suite.GetItem(ctx, item.ItemId)
	suite.Require().NoError(err)
	storedUser, err = suite.GetUser(ctx, user.UserId)
	suite.Require().NoError(err)
	checkBounds(storedItem, begin)
	checkBounds(storedUser, begin)
	suite.True(metadata(storedItem).After(previousItemTime))
	suite.True(metadata(storedUser).After(previousUserTime))
	// Empty patches must not become metadata-only writes.
	suite.Require().NoError(suite.ModifyItem(ctx, item.ItemId, ItemPatch{}))
	suite.Require().NoError(suite.ModifyUser(ctx, user.UserId, UserPatch{}))
	afterItem, err := suite.GetItem(ctx, item.ItemId)
	suite.Require().NoError(err)
	afterUser, err := suite.GetUser(ctx, user.UserId)
	suite.Require().NoError(err)
	suite.Equal(storedItem, afterItem)
	suite.Equal(storedUser, afterUser)
	previousItemTime, previousUserTime = metadata(storedItem), metadata(storedUser)
	time.Sleep(time.Second)
	begin = time.Now().UTC()
	suite.Require().NoError(suite.ModifyItem(ctx, item.ItemId, ItemPatch{Categories: []string{"metadata"}}))
	suite.Require().NoError(suite.ModifyUser(ctx, user.UserId, UserPatch{Comment: new("modified")}))
	suite.Require().NoError(suite.Optimize())
	storedItem, err = suite.GetItem(ctx, item.ItemId)
	suite.Require().NoError(err)
	storedUser, err = suite.GetUser(ctx, user.UserId)
	suite.Require().NoError(err)
	checkBounds(storedItem, begin)
	checkBounds(storedUser, begin)
	suite.True(storedItem.Timestamp.Equal(businessTime))
	suite.True(metadata(storedItem).After(previousItemTime))
	suite.True(metadata(storedUser).After(previousUserTime))
	// All full-object read paths must return the same persisted metadata.
	suite.Equal([]Item{storedItem}, suite.getItems(ctx, 10))
	suite.Equal([]Item{storedItem}, suite.getItemStream(ctx, 10))
	suite.Equal([]User{storedUser}, suite.getUsers(ctx, 10))
	suite.Equal([]User{storedUser}, suite.getUsersStream(ctx, 10))
	items, err := suite.BatchGetItems(ctx, []string{item.ItemId}, GetOptions{})
	suite.Require().NoError(err)
	suite.Equal([]Item{storedItem}, items)
	items, err = suite.GetLatestItems(ctx, 10, nil, nil)
	suite.Require().NoError(err)
	suite.Equal([]Item{storedItem}, items)
	suite.Require().NoError(suite.Reconcile(config.SearchConfig{Columns: []string{"item.Comment"}}))
	results, err := suite.SearchItems(ctx, "searchable", 10)
	suite.Require().NoError(err)
	suite.Require().Len(results, 1)
	suite.Equal(storedItem, results[0].Item)
	// Feedback insertion only timestamps newly created entities.
	begin = time.Now().UTC()
	suite.Require().NoError(suite.BatchInsertFeedback(ctx, []Feedback{
		{FeedbackKey: FeedbackKey{FeedbackType: "read", UserId: user.UserId, ItemId: item.ItemId}, Timestamp: businessTime},
		{FeedbackKey: FeedbackKey{FeedbackType: "read", UserId: "implicit-user", ItemId: "implicit-item"}, Timestamp: businessTime},
	}, true, true, true))
	suite.Require().NoError(suite.Optimize())
	afterItem, err = suite.GetItem(ctx, item.ItemId)
	suite.Require().NoError(err)
	afterUser, err = suite.GetUser(ctx, user.UserId)
	suite.Require().NoError(err)
	suite.Equal(storedItem, afterItem)
	suite.Equal(storedUser, afterUser)
	implicitItem, err := suite.GetItem(ctx, "implicit-item")
	suite.Require().NoError(err)
	implicitUser, err := suite.GetUser(ctx, "implicit-user")
	suite.Require().NoError(err)
	checkBounds(implicitItem, begin)
	checkBounds(implicitUser, begin)
}

// Existing rows have no knowable last-write time. Schema upgrades use the Unix
// epoch sentinel; the next actual write assigns the current storage timestamp.
func (suite *baseTestSuite) TestUpdateAtMigration() {
	db, ok := suite.Database.(*SQLDatabase)
	if !ok {
		suite.T().Skip("SQL schema migration contract")
	}
	ctx := suite.T().Context()
	businessTime := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	suite.Require().NoError(db.BatchInsertUsers(ctx, []User{{UserId: "legacy"}}))
	suite.Require().NoError(db.BatchInsertItems(ctx, []Item{{ItemId: "legacy", Timestamp: businessTime}}))
	if db.driver == ClickHouse {
		suite.Require().NoError(db.gormDB.Exec("DROP VIEW IF EXISTS " + db.ItemsTable() + "_latest_mv").Error)
	}
	suite.Require().NoError(db.gormDB.Migrator().DropColumn(&SQLUser{}, "update_at"))
	suite.Require().NoError(db.gormDB.Migrator().DropColumn(&SQLItem{}, "update_at"))
	suite.Require().NoError(db.Init())
	suite.Require().NoError(db.Init())
	user, err := db.GetUser(ctx, "legacy")
	suite.Require().NoError(err)
	item, err := db.GetItem(ctx, "legacy")
	suite.Require().NoError(err)
	suite.True(user.UpdateAt.Equal(time.Unix(0, 0)))
	suite.True(item.UpdateAt.Equal(time.Unix(0, 0)))
	suite.True(item.Timestamp.Equal(businessTime))
	suite.Require().NoError(db.ModifyUser(ctx, "legacy", UserPatch{Comment: new("new write")}))
	suite.Require().NoError(db.ModifyItem(ctx, "legacy", ItemPatch{Comment: new("new write")}))
	user, err = db.GetUser(ctx, "legacy")
	suite.Require().NoError(err)
	item, err = db.GetItem(ctx, "legacy")
	suite.Require().NoError(err)
	suite.True(user.UpdateAt.After(time.Unix(0, 0)))
	suite.True(item.UpdateAt.After(time.Unix(0, 0)))
	suite.True(item.Timestamp.Equal(businessTime))
}
