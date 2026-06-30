package config

import (
	"fmt"
	"os"
	"reflect"
	"strings"
	"sync"
	"time"

	"github.com/duxweb/runa/core"
)

// Paths resolves application paths for config placeholders.
type Paths interface {
	BasePath(paths ...string) string
	AppPath(paths ...string) string
	ConfigPath(paths ...string) string
	DataPath(paths ...string) string
	PublicPath(paths ...string) string
}

// Store stores merged configuration values.
type Store struct {
	root      *Store
	basePath  string
	paths     Paths
	prefix    string
	defaults  map[string]any
	loaded    map[string]any
	overrides map[string]any
	values    map[string]any
	sources   []Source
	mu        sync.RWMutex
}

// New creates a config store.
func New(basePath string, paths Paths) *Store {
	return &Store{
		basePath:  basePath,
		paths:     paths,
		defaults:  make(map[string]any),
		loaded:    make(map[string]any),
		overrides: make(map[string]any),
		values:    make(map[string]any),
	}
}

// Scope returns a scoped config view.
func (store *Store) Scope(name string) *Store {
	name = cleanKey(name)
	if name == "" {
		return store
	}
	root := store.rootStore()
	prefix := store.key(name)
	return &Store{root: root, basePath: root.basePath, paths: root.paths, prefix: prefix}
}

// Load registers and immediately loads config sources.
func (store *Store) Load(sources ...Source) error {
	root := store.rootStore()
	root.mu.Lock()
	root.sources = append(root.sources, sources...)
	root.mu.Unlock()
	return root.Reload()
}

// Reload reloads all registered config sources.
func (store *Store) Reload() error {
	root := store.rootStore()
	root.mu.Lock()
	defer root.mu.Unlock()
	loadedValues := make(map[string]any)
	for _, source := range root.sources {
		if source == nil {
			continue
		}
		loaded, err := source.Load(root.basePath)
		if err != nil {
			return err
		}
		merge(loadedValues, loaded, true)
	}
	root.loaded = loadedValues
	root.rebuildLocked()
	return nil
}

// Default sets a config value only when it does not exist.
func (store *Store) Default(key string, value any) error {
	root := store.rootStore()
	root.mu.Lock()
	defer root.mu.Unlock()
	key = store.key(key)
	if _, ok := lookup(root.values, key); ok {
		return nil
	}
	set(root.defaults, key, value)
	root.rebuildLocked()
	return nil
}

// Set sets a config value.
func (store *Store) Set(key string, value any) error {
	root := store.rootStore()
	root.mu.Lock()
	defer root.mu.Unlock()
	set(root.overrides, store.key(key), value)
	root.rebuildLocked()
	return nil
}

// Has reports whether a config key exists.
func (store *Store) Has(key string) bool {
	root := store.rootStore()
	root.mu.RLock()
	defer root.mu.RUnlock()
	_, ok := lookup(root.values, store.key(key))
	return ok
}

// Values returns a merged config snapshot.
func (store *Store) Values() map[string]any {
	root := store.rootStore()
	root.mu.RLock()
	defer root.mu.RUnlock()
	value, ok := lookup(root.values, store.prefix)
	if !ok {
		return map[string]any{}
	}
	if typed, ok := value.(map[string]any); ok {
		return core.CloneStringMap(typed)
	}
	return map[string]any{}
}

// Get reads a config key cast to T.
func (store *Store) Get[T any](key string, fallback ...T) T {
	root := store.rootStore()
	root.mu.RLock()
	defer root.mu.RUnlock()
	value, ok := lookup(root.values, store.key(key))
	if !ok {
		return core.Cast[T](nil, fallback...)
	}
	value = cloneAny(value)
	return core.Cast[T](value, fallback...)
}

// GetString reads a string config key.
func (store *Store) GetString(key string, fallback string) string {
	return store.Get[string](key, fallback)
}

// GetInt reads an int config key.
func (store *Store) GetInt(key string, fallback int) int {
	return store.Get[int](key, fallback)
}

// Bind binds a config subtree into target.
func (store *Store) Bind(key string, target any) error {
	root := store.rootStore()
	root.mu.RLock()
	defer root.mu.RUnlock()
	value, ok := lookup(root.values, store.key(key))
	if !ok {
		return nil
	}
	return bindValue(value, target)
}

// BindNamed binds scope.group.name into target and reports whether it exists.
func BindNamed(store *Store, scope string, group string, name string, target any) (bool, error) {
	if store == nil {
		return false, nil
	}
	key := strings.Join(cleanParts(scope, group, name), ".")
	if key == "" || !store.Has(key) {
		return false, nil
	}
	if err := store.Bind(key, target); err != nil {
		return false, err
	}
	return true, nil
}

