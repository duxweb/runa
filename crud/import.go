package crud

import (
	"fmt"
	"reflect"
	"strings"
	"time"

	"github.com/duxweb/runa/core"
)

// ImportConfig configures import behavior.
type ImportConfig[Model any] struct {
	config importConfig[Model]
}

// Formats sets allowed import formats.
func (config *ImportConfig[Model]) Formats(values ...string) *ImportConfig[Model] {
	config.config.formats = append([]string(nil), values...)
	return config
}

// Batch sets import batch size.
func (config *ImportConfig[Model]) Batch(size int) *ImportConfig[Model] {
	config.config.batch = size
	return config
}

func (config *ImportConfig[Model]) runImport[Query any](builder *Builder[Model, Query], c *Context[Model]) error {
	rows, err := config.readRows(c)
	if err != nil {
		return err
	}
	result := ImportResult{Total: len(rows)}
	for _, row := range rows {
		c.Action = ImportAction
		c.Model = new(Model)
		importer := &Importer[Model]{Model: c.Model, ctx: c, row: row}
		if config.config.fn != nil {
			if err := config.config.fn(c, importer); err != nil {
				result.Failed++
				result.Errors = append(result.Errors, ImportError{Row: row.index, Message: err.Error()})
				continue
			}
		}
		if err := importer.run(); err != nil {
			result.Failed++
			result.Errors = append(result.Errors, ImportError{Row: row.index, Message: err.Error()})
			continue
		}
		if err := builder.runValidate(c); err != nil {
			result.Failed++
			result.Errors = append(result.Errors, ImportError{Row: row.index, Message: err.Error()})
			continue
		}
		if err := builder.store.Tx(c, func(c *Context[Model]) error {
			if err := builder.runBefore(c); err != nil {
				return err
			}
			model, err := builder.store.Create(c)
			if err != nil {
				return err
			}
			c.Model = model
			return builder.runAfter(c)
		}); err != nil {
			result.Failed++
			result.Errors = append(result.Errors, ImportError{Row: row.index, Message: err.Error()})
			continue
		}
		result.Success++
	}
	return c.JSON(result)
}

// Importer maps CSV columns into the current model.
type Importer[Model any] struct {
	Model   *Model
	ctx     *Context[Model]
	row     *ImportRow
	sources []*ImportSource[Model]
}

// Column selects an import column.
func (importer *Importer[Model]) Column(name string) *ImportSource[Model] {
	source := &ImportSource[Model]{importer: importer, name: name}
	importer.sources = append(importer.sources, source)
	return source
}

func (importer *Importer[Model]) run() error {
	for _, source := range importer.sources {
		if err := source.assign(); err != nil {
			return err
		}
	}
	return nil
}

// ImportSource describes one import source column.
type ImportSource[Model any] struct {
	importer *Importer[Model]
	name     string
	assigner importAssignment
}

// To assigns this column to a model field.
func (source *ImportSource[Model]) To[Target any](target *Target) *ImportAssign[Model, Target] {
	assign := &ImportAssign[Model, Target]{source: source, target: target}
	source.assigner = assign
	return assign
}

func (source *ImportSource[Model]) assign() error {
	if source.assigner == nil {
		return nil
	}
	return source.assigner.assign()
}

type importAssignment interface {
	assign() error
}

// ImportAssign describes one import assignment.
type ImportAssign[Model any, Target any] struct {
	source *ImportSource[Model]
	target *Target
	set    any
}

// Set converts and assigns a column value.
func (assign *ImportAssign[Model, Target]) Set[Input any](fn func(c *Context[Model], value Input, row ImportRow) (Target, error)) *ImportAssign[Model, Target] {
	assign.set = fn
	return assign
}

func (assign *ImportAssign[Model, Target]) assign() error {
	raw := assign.source.importer.row.Get[string](assign.source.name)
	if assign.set != nil {
		return assign.callSet(raw)
	}
	value, ok := core.CastOK[Target](raw)
	if !ok {
		return fmt.Errorf("%s 类型转换失败", assign.source.name)
	}
	*assign.target = value
	assign.source.importer.ctx.markDirty(importFieldName(assign.source.importer.Model, assign.target, assign.source.name))
	return nil
}

