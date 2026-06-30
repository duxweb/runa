package route

import "testing"

type schemaInput struct {
	ID    int    `path:"id" label:"ID" desc:"用户ID"`
	Page  int    `query:"page" default:"1" label:"页码"`
	Token string `bind:"header:Authorization,cookie:token" label:"Token"`
}

func TestInputSchemaCollectsFieldMetadata(t *testing.T) {
	items := InputSchema[schemaInput]()
	if len(items) != 3 {
		t.Fatalf("items = %#v", items)
	}
	if items[0].Source != "path" || items[0].Name != "id" || items[0].Label != "ID" || items[0].Description != "用户ID" {
		t.Fatalf("id schema = %#v", items[0])
	}
	if items[1].Default != "1" || items[1].Label != "页码" {
		t.Fatalf("page schema = %#v", items[1])
	}
	if items[2].Source != "header" || items[2].Name != "Authorization" {
		t.Fatalf("token schema = %#v", items[2])
	}
}

type recursiveSchemaNode struct {
	ID       string                `json:"id" query:"id"`
	Child    *recursiveSchemaNode  `json:"child"`
	Children []recursiveSchemaNode `json:"children"`
}

type recursiveSchemaInput struct {
	ID     string `query:"id"`
	Parent *recursiveSchemaInput
}

type fidelitySchemaOutput struct {
	ID       int     `json:"id" required:"true" desc:"用户ID"`
	Name     string  `json:"name" default:"guest"`
	Nickname *string `json:"nickname"`
	Ignored  string  `json:"-"`
}

func TestSchemaHandlesRecursiveTypes(t *testing.T) {
	schema := SchemaOf(recursiveSchemaNode{})
	if schema == nil || schema.Properties["child"].Additional != true {
		t.Fatalf("child schema = %#v", schema)
	}
	if schema.Properties["children"].Items == nil || schema.Properties["children"].Items.Additional != true {
		t.Fatalf("children schema = %#v", schema.Properties["children"])
	}

	fields := InputSchema[recursiveSchemaInput]()
	if len(fields) != 1 || fields[0].Field != "ID" {
		t.Fatalf("fields = %#v", fields)
	}
}

func TestSchemaPreservesRequiredDefaultAndNullable(t *testing.T) {
	schema := SchemaOf(fidelitySchemaOutput{})
	if schema.Properties["id"].Description != "用户ID" {
		t.Fatalf("id schema = %#v", schema.Properties["id"])
	}
	if len(schema.Required) != 1 || schema.Required[0] != "id" {
		t.Fatalf("required = %#v", schema.Required)
	}
	if schema.Properties["name"].Default != "guest" {
		t.Fatalf("name schema = %#v", schema.Properties["name"])
	}
	if !schema.Properties["nickname"].Nullable {
		t.Fatalf("nickname schema = %#v", schema.Properties["nickname"])
	}
	if _, ok := schema.Properties["Ignored"]; ok {
		t.Fatalf("ignored leaked = %#v", schema.Properties)
	}
}
