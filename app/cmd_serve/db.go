package cmd_serve

import (
	"database/sql"
	_ "github.com/go-sql-driver/mysql"
	"github.com/zhenghaoz/gorse/base"
)

// PutRating puts a rating into the database.
func PutRating(db *sql.DB, userId, itemId int, rating float64) error {
	// Prepare SQL
	query := "INSERT INTO ratings VALUES(?, ?, ?) " +
		"ON DUPLICATE KEY UPDATE rating=VALUES(rating)"
	statement, err := db.Prepare(query)
	if err != nil {
		return err
	}
	// Execute SQL
	_, err = statement.Exec(userId, itemId, rating)
	if err != nil {
		return err
	}
	return nil
}

// PutTop puts a top list into the database.
func PutTop(db *sql.DB, userId int, items base.SparseVector, era int) error {
	// Prepare SQL
	query := "INSERT INTO tops VALUES(?, ?, ?, ?) " +
		"ON DUPLICATE KEY UPDATE rating=VALUES(rating), era=VALUES(era)"
	statement, err := db.Prepare(query)
	if err != nil {
		return err
	}
	// Execute SQL
	for i := 0; i < items.Len(); i++ {
		itemId := items.Indices[i]
		rating := items.Values[i]
		_, err = statement.Exec(userId, itemId, rating, era)
		if err != nil {
			return err
		}
	}
	return nil
}

// TopItem is the item in the top list.
type TopItem struct {
	ItemId int     // the item ID
	Rating float64 // the predicted rating
	Era    int     // the number of eras
}

// GetTop gets the top list for a user from the database.
func GetTop(db *sql.DB, userId int) ([]TopItem, error) {
	// Query SQL
	query := "SELECT item_id, rating, era FROM tops " +
		"WHERE user_id=? ORDER BY rating DESC;"
	rows, err := db.Query(query, userId)
	if err != nil {
		return nil, err
	}
	// Retrieve result
	res := make([]TopItem, 0)
	for rows.Next() {
		var item TopItem
		err = rows.Scan(&item.ItemId, &item.Rating, &item.Era)
		if err != nil {
			return nil, err
		}
		res = append(res, item)
	}
	return res, nil
}
