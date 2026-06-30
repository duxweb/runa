package route

import (
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"reflect"
	"strings"
	"time"

	"github.com/duxweb/runa/core"
	"github.com/duxweb/runa/validate"
)

// InputValidator is implemented by inputs that define validation rules.
type InputValidator interface {
	Validate(*validate.Validator)
}

// Input binds and validates request data into T.
func Input[T any](ctx *Context) (*T, error) {
	var input T
	if err := ctx.Bind(&input); err != nil {
		return nil, err
	}
	if err := ctx.Validate(&input); err != nil {
		return nil, err
	}
	return &input, nil
}

// Bind binds request data into a struct pointer.
func (ctx *Context) Bind(input any) error {
	value := reflect.ValueOf(input)
	if value.Kind() != reflect.Pointer || value.IsNil() {
		return validate.Invalid(validate.FieldError{Code: "bind", Message: "input must be a non-nil pointer"})
	}
	value = value.Elem()
	if value.Kind() != reflect.Struct {
		return validate.Invalid(validate.FieldError{Code: "bind", Message: "input must point to struct"})
	}
	return ctx.bindStruct(value)
}

// Validate runs input Validate(v) rules when implemented.
func (ctx *Context) Validate(input any) error {
	validator, ok := input.(InputValidator)
	if !ok {
		return nil
	}
	builder := validate.New(input, ctx)
	validator.Validate(builder)
	return builder.Run()
}

func (ctx *Context) bindStruct(value reflect.Value) error {
	if err := ctx.bindJSONBody(value); err != nil {
		return err
	}
	valueType := value.Type()
	for i := 0; i < value.NumField(); i++ {
		field := value.Field(i)
		fieldType := valueType.Field(i)
		if !field.CanSet() || fieldType.PkgPath != "" {
			continue
		}
		if isBodyField(fieldType) {
			if err := ctx.bindBodyField(field, fieldType); err != nil {
				return err
			}
			continue
		}
		if isFileField(fieldType) {
			if err := ctx.bindFileField(field, fieldType); err != nil {
				return err
			}
			continue
		}
		if field.Kind() == reflect.Struct && !hasBindingTag(fieldType) {
			if err := ctx.bindStruct(field); err != nil {
				return err
			}
			continue
		}
		raw, ok := ctx.bindingValue(fieldType)
		if !ok {
			raw = fieldType.Tag.Get("default")
			ok = raw != ""
		}
		if !ok {
			continue
		}
		if err := setField(field, raw); err != nil {
			return validate.Invalid(validate.FieldError{Field: fieldType.Name, Name: fieldName(fieldType), Code: "cast", Message: fmt.Sprintf("%s 类型转换失败", fieldName(fieldType))})
		}
	}
	return nil
}

func (ctx *Context) bindingValue(field reflect.StructField) (string, bool) {
	if value, ok := ctx.valueFromTag("path", field.Tag.Get("path")); ok {
		return value, true
	}
	if value, ok := ctx.valueFromTag("param", field.Tag.Get("param")); ok {
		return value, true
	}
	if value, ok := ctx.valueFromTag("query", field.Tag.Get("query")); ok {
		return value, true
	}
	if value, ok := ctx.valueFromTag("header", field.Tag.Get("header")); ok {
		return value, true
	}
	if value, ok := ctx.valueFromTag("cookie", field.Tag.Get("cookie")); ok {
		return value, true
	}
	if value, ok := ctx.valueFromTag("form", field.Tag.Get("form")); ok {
		return value, true
	}
	return ctx.valueFromBindTag(field.Tag.Get("bind"))
}

func (ctx *Context) valueFromBindTag(tag string) (string, bool) {
	for _, part := range strings.Split(tag, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		source, name, ok := strings.Cut(part, ":")
		if !ok {
			continue
		}
		if value, found := ctx.valueFromTag(source, name); found {
			return value, true
		}
	}
	return "", false
}

func (ctx *Context) valueFromTag(source string, name string) (string, bool) {
	if name == "" || name == "-" {
		return "", false
	}
	switch source {
	case "path", "param":
		value, ok := ctx.params[name]
		return value, ok && value != ""
	case "query":
		if ctx.request == nil {
			return "", false
		}
		values, ok := ctx.request.URL.Query()[name]
		if !ok || len(values) == 0 {
			return "", false
		}
		return strings.Join(values, ","), true
	case "header":
		if ctx.request == nil {
			return "", false
		}
		value := ctx.request.Header.Get(name)
		return value, value != ""
	case "cookie":
		if ctx.request == nil {
			return "", false
		}
		cookie, err := ctx.request.Cookie(name)
		if err != nil {
			return "", false
		}
		return cookie.Value, true
	case "form":
		if ctx.request == nil {
			return "", false
		}
		if err := ctx.request.ParseMultipartForm(32 << 20); err != nil && err != http.ErrNotMultipart {
			return "", false
		}
		value := ctx.request.FormValue(name)
		return value, value != ""
	default:
		return "", false
	}
}

