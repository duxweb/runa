package crud

import (
	"reflect"
	"strings"

	"github.com/duxweb/runa/core"
)

func treeItems(items []any, options TreeOptions) []any {
	if options.ID == "" {
		options.ID = "id"
	}
	if options.ParentID == "" {
		options.ParentID = "parent_id"
	}
	if options.Children == "" {
		options.Children = "children"
	}
	nodes := make([]core.Map, 0, len(items))
	byID := make(map[string]core.Map, len(items))
	for _, item := range items {
		node := mapFromItem(item)
		node[options.Children] = []any{}
		id := valueString(node[options.ID])
		nodes = append(nodes, node)
		if id != "" {
			byID[id] = node
		}
	}
	roots := make([]any, 0, len(nodes))
	for _, node := range nodes {
		parentID := valueString(node[options.ParentID])
		if parentID == "" || parentID == "0" {
			roots = append(roots, node)
			continue
		}
		parent := byID[parentID]
		if parent == nil {
			roots = append(roots, node)
			continue
		}
		children, _ := parent[options.Children].([]any)
		parent[options.Children] = append(children, node)
	}
	return roots
}

func mapFromItem(item any) core.Map {
	if item == nil {
		return core.Map{}
	}
	if value, ok := item.(core.Map); ok {
		return core.CloneMap(value)
	}
	if value, ok := item.(map[string]any); ok {
		return core.CloneMap(core.Map(value))
	}
	reflected := reflect.ValueOf(item)
	for reflected.Kind() == reflect.Pointer {
		if reflected.IsNil() {
			return core.Map{}
		}
		reflected = reflected.Elem()
	}
	if reflected.Kind() != reflect.Struct {
		return core.Map{"value": item}
	}
	reflectedType := reflected.Type()
	output := make(core.Map)
	for i := 0; i < reflected.NumField(); i++ {
		field := reflectedType.Field(i)
		if field.PkgPath != "" {
			continue
		}
		name := fieldJSONName(field)
		if name == "" {
			name = field.Name
		}
		if name == "-" {
			continue
		}
		output[name] = reflected.Field(i).Interface()
	}
	return output
}

func valueString(value any) string {
	if value == nil {
		return ""
	}
	return core.Cast[string](value)
}

func fieldJSONName(field reflect.StructField) string {
	value := field.Tag.Get("json")
	if value == "" {
		return ""
	}
	return strings.Split(value, ",")[0]
}
