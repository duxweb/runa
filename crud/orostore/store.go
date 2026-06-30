// Package orostore adapts Oro model queries to crud.Store.
package orostore

import (
	"context"
	"fmt"
	"reflect"
	"strings"

	"github.com/duxweb/oro"
	"github.com/duxweb/runa/core"
	"github.com/duxweb/runa/crud"
	"github.com/duxweb/runa/crud/filter"
	"github.com/duxweb/runa/route"
)

const txDBKey = "__orostore_tx_db"

// Store adapts an Oro database to CRUD.
func Store[Model any](db *oro.DB) *OroStore[Model] {
	return &OroStore[Model]{db: db}
}

// StoreFrom adapts a per-request Oro database resolver to CRUD.
func StoreFrom[Model any](fn func(*crud.Context[Model]) *oro.DB) *OroStore[Model] {
	return &OroStore[Model]{dbFor: fn}
}

// OroStore implements crud.Store for Oro.
type OroStore[Model any] struct {
	db    *oro.DB
	dbFor func(*crud.Context[Model]) *oro.DB
}

func (store *OroStore[Model]) database(ctx *crud.Context[Model]) (*oro.DB, error) {
	if ctx != nil {
		if tx, ok := ctx.Data[txDBKey].(*oro.DB); ok && tx != nil {
			return tx, nil
		}
	}
	if store.dbFor != nil {
		db := store.dbFor(ctx)
		if db == nil {
			return nil, fmt.Errorf("orostore database is nil")
		}
		return db, nil
	}
	if store.db == nil {
		return nil, fmt.Errorf("orostore database is nil")
	}
	return store.db, nil
}

// Query creates an Oro model query.
func (store *OroStore[Model]) Query(ctx *crud.Context[Model]) (*oro.ModelQuery[Model], error) {
	db, err := store.database(ctx)
	if err != nil {
		return nil, err
	}
	return db.Use[Model](), nil
}

// List returns models and list metadata.
func (store *OroStore[Model]) List(ctx *crud.Context[Model], query *oro.ModelQuery[Model]) ([]*Model, core.ListMeta, error) {
	query = store.applyRelations(ctx, store.applyFilters(ctx, query))
	query = store.applySort(ctx, query)

	switch ctx.Pagination() {
	case crud.NoPageMode:
		models, err := query.Get(requestContext(ctx))
		return models, nil, err
	case crud.ScrollMode:
		scroll := ctx.Scroll()
		if scroll.Cursor != "" {
			key, value, ok := store.keyValue(ctx, scroll.Cursor)
			if ok {
				query = query.Where(key, ">", value)
			}
		}
		limit := scroll.Limit
		if limit <= 0 {
			limit = 20
		}
		models, err := query.Limit(limit + 1).Get(requestContext(ctx))
		if err != nil {
			return nil, nil, err
		}
		next := ""
		if len(models) > limit {
			next = store.modelKeyString(ctx, models[limit])
			models = models[:limit]
		}
		return models, core.ScrollMeta{Cursor: scroll.Cursor, Limit: limit, Next: next}, nil
	default:
		page := ctx.Page()
		total, err := query.Count(requestContext(ctx))
		if err != nil {
			return nil, nil, err
		}
		models, err := query.Offset(page.Offset).Limit(page.Limit).Get(requestContext(ctx))
		return models, core.PageMeta{Page: page.Page, PageSize: page.PageSize, Total: int(total)}, err
	}
}

// Show returns one model.
func (store *OroStore[Model]) Show(ctx *crud.Context[Model], query *oro.ModelQuery[Model]) (*Model, error) {
	query, err := store.queryByKey(ctx, query)
	if err != nil {
		return nil, err
	}
	return store.one(ctx, store.applyRelations(ctx, query))
}

// Create inserts the current model.
func (store *OroStore[Model]) Create(ctx *crud.Context[Model]) (*Model, error) {
	query, err := store.Query(ctx)
	if err != nil {
		return nil, err
	}
	return query.Create(requestContext(ctx), ctx.Model)
}

// Edit updates the current model and returns a fresh copy.
func (store *OroStore[Model]) Edit(ctx *crud.Context[Model], query *oro.ModelQuery[Model]) (*Model, error) {
	fields := store.writeFields(ctx, ctx.Dirty())
	if len(fields) == 0 {
		scoped, err := store.queryByKey(ctx, query)
		if err != nil {
			return nil, err
		}
		_, err = scoped.Update(requestContext(ctx), store.modelValues(ctx, nil))
		if err != nil {
			return nil, err
		}
	} else {
		scoped, err := store.queryByKey(ctx, query)
		if err != nil {
			return nil, err
		}
		_, err = scoped.Update(requestContext(ctx), store.modelValues(ctx, fields), oro.Only(fields...))
		if err != nil {
			return nil, err
		}
	}
	return store.Show(ctx, query)
}

