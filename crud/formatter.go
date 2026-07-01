package crud

import (
	"encoding/json"
	"io"
	"reflect"
	"strings"

	"github.com/duxweb/runa/core"
	"github.com/duxweb/runa/validate"
)

// Formatter assigns request values into the current model.
type Formatter[Model any] struct {
	ctx     *Context[Model]
	sources []*Source[Model]
}

// Field defines a body/form field source.
func (f *Formatter[Model]) Field(name string) *Source[Model] {
	return f.source("field", name)
}

// Query defines a query field source.
func (f *Formatter[Model]) Query(name string) *Source[Model] {
	return f.source("query", name)
}

// Param defines a path param field source.
func (f *Formatter[Model]) Param(name string) *Source[Model] {
	return f.source("param", name)
}

// Header defines a header field source.
func (f *Formatter[Model]) Header(name string) *Source[Model] {
	return f.source("header", name)
}

// Cookie defines a cookie field source.
func (f *Formatter[Model]) Cookie(name string) *Source[Model] {
	return f.source("cookie", name)
}

// File defines a file field source.
func (f *Formatter[Model]) File(name string) *Source[Model] {
	return f.source("file", name)
}

func (f *Formatter[Model]) source(kind string, name string) *Source[Model] {
	source := &Source[Model]{formatter: f, kind: kind, name: name}
	f.sources = append(f.sources, source)
	return source
}

func (f *Formatter[Model]) run(action Action) error {
	var errors []validate.FieldError
	for _, source := range f.sources {
		if err := source.assign(action); err != nil {
			errors = append(errors, validate.FieldError{Field: source.name, Name: source.name, Code: "cast", Message: source.name + " 类型转换失败"})
		}
	}
	if len(errors) > 0 {
		return validate.Invalid(errors...)
	}
	return nil
}

// Source describes one request source.
type Source[Model any] struct {
	formatter  *Formatter[Model]
	kind       string
	name       string
	assignment assignment
}

type assignment interface {
	apply(Action) error
}

// To assigns this source into a model field pointer.
func (source *Source[Model]) To[T any](target *T) *Assign[Model, T] {
	assign := &Assign[Model, T]{source: source, target: target, actions: []Action{CreateAction, EditAction}}
	source.assignment = assign
	return assign
}

// Assign describes one model field assignment.
type Assign[Model any, T any] struct {
	source  *Source[Model]
	target  *T
	actions []Action
	set     func() T
}

// Actions limits this assignment to actions.
func (assign *Assign[Model, T]) Actions(actions ...Action) *Assign[Model, T] {
	assign.actions = append([]Action(nil), actions...)
	return assign
}

// Set overrides assignment value.
func (assign *Assign[Model, T]) Set(fn func() T) *Assign[Model, T] {
	assign.set = fn
	return assign
}

func (source *Source[Model]) assign(action Action) error {
	if source.assignment == nil {
		return nil
	}
	return source.assignment.apply(action)
}

func (assign *Assign[Model, T]) apply(action Action) error {
	if !assign.matches(action) {
		return nil
	}
	ctx := assign.source.formatter.ctx
	if assign.set != nil {
		*assign.target = assign.set()
		ctx.markDirty(assign.fieldName())
		return nil
	}
	raw, ok := assign.source.value()
	if !ok {
		return nil
	}
	value, ok := core.CastOK[T](raw)
	if !ok {
		return validate.Invalid(validate.FieldError{Field: assign.source.name, Name: assign.source.name, Code: "cast", Message: assign.source.name + " 类型转换失败"})
	}
	*assign.target = value
	ctx.markDirty(assign.fieldName())
	return nil
}

func (assign *Assign[Model, T]) fieldName() string {
	if name := fieldName(assign.source.formatter.ctx.Model, assign.target); name != "" {
		return name
	}
	return assign.source.name
}

func (assign *Assign[Model, T]) matches(action Action) bool {
	for _, item := range assign.actions {
		if item == action {
			return true
		}
	}
	return false
}

func (source *Source[Model]) value() (any, bool) {
	ctx := source.formatter.ctx
	switch source.kind {
	case "field":
		if value := ctx.Form[string](source.name); value != "" {
			return value, true
		}
		return jsonField(ctx, source.name)
	case "query":
		value := ctx.Query[string](source.name)
		return value, value != ""
	case "param":
		value := ctx.Param[string](source.name)
		return value, value != ""
	case "header":
		value := ctx.Header[string](source.name)
		return value, value != ""
	case "cookie":
		value := ctx.Cookie[string](source.name)
		return value, value != ""
	case "file":
		value, ok := ctx.File(source.name)
		return value, ok
	default:
		return nil, false
	}
}

func jsonField[Model any](ctx *Context[Model], name string) (any, bool) {
	if ctx.Request() == nil || ctx.Request().Body == nil || !strings.Contains(ctx.Request().Header.Get("Content-Type"), "application/json") {
		return nil, false
	}
	if raw, ok := ctx.Data["__json_body"].(core.Map); ok {
		value, found := raw[name]
		return value, found
	}
	body, err := io.ReadAll(ctx.Request().Body)
	if err != nil {
		return nil, false
	}
	ctx.Request().Body = io.NopCloser(strings.NewReader(string(body)))
	values := make(core.Map)
	if len(strings.TrimSpace(string(body))) > 0 {
		if err := json.Unmarshal(body, &values); err != nil {
			return nil, false
		}
	}
	ctx.Data["__json_body"] = values
	value, found := values[name]
	return value, found
}

func fieldName(model any, target any) string {
	modelValue := reflect.ValueOf(model)
	for modelValue.Kind() == reflect.Pointer {
		if modelValue.IsNil() {
			return ""
		}
		modelValue = modelValue.Elem()
	}
	targetValue := reflect.ValueOf(target)
	if targetValue.Kind() != reflect.Pointer || modelValue.Kind() != reflect.Struct {
		return ""
	}
	targetPointer := targetValue.Pointer()
	modelType := modelValue.Type()
	for index := 0; index < modelValue.NumField(); index++ {
		field := modelValue.Field(index)
		if !field.CanAddr() {
			continue
		}
		if field.Addr().Pointer() == targetPointer {
			if name := jsonName(modelType.Field(index)); name != "" {
				return name
			}
			return modelType.Field(index).Name
		}
	}
	return ""
}

func jsonName(field reflect.StructField) string {
	value := field.Tag.Get("json")
	if value == "" || value == "-" {
		return ""
	}
	return strings.Split(value, ",")[0]
}
