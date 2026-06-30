package session

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"strings"
)

// EncodeSigned signs a cookie value.
func EncodeSigned(value string, options CookieOptions) string {
	sig := sign(value, options.SignKey)
	return base64.RawURLEncoding.EncodeToString([]byte(value)) + "." + sig
}

// DecodeSigned verifies and decodes a signed cookie value.
func DecodeSigned(value string, options CookieOptions) (string, bool) {
	left, sig, ok := strings.Cut(value, ".")
	if !ok || left == "" || sig == "" {
		return "", false
	}
	body, err := base64.RawURLEncoding.DecodeString(left)
	if err != nil {
		return "", false
	}
	plain := string(body)
	if !hmac.Equal([]byte(sign(plain, options.SignKey)), []byte(sig)) {
		return "", false
	}
	return plain, true
}

// EncodeEncrypted encrypts a cookie value with AES-GCM.
func EncodeEncrypted(value string, options CookieOptions) (string, error) {
	block, err := aes.NewCipher(keyDigest(options.EncryptKey, defaultCookieKeys.encrypt))
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return "", err
	}
	ciphertext := gcm.Seal(nil, nonce, []byte(value), nil)
	payload := append(nonce, ciphertext...)
	return base64.RawURLEncoding.EncodeToString(payload), nil
}

// DecodeEncrypted decrypts an encrypted cookie value.
func DecodeEncrypted(value string, options CookieOptions) (string, bool, error) {
	payload, err := base64.RawURLEncoding.DecodeString(value)
	if err != nil {
		return "", false, nil
	}
	block, err := aes.NewCipher(keyDigest(options.EncryptKey, defaultCookieKeys.encrypt))
	if err != nil {
		return "", false, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", false, err
	}
	if len(payload) < gcm.NonceSize() {
		return "", false, nil
	}
	nonce := payload[:gcm.NonceSize()]
	ciphertext := payload[gcm.NonceSize():]
	plain, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", false, nil
	}
	return string(plain), true, nil
}

func sign(value string, key []byte) string {
	mac := hmac.New(sha256.New, keyDigest(key, defaultCookieKeys.sign))
	_, _ = mac.Write([]byte(value))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

func keyDigest(key []byte, fallback []byte) []byte {
	if len(key) == 0 {
		key = fallback
	}
	sum := sha256.Sum256(key)
	return sum[:]
}

func cryptoError(name string) error { return fmt.Errorf("session cookie %s failed", name) }
