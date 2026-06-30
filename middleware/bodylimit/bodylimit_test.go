package bodylimit

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/duxweb/runa/route"
)

func TestBodyLimitRejectsLargeContentLength(t *testing.T) {
	registry := route.New()
	group := route.NewGroup(registry, "")
	group.Use(New(Config{Limit: 4}))
	group.Post("/", func(ctx *route.Context) error { return ctx.Status(http.StatusOK).Text("ok") })

	request := httptest.NewRequest(http.MethodPost, "/", strings.NewReader("12345"))
	response := httptest.NewRecorder()
	registry.Handler().ServeHTTP(response, request)

	if response.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d", response.Code)
	}
}

func TestBodyLimitLimitsReader(t *testing.T) {
	registry := route.New()
	group := route.NewGroup(registry, "")
	group.Use(New(Config{Limit: 4}))
	group.Post("/", func(ctx *route.Context) error {
		_, err := io.ReadAll(ctx.Request().Body)
		return err
	})

	request := httptest.NewRequest(http.MethodPost, "/", strings.NewReader("12345"))
	request.ContentLength = -1
	response := httptest.NewRecorder()
	registry.Handler().ServeHTTP(response, request)

	if response.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d", response.Code)
	}
}

func TestBodyLimitNextSkips(t *testing.T) {
	registry := route.New()
	group := route.NewGroup(registry, "")
	group.Use(New(Config{Limit: 4, Next: func(*route.Context) bool { return true }}))
	group.Post("/", func(ctx *route.Context) error { return ctx.Status(http.StatusOK).Text("ok") })

	request := httptest.NewRequest(http.MethodPost, "/", strings.NewReader("12345"))
	response := httptest.NewRecorder()
	registry.Handler().ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d", response.Code)
	}
}
