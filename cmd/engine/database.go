package engine

import (
	"bufio"
	"bytes"
	"database/sql"
	"fmt"
	"github.com/go-sql-driver/mysql"
	"github.com/zhenghaoz/gorse/core"
	"io"
	"log"
	"os"
	"strings"
)

// Meta data keys
const (
	Version   = "version"
	LastCount = "last_count"
)

// Database manages connections and operations on the SQL database.
type Database struct {
	connection *sql.DB
}

// NewDatabaseConnection creates a new connection to the database.
func NewDatabaseConnection(databaseDriver, dataSource string) (db Database, err error) {
	db.connection, err = sql.Open(databaseDriver, dataSource)
	return
}

// Close the connection to the database.
func (db *Database) Close() error {
	return db.connection.Close()
}

// Initialize the SQL database for gorse. Three tables will be created:
// 1. ratings: all ratings given by users to items;
// 2. items: all items will be recommended to users;
// 3. recommends: recommended items for each user.
func (db *Database) Init() error {
	queries := []string{
		// create table for ratings
		`CREATE TABLE IF NOT EXISTS ratings (
			user_id INT NOT NULL,
			item_id INT NOT NULL,
			rating FLOAT NOT NULL,
			UNIQUE KEY unique_index (user_id,item_id)
		)`,
		// create table for items
		`CREATE TABLE IF NOT EXISTS items (
			item_id INT NOT NULL UNIQUE
		)`,
		// create table for status
		`CREATE TABLE IF NOT EXISTS status (
			name CHAR(16) NOT NULL UNIQUE,
			value INT NOT NULL
		)`,
		// insert initial values
		`INSERT IGNORE INTO status VALUES ('last_count', 0);`,
		// create table for recommends
		`CREATE TABLE IF NOT EXISTS recommends (
			user_id INT NOT NULL,
			item_id INT NOT NULL,
			rating FLOAT NOT NULL,
			UNIQUE KEY unique_index (user_id,item_id)
		)`,
		// create table for neighbors
		`CREATE TABLE IF NOT EXISTS neighbors (
			item_id INT NOT NULL,
			neighbor_id INT NOT NULL,
			similarity FLOAT NOT NULL,
			UNIQUE KEY unique_index (item_id,neighbor_id)
		)`,
	}
	// Create tables
	for _, query := range queries {
		_, err := db.connection.Exec(query)
		if err != nil {
			return err
		}
	}
	return nil
}

