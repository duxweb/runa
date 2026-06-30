package audit

import (
	"strings"

	"github.com/duxweb/runa/core"
)

// DefaultMaskFields returns default sensitive field names.
func DefaultMaskFields() []string {
	return []string{
		"password",
		"passwd",
		"pwd",
		"token",
		"access_token",
		"refresh_token",
		"secret",
		"authorization",
		"cookie",
		"set-cookie",
		"private_key",
	}
}

// Mask returns a copy of input with sensitive fields masked.
func Mask(input core.Map, fields []string, value string) core.Map {
	return maskMap(input, fields, value)
}

func maskMap(input core.Map, fields []string, value string) core.Map {
	if len(input) == 0 {
		return nil
	}
	set := make(map[string]struct{}, len(fields))
	for _, field := range fields {
		set[strings.ToLower(field)] = struct{}{}
	}
	output := make(core.Map, len(input))
	for key, item := range input {
		if _, ok := set[strings.ToLower(key)]; ok {
			output[key] = value
			continue
		}
		switch typed := item.(type) {
		case map[string]any:
			output[key] = maskMap(core.Map(typed), fields, value)
		case core.Map:
			output[key] = maskMap(typed, fields, value)
		case []any:
			items := make([]any, 0, len(typed))
			for _, child := range typed {
				if childMap, ok := child.(map[string]any); ok {
					items = append(items, maskMap(core.Map(childMap), fields, value))
					continue
				}
				items = append(items, child)
			}
			output[key] = items
		default:
			output[key] = item
		}
	}
	return output
}
