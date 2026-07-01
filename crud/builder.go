package crud

import (
	"net/http"

	"github.com/duxweb/runa/core"
	"github.com/duxweb/runa/crud/filter"
	"github.com/duxweb/runa/resource"
	"github.com/duxweb/runa/route"
	"github.com/duxweb/runa/validate"
)

// Builder configures and registers standard CRUD actions.
type Builder[Model any, Query any] struct {
	resource *resource.Resource
	store    Store[Model, Query]

	options builderOptions
	actions []Action
	routes  map[Action]*route.Route

	init        []InitFunc[Model]
	query       []QueryFunc[Model, Query]
	listQuery   []QueryFunc[Model, Query]
	oneQuery    []QueryFunc[Model, Query]
	validate    actionCallbacks[ValidateFunc[Model]]
	format      actionCallbacks[FormatFunc[Model]]
	transform   actionCallbacks[func(*Context[Model], *Model) (any, error)]
	schemas     actionCallbacks[transformSchema]
	meta        actionCallbacks[MetaFunc[Model]]
	before      actionCallbacks[BeforeFunc[Model]]
	after       actionCallbacks[AfterFunc[Model]]
	batch       BatchFunc[Model]
	exporter    func(*Builder[Model, Query], *Context[Model]) error
	exported    bool
	exportAsync bool
	importer    func(*Builder[Model, Query], *Context[Model]) error
	summaries   map[Action]string
	registered  bool
}

// New creates and registers a CRUD builder.
func New[Model any, Query any](res *resource.Resource, store Store[Model, Query]) *Builder[Model, Query] {
	builder := &Builder[Model, Query]{
		resource: res,
		store:    store,
		options: builderOptions{
			key:           "id",
			pageSize:      20,
			pageMax:       100,
			pageOffsetMax: 10000,
			scrollLimit:   20,
			scrollMax:     100,
			pagination:    PageMode,
		},
		actions: []Action{ListAction, ShowAction, CreateAction, EditAction, DeleteAction},
		routes:  make(map[Action]*route.Route),
	}
	builder.register()
	return builder
}

// Actions controls exposed standard actions.
func (builder *Builder[Model, Query]) Actions(actions ...Action) *Builder[Model, Query] {
	builder.actions = append([]Action(nil), actions...)
	builder.register()
	return builder
}

// Key sets the model key path parameter.
func (builder *Builder[Model, Query]) Key(field string) *Builder[Model, Query] {
	builder.options.key = field
	builder.register()
	return builder
}

// Page enables offset pagination.
func (builder *Builder[Model, Query]) Page(size int, max int) *Builder[Model, Query] {
	builder.options.pagination = PageMode
	builder.options.pageSize = size
	builder.options.pageMax = max
	return builder
}

// PageFields maps offset pagination query fields.
func (builder *Builder[Model, Query]) PageFields(fields PageFields) *Builder[Model, Query] {
	builder.options.pageFields = fields
	return builder
}

// Scroll enables cursor pagination.
func (builder *Builder[Model, Query]) Scroll(limit int, max int) *Builder[Model, Query] {
	builder.options.pagination = ScrollMode
	builder.options.scrollLimit = limit
	builder.options.scrollMax = max
	return builder
}

// ScrollFields maps cursor pagination query fields.
func (builder *Builder[Model, Query]) ScrollFields(fields ScrollFields) *Builder[Model, Query] {
	builder.options.scrollFields = fields
	return builder
}

// NoPage disables pagination.
func (builder *Builder[Model, Query]) NoPage() *Builder[Model, Query] {
	builder.options.pagination = NoPageMode
	return builder
}

// Tree enables tree list output.
func (builder *Builder[Model, Query]) Tree(options TreeOptions) *Builder[Model, Query] {
	builder.options.tree = &options
	builder.options.pagination = NoPageMode
	return builder
}

// Sort appends a default sort order.
func (builder *Builder[Model, Query]) Sort(field string, direction string) *Builder[Model, Query] {
	builder.options.sort = append(builder.options.sort, SortOrder{Name: field, Field: field, Direction: direction})
	return builder
}

// SortFields sets allowed request sort fields.
func (builder *Builder[Model, Query]) SortFields(fields ...SortField) *Builder[Model, Query] {
	builder.options.sortFields = append([]SortField(nil), fields...)
	return builder
}

