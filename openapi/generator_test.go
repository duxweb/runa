package openapi

import (
	"testing"

	"github.com/duxweb/runa/route"
)

type userInput struct {
	ID    int    `path:"id" desc:"用户ID"`
	Page  int    `query:"page" default:"1"`
	Token string `header:"Authorization"`
}

type userOutput struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

func TestGenerateTypedRouteDocument(t *testing.T) {
	registry := route.New()
	group := route.NewGroup(registry, "/api").Doc("api").Tags("User").Security(Bearer("api"))
	route.Get[userInput, userOutput](group, "/users/{id}", func(ctx *route.Context, input *userInput) (*userOutput, error) {
		return &userOutput{}, nil
	}).Name("user.show").Summary("用户详情")

	document := Generate(Config{Name: "api", Title: "API", Version: "1.0.0"}, registry.Routes())
	operation, ok := document.Paths["/api/users/{id}"]["get"]
	if !ok {
		t.Fatalf("operation missing: %#v", document.Paths)
	}
	if operation.OperationID != "user.show" || operation.Summary != "用户详情" {
		t.Fatalf("operation = %#v", operation)
	}
	if len(operation.Parameters) != 3 {
		t.Fatalf("parameters = %#v", operation.Parameters)
	}
	if operation.Parameters[0].In != "path" || !operation.Parameters[0].Required {
		t.Fatalf("path parameter = %#v", operation.Parameters[0])
	}
	if operation.Parameters[0].Schema.Type != "integer" {
		t.Fatalf("path parameter schema = %#v", operation.Parameters[0].Schema)
	}
	if operation.Responses["200"].Content["application/json; charset=utf-8"].Schema.Properties["id"].Type != "integer" {
		t.Fatalf("response schema = %#v", operation.Responses["200"])
	}
	if document.Components.SecuritySchemes["api"].Scheme != "bearer" {
		t.Fatalf("security schemes = %#v", document.Components.SecuritySchemes)
	}
}

type uploadInput struct {
	Name string `form:"name"`
	File string `file:"file"`
}

type uploadOutput struct {
	OK bool `json:"ok"`
}

func TestGenerateRequestBodyContentTypesAndSecuritySchemes(t *testing.T) {
	registry := route.New()
	group := route.NewGroup(registry, "/admin").Doc("admin").Security(Basic("admin"), ApiKey("token", "header", "X-Token"))
	route.Post[uploadInput, uploadOutput](group, "/upload", func(ctx *route.Context, input *uploadInput) (*uploadOutput, error) {
		return &uploadOutput{}, nil
	}).Name("upload")

	document := Generate(Config{Name: "admin"}, registry.Routes())
	operation := document.Paths["/admin/upload"]["post"]
	if _, ok := operation.RequestBody.Content["multipart/form-data"]; !ok {
		t.Fatalf("request body = %#v", operation.RequestBody)
	}
	if document.Components.SecuritySchemes["admin"].Scheme != "basic" {
		t.Fatalf("basic scheme = %#v", document.Components.SecuritySchemes)
	}
	if document.Components.SecuritySchemes["token"].Type != "apiKey" || document.Components.SecuritySchemes["token"].Name != "X-Token" {
		t.Fatalf("api key scheme = %#v", document.Components.SecuritySchemes)
	}
}

func TestGenerateFiltersDocumentDomains(t *testing.T) {
	registry := route.New()
	group := route.NewGroup(registry, "")
	group.Get("/admin", func(ctx *route.Context) error { return nil }).Doc("admin")
	group.Get("/api", func(ctx *route.Context) error { return nil }).Doc("api")
	group.Get("/skip", func(ctx *route.Context) error { return nil }).Doc("api").SkipDoc()

	document := Generate(Config{Name: "api", Title: "API"}, registry.Routes())
	if _, ok := document.Paths["/api"]; !ok {
		t.Fatalf("api route missing: %#v", document.Paths)
	}
	if _, ok := document.Paths["/admin"]; ok {
		t.Fatalf("admin route leaked: %#v", document.Paths)
	}
	if _, ok := document.Paths["/skip"]; ok {
		t.Fatalf("skip route leaked: %#v", document.Paths)
	}
}
