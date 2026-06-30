package validate

import (
	"fmt"
	"net/mail"
	"reflect"
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/duxweb/runa/core"
)

// Validator validates a bound input or an explicit value set.
type Validator struct {
	target any
	ctx    any
	fields []*Field
	checks []any
	errors []FieldError
}

// New creates a Validator.
func New(target any, ctx any) *Validator {
	return &Validator{target: target, ctx: ctx}
}

// Field selects a field by dotted Go struct field path.
func (validator *Validator) Field(name string) *Field {
	field := &Field{validator: validator, name: name}
	validator.fields = append(validator.fields, field)
	return field
}

// Check adds an input-level validation callback.
func (validator *Validator) Check(handler any) *Validator {
	validator.checks = append(validator.checks, handler)
	return validator
}

// AddError adds a field error.
func (validator *Validator) AddError(item FieldError) {
	validator.errors = append(validator.errors, item)
}

// Run executes all rules and callbacks.
func (validator *Validator) Run() error {
	for _, field := range validator.fields {
		field.run()
	}
	for _, check := range validator.checks {
		if err := callAny(check, validator.ctx); err != nil {
			validator.errors = append(validator.errors, FieldError{Code: "check", Message: err.Error()})
		}
	}
	if len(validator.errors) > 0 {
		return &ValidationError{Errors: validator.errors}
	}
	return nil
}

// Field defines validation rules for one field.
type Field struct {
	validator *Validator
	name      string
	value     any
	hasValue  bool
	rules     []rule
	calls     []any
}

type rule struct {
	code    string
	message string
	params  core.Map
	check   func(any) bool
}

// Value sets an explicit value for this field.
func (field *Field) Value(value any) *Field {
	field.value = value
	field.hasValue = true
	return field
}

// Required requires a non-zero value.
func (field *Field) Required(message string) *Field {
	return field.add("required", message, nil, func(value any) bool { return !isEmpty(value) })
}

// Min requires a numeric value to be greater than or equal to min.
func (field *Field) Min(min float64, message string) *Field {
	return field.add("min", message, core.Map{"min": min}, func(value any) bool {
		number, ok := toFloat(value)
		return ok && number >= min
	})
}

// Max requires a numeric value to be less than or equal to max.
func (field *Field) Max(max float64, message string) *Field {
	return field.add("max", message, core.Map{"max": max}, func(value any) bool {
		number, ok := toFloat(value)
		return ok && number <= max
	})
}

// MinLen requires a string length to be greater than or equal to min.
func (field *Field) MinLen(min int, message string) *Field {
	return field.add("min_len", message, core.Map{"min": min}, func(value any) bool {
		return utf8.RuneCountInString(fmt.Sprint(value)) >= min
	})
}

// MaxLen requires a string length to be less than or equal to max.
func (field *Field) MaxLen(max int, message string) *Field {
	return field.add("max_len", message, core.Map{"max": max}, func(value any) bool {
		return utf8.RuneCountInString(fmt.Sprint(value)) <= max
	})
}

// Email requires a valid email value.
func (field *Field) Email(message string) *Field {
	return field.add("email", message, nil, func(value any) bool {
		address, err := mail.ParseAddress(fmt.Sprint(value))
		return err == nil && address.Address == fmt.Sprint(value)
	})
}

// Regex requires a string value to match pattern.
func (field *Field) Regex(pattern string, message string) *Field {
	expr := regexp.MustCompile(pattern)
	return field.add("regex", message, core.Map{"pattern": pattern}, func(value any) bool {
		return expr.MatchString(fmt.Sprint(value))
	})
}

// Call adds a field-level callback.
func (field *Field) Call(handler any) *Field {
	field.calls = append(field.calls, handler)
	return field
}

func (field *Field) add(code string, message string, params core.Map, check func(any) bool) *Field {
	field.rules = append(field.rules, rule{code: code, message: message, params: params, check: check})
	return field
}