// Filters sets request filters.
func (builder *Builder[Model, Query]) Filters(filters ...filter.Filter) *Builder[Model, Query] {
	builder.options.filters = append([]filter.Filter(nil), filters...)
	return builder
}

// Relations sets relations that Store may preload for output.
func (builder *Builder[Model, Query]) Relations(names ...string) *Builder[Model, Query] {
	builder.options.relations = make([]Relation, 0, len(names))
	for _, name := range names {
		if name == "" {
			continue
		}
		builder.options.relations = append(builder.options.relations, Relation{Name: name})
	}
	return builder
}

// Init registers an init callback.
func (builder *Builder[Model, Query]) Init(fn InitFunc[Model]) *Builder[Model, Query] {
	builder.init = append(builder.init, fn)
	return builder
}

// Query registers a common query callback.
func (builder *Builder[Model, Query]) Query(fn QueryFunc[Model, Query]) *Builder[Model, Query] {
	builder.query = append(builder.query, fn)
	return builder
}

// ListQuery registers a list query callback.
func (builder *Builder[Model, Query]) ListQuery(fn QueryFunc[Model, Query]) *Builder[Model, Query] {
	builder.listQuery = append(builder.listQuery, fn)
	return builder
}

// OneQuery registers a single-record query callback.
func (builder *Builder[Model, Query]) OneQuery(fn QueryFunc[Model, Query]) *Builder[Model, Query] {
	builder.oneQuery = append(builder.oneQuery, fn)
	return builder
}

// Validate registers validation callbacks.
func (builder *Builder[Model, Query]) Validate(fn ValidateFunc[Model], actions ...Action) *Builder[Model, Query] {
	builder.validate.add(fn, actions...)
	return builder
}

// Format registers formatter callbacks.
func (builder *Builder[Model, Query]) Format(fn FormatFunc[Model], actions ...Action) *Builder[Model, Query] {
	if len(actions) == 0 {
		actions = []Action{CreateAction, EditAction, StoreAction}
	}
	builder.format.add(fn, actions...)
	return builder
}

// Transform registers an output transform.
func (builder *Builder[Model, Query]) Transform[Output any](fn TransformFunc[Model, Output], actions ...Action) *Builder[Model, Query] {
	wrapped := func(ctx *Context[Model], model *Model) (any, error) {
		return fn(ctx, model), nil
	}
	builder.transform.add(wrapped, actions...)
	var output Output
	builder.schemas.add(transformSchema{output: typeOf(output)}, actions...)
	builder.updateRouteSchemas()
	return builder
}

// TransformMap registers a map output transform.
func (builder *Builder[Model, Query]) TransformMap(fn TransformMapFunc[Model], actions ...Action) *Builder[Model, Query] {
	wrapped := func(ctx *Context[Model], model *Model) (any, error) {
		return fn(ctx, model), nil
	}
	builder.transform.add(wrapped, actions...)
	builder.schemas.add(transformSchema{mapOut: true}, actions...)
	builder.updateRouteSchemas()
	return builder
}

// Meta registers response metadata callbacks.
func (builder *Builder[Model, Query]) Meta(fn MetaFunc[Model], actions ...Action) *Builder[Model, Query] {
	builder.meta.add(fn, actions...)
	return builder
}

// Before registers before callbacks.
func (builder *Builder[Model, Query]) Before(fn BeforeFunc[Model], actions ...Action) *Builder[Model, Query] {
	builder.before.add(fn, actions...)
	return builder
}

// After registers after callbacks.
func (builder *Builder[Model, Query]) After(fn AfterFunc[Model], actions ...Action) *Builder[Model, Query] {
	builder.after.add(fn, actions...)
	return builder
}

// Batch registers the unified batch action.
func (builder *Builder[Model, Query]) Batch(fn BatchFunc[Model]) *Builder[Model, Query] {
	builder.batch = fn
	if !builder.hasAction(BatchAction) {
		builder.actions = append(builder.actions, BatchAction)
	}
	builder.register()
	return builder
}

// SoftDelete enables restore and destroy actions.
func (builder *Builder[Model, Query]) SoftDelete() *Builder[Model, Query] {
	if !builder.hasAction(RestoreAction) {
		builder.actions = append(builder.actions, RestoreAction)
	}
	if !builder.hasAction(DestroyAction) {
		builder.actions = append(builder.actions, DestroyAction)
	}
	builder.register()
	return builder
}

