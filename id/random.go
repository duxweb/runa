package id

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"fmt"
)

// Random returns a URL-safe random string.
func Random(size int) (string, error) {
	if size <= 0 {
		size = 16
	}
	body := make([]byte, size)
	if _, err := rand.Read(body); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(body), nil
}

// MustRandom returns a URL-safe random string or panics.
func MustRandom(size int) string {
	value, err := Random(size)
	if err != nil {
		panic(err)
	}
	return value
}

// RandomHex returns a hex random string.
func RandomHex(size int) (string, error) {
	if size <= 0 {
		size = 16
	}
	body := make([]byte, size)
	if _, err := rand.Read(body); err != nil {
		return "", fmt.Errorf("read random bytes: %w", err)
	}
	return hex.EncodeToString(body), nil
}
