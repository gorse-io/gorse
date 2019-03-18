package engine

import (
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
			UNIQUE KEY unique_index item_id
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
