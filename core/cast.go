package core

import (
	"encoding/json"
	"fmt"
	"math"
	"reflect"
	"strconv"
	"strings"
	"time"
)

var (
	typeDuration = reflect.TypeOf(time.Duration(0))
	typeTime     = reflect.TypeOf(time.Time{})
	typeJSONRaw  = reflect.TypeOf(JSONRaw{})
	typeMap      = reflect.TypeOf(Map{})
	typeBytes    = reflect.TypeOf([]byte{})
)

// Cast converts value to T and returns fallback or zero value when conversion fails.
func Cast[T any](value any, defaults ...T) T {
	result, ok := CastOK[T](value)
	if ok {
		return result
	}
	if len(defaults) > 0 {
		return defaults[0]
	}
	var zero T
	return zero
}

// CastOK converts value to T and reports whether conversion succeeded.
func CastOK[T any](value any) (T, bool) {
	var zero T
	if value == nil {
		return zero, false
	}
	if typed, ok := value.(T); ok {
		return typed, true
	}

	var target T
	targetType := reflect.TypeOf(target)
	if targetType == nil {
		return zero, false
	}
	converted, ok := convertValue(reflect.ValueOf(value), targetType)
	if !ok || !converted.IsValid() {
		return zero, false
	}
	if converted.Type().AssignableTo(targetType) {
		return converted.Interface().(T), true
	}
	if converted.Type().ConvertibleTo(targetType) {
		return converted.Convert(targetType).Interface().(T), true
	}
	return zero, false
}

func convertValue(value reflect.Value, target reflect.Type) (reflect.Value, bool) {
	if !value.IsValid() {
		return reflect.Value{}, false
	}
	if value.Type().AssignableTo(target) {
		return value, true
	}
	if value.Type().ConvertibleTo(target) && isSafeDirectConvert(value.Type(), target) {
		return value.Convert(target), true
	}
	if value.Kind() == reflect.Pointer {
		if value.IsNil() {
			return reflect.Value{}, false
		}
		if target.Kind() != reflect.Pointer {
			return convertValue(value.Elem(), target)
		}
	}
	if target.Kind() == reflect.Pointer {
		converted, ok := convertValue(value, target.Elem())
		if !ok {
			return reflect.Value{}, false
		}
		ptr := reflect.New(target.Elem())
		ptr.Elem().Set(converted)
		return ptr, true
	}

	if target == typeDuration {
		v, ok := toDuration(value.Interface())
		return reflect.ValueOf(v), ok
	}
	if target == typeTime {
		v, ok := toTime(value.Interface())
		return reflect.ValueOf(v), ok
	}
	if target == typeJSONRaw {
		v, ok := toJSONRaw(value.Interface())
		return reflect.ValueOf(v), ok
	}
	if target == typeMap {
		v, ok := toMap(value.Interface())
		return reflect.ValueOf(v), ok
	}
	if target == typeBytes {
		v, ok := toBytes(value.Interface())
		return reflect.ValueOf(v), ok
	}

	switch target.Kind() {
	case reflect.String:
		return reflect.ValueOf(toString(value.Interface())).Convert(target), true
	case reflect.Bool:
		v, ok := toBool(value.Interface())
		if !ok {
			return reflect.Value{}, false
		}
		return reflect.ValueOf(v).Convert(target), true
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		v, ok := toIntForKind(value.Interface(), target.Kind())
		if !ok {
			return reflect.Value{}, false
		}
		out := reflect.New(target).Elem()
		out.SetInt(v)
		return out, true
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		v, ok := toUintForKind(value.Interface(), target.Kind())
		if !ok {
			return reflect.Value{}, false
		}
		out := reflect.New(target).Elem()
		out.SetUint(v)
		return out, true
	case reflect.Float32, reflect.Float64:
		v, ok := toFloatForKind(value.Interface(), target.Kind())
		if !ok {
			return reflect.Value{}, false
		}
		out := reflect.New(target).Elem()
		out.SetFloat(v)
		return out, true
	case reflect.Slice:
		return toSlice(value.Interface(), target)
	case reflect.Map:
		return toMapType(value.Interface(), target)
	case reflect.Struct:
		return toStruct(value.Interface(), target)
	}
	return reflect.Value{}, false
}

func isSafeDirectConvert(source reflect.Type, target reflect.Type) bool {
	if source.Kind() == target.Kind() && (source.Kind() == reflect.String || source.Kind() == reflect.Bool) {
		return true
	}
	return false
}

