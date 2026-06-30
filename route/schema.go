package route

import (
	"reflect"
	"strings"

	"github.com/duxweb/runa/validate"
)

// InputSchema collects binding metadata from input struct tags.
func InputSchema[T any]() []validate.FieldSchema {
	var input T
	return CollectInputSchema(&input)
}

// CollectInputSchema collects binding metadata from an input value.
func CollectInputSchema(input any) []validate.FieldSchema {
	valueType := reflect.TypeOf(input)
	for valueType.Kind() == reflect.Pointer {
		valueType = valueType.Elem()
	}
	if valueType.Kind() != reflect.Struct {
		return nil
	}
	return collectInputSchema(valueType, "", nil)
}

func collectInputSchema(valueType reflect.Type, prefix string, visited map[reflect.Type]bool) []validate.FieldSchema {
	for valueType.Kind() == reflect.Pointer {
		valueType = valueType.Elem()
	}
	if visited == nil {
		visited = make(map[reflect.Type]bool)
	}
	if visited[valueType] {
		return nil
	}
	visited[valueType] = true
	defer delete(visited, valueType)

	items := make([]validate.FieldSchema, 0)
	for i := 0; i < valueType.NumField(); i++ {
		field := valueType.Field(i)
		if field.PkgPath != "" {
			continue
		}
		fieldPath := field.Name
		if prefix != "" {
			fieldPath = prefix + "." + field.Name
		}
		fieldType := field.Type
		for fieldType.Kind() == reflect.Pointer {
			fieldType = fieldType.Elem()
		}
		if fieldType.Kind() == reflect.Struct && !hasBindingTag(field) {
			items = append(items, collectInputSchema(fieldType, fieldPath, visited)...)
			continue
		}
		source, name := schemaSource(field)
		if source == "" {
			continue
		}
		schema := typeSchema(field.Type, nil)
		items = append(items, validate.FieldSchema{
			Source:      source,
			Name:        name,
			Field:       fieldPath,
			Label:       field.Tag.Get("label"),
			Description: firstTag(field, "desc", "description"),
			Default:     field.Tag.Get("default"),
			Type:        schema.Type,
			Format:      schema.Format,
		})
	}
	return items
}

func schemaSource(field reflect.StructField) (string, string) {
	for _, source := range []string{"path", "param", "query", "header", "cookie", "form", "file", "body"} {
		name := field.Tag.Get(source)
		if name != "" && name != "-" {
			if source == "param" {
				source = "path"
			}
			return source, strings.Split(name, ",")[0]
		}
	}
	if bind := field.Tag.Get("bind"); bind != "" {
		part := strings.TrimSpace(strings.Split(bind, ",")[0])
		source, name, ok := strings.Cut(part, ":")
		if ok {
			return source, name
		}
	}
	if name := field.Tag.Get("json"); name != "" && name != "-" {
		return "body", strings.Split(name, ",")[0]
	}
	return "", ""
}

func firstTag(field reflect.StructField, names ...string) string {
	for _, name := range names {
		if value := field.Tag.Get(name); value != "" {
			return value
		}
	}
	return ""
}
