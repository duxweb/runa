package core

import "reflect"

// CloneMap copies a Runa map.
func CloneMap(input Map) Map {
	if input == nil {
		return nil
	}
	output := make(Map, len(input))
	for key, value := range input {
		output[key] = cloneAny(value)
	}
	return output
}

// CloneStringMap deep-copies a map[string]any whose nested maps use map[string]any.
func CloneStringMap(input map[string]any) map[string]any {
	if input == nil {
		return nil
	}
	output := make(map[string]any, len(input))
	for key, value := range input {
		output[key] = cloneAny(value)
	}
	return output
}

func cloneAny(value any) any {
	switch typed := value.(type) {
	case Map:
		return CloneMap(typed)
	case map[string]any:
		return CloneStringMap(typed)
	case []any:
		items := make([]any, len(typed))
		for i, item := range typed {
			items[i] = cloneAny(item)
		}
		return items
	case []string:
		return append([]string(nil), typed...)
	case []byte:
		return append([]byte(nil), typed...)
	default:
		reflected := reflect.ValueOf(value)
		if !reflected.IsValid() {
			return value
		}
		if reflected.Kind() == reflect.Slice {
			out := reflect.MakeSlice(reflected.Type(), reflected.Len(), reflected.Len())
			reflect.Copy(out, reflected)
			return out.Interface()
		}
		return value
	}
}
