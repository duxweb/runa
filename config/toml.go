package config

import "github.com/BurntSushi/toml"

func parseTOML(body []byte) (map[string]any, error) {
	values := map[string]any{}
	if err := toml.Unmarshal(body, &values); err != nil {
		return nil, err
	}
	return normalize(values).(map[string]any), nil
}

func normalize(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		values := make(map[string]any, len(typed))
		for key, item := range typed {
			values[key] = normalize(item)
		}
		return values
	case map[any]any:
		values := make(map[string]any, len(typed))
		for key, item := range typed {
			if name, ok := key.(string); ok {
				values[name] = normalize(item)
			}
		}
		return values
	case []any:
		items := make([]any, len(typed))
		for i, item := range typed {
			items[i] = normalize(item)
		}
		return items
	default:
		return value
	}
}
