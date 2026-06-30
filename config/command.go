package config

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	runacommand "github.com/duxweb/runa/command"
	runaprovider "github.com/duxweb/runa/provider"
)

type showCommand struct {
	store *Store
}

func (command showCommand) Name() string    { return "config:show" }
func (command showCommand) Summary() string { return "Show merged config" }
func (command showCommand) Flags(flags *runacommand.FlagSet) {
	flags.Bool("json", false, "Output JSON")
}
func (command showCommand) Run(_ context.Context, commandCtx *runacommand.Context) error {
	if commandCtx.Get[bool]("json") {
		return commandCtx.JSON(maskSensitive(command.store.Values()))
	}
	return commandCtx.Table(configRows(command.store))
}

func configRows(store *Store) [][]string {
	rows := [][]string{{"KEY", "VALUE"}}
	if store == nil {
		return rows
	}
	for _, item := range flattenMap("", store.Values()) {
		value := formatValue(item.value)
		if sensitiveKey(item.key) {
			value = "***"
		}
		rows = append(rows, []string{item.key, value})
	}
	return rows
}

type flatItem struct {
	key   string
	value any
}

func flattenMap(prefix string, values map[string]any) []flatItem {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	items := []flatItem{}
	for _, key := range keys {
		name := key
		if prefix != "" {
			name = prefix + "." + key
		}
		if nested, ok := values[key].(map[string]any); ok {
			items = append(items, flattenMap(name, nested)...)
			continue
		}
		items = append(items, flatItem{key: name, value: values[key]})
	}
	return items
}

func formatValue(value any) string {
	switch typed := value.(type) {
	case nil:
		return ""
	case string:
		return typed
	case fmt.Stringer:
		return typed.String()
	}
	body, err := json.Marshal(value)
	if err != nil {
		return fmt.Sprint(value)
	}
	return strings.TrimSpace(string(body))
}

func sensitiveKey(key string) bool {
	key = strings.ToLower(key)
	parts := strings.FieldsFunc(key, func(r rune) bool {
		return r == '.' || r == '_' || r == '-' || r == ' '
	})
	for _, part := range parts {
		switch part {
		case "secret", "password", "passwd", "token", "apikey", "api_key", "key", "credential", "credentials":
			return true
		}
	}
	return false
}

func maskSensitive(values map[string]any) map[string]any {
	out := make(map[string]any, len(values))
	for key, value := range values {
		if sensitiveKey(key) {
			out[key] = "***"
			continue
		}
		if nested, ok := value.(map[string]any); ok {
			out[key] = maskSensitive(nested)
			continue
		}
		out[key] = value
	}
	return out
}

func registerCommand(ctx runaprovider.Context, store *Store) error {
	return ctx.RegisterCommand(showCommand{store: store})
}
