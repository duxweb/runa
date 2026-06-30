package crud

import (
	"bytes"
	"context"
	"encoding/csv"
	"fmt"
	"net/http"
	"net/url"
	"reflect"
	"strconv"
	"strings"

	"github.com/duxweb/runa/auth"
	"github.com/duxweb/runa/core"
	"github.com/duxweb/runa/route"
	"github.com/duxweb/runa/storage"
	"github.com/xuri/excelize/v2"
)

// Exporter configures export behavior.
type Exporter[Model any, Output any] struct {
	config exportConfig[Model, Output]
}

// Name sets exported file base name.
func (exporter *Exporter[Model, Output]) Name(value string) *Exporter[Model, Output] {
	exporter.config.name = value
	return exporter
}

// Formats sets allowed export formats.
func (exporter *Exporter[Model, Output]) Formats(values ...string) *Exporter[Model, Output] {
	exporter.config.formats = append([]string(nil), values...)
	return exporter
}

// Batch sets export batch size.
func (exporter *Exporter[Model, Output]) Batch(size int) *Exporter[Model, Output] {
	exporter.config.batch = size
	return exporter
}

// Queue configures async export callbacks.
func (exporter *Exporter[Model, Output]) Queue(name string, configure func(*ExportQueue[Model, Output]) error) *Exporter[Model, Output] {
	queue := &ExportQueue[Model, Output]{name: name}
	if configure != nil {
		_ = configure(queue)
	}
	exporter.config.queue = queue
	return exporter
}

// Dispatch sets the queue dispatcher used by async export.
func (exporter *Exporter[Model, Output]) Dispatch(dispatcher ExportDispatch) *Exporter[Model, Output] {
	exporter.config.dispatch = dispatcher
	return exporter
}

// DiskResolver sets the storage disk resolver used by async export workers.
func (exporter *Exporter[Model, Output]) DiskResolver(fn func(*Context[Model], string) (ExportDisk, error)) *Exporter[Model, Output] {
	exporter.config.diskFunc = fn
	return exporter
}

// Disk sets async export disk name.
func (exporter *Exporter[Model, Output]) Disk(name string) *Exporter[Model, Output] {
	exporter.config.disk = name
	return exporter
}

// Path sets async export path prefix.
func (exporter *Exporter[Model, Output]) Path(path string) *Exporter[Model, Output] {
	exporter.config.path = path
	return exporter
}

// Field adds one exported field.
func (exporter *Exporter[Model, Output]) Field(name string) *ExportField[Model, Output] {
	field := &ExportField[Model, Output]{name: name, title: name}
	exporter.config.fields = append(exporter.config.fields, field)
	return field
}

func (exporter *Exporter[Model, Output]) runExport[Query any](builder *Builder[Model, Query], c *Context[Model]) error {
	if exporter.config.queue != nil {
		request := &ExportRequest{
			RouteID: c.Route().RouteName,
			Format:  exporter.format(c),
			Query:   queryMap(c),
			Lang:    c.Lang(),
			Meta:    make(core.Map),
		}
		if info, ok := c.Locals("runa.auth").(interface{ AuthData() core.Map }); ok {
			request.Auth = info.AuthData()
		} else if info, ok := c.Locals("runa.auth").(interface{ DataMap() core.Map }); ok {
			request.Auth = info.DataMap()
		} else if info, ok := c.Locals("runa.auth").(*auth.Info); ok && info != nil {
			request.Auth = info.Data
		}
		if exporter.config.queue.start != nil {
			result, err := exporter.config.queue.start(c, request)
			if err != nil {
				return err
			}
			if result != nil && request.ID == "" {
				request.ID = result.ID
			}
			if err := exporter.dispatch(c.Context.Context(), c.Route().RouteName, request); err != nil {
				return err
			}
			if result != nil && result.ID == "" {
				result.ID = request.ID
			}
			return c.JSON(result)
		}
		if err := exporter.dispatch(c.Context.Context(), c.Route().RouteName, request); err != nil {
			return err
		}
		result := &ExportResult{ID: request.ID, Status: "pending"}
		return c.JSON(result)
	}
	query, err := builder.queryFor(c, true)
	if err != nil {
		return err
	}
	store, ok := builder.store.(ExportStore[Model, Query])
	if !ok {
		models, _, err := builder.store.List(c, query)
		if err != nil {
			return err
		}
		return exporter.writeCSV(c, models)
	}
	models := make([]*Model, 0)
	batch := exporter.config.batch
	if batch <= 0 {
		batch = 500
	}
	if err := store.Export(c, query, batch, func(items []*Model) error {
		models = append(models, items...)
		return nil
	}); err != nil {
		return err
	}
	if exporter.format(c) == "xlsx" {
		return exporter.writeXLSX(c, models)
	}
	return exporter.writeCSV(c, models)
}

