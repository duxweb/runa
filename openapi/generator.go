package openapi

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/duxweb/runa/core"
	"github.com/duxweb/runa/route"
	"github.com/duxweb/runa/validate"
)

// Generate creates an OpenAPI document from routes.
func Generate(config Config, routes []*route.Route) Document {
	if config.Title == "" {
		config.Title = config.Name
	}
	if config.Version == "" {
		config.Version = "1.0.0"
	}
	document := Document{
		OpenAPI: "3.1.0",
		Info: Info{
			Title:       config.Title,
			Version:     config.Version,
			Description: config.Description,
		},
		Servers: config.Servers,
		Paths:   make(map[string]map[string]Operation),
		Components: Components{
			SecuritySchemes: make(map[string]SecurityScheme),
		},
	}
	for _, item := range routes {
		if item == nil || item.SkipDocument || !hasDoc(item, config.Name) {
			continue
		}
		method := strings.ToLower(item.Method)
		if method == "any" {
			method = "get"
		}
		path := item.Path
		if document.Paths[path] == nil {
			document.Paths[path] = make(map[string]Operation)
		}
		operation := buildOperation(item)
		for _, security := range item.SecurityList {
			raw := string(security)
			if raw == "" {
				continue
			}
			name := securityName(raw)
			operation.Security = append(operation.Security, map[string][]string{name: {}})
			document.Components.SecuritySchemes[name] = securityScheme(raw)
		}
		document.Paths[path][method] = operation
	}
	if len(document.Components.SecuritySchemes) == 0 {
		document.Components.SecuritySchemes = nil
	}
	return document
}

func buildOperation(item *route.Route) Operation {
	operation := Operation{
		OperationID: item.RouteName,
		Summary:     item.SummaryText,
		Description: item.DescriptionText,
		Tags:        append([]string(nil), item.TagList...),
		Deprecated:  item.DeprecatedFlag,
		Responses:   make(map[string]Response),
	}
	if item.SchemaDef != nil {
		operation.Parameters = append(operation.Parameters, parameters(item.SchemaDef.InputFields)...)
		if item.SchemaDef.RequestBody != nil {
			contentType := item.SchemaDef.RequestContentType
			if contentType == "" {
				contentType = core.MIMEApplicationJSON
			}
			operation.RequestBody = &RequestBody{
				Required: true,
				Content:  map[string]Media{contentType: {Schema: item.SchemaDef.RequestBody}},
			}
		}
	}
	status := item.SuccessStatus
	if status == 0 {
		status = http.StatusOK
	}
	response := Response{Description: http.StatusText(status)}
	if item.SchemaDef != nil && item.SchemaDef.Response != nil {
		contentType := item.SchemaDef.ResponseContentType
		if contentType == "" {
			contentType = core.MIMEApplicationJSON
		}
		response.Content = map[string]Media{contentType: {Schema: item.SchemaDef.Response}}
	}
	operation.Responses[strconv.Itoa(status)] = response
	operation.Responses["500"] = Response{Description: http.StatusText(http.StatusInternalServerError)}
	return operation
}

func parameters(fields []validate.FieldSchema) []Parameter {
	items := make([]Parameter, 0)
	for _, field := range fields {
		location := field.Source
		if location == "param" {
			location = "path"
		}
		if location != "path" && location != "query" && location != "header" && location != "cookie" {
			continue
		}
		items = append(items, Parameter{
			Name:        field.Name,
			In:          location,
			Description: field.Description,
			Required:    location == "path",
			Schema:      parameterSchema(field),
		})
	}
	return items
}

func parameterSchema(field validate.FieldSchema) *route.TypeSchema {
	schema := &route.TypeSchema{Type: field.Type, Format: field.Format, Description: field.Description}
	if schema.Type == "" {
		schema.Type = "string"
	}
	if field.Default != "" {
		schema.Default = field.Default
	}
	return schema
}

func securityScheme(name string) SecurityScheme {
	switch {
	case strings.HasPrefix(name, "basic:"):
		return SecurityScheme{Type: "http", Scheme: "basic"}
	case strings.HasPrefix(name, "apiKey:"):
		parts := strings.SplitN(strings.TrimPrefix(name, "apiKey:"), ":", 3)
		scheme := SecurityScheme{Type: "apiKey", In: "header", Name: "Authorization"}
		if len(parts) > 0 && parts[0] != "" {
			scheme.In = parts[0]
		}
		if len(parts) > 1 && parts[1] != "" {
			scheme.Name = parts[1]
		}
		return scheme
	default:
		return SecurityScheme{Type: "http", Scheme: "bearer", BearerFormat: "JWT"}
	}
}

func securityName(name string) string {
	switch {
	case strings.HasPrefix(name, "basic:"):
		return strings.TrimPrefix(name, "basic:")
	case strings.HasPrefix(name, "apiKey:"):
		parts := strings.SplitN(strings.TrimPrefix(name, "apiKey:"), ":", 3)
		if len(parts) == 3 && parts[2] != "" {
			return parts[2]
		}
		return name
	default:
		return name
	}
}

func hasDoc(item *route.Route, name string) bool {
	if len(item.DocNames) == 0 {
		return true
	}
	for _, doc := range item.DocNames {
		if doc == name {
			return true
		}
	}
	return false
}
