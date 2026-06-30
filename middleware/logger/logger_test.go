package logger

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	runlog "github.com/duxweb/runa/log"
	"github.com/duxweb/runa/middleware/requestid"
	"github.com/duxweb/runa/route"
)

func TestLoggerOnLogged(t *testing.T) {
	registry := route.New()
	group := route.NewGroup(registry, "")
	var entry Entry
	group.Use(requestid.New(requestid.Config{Generator: func() string { return "rid" }}))
	group.Use(New(Config{OnLogged: func(ctx *route.Context, item Entry) error {
		entry = item
		return nil
	}}))
	group.Get("/ping", func(ctx *route.Context) error {
		return ctx.Status(http.StatusCreated).Text("ok")
	})

	request := httptest.NewRequest(http.MethodGet, "/ping", nil)
	request.RemoteAddr = "127.0.0.1:1234"
	response := httptest.NewRecorder()
	registry.Handler().ServeHTTP(response, request)

	if entry.RequestID != "rid" {
		t.Fatalf("request id = %q", entry.RequestID)
	}
	if entry.Method != http.MethodGet || entry.Path != "/ping" || entry.Status != http.StatusCreated {
		t.Fatalf("entry = %#v", entry)
	}
	if entry.IP != "127.0.0.1" {
		t.Fatalf("ip = %q", entry.IP)
	}
	if entry.Latency <= 0 {
		t.Fatalf("latency = %v", entry.Latency)
	}
}

func TestLoggerCapturesError(t *testing.T) {
	registry := route.New()
	group := route.NewGroup(registry, "")
	expected := errors.New("boom")
	var entry Entry
	group.Use(New(Config{OnLogged: func(ctx *route.Context, item Entry) error {
		entry = item
		return nil
	}}))
	group.Get("/", func(ctx *route.Context) error {
		return expected
	})

	request := httptest.NewRequest(http.MethodGet, "/", nil)
	response := httptest.NewRecorder()
	registry.Handler().ServeHTTP(response, request)

	if !errors.Is(entry.Error, expected) {
		t.Fatalf("error = %v", entry.Error)
	}
	if entry.Status != http.StatusInternalServerError {
		t.Fatalf("status = %d", entry.Status)
	}
}

func TestLoggerNextSkips(t *testing.T) {
	registry := route.New()
	group := route.NewGroup(registry, "")
	called := false
	group.Use(New(Config{Next: func(*route.Context) bool { return true }, OnLogged: func(ctx *route.Context, item Entry) error {
		called = true
		return nil
	}}))
	group.Get("/", func(ctx *route.Context) error {
		return ctx.Status(http.StatusOK).Text("ok")
	})

	request := httptest.NewRequest(http.MethodGet, "/", nil)
	response := httptest.NewRecorder()
	registry.Handler().ServeHTTP(response, request)

	if called {
		t.Fatal("logger should be skipped")
	}
}

func TestLoggerSkipPaths(t *testing.T) {
	registry := route.New()
	group := route.NewGroup(registry, "")
	paths := make([]string, 0)
	group.Use(New(Config{
		SkipPaths: []string{"/favicon.ico", "/static/", "/assets/*"},
		OnLogged: func(ctx *route.Context, item Entry) error {
			paths = append(paths, item.Path)
			return nil
		},
	}))
	group.Get("/favicon.ico", func(ctx *route.Context) error { return ctx.SendStatus(http.StatusNoContent) })
	group.Get("/static/{file}", func(ctx *route.Context) error { return ctx.Text("asset") })
	group.Get("/assets/{file}", func(ctx *route.Context) error { return ctx.Text("asset") })
	group.Get("/api", func(ctx *route.Context) error { return ctx.Text("ok") })

	handler := registry.Handler()
	for _, path := range []string{"/favicon.ico", "/static/app.css", "/assets/app.js", "/api"} {
		handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, path, nil))
	}

	if len(paths) != 1 || paths[0] != "/api" {
		t.Fatalf("paths = %#v", paths)
	}
}

func TestLoggerFieldsAndSlow(t *testing.T) {
	var buffer bytes.Buffer
	registry := route.New()
	loggers := runlog.New()
	loggers.Set(runlog.HTTP, runlog.Writer(&buffer, runlog.JSON()))
	registry.Service(loggers)
	group := route.NewGroup(registry, "")
	var entry Entry
	group.Use(New(Config{
		Fields: []string{"method", "slow"},
		Slow:   time.Nanosecond,
		OnLogged: func(ctx *route.Context, item Entry) error {
			entry = item
			return nil
		},
	}))
	group.Get("/", func(ctx *route.Context) error {
		time.Sleep(time.Millisecond)
		return ctx.Text("ok")
	})

	response := httptest.NewRecorder()
	registry.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/", nil))

	if !entry.Slow {
		t.Fatalf("entry should be slow: %#v", entry)
	}
	var line map[string]any
	if err := json.Unmarshal(buffer.Bytes(), &line); err != nil {
		t.Fatalf("decode log: %v body=%q", err, buffer.String())
	}
	if line["level"] != "WARN" || line["method"] != http.MethodGet || line["slow"] != true {
		t.Fatalf("line = %#v", line)
	}
	if _, ok := line["path"]; ok {
		t.Fatalf("path should be excluded: %#v", line)
	}
}