func (exporter *Exporter[Model, Output]) writeCSV(c *Context[Model], models []*Model) error {
	data, err := exporter.csvBytes(c, models)
	if err != nil {
		return err
	}
	name := exporter.config.name
	if name == "" {
		name = "export"
	}
	c.Set("Content-Disposition", `attachment; filename="`+name+`.csv"`)
	return c.Status(http.StatusOK).Blob("text/csv; charset=utf-8", data)
}

func (exporter *Exporter[Model, Output]) writeXLSX(c *Context[Model], models []*Model) error {
	data, err := exporter.xlsxBytes(c, models)
	if err != nil {
		return err
	}
	name := exporter.config.name
	if name == "" {
		name = "export"
	}
	c.Set("Content-Disposition", `attachment; filename="`+name+`.xlsx"`)
	return c.Status(http.StatusOK).Blob("application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", data)
}

func (exporter *Exporter[Model, Output]) csvBytes(c *Context[Model], models []*Model) ([]byte, error) {
	var buf bytes.Buffer
	writer := csv.NewWriter(&buf)
	fields := exporter.fields()
	headers := make([]string, 0, len(fields))
	for _, field := range fields {
		headers = append(headers, field.title)
	}
	if err := writer.Write(headers); err != nil {
		return nil, err
	}
	for _, model := range models {
		output, err := exporter.config.fn(c, model)
		if err != nil {
			return nil, err
		}
		record := make([]string, 0, len(fields))
		for _, field := range fields {
			value, err := field.value(c, output)
			if err != nil {
				return nil, err
			}
			record = append(record, safeExportText(value))
		}
		if err := writer.Write(record); err != nil {
			return nil, err
		}
	}
	writer.Flush()
	if err := writer.Error(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func (exporter *Exporter[Model, Output]) xlsxBytes(c *Context[Model], models []*Model) ([]byte, error) {
	file := excelize.NewFile()
	defer file.Close()
	sheet := file.GetSheetName(0)
	fields := exporter.fields()
	for index, field := range fields {
		cell, err := excelize.CoordinatesToCellName(index+1, 1)
		if err != nil {
			return nil, err
		}
		if err := file.SetCellValue(sheet, cell, field.title); err != nil {
			return nil, err
		}
	}
	for rowIndex, model := range models {
		output, err := exporter.config.fn(c, model)
		if err != nil {
			return nil, err
		}
		for colIndex, field := range fields {
			cell, err := excelize.CoordinatesToCellName(colIndex+1, rowIndex+2)
			if err != nil {
				return nil, err
			}
			value, err := field.value(c, output)
			if err != nil {
				return nil, err
			}
			if err := file.SetCellValue(sheet, cell, safeExportValue(value)); err != nil {
				return nil, err
			}
		}
	}
	var buf bytes.Buffer
	if err := file.Write(&buf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func safeExportValue(value any) any {
	switch typed := value.(type) {
	case string:
		return safeExportText(typed)
	case fmt.Stringer:
		return safeExportText(typed.String())
	default:
		return value
	}
}

func safeExportText(value any) string {
	text := fmt.Sprint(value)
	if text == "" {
		return text
	}
	switch text[0] {
	case '=', '+', '-', '@', '\t', '\r', '\n':
		return "'" + text
	default:
		return text
	}
}

func (exporter *Exporter[Model, Output]) fields() []*ExportField[Model, Output] {
	fields := exporter.config.fields
	if len(fields) == 0 {
		fields = defaultExportFields[Model, Output]()
	}
	return fields
}

func (exporter *Exporter[Model, Output]) format(c *Context[Model]) string {
	format := strings.ToLower(route.Query[string](c.Context, "format"))
	if format != "" {
		return format
	}
	if len(exporter.config.formats) > 0 {
		return exporter.config.formats[0]
	}
	return "csv"
}

func (exporter *Exporter[Model, Output]) dispatch(ctx context.Context, routeID string, request *ExportRequest) error {
	if exporter.config.dispatch == nil {
		return fmt.Errorf("crud export queue %s dispatcher is not configured", exporter.config.queue.name)
	}
	if request.ID == "" {
		request.ID = fmt.Sprintf("export-%d", core.Now().UnixNano())
	}
	job := exporter.config.queue.job
	if job == "" {
		job = routeID + ".export"
	}
	_, err := exporter.config.dispatch(ctx, exporter.config.queue.name, job, *request)
	return err
}

// Run executes an async export request in a worker.
func (exporter *Exporter[Model, Output]) Run[Query any](ctx context.Context, builder *Builder[Model, Query], request ExportRequest) (file *ExportFile, err error) {
	if request.Format == "" {
		request.Format = "csv"
	}
	if request.Meta == nil {
		request.Meta = make(core.Map)
	}
	c := newContext[Model](route.NewContext(noopResponseWriter{}, exportHTTPRequest(ctx, request), nil, nil), ExportAction, builder.options)
	if exporter.config.queue != nil && exporter.config.queue.failed != nil {
		defer func() {
			if err != nil {
				_ = exporter.config.queue.failed(ctx, &request, err)
			}
		}()
	}
	query, err := builder.queryFor(c, true)
	if err != nil {
		return nil, err
	}
	store, ok := builder.store.(ExportStore[Model, Query])
	if !ok {
		return nil, fmt.Errorf("async export requires ExportStore")
	}
	models := make([]*Model, 0)
	batch := exporter.config.batch
	if batch <= 0 {
		batch = 500
	}
	if err := store.Export(c, query, batch, func(items []*Model) error {
		models = append(models, items...)
		return nil
	}); err != nil {
		return nil, err
	}
	var data []byte
	contentType := "text/csv; charset=utf-8"
	switch strings.ToLower(request.Format) {
	case "xlsx":
		data, err = exporter.xlsxBytes(c, models)
		contentType = "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"
	default:
		data, err = exporter.csvBytes(c, models)
		request.Format = "csv"
	}
	if err != nil {
		return nil, err
	}
	diskName := exporter.config.disk
	if diskName == "" {
		diskName = storage.DiskLocal
	}
	if exporter.config.diskFunc == nil {
		return nil, fmt.Errorf("crud export disk resolver is not configured")
	}
	disk, err := exporter.config.diskFunc(c, diskName)
	if err != nil {
		return nil, err
	}
	path := exporter.filePath(&request)
	if err := disk.PutBytes(ctx, path, data, storage.ContentType(contentType)); err != nil {
		return nil, err
	}
	url, _ := disk.URL(ctx, path)
	file = &ExportFile{Disk: diskName, Path: path, URL: url, Size: int64(len(data))}
	if exporter.config.queue != nil && exporter.config.queue.done != nil {
		if err := exporter.config.queue.done(ctx, &request, file); err != nil {
			return nil, err
		}
	}
	return file, nil
}

func (exporter *Exporter[Model, Output]) filePath(request *ExportRequest) string {
	name := exporter.config.name
	if name == "" {
		name = "export"
	}
	id := request.ID
	if id == "" {
		id = strconv.FormatInt(core.Now().UnixNano(), 10)
	}
	filename := name + "-" + id + "." + strings.ToLower(request.Format)
	prefix := strings.Trim(exporter.config.path, "/")
	if prefix == "" {
		return filename
	}
	return prefix + "/" + filename
}

// ExportField configures one export column.
type ExportField[Model any, Output any] struct {
	name  string
	title string
	set   func(c *Context[Model], row Output) (any, error)
}

// Title sets column title.
func (field *ExportField[Model, Output]) Title(value string) *ExportField[Model, Output] {
	field.title = value
	return field
}

// Set overrides column value.
func (field *ExportField[Model, Output]) Set(fn func(c *Context[Model], row Output) (any, error)) *ExportField[Model, Output] {
	field.set = fn
	return field
}

func (field *ExportField[Model, Output]) value(c *Context[Model], row Output) (any, error) {
	if field.set != nil {
		return field.set(c, row)
	}
	return fieldValue(row, field.name), nil
}

// ExportQueue stores async export callbacks.
type ExportQueue[Model any, Output any] struct {
	name   string
	job    string
	start  func(c *Context[Model], req *ExportRequest) (*ExportResult, error)
	done   func(ctx context.Context, req *ExportRequest, file *ExportFile) error
	failed func(ctx context.Context, req *ExportRequest, err error) error
}

// Job sets async export job name.
func (queue *ExportQueue[Model, Output]) Job(name string) *ExportQueue[Model, Output] {
	queue.job = name
	return queue
}

// Start sets request-time async export callback.
func (queue *ExportQueue[Model, Output]) Start(fn func(c *Context[Model], req *ExportRequest) (*ExportResult, error)) *ExportQueue[Model, Output] {
	queue.start = fn
	return queue
}

// Done sets async export success callback.
func (queue *ExportQueue[Model, Output]) Done(fn func(ctx context.Context, req *ExportRequest, file *ExportFile) error) *ExportQueue[Model, Output] {
	queue.done = fn
	return queue
}

// Failed sets async export failure callback.
func (queue *ExportQueue[Model, Output]) Failed(fn func(ctx context.Context, req *ExportRequest, err error) error) *ExportQueue[Model, Output] {
	queue.failed = fn
	return queue
}

func defaultExportFields[Model any, Output any]() []*ExportField[Model, Output] {
	var output Output
	valueType := reflect.TypeOf(output)
	for valueType != nil && valueType.Kind() == reflect.Pointer {
		valueType = valueType.Elem()
	}
	if valueType == nil || valueType.Kind() != reflect.Struct {
		return nil
	}
	fields := make([]*ExportField[Model, Output], 0, valueType.NumField())
	for i := 0; i < valueType.NumField(); i++ {
		field := valueType.Field(i)
		if field.PkgPath != "" {
			continue
		}
		name := jsonName(field)
		if name == "" {
			name = field.Name
		}
		if name == "-" {
			continue
		}
		fields = append(fields, &ExportField[Model, Output]{name: name, title: name})
	}
	return fields
}

func fieldValue(row any, name string) any {
	value := reflect.ValueOf(row)
	for value.Kind() == reflect.Pointer {
		if value.IsNil() {
			return nil
		}
		value = value.Elem()
	}
	if value.Kind() == reflect.Map {
		item := value.MapIndex(reflect.ValueOf(name))
		if item.IsValid() {
			return item.Interface()
		}
		return nil
	}
	if value.Kind() != reflect.Struct {
		return nil
	}
	valueType := value.Type()
	for i := 0; i < value.NumField(); i++ {
		field := valueType.Field(i)
		if field.PkgPath != "" {
			continue
		}
		if field.Name == name || jsonName(field) == name {
			return value.Field(i).Interface()
		}
	}
	return nil
}

func queryMap[Model any](c *Context[Model]) core.Map {
	if c == nil || c.Request() == nil || c.Request().URL == nil {
		return core.Map{}
	}
	values := c.Request().URL.Query()
	output := make(core.Map, len(values))
	for key, items := range values {
		switch len(items) {
		case 0:
			continue
		case 1:
			output[key] = items[0]
		default:
			output[key] = append([]string(nil), items...)
		}
	}
	return output
}

func exportHTTPRequest(ctx context.Context, request ExportRequest) *http.Request {
	values := url.Values{}
	for key, value := range request.Query {
		switch typed := value.(type) {
		case []string:
			for _, item := range typed {
				values.Add(key, item)
			}
		case []any:
			for _, item := range typed {
				values.Add(key, fmt.Sprint(item))
			}
		default:
			values.Set(key, fmt.Sprint(value))
		}
	}
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, "/?"+values.Encode(), nil)
	return req
}

type noopResponseWriter struct{}

func (noopResponseWriter) Header() http.Header            { return make(http.Header) }
func (noopResponseWriter) Write(body []byte) (int, error) { return len(body), nil }
func (noopResponseWriter) WriteHeader(int)                {}
