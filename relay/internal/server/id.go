package server

import (
	"crypto/rand"
	"encoding/hex"
)

func newSessionID() string {
	b := make([]byte, 8)
	rand.Read(b)
	return hex.EncodeToString(b)
}