// LoadItemsFromCSV loads items from CSV file.
func (db *Database) LoadItemsFromCSV(fileName string, sep string, header bool) error {
	// Read first line
	file, err := os.Open(fileName)
	if err != nil {
		log.Fatal(err)
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	if !scanner.Scan() {
		return fmt.Errorf("empty file")
	}
	firstLine := scanner.Text()
	// Generate columns
	fields := strings.Split(firstLine, sep)
	columns := []string{"item_id"}
	for i := 1; i < len(fields); i++ {
		columns = append(columns, "@dummy")
	}
	mysql.RegisterLocalFile(fileName)
	query := fmt.Sprintf("LOAD DATA LOCAL INFILE '%s' INTO TABLE items FIELDS TERMINATED BY '%s' (%s)",
		fileName, sep, strings.Join(columns, ","))
	// Import CSV
	_, err = db.connection.Exec(query)
	return err
}

// LoadRatingsFromCSV loads ratings from CSV file.
func (db *Database) LoadRatingsFromCSV(fileName string, sep string, header bool) error {
	// Read first line
	file, err := os.Open(fileName)
	if err != nil {
		log.Fatal(err)
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	if !scanner.Scan() {
		return fmt.Errorf("empty file")
	}
	firstLine := scanner.Text()
	// Generate columns
	fields := strings.Split(firstLine, sep)
	columns := []string{"user_id", "item_id", "rating"}
	for i := 3; i < len(fields); i++ {
		columns = append(columns, "@dummy")
	}
	mysql.RegisterLocalFile(fileName)
	query := fmt.Sprintf("LOAD DATA LOCAL INFILE '%s' INTO TABLE ratings FIELDS TERMINATED BY '%s' (%s)",
		fileName, sep, strings.Join(columns, ","))
	// Import CSV
	_, err = db.connection.Exec(query)
	if err != nil {
		return err
	}
	// Add items
	_, err = db.connection.Exec("INSERT IGNORE INTO items SELECT DISTINCT item_id FROM ratings")
	return err
}

// GetMeta gets meta data from database.
func (db *Database) GetMeta(name string) (count int, err error) {
	// Query SQL
	rows, err := db.connection.Query("SELECT value FROM status WHERE name = ?", name)
	if err != nil {
		return
	}
	// Retrieve result
	if rows.Next() {
		err = rows.Scan(&count)
		if err != nil {
			return
		}
		return
	}
	return 0, fmt.Errorf("meta data not exist")
}

// SetMeta writes meta data into database.
func (db *Database) SetMeta(name string, val int) error {
	// Prepare SQL
	statement, err := db.connection.Prepare("INSERT INTO status VALUES(?,?) ON DUPLICATE KEY UPDATE value=VALUES(value)")
	if err != nil {
		return err
	}
	// Execute SQL
	_, err = statement.Exec(name, val)
	if err != nil {
		return err
	}
	return nil
}

// RatingCount gets the number of ratings at current.
func (db *Database) RatingCount() (count int, err error) {
	rows, err := db.connection.Query("SELECT COUNT(*) FROM ratings")
	if err != nil {
		return
	}
	// Retrieve result
	if rows.Next() {
		err = rows.Scan(&count)
		if err != nil {
			return
		}
		return
	}
	panic("SELECT COUNT(*) FROM ratings failed")
}

func (db *Database) LoadData() (*core.DataSet, error) {
	return core.LoadDataFromSQL(db.connection, "ratings", "user_id", "item_id", "rating")
}

// GetRecommends gets the top list for a user from the database.
func (db *Database) GetRecommends(userId int, n int) ([]int, error) {
	// Query SQL
	rows, err := db.connection.Query(
		"SELECT item_id FROM recommends WHERE user_id=? ORDER BY rating DESC LIMIT ?", userId, n)
	if err != nil {
		return nil, err
	}
	// Retrieve result
	res := make([]int, 0)
	for rows.Next() {
		var itemId int
		err = rows.Scan(&itemId)
		if err != nil {
			return nil, err
		}
		res = append(res, itemId)
	}
	return res, nil
}

// GetRandom get random items from database.
func (db *Database) GetRandom(n int) ([]int, error) {
	// Query SQL
	rows, err := db.connection.Query("SELECT item_id FROM items ORDER BY RAND() LIMIT ?", n)
	if err != nil {
		return nil, err
	}
	// Retrieve result
	res := make([]int, 0)
	for rows.Next() {
		var itemId int
		err = rows.Scan(&itemId)
		if err != nil {
			return nil, err
		}
		res = append(res, itemId)
	}
	return res, nil
}

// GetPopular gets most popular items from database.
func (db *Database) GetPopular(n int) ([]int, []float64, error) {
	// Query SQL
	rows, err := db.connection.Query(
		`SELECT item_id, COUNT(*) AS count FROM ratings 
		GROUP BY item_id 
		ORDER BY count DESC LIMIT ?`, n)
	if err != nil {
		return nil, nil, err
	}
	// Retrieve result
	res := make([]int, 0)
	counts := make([]float64, 0)
	for rows.Next() {
		var itemId int
		var count int
		err = rows.Scan(&itemId, &count)
		if err != nil {
			return nil, nil, err
		}
		res = append(res, itemId)
		counts = append(counts, float64(count))
	}
	return res, counts, nil
}

// PutRecommends puts a top list into the database. It removes old recommendations as well.
func (db *Database) PutRecommends(userId int, items []int, ratings []float64) error {
	buf := bytes.NewBuffer(nil)
	for i, itemId := range items {
		buf.WriteString(fmt.Sprintf("%d\t%d\t%f\n", userId, itemId, ratings[i]))
	}
	mysql.RegisterReaderHandler("update_recommends", func() io.Reader {
		return bytes.NewReader(buf.Bytes())
	})
	// Begin a transaction
	tx, err := db.connection.Begin()
	if err != nil {
		return err
	}
	// Remove old recommendations
	_, err = tx.Exec("DELETE FROM recommends WHERE user_id = ?", userId)
	if err != nil {
		return err
	}
	// Update new recommendations
	_, err = tx.Exec("LOAD DATA LOCAL INFILE 'Reader::update_recommends' INTO TABLE recommends")
	if err != nil {
		return err
	}
	// Commit a transaction
	return tx.Commit()
}

// RatingTuple is the tuple of a rating record.
type RatingTuple struct {
	UserId int     // the identifier of the user
	ItemId int     // the identifier of the item
	Rating float64 // the rating
}

// PutRatings puts ratings into the database.
func (db *Database) PutRatings(ratings []RatingTuple) error {
	buf := bytes.NewBuffer(nil)
	for _, rating := range ratings {
		buf.WriteString(fmt.Sprintf("%d\t%d\t%f\n", rating.UserId, rating.ItemId, rating.Rating))
	}
	mysql.RegisterReaderHandler("put_ratings", func() io.Reader {
		return bytes.NewReader(buf.Bytes())
	})
	_, err := db.connection.Exec("LOAD DATA LOCAL INFILE 'Reader::put_ratings' INTO TABLE ratings")
	if err != nil {
		return nil
	}
	// Add items
	_, err = db.connection.Exec("INSERT IGNORE INTO items SELECT DISTINCT item_id FROM ratings")
	return err
}

// PutItems put items into database.
func (db *Database) PutItems(items []int) error {
	buf := bytes.NewBuffer(nil)
	for _, itemId := range items {
		buf.WriteString(fmt.Sprintf("%d\n", itemId))
	}
	mysql.RegisterReaderHandler("put_items", func() io.Reader {
		return bytes.NewReader(buf.Bytes())
	})
	_, err := db.connection.Exec("LOAD DATA LOCAL INFILE 'Reader::put_items' INTO TABLE items")
	return err
}

// PutNeighbors put neighbors of a item into database. It removes old neighbors as well.
func (db *Database) PutNeighbors(itemId int, neighbors []int, similarity []float64) error {
	buf := bytes.NewBuffer(nil)
	for i, neighborId := range neighbors {
		buf.WriteString(fmt.Sprintf("%d\t%d\t%f\n", itemId, neighborId, similarity[i]))
	}
	mysql.RegisterReaderHandler("update_neighbors", func() io.Reader {
		return bytes.NewReader(buf.Bytes())
	})
	// Begin a transaction
	tx, err := db.connection.Begin()
	if err != nil {
		return err
	}
	// Remove old recommendations
	_, err = tx.Exec("DELETE FROM neighbors WHERE item_id = ?", itemId)
	if err != nil {
		return err
	}
	// Update new recommendations
	_, err = tx.Exec("LOAD DATA LOCAL INFILE 'Reader::update_neighbors' INTO TABLE neighbors")
	if err != nil {
		return err
	}
	// Commit a transaction
	return tx.Commit()
}

// GetNeighbors gets neighbors of a item from database.
func (db *Database) GetNeighbors(itemId, n int) ([]int, error) {
	// Query SQL
	rows, err := db.connection.Query(
		"SELECT neighbor_id FROM neighbors WHERE item_id=? ORDER BY similarity DESC LIMIT ?", itemId, n)
	if err != nil {
		return nil, err
	}
	// Retrieve result
	res := make([]int, 0)
	for rows.Next() {
		var itemId int
		err = rows.Scan(&itemId)
		if err != nil {
			return nil, err
		}
		res = append(res, itemId)
	}
	return res, nil
}
