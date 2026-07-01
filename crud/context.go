package crud

import (
	"strings"

	"github.com/duxweb/runa/core"
	"github.com/duxweb/runa/crud/filter"
	"github.com/duxweb/runa/route"
)

// Context is the current CRUD action context.
type Context[Model any] struct {
	*route.Context

	Action Action
	Model  *Model
	Data   core.Map

	options builderOptions
	dirty   []string
	filters []filter.Value
}

func newContext[Model any](ctx *route.Context, action Action, options builderOptions) *Context[Model] {
	if options.key == "" {
		options.key = "id"
	}
	if options.pageFields.Page == "" {
		options.pageFields.Page = "page"
	}
	if options.pageFields.PageSize == "" {
		options.pageFields.PageSize = "page_size"
	}
	if options.scrollFields.Cursor == "" {
		options.scrollFields.Cursor = "cursor"
	}
	if options.scrollFields.Limit == "" {
		options.scrollFields.Limit = "limit"
	}
	if options.pagination == "" {
		options.pagination = PageMode
	}
	if options.pageSize <= 0 {
		options.pageSize = 20
	}
	if options.pageMax <= 0 {
		options.pageMax = 100
	}
	return &Context[Model]{
		Context: ctx,
		Action:  action,
		Data:    make(core.Map),
		options: options,
	}
}

// Set stores shared CRUD context data.
func (c *Context[Model]) Set(key string, value any) {
	c.Data[key] = value
}

// Get reads shared CRUD context data cast to T.
func (c *Context[Model]) Get[T any](key string, defaults ...T) T {
	return core.Cast[T](c.Data[key], defaults...)
}

// Has reports whether shared CRUD context data exists.
func (c *Context[Model]) Has(key string) bool {
	_, ok := c.Data[key]
	return ok
}

// Pagination returns the current list pagination mode.
func (c *Context[Model]) Pagination() PaginationMode { return c.options.pagination }

// Page returns current page request.
func (c *Context[Model]) Page() core.PageRequest {
	page := c.Query[int](c.options.pageFields.Page, 1)
	if page <= 0 {
		page = 1
	}
	size := c.Query[int](c.options.pageFields.PageSize, c.options.pageSize)
	if size <= 0 {
		size = c.options.pageSize
	}
	if c.options.pageMax > 0 && size > c.options.pageMax {
		size = c.options.pageMax
	}
	offset := (page - 1) * size
	maxOffset := c.options.pageOffsetMax
	if maxOffset <= 0 {
		maxOffset = 10000
	}
	if offset > maxOffset {
		page = maxOffset/size + 1
		offset = (page - 1) * size
	}
	return core.PageRequest{Page: page, PageSize: size, Offset: offset, Limit: size}
}

// Scroll returns current scroll request.
func (c *Context[Model]) Scroll() core.ScrollRequest {
	limit := c.Query[int](c.options.scrollFields.Limit, c.options.scrollLimit)
	if limit <= 0 {
		limit = c.options.scrollLimit
	}
	if limit <= 0 {
		limit = 20
	}
	if c.options.scrollMax > 0 && limit > c.options.scrollMax {
		limit = c.options.scrollMax
	}
	if c.options.scrollMax <= 0 && limit > 100 {
		limit = 100
	}
	return core.ScrollRequest{Cursor: c.Query[string](c.options.scrollFields.Cursor), Limit: limit}
}

// Sort returns parsed sort orders.
func (c *Context[Model]) Sort() []SortOrder {
	items := append([]SortOrder(nil), c.options.sort...)
	for _, field := range c.options.sortFields {
		target := field.Field
		if target == "" {
			target = field.Name
		}
		direction := strings.ToLower(c.Query[string](field.Name + "_sort"))
		if direction != "asc" && direction != "desc" {
			continue
		}
		items = append(items, SortOrder{Name: field.Name, Field: target, Direction: direction})
	}
	return items
}

// Filters returns parsed filter values.
func (c *Context[Model]) Filters() []filter.Value {
	if c.filters != nil {
		return append([]filter.Value(nil), c.filters...)
	}
	items := make([]filter.Value, 0, len(c.options.filters))
	for _, item := range c.options.filters {
		value := c.Query[string](item.Name)
		if value == "" {
			continue
		}
		items = append(items, item.Parse(value))
	}
	c.filters = items
	return append([]filter.Value(nil), c.filters...)
}

// Relations returns configured relation preload hints.
func (c *Context[Model]) Relations() []Relation {
	return append([]Relation(nil), c.options.relations...)
}

// Key returns the configured model key path parameter name.
func (c *Context[Model]) Key() string {
	if c.options.key == "" {
		return "id"
	}
	return c.options.key
}

// Dirty returns model fields assigned by Formatter.
func (c *Context[Model]) Dirty() []string {
	return append([]string(nil), c.dirty...)
}

// IsDirty reports whether a model field was assigned by Formatter.
func (c *Context[Model]) IsDirty(field string) bool {
	for _, item := range c.dirty {
		if item == field {
			return true
		}
	}
	return false
}

func (c *Context[Model]) markDirty(field string) {
	if field == "" || c.IsDirty(field) {
		return
	}
	c.dirty = append(c.dirty, field)
}
