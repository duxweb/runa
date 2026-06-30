package openapi

import "github.com/duxweb/runa/route"

// Document is a minimal OpenAPI 3.1 document.
type Document struct {
	OpenAPI    string                          `json:"openapi"`
	Info       Info                            `json:"info"`
	Servers    []ServerInfo                    `json:"servers,omitempty"`
	Paths      map[string]map[string]Operation `json:"paths"`
	Components Components                      `json:"components,omitempty"`
}

// Info describes the API.
type Info struct {
	Title       string `json:"title"`
	Version     string `json:"version"`
	Description string `json:"description,omitempty"`
}

// Operation describes an endpoint operation.
type Operation struct {
	OperationID string                `json:"operationId,omitempty"`
	Summary     string                `json:"summary,omitempty"`
	Description string                `json:"description,omitempty"`
	Tags        []string              `json:"tags,omitempty"`
	Deprecated  bool                  `json:"deprecated,omitempty"`
	Security    []map[string][]string `json:"security,omitempty"`
	Parameters  []Parameter           `json:"parameters,omitempty"`
	RequestBody *RequestBody          `json:"requestBody,omitempty"`
	Responses   map[string]Response   `json:"responses"`
}

// Parameter describes an operation parameter.
type Parameter struct {
	Name        string            `json:"name"`
	In          string            `json:"in"`
	Description string            `json:"description,omitempty"`
	Required    bool              `json:"required,omitempty"`
	Schema      *route.TypeSchema `json:"schema,omitempty"`
}

// RequestBody describes an operation request body.
type RequestBody struct {
	Required bool             `json:"required,omitempty"`
	Content  map[string]Media `json:"content"`
}

// Response describes an operation response.
type Response struct {
	Description string            `json:"description"`
	Content     map[string]Media  `json:"content,omitempty"`
	Headers     map[string]Header `json:"headers,omitempty"`
}

// Header describes a response header.
type Header struct {
	Description string            `json:"description,omitempty"`
	Schema      *route.TypeSchema `json:"schema,omitempty"`
}

// Media describes media type content.
type Media struct {
	Schema *route.TypeSchema `json:"schema,omitempty"`
}

// Components stores reusable OpenAPI components.
type Components struct {
	SecuritySchemes map[string]SecurityScheme `json:"securitySchemes,omitempty"`
}

// SecurityScheme describes a security scheme.
type SecurityScheme struct {
	Type         string `json:"type"`
	Scheme       string `json:"scheme,omitempty"`
	BearerFormat string `json:"bearerFormat,omitempty"`
	In           string `json:"in,omitempty"`
	Name         string `json:"name,omitempty"`
}