// Store updates selected fields and returns a fresh copy.
func (store *OroStore[Model]) Store(ctx *crud.Context[Model], query *oro.ModelQuery[Model], fields []string) (*Model, error) {
	writeFields := store.writeFields(ctx, fields)
	if len(writeFields) == 0 {
		return store.Show(ctx, query)
	}
	scoped, err := store.queryByKey(ctx, query)
	if err != nil {
		return nil, err
	}
	if _, err := scoped.Update(requestContext(ctx), store.modelValues(ctx, writeFields), oro.Only(writeFields...)); err != nil {
		return nil, err
	}
	return store.Show(ctx, query)
}

// Delete deletes the current model.
func (store *OroStore[Model]) Delete(ctx *crud.Context[Model], query *oro.ModelQuery[Model]) error {
	scoped, err := store.queryByKey(ctx, query)
	if err != nil {
		return err
	}
	_, err = scoped.Delete(requestContext(ctx))
	return err
}

// Tx runs fn in an Oro transaction.
func (store *OroStore[Model]) Tx(ctx *crud.Context[Model], fn func(ctx *crud.Context[Model]) error) error {
	db, err := store.database(ctx)
	if err != nil {
		return err
	}
	return db.Transaction(requestContext(ctx), func(tx *oro.DB) error {
		previous, hadPrevious := ctx.Data[txDBKey]
		ctx.Data[txDBKey] = tx
		defer func() {
			if hadPrevious {
				ctx.Data[txDBKey] = previous
			} else {
				delete(ctx.Data, txDBKey)
			}
		}()
		return fn(ctx)
	})
}

// ShowDeleted returns one soft-deleted model.
func (store *OroStore[Model]) ShowDeleted(ctx *crud.Context[Model], query *oro.ModelQuery[Model]) (*Model, error) {
	scoped, err := store.queryByKey(ctx, query.OnlyDeleted())
	if err != nil {
		return nil, err
	}
	return store.one(ctx, store.applyRelations(ctx, scoped))
}

// Restore restores a soft-deleted model.
func (store *OroStore[Model]) Restore(ctx *crud.Context[Model], query *oro.ModelQuery[Model]) (*Model, error) {
	var err error
	query, err = store.queryByKey(ctx, query.OnlyDeleted())
	if err != nil {
		return nil, err
	}
	if _, err := query.Restore(requestContext(ctx)); err != nil {
		return nil, err
	}
	return store.one(ctx, store.applyRelations(ctx, query.WithDeleted()))
}

// Destroy permanently deletes a soft-deleted model.
func (store *OroStore[Model]) Destroy(ctx *crud.Context[Model], query *oro.ModelQuery[Model]) error {
	scoped, err := store.queryByKey(ctx, query.OnlyDeleted())
	if err != nil {
		return err
	}
	_, err = scoped.ForceDelete(requestContext(ctx))
	return err
}

// Export streams models by chunk.
func (store *OroStore[Model]) Export(ctx *crud.Context[Model], query *oro.ModelQuery[Model], batch int, fn func(models []*Model) error) error {
	query = store.applyRelations(ctx, store.applyFilters(ctx, query))
	query = store.applySort(ctx, query)
	if batch <= 0 {
		batch = 500
	}
	return query.Chunk(requestContext(ctx), batch, fn)
}

func (store *OroStore[Model]) one(ctx *crud.Context[Model], query *oro.ModelQuery[Model]) (*Model, error) {
	return query.First(requestContext(ctx))
}

func (store *OroStore[Model]) queryByKey(ctx *crud.Context[Model], query *oro.ModelQuery[Model]) (*oro.ModelQuery[Model], error) {
	key, value, ok := store.keyValue(ctx, route.Param[string](ctx.Context, ctx.Key()))
	if !ok {
		return nil, fmt.Errorf("crud key %s is required", ctx.Key())
	}
	return query.Where(key, value), nil
}

func requestContext[Model any](ctx *crud.Context[Model]) context.Context {
	if ctx == nil || ctx.Request() == nil {
		return context.Background()
	}
	return ctx.Request().Context()
}

func (store *OroStore[Model]) applyRelations(ctx *crud.Context[Model], query *oro.ModelQuery[Model]) *oro.ModelQuery[Model] {
	for _, relation := range ctx.Relations() {
		if relation.Name == "" {
			continue
		}
		query = query.With(relation.Name)
	}
	return query
}

func (store *OroStore[Model]) applySort(ctx *crud.Context[Model], query *oro.ModelQuery[Model]) *oro.ModelQuery[Model] {
	for _, item := range ctx.Sort() {
		field := store.fieldName(ctx, item.Field)
		if field == "" {
			continue
		}
		if strings.EqualFold(item.Direction, "desc") {
			query = query.OrderByDesc(field)
			continue
		}
		query = query.OrderBy(field)
	}
	return query
}