func (field *Field) run() {
	value := field.currentValue()
	for _, item := range field.rules {
		if !item.check(value) {
			field.validator.errors = append(field.validator.errors, FieldError{
				Field:   field.name,
				Name:    field.name,
				Code:    item.code,
				Message: item.message,
				Params:  item.params,
			})
			return
		}
	}
	for _, call := range field.calls {
		if err := callAny(call, field.validator.ctx, value); err != nil {
			field.validator.errors = append(field.validator.errors, FieldError{Field: field.name, Name: field.name, Code: "call", Message: err.Error()})
			return
		}
	}
}

func (field *Field) currentValue() any {
	if field.hasValue {
		return field.value
	}
	value, ok := fieldByPath(field.validator.target, field.name)
	if !ok {
		return nil
	}
	if !value.CanInterface() {
		return nil
	}
	return value.Interface()
}

func fieldByPath(target any, path string) (reflect.Value, bool) {
	value := reflect.ValueOf(target)
	for value.Kind() == reflect.Pointer {
		if value.IsNil() {
			return reflect.Value{}, false
		}
		value = value.Elem()
	}
	for _, name := range strings.Split(path, ".") {
		if value.Kind() != reflect.Struct {
			return reflect.Value{}, false
		}
		value = value.FieldByName(name)
		if !value.IsValid() {
			return reflect.Value{}, false
		}
		for value.Kind() == reflect.Pointer {
			if value.IsNil() {
				return reflect.Value{}, false
			}
			value = value.Elem()
		}
	}
	return value, true
}

func isEmpty(value any) bool {
	if value == nil {
		return true
	}
	reflected := reflect.ValueOf(value)
	switch reflected.Kind() {
	case reflect.String, reflect.Array, reflect.Slice, reflect.Map:
		return reflected.Len() == 0
	case reflect.Bool:
		return !reflected.Bool()
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return reflected.Int() == 0
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		return reflected.Uint() == 0
	case reflect.Float32, reflect.Float64:
		return reflected.Float() == 0
	case reflect.Pointer, reflect.Interface:
		return reflected.IsNil()
	}
	return reflected.IsZero()
}

func toFloat(value any) (float64, bool) {
	switch typed := value.(type) {
	case int:
		return float64(typed), true
	case int8:
		return float64(typed), true
	case int16:
		return float64(typed), true
	case int32:
		return float64(typed), true
	case int64:
		return float64(typed), true
	case uint:
		return float64(typed), true
	case uint8:
		return float64(typed), true
	case uint16:
		return float64(typed), true
	case uint32:
		return float64(typed), true
	case uint64:
		return float64(typed), true
	case float32:
		return float64(typed), true
	case float64:
		return typed, true
	case string:
		return core.CastOK[float64](typed)
	default:
		return 0, false
	}
}

func callAny(handler any, args ...any) error {
	if handler == nil {
		return nil
	}
	fn := reflect.ValueOf(handler)
	if fn.Kind() != reflect.Func {
		return fmt.Errorf("validator callback must be func")
	}
	fnType := fn.Type()
	inputs := make([]reflect.Value, 0, fnType.NumIn())
	for i := 0; i < fnType.NumIn(); i++ {
		inputType := fnType.In(i)
		var matched reflect.Value
		if i < len(args) && args[i] != nil {
			argValue := reflect.ValueOf(args[i])
			if argValue.Type().AssignableTo(inputType) {
				matched = argValue
			} else if argValue.Type().ConvertibleTo(inputType) {
				matched = argValue.Convert(inputType)
			}
		}
		for _, arg := range args {
			if matched.IsValid() {
				break
			}
			if arg == nil {
				continue
			}
			argValue := reflect.ValueOf(arg)
			if argValue.Type().AssignableTo(inputType) {
				matched = argValue
				break
			}
			if argValue.Type().ConvertibleTo(inputType) {
				matched = argValue.Convert(inputType)
				break
			}
		}
		if !matched.IsValid() {
			matched = reflect.Zero(inputType)
		}
		inputs = append(inputs, matched)
	}
	outputs := fn.Call(inputs)
	if len(outputs) == 0 {
		return nil
	}
	last := outputs[len(outputs)-1]
	if err, ok := last.Interface().(error); ok {
		return err
	}
	return nil
}
