package logger

import (
	"log/slog"
	"math"
	"slices"
	"strings"
	"time"

	runlog "github.com/duxweb/runa/log"
	"github.com/duxweb/runa/route"
)

// Entry is an HTTP access log entry.
type Entry struct {
	RequestID string
	Method    string
	Path      string
	Status    int
	IP        string
	Latency   time.Duration
	Error     error
	Slow      bool
}

// Config configures logger middleware.
type Config struct {
	Next      func(*route.Context) bool
	SkipPaths []string
	Channel   string
	Fields    []string
	Slow      time.Duration
	OnLogged  func(*route.Context, Entry) error
}

// New creates logger middleware.
func New(configs ...Config) route.Middleware {
	config := firstConfig(configs...)
	return func(next route.Handler) route.Handler {
		return func(ctx *route.Context) error {
			if ShouldSkip(ctx, config.Next, config.SkipPaths...) {
				return next(ctx)
			}
			start := time.Now()
			writer := ctx.Response()
			recorder := route.NewStatusRecorder(writer)
			ctx.SetResponse(recorder)
			defer ctx.SetResponse(writer)
			err := next(ctx)
			status := recorder.Status()
			if err != nil && !recorder.Written() {
				status = route.ErrorStatus(err)
			}
			entry := Entry{
				RequestID: ctx.RequestID(),
				Method:    ctx.Request().Method,
				Path:      ctx.Request().URL.Path,
				Status:    status,
				IP:        ctx.IP(),
				Latency:   time.Since(start),
				Error:     err,
			}
			entry.Slow = config.Slow > 0 && entry.Latency >= config.Slow
			if config.OnLogged != nil {
				if logErr := config.OnLogged(ctx, entry); logErr != nil && err == nil {
					return logErr
				}
			}
			if logs := route.Service[*runlog.Registry](ctx); logs != nil {
				logs.Get(config.Channel).LogAttrs(ctx.Request().Context(), level(entry), "http request", attrs(config, entry)...)
			}
			return err
		}
	}
}

func level(entry Entry) slog.Level {
	if entry.Status >= 500 {
		return slog.LevelError
	}
	if entry.Status >= 400 || entry.Error != nil {
		return slog.LevelWarn
	}
	if entry.Slow {
		return slog.LevelWarn
	}
	return slog.LevelInfo
}

func attrs(config Config, entry Entry) []slog.Attr {
	fields := config.Fields
	if fields == nil {
		fields = []string{"component", "request_id", "method", "path", "status", "ip", "latency_ms", "slow", "err"}
	}
	items := make([]slog.Attr, 0, len(fields))
	add := func(name string, attr slog.Attr) {
		if slices.Contains(fields, name) {
			items = append(items, attr)
		}
	}
	add("component", slog.String("component", "http"))
	add("request_id", slog.String("request_id", entry.RequestID))
	add("method", slog.String("method", entry.Method))
	add("path", slog.String("path", entry.Path))
	add("status", slog.Int("status", entry.Status))
	add("ip", slog.String("ip", entry.IP))
	add("latency_ms", slog.Float64("latency_ms", latencyMillis(entry.Latency)))
	if entry.Slow {
		add("slow", slog.Bool("slow", true))
	}
	if entry.Error != nil {
		add("err", slog.Any("err", entry.Error))
	}
	return items
}

func latencyMillis(duration time.Duration) float64 {
	return math.Round(float64(duration)/float64(time.Microsecond)) / 1000
}

func firstConfig(configs ...Config) Config {
	config := Config{Channel: runlog.HTTP}
	if len(configs) > 0 {
		provided := configs[0]
		if provided.Next != nil {
			config.Next = provided.Next
		}
		if provided.SkipPaths != nil {
			config.SkipPaths = append([]string(nil), provided.SkipPaths...)
		}
		if provided.Channel != "" {
			config.Channel = provided.Channel
		}
		if provided.Fields != nil {
			config.Fields = provided.Fields
		}
		if provided.Slow > 0 {
			config.Slow = provided.Slow
		}
		if provided.OnLogged != nil {
			config.OnLogged = provided.OnLogged
		}
	}
	return config
}

// ShouldSkip reports whether an HTTP access log should be skipped.
func ShouldSkip(ctx *route.Context, next func(*route.Context) bool, patterns ...string) bool {
	if next != nil && next(ctx) {
		return true
	}
	if len(patterns) == 0 || ctx == nil || ctx.Request() == nil {
		return false
	}
	path := ctx.Request().URL.Path
	for _, pattern := range patterns {
		if MatchPath(path, pattern) {
			return true
		}
	}
	return false
}

// MatchPath matches exact paths, directory prefixes, and simple * wildcards.
func MatchPath(path string, pattern string) bool {
	path = cleanPath(path)
	pattern = strings.TrimSpace(pattern)
	if pattern == "" {
		return false
	}
	if strings.Contains(pattern, "*") {
		return wildcardMatch(path, cleanPath(pattern))
	}
	pattern = cleanPath(pattern)
	if strings.HasSuffix(pattern, "/") {
		return path == strings.TrimSuffix(pattern, "/") || strings.HasPrefix(path, pattern)
	}
	return path == pattern
}

func cleanPath(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "/"
	}
	if !strings.HasPrefix(value, "/") {
		value = "/" + value
	}
	return value
}

func wildcardMatch(value string, pattern string) bool {
	if pattern == "*" || pattern == "/*" {
		return true
	}
	parts := strings.Split(pattern, "*")
	if len(parts) == 1 {
		return value == pattern
	}
	if parts[0] != "" && !strings.HasPrefix(value, parts[0]) {
		return false
	}
	offset := len(parts[0])
	for index := 1; index < len(parts)-1; index++ {
		part := parts[index]
		if part == "" {
			continue
		}
		found := strings.Index(value[offset:], part)
		if found < 0 {
			return false
		}
		offset += found + len(part)
	}
	last := parts[len(parts)-1]
	return last == "" || strings.HasSuffix(value, last)
}