func (ctx *Context) bindJSONBody(value reflect.Value) error {
	if ctx.request == nil || ctx.request.Body == nil || !strings.Contains(ctx.request.Header.Get("Content-Type"), "application/json") {
		return nil
	}
	if !hasJSONFields(value.Type()) {
		return nil
	}
	body, err := io.ReadAll(ctx.request.Body)
	if err != nil {
		return validate.Invalid(validate.FieldError{Code: "body", Message: "请求体读取失败"})
	}
	ctx.request.Body = io.NopCloser(strings.NewReader(string(body)))
	if len(strings.TrimSpace(string(body))) == 0 {
		return nil
	}
	target := value.Addr().Interface()
	if err := json.Unmarshal(body, target); err != nil {
		return validate.Invalid(validate.FieldError{Code: "json", Message: "JSON 格式错误"})
	}
	ctx.request.Body = io.NopCloser(strings.NewReader(string(body)))
	return nil
}

func hasJSONFields(valueType reflect.Type) bool {
	for i := 0; i < valueType.NumField(); i++ {
		field := valueType.Field(i)
		if field.PkgPath != "" {
			continue
		}
		if field.Tag.Get("json") != "" || field.Tag.Get("body") == "json" {
			return true
		}
	}
	return false
}

func isBodyField(field reflect.StructField) bool {
	return field.Tag.Get("body") != "" || field.Name == "Body"
}

func (ctx *Context) bindBodyField(field reflect.Value, fieldType reflect.StructField) error {
	if ctx.request == nil || ctx.request.Body == nil {
		return nil
	}
	mode := fieldType.Tag.Get("body")
	if mode == "" {
		mode = "json"
	}
	if mode == "stream" {
		return setStreamField(field, ctx.request)
	}
	body, err := io.ReadAll(ctx.request.Body)
	if err != nil {
		return validate.Invalid(validate.FieldError{Field: fieldType.Name, Name: fieldName(fieldType), Code: "body", Message: "请求体读取失败"})
	}
	ctx.request.Body = io.NopCloser(strings.NewReader(string(body)))
	switch mode {
	case "json":
		if field.Type() == reflect.TypeOf(core.JSONRaw{}) {
			field.Set(reflect.ValueOf(core.JSONRaw(body)))
			return nil
		}
		if len(strings.TrimSpace(string(body))) == 0 {
			return nil
		}
		if err := json.Unmarshal(body, field.Addr().Interface()); err != nil {
			return validate.Invalid(validate.FieldError{Field: fieldType.Name, Name: fieldName(fieldType), Code: "json", Message: "JSON 格式错误"})
		}
	case "bytes":
		if field.Kind() == reflect.Slice && field.Type().Elem().Kind() == reflect.Uint8 {
			field.Set(reflect.ValueOf(body).Convert(field.Type()))
			return nil
		}
		return setField(field, string(body))
	case "string":
		return setField(field, string(body))
	default:
		return setField(field, string(body))
	}
	return nil
}

func setStreamField(field reflect.Value, request *http.Request) error {
	stream := core.Stream{Reader: request.Body, Size: request.ContentLength, ContentType: request.Header.Get("Content-Type")}
	if field.Type() == reflect.TypeOf(core.Stream{}) {
		field.Set(reflect.ValueOf(stream))
		return nil
	}
	if field.Kind() == reflect.Pointer && field.Type().Elem() == reflect.TypeOf(core.Stream{}) {
		field.Set(reflect.ValueOf(&stream))
		return nil
	}
	return nil
}

func isFileField(field reflect.StructField) bool {
	return field.Tag.Get("file") != ""
}

func (ctx *Context) bindFileField(field reflect.Value, fieldType reflect.StructField) error {
	if ctx.request == nil {
		return nil
	}
	if err := ctx.request.ParseMultipartForm(32 << 20); err != nil && err != http.ErrNotMultipart {
		return validate.Invalid(validate.FieldError{Field: fieldType.Name, Name: fieldName(fieldType), Code: "file", Message: "文件上传解析失败"})
	}
	name := fieldType.Tag.Get("file")
	if name == "" || name == "-" || ctx.request.MultipartForm == nil || ctx.request.MultipartForm.File == nil {
		return nil
	}
	files := ctx.request.MultipartForm.File[name]
	if len(files) == 0 {
		return nil
	}
	if field.Kind() == reflect.Slice {
		items := reflect.MakeSlice(field.Type(), 0, len(files))
		for _, header := range files {
			item, ok := uploadFileValue(header, field.Type().Elem())
			if !ok {
				return validate.Invalid(validate.FieldError{Field: fieldType.Name, Name: fieldName(fieldType), Code: "file", Message: "文件字段类型错误"})
			}
			items = reflect.Append(items, item)
		}
		field.Set(items)
		return nil
	}
	item, ok := uploadFileValue(files[0], field.Type())
	if !ok {
		return validate.Invalid(validate.FieldError{Field: fieldType.Name, Name: fieldName(fieldType), Code: "file", Message: "文件字段类型错误"})
	}
	field.Set(item)
	return nil
}

