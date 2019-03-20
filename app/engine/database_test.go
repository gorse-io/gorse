package engine

import (
	"github.com/stretchr/testify/assert"
	"gopkg.in/DATA-DOG/go-sqlmock.v1"
	"testing"
)

func TestDatabase_Init(t *testing.T) {
	// Create mock database
	connection, mock, err := sqlmock.New()
	db := Database{connection: connection}
	if err != nil {
		t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
	}
	defer db.Close()
	// Create expect query
	mock.ExpectExec(`CREATE TABLE ratings \(
			user_id int NOT NULL,
			item_id int NOT NULL,
			rating int NOT NULL,
			UNIQUE KEY unique_index \(user_id,item_id\)
		\)`).WillReturnResult(sqlmock.NewResult(0, 9))
	mock.ExpectExec(`CREATE TABLE recommends \(
			user_id int NOT NULL,
			item_id int NOT NULL,
			ranking double NOT NULL,
			UNIQUE KEY unique_index \(user_id,item_id\)
		\)`).WillReturnResult(sqlmock.NewResult(0, 9))
	mock.ExpectExec(`CREATE TABLE items \(
			item_id int NOT NULL,
			UNIQUE KEY unique_index \(item_id\)
		\)`).WillReturnResult(sqlmock.NewResult(0, 9))
	// Initialize database
	if err = db.Init(); err != nil {
		t.Fatal(err)
	}
	// we make sure that all expectations were met
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("there were unfulfilled expectations: %s", err)
	}
}

func TestDatabase_GetMeta(t *testing.T) {
	// Create mock database
	connection, mock, err := sqlmock.New()
	db := Database{connection: connection}
	if err != nil {
		t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
	}
	defer db.Close()
	// Create expect query
	expectRows := sqlmock.NewRows([]string{"value"})
	expectRows.AddRow(0)
	mock.ExpectQuery("SELECT value FROM status WHERE name = ?").
		WithArgs(Version).
		WillReturnRows(expectRows)
	// Get meta data
	version, err := db.GetMeta(Version)
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, 0, version)
	// we make sure that all expectations were met
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("there were unfulfilled expectations: %s", err)
	}
}

func TestDatabase_SetMeta(t *testing.T) {
	// Create mock database
	connection, mock, err := sqlmock.New()
	db := Database{connection: connection}
	if err != nil {
		t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
	}
	defer db.Close()
	// Create expect query
	expectRows := sqlmock.NewRows([]string{"value"})
	expectRows.AddRow(0)
	mock.ExpectPrepare(`INSERT INTO status VALUES\(\?,\?\) ON DUPLICATE KEY UPDATE value=VALUES\(value\)`).
		ExpectExec().WithArgs(Version, 0).WillReturnResult(sqlmock.NewResult(0, 1))
	// Set meta data
	if err := db.SetMeta(Version, 0); err != nil {
		t.Fatal(err)
	}
	// we make sure that all expectations were met
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("there were unfulfilled expectations: %s", err)
	}
}

func TestDatabase_RatingCount(t *testing.T) {
	// Create mock database
	connection, mock, err := sqlmock.New()
	db := Database{connection: connection}
	if err != nil {
		t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
	}
	defer db.Close()
	// Create expect query
	expectRows := sqlmock.NewRows([]string{"count(*)"})
	expectRows.AddRow(100000)
	mock.ExpectQuery(`SELECT COUNT\(\*\) FROM ratings`).
		WillReturnRows(expectRows)
	// Count ratings
	count, err := db.RatingCount()
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, 100000, count)
	// we make sure that all expectations were met
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("there were unfulfilled expectations: %s", err)
	}
}

func TestDatabase_GetRecommends(t *testing.T) {
	// Create mock database
	connection, mock, err := sqlmock.New()
	db := Database{connection: connection}
	if err != nil {
		t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
	}
	defer db.Close()
	// Create expect query
	expectRows := sqlmock.NewRows([]string{"item_id"})
	expectRows.AddRow(1)
	expectRows.AddRow(2)
	expectRows.AddRow(3)
	mock.ExpectQuery(`SELECT item_id FROM recommends WHERE user_id=\? ORDER BY rating DESC`).
		WithArgs(0).
		WillReturnRows(expectRows)
	// Count ratings
	recommendations, err := db.GetRecommends(0)
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, []int{1, 2, 3}, recommendations)
	_ = recommendations
	// we make sure that all expectations were met
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("there were unfulfilled expectations: %s", err)
	}
}

func TestDatabase_GetRandom(t *testing.T) {

}

func TestDatabase_GetPopular(t *testing.T) {

}
