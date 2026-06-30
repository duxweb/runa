package timeout

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/duxweb/runa/route"
)

func TestTimeoutSetsRequestDeadline(t *testing.T) {
	registry := route.New()
	group := route.NewGroup(registry, "")
	group.Use(New(Config{Timeout: time.Second}))
	group.Get("/", func(ctx *route.Context) error {
		if _, ok := ctx.Request().Context().Deadline(); !ok {
			t.Fatal("deadline missing")
		}
		return ctx.Status(http.StatusOK).Text("ok")
	})

	request := httptest.NewRequest(http.MethodGet, "/", nil)
	response := httptest.NewRecorder()
	registry.Handler().ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d", response.Code)
	}
}

func TestTimeoutReturnsErrorAfterDeadline(t *testing.T) {
	registry := route.New()
	group := route.NewGroup(registry, "")
	group.Use(New(Config{Timeout: time.Millisecond, Message: "slow"}))
	group.Get("/", func(ctx *route.Context) error {
		time.Sleep(20 * time.Millisecond)
		return nil
	})

	request := httptest.NewRequest(http.MethodGet, "/", nil)
	response := httptest.NewRecorder()
	registry.Handler().ServeHTTP(response, request)

	if response.Code != http.StatusRequestTimeout {
		t.Fatalf("status = %d", response.Code)
	}
	if response.Body.String() != "slow" {
		t.Fatalf("body = %q", response.Body.String())
	}
}

func TestTimeoutPropagatesHandlerErrorBeforeWrite(t *testing.T) {
	registry := route.New()
	group := route.NewGroup(registry, "")
	group.Use(New(Config{Timeout: time.Second}))
	group.Get("/", func(ctx *route.Context) error {
		return errors.New("boom")
	})

	request := httptest.NewRequest(http.MethodGet, "/", nil)
	response := httptest.NewRecorder()
	registry.Handler().ServeHTTP(response, request)

	if response.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d body=%q", response.Code, response.Body.String())
	}
}

func TestTimeoutDropsLateWrites(t *testing.T) {
	registry := route.New()
	group := route.NewGroup(registry, "")
	group.Use(New(Config{Timeout: time.Millisecond, Message: "slow"}))
	done := make(chan struct{})
	group.Get("/", func(ctx *route.Context) error {
		time.Sleep(20 * time.Millisecond)
		_ = ctx.Text("late")
		close(done)
		return nil
	})

	request := httptest.NewRequest(http.MethodGet, "/", nil)
	response := httptest.NewRecorder()
	registry.Handler().ServeHTTP(response, request)
	<-done

	if response.Code != http.StatusRequestTimeout {
		t.Fatalf("status = %d", response.Code)
	}
	if response.Body.String() != "slow" {
		t.Fatalf("body = %q", response.Body.String())
	}
}

func TestTimeoutNextSkips(t *testing.T) {
	registry := route.New()
	group := route.NewGroup(registry, "")
	group.Use(New(Config{Next: func(*route.Context) bool { return true }, Timeout: time.Nanosecond}))
	group.Get("/", func(ctx *route.Context) error {
		if _, ok := ctx.Request().Context().Deadline(); ok {
			t.Fatal("deadline should be skipped")
		}
		return ctx.Status(http.StatusOK).Text("ok")
	})

	request := httptest.NewRequest(http.MethodGet, "/", nil)
	response := httptest.NewRecorder()
	registry.Handler().ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d", response.Code)
	}
}
