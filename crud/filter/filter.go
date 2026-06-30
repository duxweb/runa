package filter

import (
	"reflect"
	"strings"

	"github.com/duxweb/runa/core"
)

// Operator describes a filter operation.
type Operator string

const (
	EqOp      Operator = "eq"
	LikeOp    Operator = "like"
	InOp      Operator = "in"
	BetweenOp Operator = "between"
	SearchOp  Operator = "search"
	SwitchOp  Operator = "switch"
)

// Filter describes one request filter mapping.
type Filter struct {
	Name     string
	Target   string
	Operator Operator
	Type     reflect.Type
	Meta     core.Map
}

// Field sets the target store field.
func (filter Filter) Field(name string) Filter {
	filter.Target = name
	return filter
}

// MetaValue sets filter metadata.
func (filter Filter) MetaValue(key string, value any) Filter {
	if filter.Meta == nil {
		filter.Meta = make(core.Map)
	}
	filter.Meta[key] = value
	return filter
}

// Value stores a parsed filter value for the current request.
type Value struct {
	Name     string
	Target   string
	Operator Operator
	Value    any
	Meta     core.Map
}

// Eq creates an equality filter.
func Eq[T any](name string) Filter {
	return Filter{Name: name, Target: name, Operator: EqOp, Type: core.TypeOf[T]()}
}

// Like creates a like filter.
func Like(name string) Filter {
	return Filter{Name: name, Target: name, Operator: LikeOp}
}

// In creates an in filter.
func In[T any](name string) Filter {
	return Filter{Name: name, Target: name, Operator: InOp, Type: core.TypeOf[T]()}
}

// Between creates a between filter.
func Between[T any](name string) Filter {
	return Filter{Name: name, Target: name, Operator: BetweenOp, Type: core.TypeOf[T]()}
}

// Search creates a full-text search filter.
func Search(name string, fields ...string) Filter {
	return Filter{Name: name, Target: name, Operator: SearchOp, Meta: core.Map{"fields": fields}}
}

// Switch creates a mapped value filter.
func Switch[T comparable](name string, values map[T]T) Filter {
	return Filter{Name: name, Target: name, Operator: SwitchOp, Type: core.TypeOf[T](), Meta: core.Map{"values": values}}
}

// Parse converts a raw request value into a filter value.
func (filter Filter) Parse(raw string) Value {
	value := any(raw)
	switch filter.Operator {
	case InOp:
		parts := strings.Split(raw, ",")
		values := make([]any, 0, len(parts))
		for _, part := range parts {
			values = append(values, filter.cast(strings.TrimSpace(part)))
		}
		value = values
	case BetweenOp:
		parts := strings.Split(raw, ",")
		values := make([]any, 0, len(parts))
		for _, part := range parts {
			values = append(values, filter.cast(strings.TrimSpace(part)))
		}
		value = values
	case SwitchOp:
		casted := filter.cast(raw)
		value = switchValue(filter.Meta["values"], casted)
	default:
		value = filter.cast(raw)
	}
	return Value{Name: filter.Name, Target: filter.Target, Operator: filter.Operator, Value: value, Meta: filter.Meta}
}

func (filter Filter) cast(value string) any {
	if filter.Type == nil {
		return value
	}
	switch filter.Type.Kind() {
	case reflect.Int:
		return core.Cast[int](value)
	case reflect.Int64:
		return core.Cast[int64](value)
	case reflect.Bool:
		return core.Cast[bool](value)
	case reflect.Float64:
		return core.Cast[float64](value)
	case reflect.String:
		return value
	default:
		return value
	}
}

func switchValue(values any, key any) any {
	reflected := reflect.ValueOf(values)
	if reflected.Kind() != reflect.Map {
		return key
	}
	keyValue := reflect.ValueOf(key)
	if !keyValue.Type().AssignableTo(reflected.Type().Key()) {
		if keyValue.Type().ConvertibleTo(reflected.Type().Key()) {
			keyValue = keyValue.Convert(reflected.Type().Key())
		} else {
			return key
		}
	}
	value := reflected.MapIndex(keyValue)
	if !value.IsValid() {
		return key
	}
	return value.Interface()
}