func (store *OroStore[Model]) applyFilters(ctx *crud.Context[Model], query *oro.ModelQuery[Model]) *oro.ModelQuery[Model] {
	for _, item := range ctx.Filters() {
		switch item.Operator {
		case filter.EqOp, filter.SwitchOp:
			field := store.fieldName(ctx, item.Target)
			if field == "" {
				continue
			}
			query = query.Where(field, item.Value)
		case filter.LikeOp:
			field := store.fieldName(ctx, item.Target)
			if field == "" {
				continue
			}
			query = query.Where(oro.Field(field).Contains(fmt.Sprint(item.Value)))
		case filter.InOp:
			field := store.fieldName(ctx, item.Target)
			if field == "" {
				continue
			}
			query = query.Where(field, "in_values", anySlice(item.Value))
		case filter.BetweenOp:
			field := store.fieldName(ctx, item.Target)
			if field == "" {
				continue
			}
			values := anySlice(item.Value)
			if len(values) >= 2 {
				query = query.Where(field, "between", []any{values[0], values[1]})
			}
		case filter.SearchOp:
			fields := store.searchFieldNames(ctx, item)
			if len(fields) == 0 {
				continue
			}
			query = query.Where(oro.FullText(fields...).Match(fmt.Sprint(item.Value)))
		}
	}
	return query
}

func (store *OroStore[Model]) keyValue(ctx *crud.Context[Model], raw any) (string, any, bool) {
	field := store.fieldName(ctx, ctx.Key())
	if field == "" {
		field = ctx.Key()
	}
	value, ok := store.castField(ctx, field, raw)
	if !ok {
		value = raw
	}
	if value == "" || value == nil {
		return field, value, false
	}
	return field, value, true
}

func (store *OroStore[Model]) writeFields(ctx *crud.Context[Model], fields []string) []string {
	output := make([]string, 0, len(fields))
	seen := make(map[string]struct{}, len(fields))
	for _, field := range fields {
		name := store.fieldName(ctx, field)
		if name == "" {
			name = field
		}
		if name == "" {
			continue
		}
		if _, ok := seen[name]; ok {
			continue
		}
		seen[name] = struct{}{}
		output = append(output, name)
	}
	return output
}

func (store *OroStore[Model]) fieldName(ctx *crud.Context[Model], name string) string {
	if name == "" {
		return ""
	}
	schema := store.schema(ctx)
	if schema == nil {
		return exportedName(name)
	}
	if field, ok := schema.FieldByGo[name]; ok {
		return field.Name
	}
	if field, ok := schema.FieldByDB[name]; ok {
		return field.Name
	}
	candidates := []string{name, exportedName(name), strings.ReplaceAll(name, "_", "")}
	for _, candidate := range candidates {
		for _, field := range schema.Fields {
			if strings.EqualFold(field.Name, candidate) || strings.EqualFold(field.Column, candidate) {
				return field.Name
			}
		}
	}
	return exportedName(name)
}

func (store *OroStore[Model]) schema(ctx *crud.Context[Model]) *oro.ModelSchema {
	db, err := store.database(ctx)
	if err != nil {
		return nil
	}
	schema, err := oro.SchemaOf[Model](db)
	if err != nil {
		return nil
	}
	return schema
}

func (store *OroStore[Model]) castField(ctx *crud.Context[Model], field string, value any) (any, bool) {
	schema := store.schema(ctx)
	if schema == nil {
		return value, true
	}
	info, ok := schema.FieldByGo[field]
	if !ok {
		if byDB, found := schema.FieldByDB[field]; found {
			info = byDB
			ok = true
		}
	}
	if !ok {
		return value, true
	}
	target := modelFieldValue(ctx.Model, info.Index)
	if !target.IsValid() {
		var zero Model
		target = modelFieldValue(&zero, info.Index)
	}
	if !target.IsValid() {
		return value, true
	}
	return castToType(value, target.Type())
}

func (store *OroStore[Model]) modelValues(ctx *crud.Context[Model], fields []string) oro.Map {
	values := make(oro.Map)
	if ctx.Model == nil {
		return values
	}
	schema := store.schema(ctx)
	if schema == nil {
		return values
	}
	allowed := make(map[string]struct{}, len(fields))
	for _, field := range fields {
		allowed[field] = struct{}{}
	}
	for _, field := range schema.Fields {
		if field.Primary || field.Virtual || field.AutoCreate || field.AutoUpdate {
			continue
		}
		if len(allowed) > 0 {
			if _, ok := allowed[field.Name]; !ok {
				continue
			}
		}
		value := modelFieldValue(ctx.Model, field.Index)
		if !value.IsValid() {
			continue
		}
		values[field.Name] = value.Interface()
	}
	return values
}