// Import registers a CSV import action.
func (builder *Builder[Model, Query]) Import(fn ImportFunc[Model], configure func(*ImportConfig[Model]) error) *Builder[Model, Query] {
	config := &ImportConfig[Model]{config: importConfig[Model]{formats: defaultImportFormats(), batch: 100, fn: fn}}
	if configure != nil {
		_ = configure(config)
	}
	builder.importer = func(builder *Builder[Model, Query], c *Context[Model]) error {
		return config.runImport(builder, c)
	}
	if !builder.hasAction(ImportAction) {
		builder.actions = append(builder.actions, ImportAction)
	}
	builder.register()
	return builder
}

// Export registers a CSV export action.
func (builder *Builder[Model, Query]) Export[Output any](fn ExportFunc[Model, Output], configure func(*Exporter[Model, Output]) error) *Builder[Model, Query] {
	exporter := &Exporter[Model, Output]{config: exportConfig[Model, Output]{formats: defaultExportFormats(), batch: 500, fn: fn}}
	if configure != nil {
		_ = configure(exporter)
	}
	builder.exported = true
	builder.exportAsync = exporter.config.queue != nil
	builder.exporter = func(builder *Builder[Model, Query], c *Context[Model]) error {
		return exporter.runExport(builder, c)
	}
	if !builder.hasAction(ExportAction) {
		builder.actions = append(builder.actions, ExportAction)
	}
	builder.register()
	return builder
}

// Summary overrides one action summary.
func (builder *Builder[Model, Query]) Summary(action Action, value string) *Builder[Model, Query] {
	if builder.summaries == nil {
		builder.summaries = make(map[Action]string)
	}
	builder.summaries[action] = value
	if item := builder.routes[action]; item != nil {
		item.Summary(value)
	}
	return builder
}

// Route returns a generated action route.
func (builder *Builder[Model, Query]) Route(action Action) *route.Route {
	return builder.routes[action]
}

func (builder *Builder[Model, Query]) register() {
	if builder.resource == nil || builder.store == nil {
		return
	}
	for _, action := range builder.actions {
		if builder.routes[action] != nil {
			continue
		}
		var item *route.Route
		switch action {
		case ListAction:
			item = builder.resource.RouteGroup().Get(builder.resource.Path(), builder.handle(action))
			builder.resourceRoute(action, item)
		case ShowAction:
			item = builder.resource.RouteGroup().Get(joinPath(builder.resource.Path(), "/{"+builder.options.key+"}"), builder.handle(action))
			builder.resourceRoute(action, item)
		case CreateAction:
			item = builder.resource.RouteGroup().Post(builder.resource.Path(), builder.handle(action))
			builder.resourceRoute(action, item)
		case EditAction:
			item = builder.resource.RouteGroup().Put(joinPath(builder.resource.Path(), "/{"+builder.options.key+"}"), builder.handle(action))
			builder.resourceRoute(action, item)
		case StoreAction:
			item = builder.resource.RouteGroup().Patch(joinPath(builder.resource.Path(), "/{"+builder.options.key+"}"), builder.handle(action))
			builder.resourceRoute(action, item)
		case DeleteAction:
			item = builder.resource.RouteGroup().Delete(joinPath(builder.resource.Path(), "/{"+builder.options.key+"}"), builder.handle(action))
			builder.resourceRoute(action, item)
		case BatchAction:
			item = builder.resource.RouteGroup().Post(joinPath(builder.resource.Path(), "/batch"), builder.handle(action))
			builder.resourceRoute(action, item)
		case RestoreAction:
			item = builder.resource.RouteGroup().Put(joinPath(builder.resource.Path(), "/{"+builder.options.key+"}/restore"), builder.handle(action))
			builder.resourceRoute(action, item)
		case DestroyAction:
			item = builder.resource.RouteGroup().Delete(joinPath(builder.resource.Path(), "/{"+builder.options.key+"}/destroy"), builder.handle(action))
			builder.resourceRoute(action, item)
		case ExportAction:
			item = builder.resource.RouteGroup().Get(joinPath(builder.resource.Path(), "/export"), builder.handle(action))
			builder.resourceRoute(action, item)
		case ImportAction:
			item = builder.resource.RouteGroup().Post(joinPath(builder.resource.Path(), "/import"), builder.handle(action))
			builder.resourceRoute(action, item)
		}
		if item != nil {
			builder.routes[action] = item
		}
	}
}

