package crud

import (
	"reflect"

	"github.com/duxweb/runa/core"
	"github.com/duxweb/runa/route"
	"github.com/duxweb/runa/validate"
)

type transformSchema struct {
	output reflect.Type
	mapOut bool
}

func (builder *Builder[Model, Query]) applyRouteSchema(action Action, item *route.Route) {
	schema := route.SchemaInfo{
		InputFields:         builder.inputFields(action),
		RequestBody:         builder.requestBody(action),
		RequestContentType:  builder.requestContentType(action),
		Response:            builder.responseSchema(action),
		ResponseContentType: core.MIMEApplicationJSON,
	}
	if len(schema.InputFields) == 0 && schema.RequestBody == nil && schema.Response == nil {
		return
	}
	item.Schema(schema)
}

func (builder *Builder[Model, Query]) inputFields(action Action) []validate.FieldSchema {
	switch action {
	case ListAction:
		return builder.listInputFields(false)
	case ShowAction, EditAction, StoreAction, DeleteAction, RestoreAction, DestroyAction:
		return []validate.FieldSchema{pathField(builder.options.key, "id")}
	case ExportAction:
		return builder.listInputFields(true)
	}
	return nil
}

func (builder *Builder[Model, Query]) listInputFields(includeFormat bool) []validate.FieldSchema {
	fields := make([]validate.FieldSchema, 0)
	if builder.options.pagination == ScrollMode {
		cursor := builder.options.scrollFields.Cursor
		if cursor == "" {
			cursor = "cursor"
		}
		limit := builder.options.scrollFields.Limit
		if limit == "" {
			limit = "limit"
		}
		fields = append(fields, queryField(cursor), queryField(limit))
	} else if builder.options.pagination == PageMode || builder.options.pagination == "" {
		page := builder.options.pageFields.Page
		if page == "" {
			page = "page"
		}
		pageSize := builder.options.pageFields.PageSize
		if pageSize == "" {
			pageSize = "page_size"
		}
		fields = append(fields, queryField(page), queryField(pageSize))
	}
	for _, field := range builder.options.sortFields {
		if field.Name == "" {
			continue
		}
		fields = append(fields, queryField(field.Name+"_sort"))
	}
	for _, filter := range builder.options.filters {
		if filter.Name == "" {
			continue
		}
		fields = append(fields, queryField(filter.Name))
	}
	if includeFormat {
		fields = append(fields, validate.FieldSchema{Source: "query", Name: "format", Field: "format"})
	}
	return fields
}

func (builder *Builder[Model, Query]) requestBody(action Action) *route.TypeSchema {
	switch action {
	case CreateAction, EditAction, StoreAction:
		return &route.TypeSchema{Type: "object", Additional: true}
	case BatchAction:
		return route.SchemaOf(reflect.TypeOf(BatchRequest{}))
	case ImportAction:
		return &route.TypeSchema{Type: "object", Properties: map[string]*route.TypeSchema{
			"file": {Type: "string", Format: "binary"},
		}}
	default:
		return nil
	}
}

func (builder *Builder[Model, Query]) requestContentType(action Action) string {
	switch action {
	case ImportAction:
		return core.MIMEMultipartForm
	case CreateAction, EditAction, StoreAction, BatchAction:
		return core.MIMEApplicationJSON
	default:
		return ""
	}
}

func (builder *Builder[Model, Query]) responseSchema(action Action) *route.TypeSchema {
	switch action {
	case DeleteAction, DestroyAction:
		return route.SchemaOf(core.Empty{})
	case BatchAction:
		return &route.TypeSchema{Type: "object", Additional: true}
	case ImportAction:
		return route.SchemaOf(ImportResult{})
	case ExportAction:
		if builder.exportSchemaIsQueued() {
			return route.SchemaOf(ExportResult{})
		}
		return &route.TypeSchema{Type: "string", Format: "binary"}
	}
	transform, ok := builder.transformSchema(action)
	if !ok {
		return &route.TypeSchema{Type: "object", Additional: true}
	}
	itemSchema := &route.TypeSchema{Type: "object", Additional: true}
	if !transform.mapOut {
		itemSchema = route.SchemaOf(transform.output)
	}
	if action == ListAction {
		if builder.options.tree != nil || builder.options.pagination == NoPageMode {
			return &route.TypeSchema{Type: "array", Items: itemSchema}
		}
		return listSchema(itemSchema)
	}
	return itemSchema
}

func (builder *Builder[Model, Query]) transformSchema(action Action) (transformSchema, bool) {
	if schema, ok := builder.schemas.first(action); ok {
		return schema, true
	}
	return transformSchema{}, false
}

func (builder *Builder[Model, Query]) exportSchemaIsQueued() bool {
	return builder.exportAsync
}

func typeOf(value any) reflect.Type {
	valueType := reflect.TypeOf(value)
	if valueType == nil {
		return nil
	}
	return valueType
}

func listSchema(item *route.TypeSchema) *route.TypeSchema {
	return &route.TypeSchema{
		Type: "object",
		Properties: map[string]*route.TypeSchema{
			"items": {Type: "array", Items: item},
			"meta":  {Type: "object", Additional: true},
		},
	}
}

func pathField(name string, fallback string) validate.FieldSchema {
	if name == "" {
		name = fallback
	}
	return validate.FieldSchema{Source: "path", Name: name, Field: name, Type: "string"}
}

func queryField(name string) validate.FieldSchema {
	return validate.FieldSchema{Source: "query", Name: name, Field: name, Type: "string"}
}