func (store *OroStore[Model]) modelKeyString(ctx *crud.Context[Model], model *Model) string {
	if model == nil {
		return ""
	}
	schema := store.schema(ctx)
	if schema == nil {
		return ""
	}
	fieldName := store.fieldName(ctx, ctx.Key())
	field, ok := schema.FieldByGo[fieldName]
	if !ok {
		return ""
	}
	value := modelFieldValue(model, field.Index)
	if !value.IsValid() {
		return ""
	}
	return fmt.Sprint(value.Interface())
}

func modelFieldValue(model any, index []int) reflect.Value {
	value := reflect.ValueOf(model)
	for value.Kind() == reflect.Pointer {
		if value.IsNil() {
			return reflect.Value{}
		}
		value = value.Elem()
	}
	if value.Kind() != reflect.Struct {
		return reflect.Value{}
	}
	field := value.FieldByIndex(index)
	for field.Kind() == reflect.Pointer {
		if field.IsNil() {
			return reflect.Value{}
		}
		field = field.Elem()
	}
	if !field.IsValid() || !field.CanInterface() {
		return reflect.Value{}
	}
	return field
}

func castToType(value any, target reflect.Type) (any, bool) {
	if value == nil {
		return nil, false
	}
	source := reflect.ValueOf(value)
	if source.IsValid() && source.Type().AssignableTo(target) {
		return value, true
	}
	switch target.Kind() {
	case reflect.String:
		return core.Cast[string](value), true
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		parsed, ok := core.CastOK[int64](value)
		if !ok {
			return nil, false
		}
		output := reflect.New(target).Elem()
		if output.OverflowInt(parsed) {
			return nil, false
		}
		output.SetInt(parsed)
		return output.Interface(), true
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		parsed, ok := core.CastOK[uint64](value)
		if !ok {
			return nil, false
		}
		output := reflect.New(target).Elem()
		if output.OverflowUint(parsed) {
			return nil, false
		}
		output.SetUint(parsed)
		return output.Interface(), true
	case reflect.Bool:
		parsed, ok := core.CastOK[bool](value)
		return parsed, ok
	case reflect.Float32, reflect.Float64:
		parsed, ok := core.CastOK[float64](value)
		if !ok {
			return nil, false
		}
		output := reflect.New(target).Elem()
		if output.OverflowFloat(parsed) {
			return nil, false
		}
		output.SetFloat(parsed)
		return output.Interface(), true
	default:
		return value, true
	}
}

func anySlice(value any) []any {
	if value == nil {
		return nil
	}
	if values, ok := value.([]any); ok {
		return values
	}
	reflected := reflect.ValueOf(value)
	if reflected.Kind() != reflect.Slice && reflected.Kind() != reflect.Array {
		return []any{value}
	}
	values := make([]any, 0, reflected.Len())
	for index := 0; index < reflected.Len(); index++ {
		values = append(values, reflected.Index(index).Interface())
	}
	return values
}

func searchFields(item filter.Value) []string {
	if item.Meta == nil {
		return nil
	}
	values, ok := item.Meta["fields"].([]string)
	if ok {
		return values
	}
	raw := anySlice(item.Meta["fields"])
	fields := make([]string, 0, len(raw))
	for _, field := range raw {
		if value := fmt.Sprint(field); value != "" {
			fields = append(fields, value)
		}
	}
	return fields
}

func (store *OroStore[Model]) searchFieldNames(ctx *crud.Context[Model], item filter.Value) []string {
	fields := searchFields(item)
	output := make([]string, 0, len(fields))
	for _, field := range fields {
		name := store.fieldName(ctx, field)
		if name != "" {
			output = append(output, name)
		}
	}
	return output
}

func exportedName(name string) string {
	if name == "" {
		return ""
	}
	parts := strings.FieldsFunc(name, func(r rune) bool {
		return r == '_' || r == '-' || r == '.'
	})
	if len(parts) == 0 {
		return strings.ToUpper(name[:1]) + name[1:]
	}
	for index, part := range parts {
		if part == "" {
			continue
		}
		parts[index] = strings.ToUpper(part[:1]) + part[1:]
	}
	return strings.Join(parts, "")
}

var _ crud.Store[struct{}, *oro.ModelQuery[struct{}]] = (*OroStore[struct{}])(nil)
var _ crud.SoftDeleteStore[struct{}, *oro.ModelQuery[struct{}]] = (*OroStore[struct{}])(nil)
var _ crud.ExportStore[struct{}, *oro.ModelQuery[struct{}]] = (*OroStore[struct{}])(nil)