func uploadFileValue(header *multipart.FileHeader, target reflect.Type) (reflect.Value, bool) {
	contentType := ""
	if header.Header != nil {
		contentType = header.Header.Get("Content-Type")
	}
	file := core.UploadFile{Filename: header.Filename, Size: header.Size, ContentType: contentType, Header: header}
	value := reflect.ValueOf(file)
	if value.Type().AssignableTo(target) {
		return value, true
	}
	if target.Kind() == reflect.Pointer && value.Type().AssignableTo(target.Elem()) {
		pointer := reflect.New(target.Elem())
		pointer.Elem().Set(value)
		return pointer, true
	}
	return reflect.Value{}, false
}

func hasBindingTag(field reflect.StructField) bool {
	for _, tag := range []string{"path", "param", "query", "header", "cookie", "form", "bind", "body", "file"} {
		if field.Tag.Get(tag) != "" {
			return true
		}
	}
	return false
}

func fieldName(field reflect.StructField) string {
	for _, tag := range []string{"label", "json", "query", "path", "param", "header", "cookie", "form", "file"} {
		value := field.Tag.Get(tag)
		if value != "" && value != "-" {
			return strings.Split(value, ",")[0]
		}
	}
	return field.Name
}

func setField(field reflect.Value, raw string) error {
	if field.Kind() == reflect.Pointer {
		converted, ok := convertValue(raw, field.Type())
		if !ok {
			return fmt.Errorf("cast failed")
		}
		field.Set(converted)
		return nil
	}
	if field.Kind() == reflect.Slice && field.Type().Elem().Kind() != reflect.Uint8 {
		parts := splitValues(raw)
		slice := reflect.MakeSlice(field.Type(), 0, len(parts))
		for _, part := range parts {
			item := reflect.New(field.Type().Elem()).Elem()
			if err := setField(item, part); err != nil {
				return err
			}
			slice = reflect.Append(slice, item)
		}
		field.Set(slice)
		return nil
	}
	converted, ok := convertValue(raw, field.Type())
	if !ok {
		return fmt.Errorf("cast failed")
	}
	field.Set(converted)
	return nil
}

func splitValues(raw string) []string {
	parts := strings.Split(raw, ",")
	items := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			items = append(items, part)
		}
	}
	return items
}

func convertValue(raw string, target reflect.Type) (reflect.Value, bool) {
	if target.Kind() == reflect.Pointer {
		converted, ok := convertValue(raw, target.Elem())
		if !ok {
			return reflect.Value{}, false
		}
		pointer := reflect.New(target.Elem())
		pointer.Elem().Set(converted)
		return pointer, true
	}
	if target.Kind() == reflect.Slice && target.Elem().Kind() == reflect.Uint8 {
		return reflect.ValueOf([]byte(raw)).Convert(target), true
	}
	if target == reflect.TypeOf(time.Duration(0)) {
		value, ok := core.CastOK[time.Duration](raw)
		return reflect.ValueOf(value), ok
	}
	if target == reflect.TypeOf(time.Time{}) {
		value, ok := core.CastOK[time.Time](raw)
		return reflect.ValueOf(value), ok
	}
	switch target.Kind() {
	case reflect.String:
		return reflect.ValueOf(core.Cast[string](raw)).Convert(target), true
	case reflect.Bool:
		value, ok := core.CastOK[bool](raw)
		return reflect.ValueOf(value).Convert(target), ok
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		value, ok := core.CastOK[int64](raw)
		if !ok || target.OverflowInt(value) {
			return reflect.Value{}, false
		}
		result := reflect.New(target).Elem()
		result.SetInt(value)
		return result, true
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		value, ok := core.CastOK[uint64](raw)
		if !ok || target.OverflowUint(value) {
			return reflect.Value{}, false
		}
		result := reflect.New(target).Elem()
		result.SetUint(value)
		return result, true
	case reflect.Float32, reflect.Float64:
		value, ok := core.CastOK[float64](raw)
		if !ok || target.OverflowFloat(value) {
			return reflect.Value{}, false
		}
		result := reflect.New(target).Elem()
		result.SetFloat(value)
		return result, true
	default:
		return reflect.Value{}, false
	}
}
