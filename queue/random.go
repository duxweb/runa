package queue

import (
	"crypto/rand"
	"encoding/hex"
)

func randomHex(byteCount int) (string, error) {
	if byteCount <= 0 {
		byteCount = 8
	}
	body := make([]byte, byteCount)
	if _, err := rand.Read(body); err != nil {
		return "", err
	}
	return hex.EncodeToString(body), nil
}