func toString(value any) string {
	switch typed := value.(type) {
	case []byte:
		return string(typed)
	case JSONRaw:
		return typed.String()
	case fmt.Stringer:
		return typed.String()
	default:
		return fmt.Sprint(value)
	}
}

func toBytes(value any) ([]byte, bool) {
	switch typed := value.(type) {
	case []byte:
		return append([]byte(nil), typed...), true
	case string:
		return []byte(typed), true
	case JSONRaw:
		return typed.Bytes(), true
	case json.RawMessage:
		return append([]byte(nil), typed...), true
	default:
		body, err := json.Marshal(value)
		return body, err == nil
	}
}

func toJSONRaw(value any) (JSONRaw, bool) {
	switch typed := value.(type) {
	case JSONRaw:
		return append(JSONRaw(nil), typed...), true
	case json.RawMessage:
		return append(JSONRaw(nil), typed...), true
	case []byte:
		return append(JSONRaw(nil), typed...), true
	case string:
		return JSONRaw(typed), true
	default:
		body, err := json.Marshal(value)
		return JSONRaw(body), err == nil
	}
}

func toMap(value any) (Map, bool) {
	switch typed := value.(type) {
	case Map:
		return cloneCastMap(typed), true
	case map[string]any:
		return cloneCastMap(Map(typed)), true
	case string:
		var out Map
		if err := json.Unmarshal([]byte(typed), &out); err != nil || out == nil {
			return nil, false
		}
		return out, true
	case []byte:
		var out Map
		if err := json.Unmarshal(typed, &out); err != nil || out == nil {
			return nil, false
		}
		return out, true
	case JSONRaw:
		return toMap(typed.Bytes())
	case json.RawMessage:
		return toMap([]byte(typed))
	default:
		body, err := json.Marshal(value)
		if err != nil {
			return nil, false
		}
		return toMap(body)
	}
}

func toSlice(value any, target reflect.Type) (reflect.Value, bool) {
	if target == typeBytes {
		bytes, ok := toBytes(value)
		if !ok {
			return reflect.Value{}, false
		}
		return reflect.ValueOf(bytes), true
	}
	items := []any{}
	valueReflect := reflect.ValueOf(value)
	if valueReflect.IsValid() && (valueReflect.Kind() == reflect.Slice || valueReflect.Kind() == reflect.Array) {
		items = make([]any, 0, valueReflect.Len())
		for i := 0; i < valueReflect.Len(); i++ {
			items = append(items, valueReflect.Index(i).Interface())
		}
	} else if text, ok := value.(string); ok {
		if strings.TrimSpace(text) == "" {
			items = []any{}
		} else if strings.HasPrefix(strings.TrimSpace(text), "[") {
			if err := json.Unmarshal([]byte(text), &items); err != nil {
				return reflect.Value{}, false
			}
		} else {
			parts := strings.Split(text, ",")
			items = make([]any, 0, len(parts))
			for _, part := range parts {
				items = append(items, strings.TrimSpace(part))
			}
		}
	} else if raw, ok := toJSONRaw(value); ok && len(raw) > 0 && raw[0] == '[' {
		if err := json.Unmarshal(raw, &items); err != nil {
			return reflect.Value{}, false
		}
	} else {
		return reflect.Value{}, false
	}
	out := reflect.MakeSlice(target, 0, len(items))
	for _, item := range items {
		converted, ok := convertValue(reflect.ValueOf(item), target.Elem())
		if !ok {
			return reflect.Value{}, false
		}
		out = reflect.Append(out, converted)
	}
	return out, true
}

func toMapType(value any, target reflect.Type) (reflect.Value, bool) {
	if target.Key().Kind() != reflect.String {
		return reflect.Value{}, false
	}
	base, ok := toMap(value)
	if !ok {
		return reflect.Value{}, false
	}
	out := reflect.MakeMapWithSize(target, len(base))
	for key, item := range base {
		converted, ok := convertValue(reflect.ValueOf(item), target.Elem())
		if !ok {
			return reflect.Value{}, false
		}
		out.SetMapIndex(reflect.ValueOf(key).Convert(target.Key()), converted)
	}
	return out, true
}

func toStruct(value any, target reflect.Type) (reflect.Value, bool) {
	if data, ok := toMap(value); ok {
		out := reflect.New(target).Elem()
		if !fillStruct(out, data) {
			return reflect.Value{}, false
		}
		return out, true
	}
	out := reflect.New(target)
	body, ok := toJSONRaw(value)
	if !ok {
		return reflect.Value{}, false
	}
	if err := json.Unmarshal(body, out.Interface()); err != nil {
		return reflect.Value{}, false
	}
	return out.Elem(), true
}