func (builder *Builder[Model, Query]) resourceRoute(action Action, item *route.Route) {
	name := string(action)
	if builder.resource.NameValue() != "" {
		name = builder.resource.NameValue() + "." + string(action)
	}
	item.Name(name).Meta("resource", builder.resource.NameValue()).Meta("action", string(action))
	if summary := builder.resource.SummaryValue(); summary != "" {
		item.Summary(defaultSummary(summary, action))
	}
	if override := builder.summaries[action]; override != "" {
		item.Summary(override)
	}
	builder.applyRouteSchema(action, item)
}

func (builder *Builder[Model, Query]) updateRouteSchemas() {
	for action, item := range builder.routes {
		if item != nil {
			builder.applyRouteSchema(action, item)
		}
	}
}

func (builder *Builder[Model, Query]) handle(action Action) route.Handler {
	return func(ctx *route.Context) error {
		c := newContext[Model](ctx, action, builder.options)
		for _, fn := range builder.init {
			if err := fn(c); err != nil {
				return err
			}
		}
		switch action {
		case ListAction:
			return builder.list(c)
		case ShowAction:
			return builder.show(c)
		case CreateAction:
			return builder.create(c)
		case EditAction:
			return builder.edit(c)
		case StoreAction:
			return builder.storeAction(c)
		case DeleteAction:
			return builder.delete(c)
		case BatchAction:
			return builder.batchAction(c)
		case RestoreAction:
			return builder.restore(c)
		case DestroyAction:
			return builder.destroy(c)
		case ExportAction:
			if builder.exporter == nil {
				return ctx.Error(http.StatusNotFound, "export action is not registered")
			}
			return builder.exporter(builder, c)
		case ImportAction:
			if builder.importer == nil {
				return ctx.Error(http.StatusNotFound, "import action is not registered")
			}
			return builder.importer(builder, c)
		default:
			return ctx.Error(http.StatusNotFound, "unknown action")
		}
	}
}

func (builder *Builder[Model, Query]) list(c *Context[Model]) error {
	query, err := builder.queryFor(c, true)
	if err != nil {
		return err
	}
	models, meta, err := builder.store.List(c, query)
	if err != nil {
		return err
	}
	items, err := builder.transformList(c, models)
	if err != nil {
		return err
	}
	if builder.options.tree != nil {
		return c.JSON(treeItems(items, *builder.options.tree))
	}
	if c.Pagination() == NoPageMode {
		return c.JSON(items)
	}
	payload := core.Map{"items": items}
	if meta != nil {
		payload["meta"] = meta.ListMeta()
	}
	if extra := builder.metaFor(c, Result[Model]{Models: models, ListMeta: meta}); len(extra) > 0 {
		payload["extra"] = extra
	}
	return c.JSON(payload)
}

func (builder *Builder[Model, Query]) show(c *Context[Model]) error {
	model, _, err := builder.loadOne(c)
	if err != nil {
		return err
	}
	return builder.respondModel(c, model, Result[Model]{Model: model})
}

func (builder *Builder[Model, Query]) create(c *Context[Model]) error {
	var model *Model
	if err := builder.store.Tx(c, func(c *Context[Model]) error {
		c.Model = new(Model)
		if err := builder.validateAndFormat(c); err != nil {
			return err
		}
		if err := builder.runBefore(c); err != nil {
			return err
		}
		created, err := builder.store.Create(c)
		if err != nil {
			return err
		}
		c.Model = created
		if err := builder.runAfter(c); err != nil {
			return err
		}
		model = created
		return nil
	}); err != nil {
		return err
	}
	return builder.respondModel(c, model, Result[Model]{Model: model})
}