func (assign *ImportAssign[Model, Target]) callSet(raw string) error {
	value := reflect.ValueOf(assign.set)
	valueType := value.Type()
	if valueType.NumIn() != 3 || valueType.NumOut() != 2 {
		return fmt.Errorf("%s Set 回调签名不正确", assign.source.name)
	}
	inputType := valueType.In(1)
	input, ok := castReflect(raw, inputType)
	if !ok {
		return fmt.Errorf("%s 类型转换失败", assign.source.name)
	}
	results := value.Call([]reflect.Value{
		reflect.ValueOf(assign.source.importer.ctx),
		input,
		reflect.ValueOf(*assign.source.importer.row),
	})
	if !results[1].IsNil() {
		return results[1].Interface().(error)
	}
	*assign.target = results[0].Interface().(Target)
	assign.source.importer.ctx.markDirty(importFieldName(assign.source.importer.Model, assign.target, assign.source.name))
	return nil
}

func (config *ImportConfig[Model]) readRows(c *Context[Model]) ([]*ImportRow, error) {
	reader := c.Request().Body
	format := importFormat(c)
	if file, ok := c.File("file"); ok {
		opened, err := file.Open()
		if err != nil {
			return nil, err
		}
		defer opened.Close()
		if format == "" {
			format = formatFromFilename(file.Filename)
		}
		reader = opened
	}
	if reader == nil {
		return nil, c.Error(400, "import file is required")
	}
	if format == "" {
		format = formatFromContentType(c.Request().Header.Get("Content-Type"))
	}
	if format == "" {
		format = "csv"
	}
	if !formatAllowed(config.config.formats, format) {
		return nil, unsupportedImportFormat(format)
	}
	decoder, ok := importDecoder(format)
	if !ok {
		return nil, unsupportedImportFormat(format)
	}
	return decoder(reader)
}

func importFormat[Model any](c *Context[Model]) string {
	if c == nil || c.Context == nil {
		return ""
	}
	return normalizeFormat(c.Query[string]("format"))
}

func formatFromFilename(name string) string {
	parts := strings.Split(name, ".")
	if len(parts) < 2 {
		return ""
	}
	return normalizeFormat(parts[len(parts)-1])
}

func formatFromContentType(value string) string {
	value = strings.ToLower(value)
	if strings.Contains(value, "spreadsheet") || strings.Contains(value, "excel") {
		return "xlsx"
	}
	if strings.Contains(value, "csv") {
		return "csv"
	}
	return ""
}

func castReflect(raw string, target reflect.Type) (reflect.Value, bool) {
	if target.Kind() == reflect.Pointer {
		converted, ok := castReflect(raw, target.Elem())
		if !ok {
			return reflect.Value{}, false
		}
		pointer := reflect.New(target.Elem())
		pointer.Elem().Set(converted)
		return pointer, true
	}
	if target == reflect.TypeOf(time.Duration(0)) {
		converted, ok := core.CastOK[time.Duration](raw)
		return reflect.ValueOf(converted), ok
	}
	if target == reflect.TypeOf(time.Time{}) {
		converted, ok := core.CastOK[time.Time](raw)
		return reflect.ValueOf(converted), ok
	}
	switch target.Kind() {
	case reflect.String:
		return reflect.ValueOf(raw).Convert(target), true
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		converted, ok := core.CastOK[int64](raw)
		if !ok || target.OverflowInt(converted) {
			return reflect.Value{}, false
		}
		out := reflect.New(target).Elem()
		out.SetInt(converted)
		return out, true
	case reflect.Bool:
		converted, ok := core.CastOK[bool](raw)
		return reflect.ValueOf(converted).Convert(target), ok
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		converted, ok := core.CastOK[uint64](raw)
		if !ok || target.OverflowUint(converted) {
			return reflect.Value{}, false
		}
		out := reflect.New(target).Elem()
		out.SetUint(converted)
		return out, true
	case reflect.Float32, reflect.Float64:
		converted, ok := core.CastOK[float64](raw)
		if !ok || target.OverflowFloat(converted) {
			return reflect.Value{}, false
		}
		out := reflect.New(target).Elem()
		out.SetFloat(converted)
		return out, true
	default:
		return reflect.Value{}, false
	}
}

func importFieldName(model any, target any, fallback string) string {
	if name := fieldName(model, target); name != "" {
		return name
	}
	return fallback
}
