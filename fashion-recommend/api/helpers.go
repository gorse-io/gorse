package api

import (
	"crypto/rand"
	"encoding/hex"
)

// generateSessionID 生成随机 session ID
func generateSessionID() string {
	b := make([]byte, 16)
	rand.Read(b)
	return hex.EncodeToString(b)
}