func (builder *Builder[Model, Query]) edit(c *Context[Model]) error {
	var model *Model
	if err := builder.store.Tx(c, func(c *Context[Model]) error {
		current, query, err := builder.loadOne(c)
		if err != nil {
			return err
		}
		c.Model = current
		if err := builder.validateAndFormat(c); err != nil {
			return err
		}
		if err := builder.runBefore(c); err != nil {
			return err
		}
		updated, err := builder.store.Edit(c, query)
		if err != nil {
			return err
		}
		c.Model = updated
		if err := builder.runAfter(c); err != nil {
			return err
		}
		model = updated
		return nil
	}); err != nil {
		return err
	}
	return builder.respondModel(c, model, Result[Model]{Model: model})
}

func (builder *Builder[Model, Query]) storeAction(c *Context[Model]) error {
	var model *Model
	if err := builder.store.Tx(c, func(c *Context[Model]) error {
		current, query, err := builder.loadOne(c)
		if err != nil {
			return err
		}
		c.Model = current
		if err := builder.validateAndFormat(c); err != nil {
			return err
		}
		if len(c.Dirty()) == 0 {
			model = c.Model
			return nil
		}
		if err := builder.runBefore(c); err != nil {
			return err
		}
		updated, err := builder.store.Store(c, query, c.Dirty())
		if err != nil {
			return err
		}
		c.Model = updated
		if err := builder.runAfter(c); err != nil {
			return err
		}
		model = updated
		return nil
	}); err != nil {
		return err
	}
	return builder.respondModel(c, model, Result[Model]{Model: model})
}

func (builder *Builder[Model, Query]) delete(c *Context[Model]) error {
	if err := builder.store.Tx(c, func(c *Context[Model]) error {
		model, query, err := builder.loadOne(c)
		if err != nil {
			return err
		}
		c.Model = model
		if err := builder.runValidate(c); err != nil {
			return err
		}
		if err := builder.runBefore(c); err != nil {
			return err
		}
		if err := builder.store.Delete(c, query); err != nil {
			return err
		}
		return builder.runAfter(c)
	}); err != nil {
		return err
	}
	return c.JSON(core.Empty{})
}

func (builder *Builder[Model, Query]) batchAction(c *Context[Model]) error {
	if builder.batch == nil {
		return c.Error(http.StatusNotFound, "batch action is not registered")
	}
	var request BatchRequest
	if err := c.Bind(&request); err != nil {
		return err
	}
	output, err := builder.batch(c, request)
	if err != nil {
		return err
	}
	return c.JSON(output)
}

func (builder *Builder[Model, Query]) restore(c *Context[Model]) error {
	store, ok := builder.store.(SoftDeleteStore[Model, Query])
	if !ok {
		return c.Error(http.StatusInternalServerError, "soft delete store is required")
	}
	var model *Model
	if err := builder.store.Tx(c, func(c *Context[Model]) error {
		current, query, err := builder.loadDeleted(c, store)
		if err != nil {
			return err
		}
		c.Model = current
		if err := builder.runValidate(c); err != nil {
			return err
		}
		if err := builder.runBefore(c); err != nil {
			return err
		}
		restored, err := store.Restore(c, query)
		if err != nil {
			return err
		}
		c.Model = restored
		if err := builder.runAfter(c); err != nil {
			return err
		}
		model = restored
		return nil
	}); err != nil {
		return err
	}
	return builder.respondModel(c, model, Result[Model]{Model: model})
}

func (builder *Builder[Model, Query]) destroy(c *Context[Model]) error {
	store, ok := builder.store.(SoftDeleteStore[Model, Query])
	if !ok {
		return c.Error(http.StatusInternalServerError, "soft delete store is required")
	}
	if err := builder.store.Tx(c, func(c *Context[Model]) error {
		model, query, err := builder.loadDeleted(c, store)
		if err != nil {
			return err
		}
		c.Model = model
		if err := builder.runValidate(c); err != nil {
			return err
		}
		if err := builder.runBefore(c); err != nil {
			return err
		}
		if err := store.Destroy(c, query); err != nil {
			return err
		}
		return builder.runAfter(c)
	}); err != nil {
		return err
	}
	return c.JSON(core.Empty{})
}

func (builder *Builder[Model, Query]) loadDeleted(c *Context[Model], store SoftDeleteStore[Model, Query]) (*Model, Query, error) {
	query, err := builder.queryFor(c, false)
	if err != nil {
		var zero Query
		return nil, zero, err
	}
	model, err := store.ShowDeleted(c, query)
	if err != nil {
		var zero Query
		return nil, zero, err
	}
	if model == nil {
		var zero Query
		return nil, zero, c.Error(http.StatusNotFound, "not found")
	}
	return model, query, nil
}