func cleanParts(values ...string) []string {
	parts := make([]string, 0, len(values))
	for _, value := range values {
		if value = cleanKey(value); value != "" {
			parts = append(parts, value)
		}
	}
	return parts
}

func (store *Store) key(key string) string {
	key = cleanKey(key)
	if store.prefix == "" {
		return key
	}
	if key == "" {
		return store.prefix
	}
	return store.prefix + "." + key
}

func (store *Store) rootStore() *Store {
	if store.root != nil {
		return store.root
	}
	return store
}

func (store *Store) rebuildLocked() {
	values := core.CloneStringMap(store.defaults)
	merge(values, store.loaded, true)
	merge(values, store.overrides, true)
	store.values = resolveMap(values, store.paths)
}

func lookup(values map[string]any, key string) (any, bool) {
	key = cleanKey(key)
	if key == "" {
		return values, true
	}
	current := any(values)
	for _, part := range strings.Split(key, ".") {
		item, ok := current.(map[string]any)
		if !ok {
			return nil, false
		}
		current, ok = item[part]
		if !ok {
			return nil, false
		}
	}
	return current, true
}

func set(values map[string]any, key string, value any) {
	key = cleanKey(key)
	if key == "" {
		return
	}
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

func cleanKey(key string) string {
	parts := strings.Split(strings.TrimSpace(key), ".")
	cleaned := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			cleaned = append(cleaned, part)
		}
	}
	return strings.Join(cleaned, ".")
}

func merge(target map[string]any, source map[string]any, override bool) {
	for key, value := range source {
		sourceMap, sourceIsMap := value.(map[string]any)
		targetMap, targetIsMap := target[key].(map[string]any)
		if sourceIsMap && targetIsMap {
			merge(targetMap, sourceMap, override)
			continue
		}
		if override {
			target[key] = value
			continue
		}
		if _, ok := target[key]; !ok {
			target[key] = value
		}
	}
}

func cloneAny(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		return core.CloneStringMap(typed)
	case core.Map:
		return core.CloneMap(typed)
	case []byte:
		return append([]byte(nil), typed...)
	case []any:
		items := make([]any, len(typed))
		for i, item := range typed {
			items[i] = cloneAny(item)
		}
		return items
	default:
		return value
	}
}

func resolveMap(values map[string]any, paths Paths) map[string]any {
	resolved := core.CloneStringMap(values)
	for key, value := range resolved {
		switch typed := value.(type) {
		case map[string]any:
			resolved[key] = resolveMap(typed, paths)
		case string:
			resolved[key] = resolveValue(typed, paths)
		}
	}
	return resolved
}

func resolveValue(value string, paths Paths) string {
	replacers := []struct {
		prefix string
		fn     func(string) string
	}{
		{"%env(", func(name string) string { return os.Getenv(name) }},
		{"%base_path(", func(path string) string { return paths.BasePath(path) }},
		{"%app_path(", func(path string) string { return paths.AppPath(path) }},
		{"%config_path(", func(path string) string { return paths.ConfigPath(path) }},
		{"%data_path(", func(path string) string { return paths.DataPath(path) }},
		{"%public_path(", func(path string) string { return paths.PublicPath(path) }},
	}
	for _, replacer := range replacers {
		offset := 0
		for {
			index := strings.Index(value[offset:], replacer.prefix)
			if index < 0 {
				break
			}
			start := offset + index
			if start < 0 {
				break
			}
			end := strings.Index(value[start:], ")%")
			if end < 0 {
				break
			}
			end += start
			arg := value[start+len(replacer.prefix) : end]
			replacement := replacer.fn(arg)
			value = value[:start] + replacement + value[end+2:]
			offset = start + len(replacement)
		}
	}
	return value
}

func bindValue(value any, target any) error {
	targetValue := reflect.ValueOf(target)
	if targetValue.Kind() != reflect.Pointer || targetValue.IsNil() {
		return fmt.Errorf("target must be a non-nil pointer")
	}
	return bindReflect(reflect.ValueOf(value), targetValue.Elem())
}

