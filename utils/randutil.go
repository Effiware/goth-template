package utils

import (
	"crypto/rand"
	"encoding/hex"
)

func RandomString(length int) string {
	b := make([]byte, length/2+1)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
