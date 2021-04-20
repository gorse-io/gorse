package local

import (
	"github.com/dgraph-io/badger/v2"
	"github.com/stretchr/testify/assert"
	"github.com/zhenghaoz/gorse/base"
	"os"
	"path"
	"testing"
)

type tempDir struct {
	path string
}

func createTempDir(t *testing.T) tempDir {
	dir := path.Join(os.TempDir(), base.GetRandomName(0))
	err := os.Mkdir(dir, 0777)
	assert.Nil(t, err)
	return tempDir{dir}
}

func (d *tempDir) Delete(t *testing.T) {
	assert.Nil(t, os.RemoveAll(d.path))
}

func TestDB_SetGetMeta(t *testing.T) {
	// Create database
	dir := createTempDir(t)
	defer dir.Delete(t)
	db, err := Open(dir.path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	// Set meta
	err = db.SetString(NodeName, "zhenghaoz")
	assert.Nil(t, err)
	// Get meta
	value, err := db.GetString(NodeName)
	assert.Nil(t, err)
	assert.Equal(t, "zhenghaoz", value)
	// Get meta not existed
	_, err = db.GetString("NULL")
	assert.Error(t, badger.ErrKeyNotFound)
}
