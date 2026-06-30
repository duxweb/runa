package config

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/duxweb/runa/core"
)

// Source loads configuration values.
type Source interface {
	Load(basePath string) (map[string]any, error)
}

// FileSource loads a TOML file.
type FileSource struct {
	Path   string
	Domain string
}

// File creates a file config source.
func File(path string) FileSource {
	return FileSource{Path: path}
}

// FileDomain creates a file config source scoped under a domain.
func FileDomain(path string, domain string) FileSource {
	return FileSource{Path: path, Domain: domain}
}

// Load loads file source values.
func (source FileSource) Load(basePath string) (map[string]any, error) {
	path := source.Path
	if !filepath.IsAbs(path) {
		path = filepath.Join(basePath, path)
	}
	body, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return map[string]any{}, nil
		}
		return nil, err
	}
	values, err := parseTOML(body)
	if err != nil {
		return nil, err
	}
	domain := cleanKey(source.Domain)
	if domain == "" {
		return values, nil
	}
	return map[string]any{domain: values}, nil
}

// MapSource loads config values from a map.
type MapSource struct {
	Values map[string]any
}

// Map creates a map config source.
func Map(values map[string]any) MapSource {
	return MapSource{Values: values}
}

// Load loads map source values.
func (source MapSource) Load(string) (map[string]any, error) {
	return core.CloneStringMap(source.Values), nil
}

// EnvSource loads config values from environment variables.
type EnvSource struct {
	Prefix string
}

// Env creates an environment config source.
func Env(prefix string) EnvSource {
	return EnvSource{Prefix: prefix}
}

// Load loads environment values.
func (source EnvSource) Load(string) (map[string]any, error) {
	values := make(map[string]any)
	prefix := strings.ToUpper(strings.TrimSpace(source.Prefix))
	for _, item := range os.Environ() {
		key, value, ok := strings.Cut(item, "=")
		if !ok {
			continue
		}
		key = strings.ToUpper(strings.TrimSpace(key))
		if prefix != "" {
			if !strings.HasPrefix(key, prefix) {
				continue
			}
			key = strings.TrimPrefix(key, prefix)
		}
		key = strings.Trim(key, "_")
		if key == "" {
			continue
		}
		setEnvValue(values, envKey(key), value)
	}
	return values, nil
}

func envKey(key string) string {
	key = strings.ToLower(key)
	return strings.ReplaceAll(key, "_", ".")
}

func setEnvValue(values map[string]any, key string, value any) {
	parts := strings.Split(key, ".")
	current := values
	for _, part := range parts[:len(parts)-1] {
		next, ok := current[part].(map[string]any)
		if !ok {
			next = make(map[string]any)
			current[part] = next
		}
		current = next
	}
	current[parts[len(parts)-1]] = value
}