func fillStruct(out reflect.Value, data Map) bool {
	fields := structFields(out.Type())
	for key, value := range data {
		fieldIndex, ok := fields[strings.ToLower(key)]
		if !ok {
			continue
		}
		field := out.Field(fieldIndex)
		if !field.CanSet() {
			continue
		}
		converted, ok := convertValue(reflect.ValueOf(value), field.Type())
		if !ok {
			return false
		}
		field.Set(converted)
	}
	return true
}

func structFields(target reflect.Type) map[string]int {
	fields := make(map[string]int)
	for i := 0; i < target.NumField(); i++ {
		field := target.Field(i)
		if field.PkgPath != "" {
			continue
		}
		name := field.Name
		if tag := field.Tag.Get("json"); tag != "" {
			part := strings.Split(tag, ",")[0]
			if part == "-" {
				continue
			}
			if part != "" {
				name = part
			}
		}
		fields[strings.ToLower(name)] = i
	}
	return fields
}

func toDuration(value any) (time.Duration, bool) {
	switch typed := value.(type) {
	case time.Duration:
		return typed, true
	case string:
		parsed, err := time.ParseDuration(typed)
		if err == nil {
			return parsed, true
		}
		value, ok := toInt64(typed)
		return time.Duration(value), ok
	default:
		value, ok := toInt64(value)
		return time.Duration(value), ok
	}
}

func toTime(value any) (time.Time, bool) {
	switch typed := value.(type) {
	case time.Time:
		return typed, true
	case string:
		parsed, err := time.Parse(time.RFC3339, typed)
		return parsed, err == nil
	default:
		return time.Time{}, false
	}
}

func toIntForKind(value any, kind reflect.Kind) (int64, bool) {
	switch kind {
	case reflect.Int8:
		return toIntRange(value, math.MinInt8, math.MaxInt8)
	case reflect.Int16:
		return toIntRange(value, math.MinInt16, math.MaxInt16)
	case reflect.Int32:
		return toIntRange(value, math.MinInt32, math.MaxInt32)
	case reflect.Int64:
		return toInt64(value)
	default:
		return toIntRange(value, int64(^uint(0)>>1)*-1-1, int64(^uint(0)>>1))
	}
}

func toUintForKind(value any, kind reflect.Kind) (uint64, bool) {
	switch kind {
	case reflect.Uint8:
		return toUintRange(value, 0, math.MaxUint8)
	case reflect.Uint16:
		return toUintRange(value, 0, math.MaxUint16)
	case reflect.Uint32:
		return toUintRange(value, 0, math.MaxUint32)
	case reflect.Uint64:
		return toUint64(value)
	case reflect.Uintptr:
		return toUintRange(value, 0, math.MaxUint64)
	default:
		return toUintRange(value, 0, uint64(^uint(0)))
	}
}

func toFloatForKind(value any, kind reflect.Kind) (float64, bool) {
	parsed, ok := toFloat64(value)
	if !ok {
		return 0, false
	}
	if kind == reflect.Float32 && (parsed > math.MaxFloat32 || parsed < -math.MaxFloat32) {
		return 0, false
	}
	return parsed, true
}

func toInt(value any) (int, bool) {
	parsed, ok := toIntRange(value, int64(^uint(0)>>1)*-1-1, int64(^uint(0)>>1))
	return int(parsed), ok
}

func toIntRange(value any, min int64, max int64) (int64, bool) {
	parsed, ok := toInt64(value)
	if !ok || parsed < min || parsed > max {
		return 0, false
	}
	return parsed, true
}

func toUintRange(value any, min uint64, max uint64) (uint64, bool) {
	parsed, ok := toUint64(value)
	if !ok || parsed < min || parsed > max {
		return 0, false
	}
	return parsed, true
}

func toInt64(value any) (int64, bool) {
	switch v := value.(type) {
	case int8:
		return int64(v), true
	case int16:
		return int64(v), true
	case int32:
		return int64(v), true
	case int64:
		return v, true
	case int:
		return int64(v), true
	case uint:
		if uint64(v) > math.MaxInt64 {
			return 0, false
		}
		return int64(v), true
	case uint8:
		return int64(v), true
	case uint16:
		return int64(v), true
	case uint32:
		return int64(v), true
	case uint64:
		if v > math.MaxInt64 {
			return 0, false
		}
		return int64(v), true
	case float32:
		parsed := float64(v)
		if parsed < math.MinInt64 || parsed > math.MaxInt64 || math.Trunc(parsed) != parsed {
			return 0, false
		}
		return int64(v), true
	case float64:
		if v < math.MinInt64 || v > math.MaxInt64 || math.Trunc(v) != v {
			return 0, false
		}
		return int64(v), true
	case string:
		parsed, err := strconv.ParseInt(strings.TrimSpace(v), 10, 64)
		return parsed, err == nil
	default:
		reflected := reflect.ValueOf(value)
		if !reflected.IsValid() {
			return 0, false
		}
		switch reflected.Kind() {
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			return reflected.Int(), true
		case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
			parsed := reflected.Uint()
			if parsed > math.MaxInt64 {
				return 0, false
			}
			return int64(parsed), true
		case reflect.Float32, reflect.Float64:
			parsed := reflected.Float()
			if parsed < math.MinInt64 || parsed > math.MaxInt64 || math.Trunc(parsed) != parsed {
				return 0, false
			}
			return int64(parsed), true
		default:
			return 0, false
		}
	}
}

