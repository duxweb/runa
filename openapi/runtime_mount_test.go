package openapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/duxweb/runa/route"
)

type openAPIInput struct {
	ID int `path:"id"`
}

type openAPIOutput struct {
	Name string `json:"name"`
}

func TestAppOpenAPIJSONRoute(t *testing.T) {
	registry := route.New()
	runtime := New()
	runtime.Mount(registry, "api", Title("API"), JSON("/docs/openapi.json"))
	route.Get[openAPIInput, openAPIOutput](registry.RouteGroup(), "/users/{id}", func(ctx *route.Context, input *openAPIInput) (*openAPIOutput, error) {
		return &openAPIOutput{}, nil
	}).Doc("api").Name("user.show").Summary("用户详情")

	response := httptest.NewRecorder()
	registry.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/docs/openapi.json", nil))
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d", response.Code)
	}
	var document Document
	if err := json.Unmarshal(response.Body.Bytes(), &document); err != nil {
		t.Fatalf("json decode: %v", err)
	}
	if document.Info.Title != "API" {
		t.Fatalf("title = %q", document.Info.Title)
	}
	if _, ok := document.Paths["/docs/openapi.json"]; ok {
		t.Fatalf("openapi route leaked into doc: %#v", document.Paths)
	}
	if document.Paths["/users/{id}"]["get"].OperationID != "user.show" {
		t.Fatalf("paths = %#v", document.Paths)
	}
}

func TestAppOpenAPIUIRoute(t *testing.T) {
	registry := route.New()
	runtime := New()
	runtime.Mount(registry, "api", Title("Docs"), JSON("/docs/openapi.json"), UI("/docs"))

	uiResponse := httptest.NewRecorder()
	registry.Handler().ServeHTTP(uiResponse, httptest.NewRequest(http.MethodGet, "/docs", nil))
	body := uiResponse.Body.String()
	if uiResponse.Code != http.StatusOK || !strings.Contains(body, "/docs/openapi.json") || !strings.Contains(body, "@scalar/api-reference") {
		t.Fatalf("ui = %d %q", uiResponse.Code, uiResponse.Body.String())
	}
}

func TestAppOpenAPICustomViewer(t *testing.T) {
	registry := route.New()
	runtime := New()
	viewer := ViewerFunc(func(config Config, specURL string) string {
		return "custom " + config.Name + " " + specURL
	})
	runtime.Mount(registry, "api", JSON("/docs/openapi.json"), UI("/docs", viewer))

	response := httptest.NewRecorder()
	registry.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/docs", nil))
	if response.Code != http.StatusOK || response.Body.String() != "custom api /docs/openapi.json" {
		t.Fatalf("custom viewer = %d %q", response.Code, response.Body.String())
	}
}
