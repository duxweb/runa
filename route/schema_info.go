package route

import (
	"encoding/json"
	"reflect"
	"strings"

	"github.com/duxweb/runa/core"
	"github.com/duxweb/runa/validate"
)

// SchemaInfo stores OpenAPI-neutral route schema metadata.
type SchemaInfo struct {
	InputFields         []validate.FieldSchema
	RequestBody         *TypeSchema
	RequestContentType  string
	Response            *TypeSchema
	ResponseContentType string
}

// TypeSchema stores a minimal JSON-schema-like description.
type TypeSchema struct {
	Type        string                 `json:"type,omitempty"`
	Format      string                 `json:"format,omitempty"`
	Description string                 `json:"description,omitempty"`
	Default     any                    `json:"default,omitempty"`
	Required    []string               `json:"required,omitempty"`
	Nullable    bool                   `json:"nullable,omitempty"`
	Properties  map[string]*TypeSchema `json:"properties,omitempty"`
	Items       *TypeSchema            `json:"items,omitempty"`
	Additional  any                    `json:"additionalProperties,omitempty"`
	Ref         string                 `json:"$ref,omitempty"`
}

// TypedSchema builds route schema metadata from typed route generics.
func TypedSchema[Input any, Output any]() SchemaInfo {
	var output Output
	body, contentType := requestBodySchema[Input]()
	return SchemaInfo{
		InputFields:         InputSchema[Input](),
		RequestBody:         body,
		RequestContentType:  contentType,
		Response:            OutputSchema(output),
		ResponseContentType: core.MIMEApplicationJSON,
	}
}

// OutputSchema builds a response schema from an output value/type.
func OutputSchema(output any) *TypeSchema {
	valueType := reflect.TypeOf(output)
	if valueType == nil {
		return &TypeSchema{Type: "object", Additional: true}
	}
	for valueType.Kind() == reflect.Pointer {
		valueType = valueType.Elem()
	}
	if valueType.Kind() == reflect.Struct {
		if field, ok := valueType.FieldByName("Body"); ok && field.PkgPath == "" {
			return typeSchema(field.Type, nil)
		}
	}
	return typeSchema(valueType, nil)
}

// SchemaOf builds a schema from a Go value or reflect.Type.
func SchemaOf(value any) *TypeSchema {
	if valueType, ok := value.(reflect.Type); ok {
		if valueType == nil {
			return &TypeSchema{Type: "object", Additional: true}
		}
		return typeSchema(valueType, nil)
	}
	return OutputSchema(value)
}

func requestBodySchema[Input any]() (*TypeSchema, string) {
	var input Input
	valueType := reflect.TypeOf(input)
	if valueType == nil {
		return nil, ""
	}
	for valueType.Kind() == reflect.Pointer {
		valueType = valueType.Elem()
	}
	if valueType.Kind() != reflect.Struct {
		return nil, ""
	}
	bodyFields := make(map[string]*TypeSchema)
	contentType := core.MIMEApplicationJSON
	for i := 0; i < valueType.NumField(); i++ {
		field := valueType.Field(i)
		if field.PkgPath != "" {
			continue
		}
		source, name := schemaSource(field)
		if source != "body" && source != "form" && source != "file" {
			continue
		}
		if source == "file" {
			contentType = core.MIMEMultipartForm
		} else if source == "form" && contentType != core.MIMEMultipartForm {
			contentType = core.MIMEFormURLEncoded
		}
		if name == "" || name == "-" {
			name = jsonName(field)
		}
		if name == "" {
			name = field.Name
		}
		schema := typeSchema(field.Type, nil)
		schema.Description = firstTag(field, "desc", "description")
		if value := field.Tag.Get("default"); value != "" {
			schema.Default = value
		}
		bodyFields[name] = schema
	}
	if len(bodyFields) == 0 {
		return nil, ""
	}
	return &TypeSchema{Type: "object", Properties: bodyFields, Required: requiredFields(valueType, bodyFields)}, contentType
}