func (builder *Builder[Model, Query]) loadOne(c *Context[Model]) (*Model, Query, error) {
	query, err := builder.queryFor(c, false)
	if err != nil {
		var zero Query
		return nil, zero, err
	}
	model, err := builder.store.Show(c, query)
	if err != nil {
		var zero Query
		return nil, zero, err
	}
	if model == nil {
		var zero Query
		return nil, zero, c.Error(http.StatusNotFound, "not found")
	}
	return model, query, nil
}

func (builder *Builder[Model, Query]) queryFor(c *Context[Model], list bool) (Query, error) {
	query, err := builder.store.Query(c)
	if err != nil {
		return query, err
	}
	for _, fn := range builder.query {
		query, err = fn(c, query)
		if err != nil {
			return query, err
		}
	}
	callbacks := builder.oneQuery
	if list {
		callbacks = builder.listQuery
	}
	for _, fn := range callbacks {
		query, err = fn(c, query)
		if err != nil {
			return query, err
		}
	}
	return query, nil
}

func (builder *Builder[Model, Query]) validateAndFormat(c *Context[Model]) error {
	if err := builder.runValidate(c); err != nil {
		return err
	}
	for _, fn := range builder.format.all(c.Action) {
		formatter := &Formatter[Model]{ctx: c}
		fn(c, formatter)
		if err := formatter.run(c.Action); err != nil {
			return err
		}
	}
	return nil
}

func (builder *Builder[Model, Query]) runValidate(c *Context[Model]) error {
	validator := validate.New(c.Model, c)
	for _, fn := range builder.validate.all(c.Action) {
		fn(c, validator)
	}
	return validator.Run()
}

func (builder *Builder[Model, Query]) runBefore(c *Context[Model]) error {
	for _, fn := range builder.before.all(c.Action) {
		if err := fn(c); err != nil {
			return err
		}
	}
	return nil
}

func (builder *Builder[Model, Query]) runAfter(c *Context[Model]) error {
	for _, fn := range builder.after.all(c.Action) {
		if err := fn(c); err != nil {
			return err
		}
	}
	return nil
}

func (builder *Builder[Model, Query]) respondModel(c *Context[Model], model *Model, result Result[Model]) error {
	output, err := builder.transformOne(c, model)
	if err != nil {
		return err
	}
	if extra := builder.metaFor(c, result); len(extra) > 0 {
		return c.JSON(core.Map{"data": output, "meta": extra})
	}
	return c.JSON(output)
}

func (builder *Builder[Model, Query]) transformOne(c *Context[Model], model *Model) (any, error) {
	if fn, ok := builder.transform.first(c.Action); ok {
		return fn(c, model)
	}
	return model, nil
}

func (builder *Builder[Model, Query]) transformList(c *Context[Model], models []*Model) ([]any, error) {
	items := make([]any, 0, len(models))
	for _, model := range models {
		item, err := builder.transformOne(c, model)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, nil
}

func (builder *Builder[Model, Query]) metaFor(c *Context[Model], result Result[Model]) core.Map {
	if fn, ok := builder.meta.first(c.Action); ok {
		return fn(c, result)
	}
	return nil
}

func (builder *Builder[Model, Query]) hasAction(action Action) bool {
	for _, item := range builder.actions {
		if item == action {
			return true
		}
	}
	return false
}

func joinPath(prefix string, path string) string {
	if prefix == "" || prefix == "/" {
		if path == "" {
			return "/"
		}
		return path
	}
	if path == "" || path == "/" {
		return prefix
	}
	if path[0] != '/' {
		path = "/" + path
	}
	return prefix + path
}

func defaultSummary(summary string, action Action) string {
	switch action {
	case ListAction:
		return summary + "列表"
	case ShowAction:
		return summary + "详情"
	case CreateAction:
		return "创建" + summary
	case EditAction:
		return "编辑" + summary
	case StoreAction:
		return "保存" + summary
	case DeleteAction:
		return "删除" + summary
	case RestoreAction:
		return "恢复" + summary
	case DestroyAction:
		return "彻底删除" + summary
	case ImportAction:
		return "导入" + summary
	case ExportAction:
		return "导出" + summary
	default:
		return summary
	}
}
