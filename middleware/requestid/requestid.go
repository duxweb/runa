package requestid

import (
	"crypto/rand"
	"encoding/hex"
	"strings"
	"sync/atomic"
	"unicode"

	"github.com/duxweb/runa/core"
	"github.com/duxweb/runa/route"
)

const defaultHeader = "X-Request-ID"
const maxRequestIDLength = 128

var fallbackCounter uint64

// Config configures request id middleware.
type Config struct {
	Next      func(*route.Context) bool
	Header    string
	Generator func() string
}

// New creates request id middleware.
func New(configs ...Config) route.Middleware {
	config := firstConfig(configs...)
	return func(next route.Handler) route.Handler {
		return func(ctx *route.Context) error {
			if config.Next != nil && config.Next(ctx) {
				return next(ctx)
			}
			id := cleanID(route.Header[string](ctx, config.Header))
			if id == "" {
				id = cleanID(config.Generator())
			}
			if id == "" {
				id = fallbackID()
			}
			ctx.Locals(route.LocalRequestID, id)
			ctx.Response().Header().Set(config.Header, id)
			return next(ctx)
		}
	}
}

func firstConfig(configs ...Config) Config {
	config := Config{
		Header:    defaultHeader,
		Generator: generate,
	}
	if len(configs) > 0 {
		provided := configs[0]
		if provided.Next != nil {
			config.Next = provided.Next
		}
		if provided.Header != "" {
			config.Header = provided.Header
		}
		if provided.Generator != nil {
			config.Generator = provided.Generator
		}
	}
	return config
}

func generate() string {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return fallbackID()
	}
	return hex.EncodeToString(buf)
}

func cleanID(value string) string {
	value = strings.TrimSpace(value)
	if value == "" || len(value) > maxRequestIDLength {
		return ""
	}
	for _, r := range value {
		if r < 0x21 || r > 0x7e || unicode.IsControl(r) {
			return ""
		}
	}
	return value
}

func fallbackID() string {
	var buf [16]byte
	now := uint64(core.Now().UnixNano())
	seq := atomic.AddUint64(&fallbackCounter, 1)
	for i := 0; i < 8; i++ {
		buf[i] = byte(now >> (56 - i*8))
		buf[i+8] = byte(seq >> (56 - i*8))
	}
	return hex.EncodeToString(buf[:])
}