func toUint64(value any) (uint64, bool) {
	switch v := value.(type) {
	case uint64:
		return v, true
	case uint:
		return uint64(v), true
	case uint8:
		return uint64(v), true
	case uint16:
		return uint64(v), true
	case uint32:
		return uint64(v), true
	case int:
		if v < 0 {
			return 0, false
		}
		return uint64(v), true
	case int8:
		if v < 0 {
			return 0, false
		}
		return uint64(v), true
	case int16:
		if v < 0 {
			return 0, false
		}
		return uint64(v), true
	case int32:
		if v < 0 {
			return 0, false
		}
		return uint64(v), true
	case int64:
		if v < 0 {
			return 0, false
		}
		return uint64(v), true
	case float32:
		parsed := float64(v)
		if parsed < 0 || parsed > math.MaxUint64 || math.Trunc(parsed) != parsed {
			return 0, false
		}
		return uint64(v), true
	case float64:
		if v < 0 || v > math.MaxUint64 || math.Trunc(v) != v {
			return 0, false
		}
		return uint64(v), true
	case string:
		parsed, err := strconv.ParseUint(strings.TrimSpace(v), 10, 64)
		return parsed, err == nil
	default:
		reflected := reflect.ValueOf(value)
		if !reflected.IsValid() {
			return 0, false
		}
		switch reflected.Kind() {
		case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
			return reflected.Uint(), true
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			parsed := reflected.Int()
			if parsed < 0 {
				return 0, false
			}
			return uint64(parsed), true
		case reflect.Float32, reflect.Float64:
			parsed := reflected.Float()
			if parsed < 0 || parsed > math.MaxUint64 || math.Trunc(parsed) != parsed {
				return 0, false
			}
			return uint64(parsed), true
		default:
			return 0, false
		}
	}
}

func toBool(value any) (bool, bool) {
	switch v := value.(type) {
	case bool:
		return v, true
	case string:
		switch strings.ToLower(strings.TrimSpace(v)) {
		case "1", "t", "true", "yes", "y", "on":
			return true, true
		case "0", "f", "false", "no", "n", "off":
			return false, true
		default:
			return false, false
		}
	case int, int8, int16, int32, int64:
		parsed, ok := toInt64(v)
		return parsed != 0, ok
	case uint, uint8, uint16, uint32, uint64:
		parsed, ok := toUint64(v)
		return parsed != 0, ok
	default:
		reflected := reflect.ValueOf(value)
		if !reflected.IsValid() {
			return false, false
		}
		switch reflected.Kind() {
		case reflect.Bool:
			return reflected.Bool(), true
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			return reflected.Int() != 0, true
		case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
			return reflected.Uint() != 0, true
		default:
			return false, false
		}
	}
}

func toFloat64(value any) (float64, bool) {
	switch v := value.(type) {
	case float64:
		return v, true
	case float32:
		return float64(v), true
	case int, int8, int16, int32, int64:
		parsed, ok := toInt64(v)
		return float64(parsed), ok
	case uint, uint8, uint16, uint32, uint64:
		parsed, ok := toUint64(v)
		return float64(parsed), ok
	case string:
		parsed, err := strconv.ParseFloat(strings.TrimSpace(v), 64)
		return parsed, err == nil
	default:
		reflected := reflect.ValueOf(value)
		if !reflected.IsValid() {
			return 0, false
		}
		switch reflected.Kind() {
		case reflect.Float32, reflect.Float64:
			return reflected.Float(), true
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			return float64(reflected.Int()), true
		case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
			return float64(reflected.Uint()), true
		default:
			return 0, false
		}
	}
}

func cloneCastMap(input Map) Map {
	out := make(Map, len(input))
	for key, value := range input {
		out[key] = value
	}
	return out
}
