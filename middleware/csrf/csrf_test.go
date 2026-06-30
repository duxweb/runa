package csrf

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/duxweb/runa/route"
)

func TestCSRFIssuesTokenOnSafeRequest(t *testing.T) {
	registry := route.New()
	group := route.NewGroup(registry, "")
	group.Use(New())
	group.Get("/", func(ctx *route.Context) error {
		token := Token(ctx)
		if token == "" {
			t.Fatal("token is empty")
		}
		return ctx.Text(token)
	})

	response := httptest.NewRecorder()
	registry.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/", nil))

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d", response.Code)
	}
	if response.Body.String() == "" {
		t.Fatal("body token is empty")
	}
	if cookie := csrfCookie(response.Result()); cookie == nil || cookie.Value == "" {
		t.Fatalf("csrf cookie missing: %#v", cookie)
	}
}

func TestCSRFRejectsMissingToken(t *testing.T) {
	registry := newCSRFRegistry()

	response := httptest.NewRecorder()
	registry.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/submit", nil))

	if response.Code != http.StatusForbidden {
		t.Fatalf("status = %d", response.Code)
	}
}

func TestCSRFRejectsWrongHeaderToken(t *testing.T) {
	registry := newCSRFRegistry()
	request := httptest.NewRequest(http.MethodPost, "/submit", nil)
	request.AddCookie(&http.Cookie{Name: "runa_csrf_token", Value: "expected"})
	request.Header.Set("X-CSRF-Token", "wrong")

	response := httptest.NewRecorder()
	registry.Handler().ServeHTTP(response, request)

	if response.Code != http.StatusForbidden {
		t.Fatalf("status = %d", response.Code)
	}
}

func TestCSRFAcceptsHeaderToken(t *testing.T) {
	registry := newCSRFRegistry()
	request := httptest.NewRequest(http.MethodPost, "/submit", nil)
	request.AddCookie(&http.Cookie{Name: "runa_csrf_token", Value: "expected"})
	request.Header.Set("X-CSRF-Token", "expected")

	response := httptest.NewRecorder()
	registry.Handler().ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d body=%q", response.Code, response.Body.String())
	}
	if response.Body.String() != "expected" {
		t.Fatalf("body = %q", response.Body.String())
	}
}

func TestCSRFAcceptsFormToken(t *testing.T) {
	registry := newCSRFRegistry()
	form := url.Values{"_csrf": {"expected"}}
	request := httptest.NewRequest(http.MethodPost, "/submit", strings.NewReader(form.Encode()))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	request.AddCookie(&http.Cookie{Name: "runa_csrf_token", Value: "expected"})

	response := httptest.NewRecorder()
	registry.Handler().ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d body=%q", response.Code, response.Body.String())
	}
}

func TestCSRFSkipPaths(t *testing.T) {
	registry := route.New()
	group := route.NewGroup(registry, "")
	group.Use(New(SkipPaths("/webhook")))
	group.Post("/webhook", func(ctx *route.Context) error {
		return ctx.Text("ok")
	})

	response := httptest.NewRecorder()
	registry.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/webhook", nil))

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d", response.Code)
	}
}

func TestCSRFNextSkips(t *testing.T) {
	registry := route.New()
	group := route.NewGroup(registry, "")
	group.Use(New(Next(func(*route.Context) bool { return true })))
	group.Post("/submit", func(ctx *route.Context) error {
		return ctx.Text("ok")
	})

	response := httptest.NewRecorder()
	registry.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/submit", nil))

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d", response.Code)
	}
}

func newCSRFRegistry() *route.Registry {
	registry := route.New()
	group := route.NewGroup(registry, "")
	group.Use(New())
	group.Post("/submit", func(ctx *route.Context) error {
		return ctx.Text(Token(ctx))
	})
	return registry
}

func csrfCookie(response *http.Response) *http.Cookie {
	for _, cookie := range response.Cookies() {
		if cookie.Name == "runa_csrf_token" {
			return cookie
		}
	}
	return nil
}
