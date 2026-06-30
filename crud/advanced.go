package crud

import (
	"context"

	"github.com/duxweb/runa/core"
	"github.com/duxweb/runa/storage"
)

// SoftDeleteStore adds restore and destroy actions.
type SoftDeleteStore[Model any, Query any] interface {
	ShowDeleted(ctx *Context[Model], query Query) (*Model, error)
	Restore(ctx *Context[Model], query Query) (*Model, error)
	Destroy(ctx *Context[Model], query Query) error
}

// ExportStore streams models for export.
type ExportStore[Model any, Query any] interface {
	Export(ctx *Context[Model], query Query, batch int, fn func(models []*Model) error) error
}

// ImportResult stores import summary.
type ImportResult struct {
	Total   int           `json:"total"`
	Success int           `json:"success"`
	Failed  int           `json:"failed"`
	Errors  []ImportError `json:"errors"`
}

// ImportError stores one row import error.
type ImportError struct {
	Row     int    `json:"row"`
	Field   string `json:"field,omitempty"`
	Message string `json:"message"`
}

// ImportRow reads one import row.
type ImportRow struct {
	index int
	data  core.Map
}

// Index returns the 1-based source row index.
func (row ImportRow) Index() int { return row.index }

// Get reads a column value cast to T.
func (row ImportRow) Get[T any](name string, defaults ...T) T {
	return core.Cast[T](row.data[name], defaults...)
}

// Data returns raw row data.
func (row ImportRow) Data() core.Map { return row.data }

// ExportRequest stores async export request metadata.
type ExportRequest struct {
	ID      string
	RouteID string
	Format  string
	Query   core.Map
	Auth    core.Map
	Lang    string
	Meta    core.Map
}

// Set stores export request metadata.
func (request *ExportRequest) Set(key string, value any) {
	if request.Meta == nil {
		request.Meta = make(core.Map)
	}
	request.Meta[key] = value
}

// Get reads export request metadata cast to T.
func (request *ExportRequest) Get[T any](key string, defaults ...T) T {
	if request == nil {
		return core.Cast[T](nil, defaults...)
	}
	return core.Cast[T](request.Meta[key], defaults...)
}

// ExportFile stores exported file metadata.
type ExportFile struct {
	Disk string
	Path string
	URL  string
	Size int64
}

// ExportDispatch pushes an export request to an external queue.
type ExportDispatch func(ctx context.Context, queue string, job string, request ExportRequest) (string, error)

// ExportDisk stores async export files.
type ExportDisk = storage.Disk

// ExportResult stores async export result.
type ExportResult struct {
	ID      string `json:"id"`
	Status  string `json:"status"`
	File    string `json:"file,omitempty"`
	URL     string `json:"url,omitempty"`
	Message string `json:"message,omitempty"`
}

type ImportFunc[Model any] func(c *Context[Model], f *Importer[Model]) error
type ExportFunc[Model any, Output any] func(c *Context[Model], model *Model) (Output, error)

type exportConfig[Model any, Output any] struct {
	name     string
	formats  []string
	batch    int
	disk     string
	path     string
	fields   []*ExportField[Model, Output]
	queue    *ExportQueue[Model, Output]
	dispatch ExportDispatch
	diskFunc func(*Context[Model], string) (ExportDisk, error)
	fn       ExportFunc[Model, Output]
}

type importConfig[Model any] struct {
	formats []string
	batch   int
	fn      ImportFunc[Model]
}

var _ = context.Background
