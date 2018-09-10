package core

import (
	"crypto/md5"
	"fmt"
	"io"
	"log"
	"os"
	"testing"
)

func MD5Sum(fileName string) string {
	// Open file
	f, err := os.Open(fileName)
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()
	// Generate check sum
	h := md5.New()
	if _, err := io.Copy(h, f); err != nil {
		log.Fatal(err)
	}
	return fmt.Sprintf("%x", h.Sum(nil))
}

func TestDownloadFromUrl(t *testing.T) {
	// Download
	downloadFromUrl("http://files.grouplens.org/datasets/movielens/ml-100k.zip", "download")
	// Checksum
	if MD5Sum("download/ml-100k.zip") != "0e33842e24a9c977be4e0107933c0723" {
		t.Fatal("MD5 sum doesn't match")
	}
}

func TestUnzip(t *testing.T) {
	// Download
	downloadFromUrl("http://files.grouplens.org/datasets/movielens/ml-100k.zip", "download")
	// Extract files
	fileNames, err := unzip("download/ml-100k.zip", "unzip")
	// Check
	if err != nil {
		t.Fatal("unzip file failed ", err)
	}
	if len(fileNames) != 24 {
		t.Fatal("Number of file doesn't match")
	}
}
