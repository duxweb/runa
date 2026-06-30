package crud

import (
	"github.com/duxweb/runa/core"
	"github.com/duxweb/runa/crud/filter"
	"github.com/duxweb/runa/validate"
)

// Action is a CRUD action name.
type Action string

const (
	ListAction    Action = "list"
	ShowAction    Action = "show"
	CreateAction  Action = "create"
	EditAction    Action = "edit"
	StoreAction   Action = "store"
	DeleteAction  Action = "delete"
	BatchAction   Action = "batch"
	ImportAction  Action = "import"
	ExportAction  Action = "export"
	RestoreAction Action = "restore"
	DestroyAction Action = "destroy"
)

// PaginationMode describes list pagination behavior.
type PaginationMode string

const (
	PageMode   PaginationMode = "page"
	ScrollMode PaginationMode = "scroll"
	NoPageMode PaginationMode = "none"
)

// Store adapts any persistence backend to CRUD.
type Store[Model any, Query any] interface {
	Query(ctx *Context[Model]) (Query, error)
	List(ctx *Context[Model], query Query) ([]*Model, core.ListMeta, error)
	Show(ctx *Context[Model], query Query) (*Model, error)
	Create(ctx *Context[Model]) (*Model, error)
	Edit(ctx *Context[Model], query Query) (*Model, error)
	Store(ctx *Context[Model], query Query, fields []string) (*Model, error)
	Delete(ctx *Context[Model], query Query) error
	Tx(ctx *Context[Model], fn func(ctx *Context[Model]) error) error
}

// Result is passed to Meta callbacks.
type Result[Model any] struct {
	Model    *Model
	Models   []*Model
	ListMeta core.ListMeta
}

// BatchRequest is the unified batch action request.
type BatchRequest struct {
	Action string     `json:"action"`
	IDs    []string   `json:"ids"`
	Data   core.Map   `json:"data"`
	Items  []core.Map `json:"items"`
}

// PageFields maps page query names.
type PageFields struct {
	Page     string
	PageSize string
}

// ScrollFields maps scroll query names.
type ScrollFields struct {
	Cursor string
	Limit  string
}

// TreeOptions configures list tree output.
type TreeOptions struct {
	ID       string
	ParentID string
	Children string
}

// SortField maps public sort names to store fields.
type SortField struct {
	Name  string
	Field string
}

// SortOrder stores one parsed sort order.
type SortOrder struct {
	Name      string
	Field     string
	Direction string
}

// Relation describes a relation requested by CRUD.
type Relation struct {
	Name string
	Meta core.Map
}

type InitFunc[Model any] func(c *Context[Model]) error
type QueryFunc[Model any, Query any] func(c *Context[Model], query Query) (Query, error)
type ValidateFunc[Model any] func(c *Context[Model], v *validate.Validator)
type FormatFunc[Model any] func(c *Context[Model], f *Formatter[Model])
type TransformFunc[Model any, Output any] func(c *Context[Model], model *Model) Output
type TransformMapFunc[Model any] func(c *Context[Model], model *Model) core.Map
type MetaFunc[Model any] func(c *Context[Model], result Result[Model]) core.Map
type BeforeFunc[Model any] func(c *Context[Model]) error
type AfterFunc[Model any] func(c *Context[Model]) error
type BatchFunc[Model any] func(c *Context[Model], batch BatchRequest) (any, error)

type actionCallbacks[T any] struct {
	defaults []T
	actions  map[Action][]T
}

func (callbacks *actionCallbacks[T]) add(fn T, actions ...Action) {
	if len(actions) == 0 {
		callbacks.defaults = append(callbacks.defaults, fn)
		return
	}
	if callbacks.actions == nil {
		callbacks.actions = make(map[Action][]T)
	}
	for _, action := range actions {
		callbacks.actions[action] = append(callbacks.actions[action], fn)
	}
}

func (callbacks actionCallbacks[T]) all(action Action) []T {
	items := append([]T(nil), callbacks.defaults...)
	items = append(items, callbacks.actions[action]...)
	return items
}

func (callbacks actionCallbacks[T]) first(action Action) (T, bool) {
	if items := callbacks.actions[action]; len(items) > 0 {
		return items[len(items)-1], true
	}
	if len(callbacks.defaults) > 0 {
		return callbacks.defaults[len(callbacks.defaults)-1], true
	}
	var zero T
	return zero, false
}

type builderOptions struct {
	key           string
	pageSize      int
	pageMax       int
	pageOffsetMax int
	pageFields    PageFields
	scrollLimit   int
	scrollMax     int
	scrollFields  ScrollFields
	pagination    PaginationMode
	tree          *TreeOptions
	sort          []SortOrder
	sortFields    []SortField
	filters       []filter.Filter
	relations     []Relation
}