func typeSchema(valueType reflect.Type, visited map[reflect.Type]bool) *TypeSchema {
	nullable := false
	for valueType.Kind() == reflect.Pointer {
		nullable = true
		valueType = valueType.Elem()
	}
	schema := baseTypeSchema(valueType, visited)
	if nullable {
		schema.Nullable = true
	}
	return schema
}

func baseTypeSchema(valueType reflect.Type, visited map[reflect.Type]bool) *TypeSchema {
	if valueType == reflect.TypeOf(core.JSONRaw{}) || valueType == reflect.TypeOf(json.RawMessage{}) {
		return &TypeSchema{Type: "object", Additional: true}
	}
	if valueType == reflect.TypeOf(core.Map{}) {
		return &TypeSchema{Type: "object", Additional: true}
	}
	if valueType == reflect.TypeOf(core.Stream{}) {
		return &TypeSchema{Type: "string", Format: "binary"}
	}
	if valueType == reflect.TypeOf(core.UploadFile{}) {
		return &TypeSchema{Type: "string", Format: "binary"}
	}
	switch valueType.Kind() {
	case reflect.Bool:
		return &TypeSchema{Type: "boolean"}
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return integerSchema(valueType)
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return integerSchema(valueType)
	case reflect.Float32:
		return &TypeSchema{Type: "number", Format: "float"}
	case reflect.Float64:
		return &TypeSchema{Type: "number", Format: "double"}
	case reflect.String:
		return &TypeSchema{Type: "string"}
	case reflect.Slice, reflect.Array:
		if valueType.Elem().Kind() == reflect.Uint8 {
			return &TypeSchema{Type: "string", Format: "binary"}
		}
		return &TypeSchema{Type: "array", Items: typeSchema(valueType.Elem(), visited)}
	case reflect.Map:
		return &TypeSchema{Type: "object", Additional: true}
	case reflect.Struct:
		return structSchema(valueType, visited)
	case reflect.Interface:
		return &TypeSchema{Type: "object", Additional: true}
	default:
		return &TypeSchema{Type: "string"}
	}
}

func integerSchema(valueType reflect.Type) *TypeSchema {
	if valueType.Bits() <= 32 {
		return &TypeSchema{Type: "integer", Format: "int32"}
	}
	return &TypeSchema{Type: "integer", Format: "int64"}
}

func structSchema(valueType reflect.Type, visited map[reflect.Type]bool) *TypeSchema {
	if visited == nil {
		visited = make(map[reflect.Type]bool)
	}
	if visited[valueType] {
		return &TypeSchema{Type: "object", Additional: true}
	}
	visited[valueType] = true
	defer delete(visited, valueType)

	properties := make(map[string]*TypeSchema)
	for i := 0; i < valueType.NumField(); i++ {
		field := valueType.Field(i)
		if field.PkgPath != "" {
			continue
		}
		name := jsonName(field)
		if name == "-" {
			continue
		}
		if name == "" {
			name = field.Name
		}
		schema := typeSchema(field.Type, visited)
		schema.Description = firstTag(field, "desc", "description")
		if value := field.Tag.Get("default"); value != "" {
			schema.Default = value
		}
		properties[name] = schema
	}
	return &TypeSchema{Type: "object", Properties: properties, Required: requiredFields(valueType, properties)}
}

func jsonName(field reflect.StructField) string {
	value := field.Tag.Get("json")
	if value == "" {
		return ""
	}
	return strings.Split(value, ",")[0]
}

func requiredFields(valueType reflect.Type, properties map[string]*TypeSchema) []string {
	required := make([]string, 0)
	for i := 0; i < valueType.NumField(); i++ {
		field := valueType.Field(i)
		if field.PkgPath != "" || !isRequiredField(field) {
			continue
		}
		name := jsonName(field)
		if name == "-" {
			continue
		}
		if name == "" {
			name = field.Name
		}
		if _, ok := properties[name]; ok {
			required = append(required, name)
		}
	}
	return required
}

func isRequiredField(field reflect.StructField) bool {
	return field.Tag.Get("required") == "true"
}