func bindReflect(source reflect.Value, target reflect.Value) error {
	if !source.IsValid() {
		return nil
	}
	if source.Type().AssignableTo(target.Type()) {
		target.Set(cloneValue(source))
		return nil
	}
	if converted, ok := castReflect(source.Interface(), target.Type()); ok {
		target.Set(converted)
		return nil
	}
	if source.Type().ConvertibleTo(target.Type()) && source.Kind() == target.Kind() {
		target.Set(source.Convert(target.Type()))
		return nil
	}
	if target.Kind() == reflect.Struct {
		sourceMap, ok := source.Interface().(map[string]any)
		if !ok {
			return nil
		}
		for i := 0; i < target.NumField(); i++ {
			field := target.Type().Field(i)
			if field.PkgPath != "" {
				continue
			}
			name := field.Tag.Get("toml")
			if name == "" {
				name = field.Name
			}
			name = strings.Split(name, ",")[0]
			if name == "-" {
				continue
			}
			if value, ok := sourceMap[name]; ok {
				_ = bindReflect(reflect.ValueOf(value), target.Field(i))
			}
		}
		return nil
	}
	if target.Kind() == reflect.Map && target.Type().Key().Kind() == reflect.String {
		sourceMap, ok := source.Interface().(map[string]any)
		if !ok {
			return nil
		}
		output := reflect.MakeMapWithSize(target.Type(), len(sourceMap))
		for key, value := range sourceMap {
			item := reflect.New(target.Type().Elem()).Elem()
			if err := bindReflect(reflect.ValueOf(value), item); err != nil {
				return err
			}
			output.SetMapIndex(reflect.ValueOf(key).Convert(target.Type().Key()), item)
		}
		target.Set(output)
		return nil
	}
	if target.Kind() == reflect.Slice {
		if source.Kind() != reflect.Slice && source.Kind() != reflect.Array {
			return nil
		}
		output := reflect.MakeSlice(target.Type(), 0, source.Len())
		for i := 0; i < source.Len(); i++ {
			item := reflect.New(target.Type().Elem()).Elem()
			if err := bindReflect(source.Index(i), item); err != nil {
				return err
			}
			output = reflect.Append(output, item)
		}
		target.Set(output)
	}
	return nil
}

func castReflect(value any, target reflect.Type) (reflect.Value, bool) {
	if target.Kind() == reflect.Pointer {
		converted, ok := castReflect(value, target.Elem())
		if !ok {
			return reflect.Value{}, false
		}
		pointer := reflect.New(target.Elem())
		pointer.Elem().Set(converted)
		return pointer, true
	}
	if target == reflect.TypeOf(time.Duration(0)) {
		converted, ok := core.CastOK[time.Duration](value)
		return reflect.ValueOf(converted), ok
	}
	if target == reflect.TypeOf(time.Time{}) {
		converted, ok := core.CastOK[time.Time](value)
		return reflect.ValueOf(converted), ok
	}
	switch target.Kind() {
	case reflect.String:
		converted, ok := core.CastOK[string](value)
		if !ok {
			return reflect.Value{}, false
		}
		return reflect.ValueOf(converted).Convert(target), true
	case reflect.Bool:
		converted, ok := core.CastOK[bool](value)
		if !ok {
			return reflect.Value{}, false
		}
		return reflect.ValueOf(converted).Convert(target), true
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		converted, ok := core.CastOK[int64](value)
		if !ok || target.OverflowInt(converted) {
			return reflect.Value{}, false
		}
		out := reflect.New(target).Elem()
		out.SetInt(converted)
		return out, true
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		converted, ok := core.CastOK[uint64](value)
		if !ok || target.OverflowUint(converted) {
			return reflect.Value{}, false
		}
		out := reflect.New(target).Elem()
		out.SetUint(converted)
		return out, true
	case reflect.Float32, reflect.Float64:
		converted, ok := core.CastOK[float64](value)
		if !ok || target.OverflowFloat(converted) {
			return reflect.Value{}, false
		}
		out := reflect.New(target).Elem()
		out.SetFloat(converted)
		return out, true
	default:
		source := reflect.ValueOf(value)
		if !source.IsValid() {
			return reflect.Value{}, false
		}
		if source.Type().AssignableTo(target) {
			return cloneValue(source), true
		}
		return reflect.Value{}, false
	}
}

func cloneValue(value reflect.Value) reflect.Value {
	if !value.IsValid() || !value.CanInterface() {
		return value
	}
	switch typed := value.Interface().(type) {
	case map[string]any:
		return reflect.ValueOf(core.CloneStringMap(typed))
	case core.Map:
		return reflect.ValueOf(core.CloneMap(typed))
	case []byte:
		return reflect.ValueOf(append([]byte(nil), typed...))
	default:
		return value
	}
}
