package engine

import (
	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
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
			user_id INT NOT NULL,
			item_id INT NOT NULL,
			rating FLOAT NOT NULL,
			UNIQUE KEY unique_index \(user_id,item_id\)
		\)`).WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec(`CREATE TABLE recommends \(
			user_id INT NOT NULL,
			item_id INT NOT NULL,
			rating FLOAT NOT NULL,
			UNIQUE KEY unique_index \(user_id,item_id\)
		\)`).WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec(`CREATE TABLE items \(
			item_id INT NOT NULL,
			UNIQUE KEY unique_index \(item_id\)
		\)`).WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec(`CREATE TABLE status \(
			name CHAR\(16\) NOT NULL,
			value INT NULL,
			UNIQUE KEY unique_index \(name\)
		\)`).WillReturnResult(sqlmock.NewResult(0, 0))
	// Initialize database
	if err = db.Init(); err != nil {
		t.Fatal(err)
	}
	// we make sure that all expectations were met
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("there were unfulfilled expectations: %s", err)
	}
}

func TestDatabase_LoadItemsFromCSV(t *testing.T) {
	// Create mock database
	connection, mock, err := sqlmock.New()
	db := Database{connection: connection}
	if err != nil {
		t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
	}
	defer db.Close()
	// Create expect query
	mock.ExpectExec(`LOAD DATA INFILE '\?' INTO TABLE items FIELDS TERMINATED BY ',' \(item_id,@dummy\)`).
		WithArgs("../../example/data/import_items.csv").
		WillReturnResult(sqlmock.NewResult(0, 0))
	// Get meta data
	err = db.LoadItemsFromCSV("../../example/data/import_items.csv", ",", false)
	if err != nil {
		t.Fatal(err)
	}
	// we make sure that all expectations were met
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("there were unfulfilled expectations: %s", err)
	}
}

func TestDatabase_LoadRatingsFromCSV(t *testing.T) {
	// Create mock database
	connection, mock, err := sqlmock.New()
	db := Database{connection: connection}
	if err != nil {
		t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
	}
	defer db.Close()
	// Create expect query
	mock.ExpectExec(`LOAD DATA INFILE '\?' INTO TABLE ratings FIELDS TERMINATED BY ',' \(user_id,item_id,rating,@dummy\)`).
		WithArgs("../../example/data/import_ratings.csv").
		WillReturnResult(sqlmock.NewResult(0, 0))
	// Get meta data
	err = db.LoadRatingsFromCSV("../../example/data/import_ratings.csv", ",", false)
	if err != nil {
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
		WithArgs(0, 3).
		WillReturnRows(expectRows)
	// Count ratings
	recommendations, err := db.GetRecommends(0, 3)
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
	mock.ExpectQuery(`SELECT item_id FROM items ORDER BY RAND\(\) LIMIT ?`).
		WithArgs(3).
		WillReturnRows(expectRows)
	// Count ratings
	recommendations, err := db.GetRandom(3)
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

func TestDatabase_GetPopular(t *testing.T) {
	// Create mock database
	connection, mock, err := sqlmock.New()
	db := Database{connection: connection}
	if err != nil {
		t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
	}
	defer db.Close()
	// Create expect query
	expectRows := sqlmock.NewRows([]string{"item_id", "count"})
	expectRows.AddRow(1, 10)
	expectRows.AddRow(2, 9)
	expectRows.AddRow(3, 8)
	mock.ExpectQuery(`SELECT item_id, COUNT\(\*\) AS count FROM ratings
		GROUP BY item_id
		ORDER BY count DESC LIMIT ?`).
		WithArgs(3).
		WillReturnRows(expectRows)
	// Count ratings
	recommendations, scores, err := db.GetPopular(3)
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, []int{1, 2, 3}, recommendations)
	assert.Equal(t, []float64{10, 9, 8}, scores)
	_ = recommendations
	// we make sure that all expectations were met
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("there were unfulfilled expectations: %s", err)
	}
}

func TestDatabase_UpdateRecommends(t *testing.T) {
	// Create mock database
	connection, mock, err := sqlmock.New()
	db := Database{connection: connection}
	if err != nil {
		t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
	}
	defer db.Close()
	// Create expect query
	mock.ExpectBegin()
	mock.ExpectExec("DELETE FROM recommends WHERE user_id = ?").
		WithArgs(0).
		WillReturnResult(sqlmock.NewResult(0, 9))
	mock.ExpectExec(`LOAD DATA LOCAL INFILE 'Reader::update_recommends' INTO TABLE recommends`).
		WillReturnResult(sqlmock.NewResult(0, 9))
	mock.ExpectCommit()
	// Update recommendations
	if err := db.PutRecommends(0, []int{1, 2, 3}, []float64{1, 2, 3}); err != nil {
		t.Fatal(err)
	}
	// we make sure that all expectations were met
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("there were unfulfilled expectations: %s", err)
	}
}

func TestDatabase_PutRating(t *testing.T) {
	// Create mock database
	connection, mock, err := sqlmock.New()
	db := Database{connection: connection}
	if err != nil {
		t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
	}
	defer db.Close()
	// Create expect query
	mock.ExpectPrepare(`INSERT INTO ratings VALUES`).
		ExpectExec().
		WithArgs(0, 1, float64(2)).
		WillReturnResult(sqlmock.NewResult(0, 9))
	// Update recommendations
	if err := db.PutRating(0, 1, 2); err != nil {
		t.Fatal(err)
	}
	// we make sure that all expectations were met
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("there were unfulfilled expectations: %s", err)
	}
}
