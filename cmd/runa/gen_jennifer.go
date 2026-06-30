package main

import (
	"bytes"
	"strings"

	. "github.com/dave/jennifer/jen"
)

func goSource(file *File) string {
	var out bytes.Buffer
	if err := file.Render(&out); err != nil {
		panic(err)
	}
	return out.String()
}

func moduleSource(data genData) string {
	file := NewFile(data.Package)
	file.ImportAlias(data.ModuleAdminImport, data.Package+"admin")
	file.ImportName("github.com/duxweb/runa/provider", "provider")
	file.ImportName("github.com/duxweb/runa/route", "route")
	file.Comment("Module is the " + data.Name + " business module entry.")
	file.Type().Id("Module").Struct(Qual("github.com/duxweb/runa/provider", "ModuleBase"))
	file.Comment("New creates the " + data.Name + " module.")
	file.Func().Id("New").Params().Id("Module").Block(Return(Id("Module").Values()))
	file.Func().Params(Id("Module")).Id("Name").Params().String().Block(Return(Lit(data.Name)))
	file.Func().Params(Id("Module")).Id("Register").Params(
		Id("ctx").Qual("context", "Context"),
		Id("app").Qual("github.com/duxweb/runa/provider", "Context"),
	).Error().Block(
		Id("_").Op("=").Id("ctx"),
		List(Id("routes"), Err()).Op(":=").Qual("github.com/duxweb/runa/provider", "Invoke").Types(Op("*").Qual("github.com/duxweb/runa/route", "Registry")).Call(Id("app")),
		If(Err().Op("!=").Nil()).Block(Return(Err())),
		Qual(data.ModuleAdminImport, "Register").Call(Id("routes").Dot("Group").Call(Lit("/"+data.Name))),
		Return(Nil()),
	)
	return goSource(file)
}

func moduleTestSource(data genData) string {
	file := NewFile(data.Package)
	file.Func().Id("TestModuleName").Params(Id("t").Op("*").Qual("testing", "T")).Block(
		If(Id("New").Call().Dot("Name").Call().Op("!=").Lit(data.Name)).Block(
			Id("t").Dot("Fatalf").Call(Lit("unexpected module name")),
		),
	)
	return goSource(file)
}

func adminRegisterSource(data genData) string {
	file := NewFile("admin")
	file.ImportName("github.com/duxweb/runa/route", "route")
	file.Comment("Register wires " + data.Module + " admin routes into the module group.")
	body := []Code{
		Id("group").Dot("Get").Call(
			Lit("/"),
			Func().Params(Id("ctx").Op("*").Qual("github.com/duxweb/runa/route", "Context")).Error().Block(
				Return(Id("ctx").Dot("JSON").Call(Map(String()).Any().Values(Dict{Lit("module"): Lit(data.Module)}))),
			),
		).Dot("Name").Call(Lit(data.Module + ".admin.index")),
	}
	for _, resource := range data.Resources {
		if resource.CRUD {
			body = append(body, Id("Register"+resource.Type+"CRUD").Call(Id("group"), Id("New"+resource.Type+"Store").Call()))
			continue
		}
		body = append(body, Id("Register"+resource.Type+"Resource").Call(Id("group")))
	}
	file.Func().Id("Register").Params(Id("group").Op("*").Qual("github.com/duxweb/runa/route", "Group")).Block(body...)
	return goSource(file)
}

func adminRegisterTestSource() string {
	file := NewFile("admin")
	file.ImportName("github.com/duxweb/runa/route", "route")
	file.Func().Id("TestRegister").Params(Id("t").Op("*").Qual("testing", "T")).Block(
		Id("registry").Op(":=").Qual("github.com/duxweb/runa/route", "New").Call(),
		Id("group").Op(":=").Qual("github.com/duxweb/runa/route", "NewGroup").Call(Id("registry"), Lit("")),
		Id("Register").Call(Id("group")),
		If(Len(Id("registry").Dot("Routes").Call()).Op("==").Lit(0)).Block(
			Id("t").Dot("Fatalf").Call(Lit("routes were not registered")),
		),
	)
	return goSource(file)
}

func resourceSource(data genData) string {
	ensureFields(&data)
	file := NewFile(data.Package)
	file.ImportName("github.com/duxweb/runa/core", "core")
	file.ImportName("github.com/duxweb/runa/resource", "resource")
	file.ImportName("github.com/duxweb/runa/route", "route")
	file.ImportName("github.com/duxweb/runa/validate", "validate")

	file.Type().Id(data.Type + "ListInput").Struct()
	file.Type().Id(data.Type + "ShowInput").Struct(Id("ID").String().Tag(map[string]string{"path": "id"}))
	file.Type().Id(data.Type + "CreateInput").Struct(fieldCodes(data, true)...)
	file.Func().Params(Id("input").Op("*").Id(data.Type + "CreateInput")).Id("Validate").Params(Id("v").Op("*").Qual("github.com/duxweb/runa/validate", "Validator")).Block(validateCodes(data)...)
	file.Type().Id(data.Type + "Output").Struct(outputFieldCodes(data)...)

	file.Comment("Register" + data.Type + "Resource registers conventional resource routes.")
	file.Func().Id("Register"+data.Type+"Resource").Params(Id("group").Op("*").Qual("github.com/duxweb/runa/route", "Group")).Op("*").Qual("github.com/duxweb/runa/resource", "Resource").Block(resourceRegisterCodes(data)...)
	return goSource(file)
}

func resourceTestSource(data genData) string {
	file := NewFile(data.Package)
	file.ImportName("github.com/duxweb/runa/route", "route")
	file.Func().Id("TestRegister"+data.Type+"Resource").Params(Id("t").Op("*").Qual("testing", "T")).Block(
		Id("registry").Op(":=").Qual("github.com/duxweb/runa/route", "New").Call(),
		Id("group").Op(":=").Qual("github.com/duxweb/runa/route", "NewGroup").Call(Id("registry"), Lit("")),
		Id("res").Op(":=").Id("Register"+data.Type+"Resource").Call(Id("group")),
		If(Id("res").Op("==").Nil().Op("||").Id("res").Dot("Route").Call(Lit("list")).Op("==").Nil().Op("||").Id("res").Dot("Route").Call(Lit("show")).Op("==").Nil().Op("||").Id("res").Dot("Route").Call(Lit("create")).Op("==").Nil()).Block(
			Id("t").Dot("Fatalf").Call(Lit("resource routes were not registered")),
		),
	)
	return goSource(file)
}

func crudSource(data genData) string {
	ensureFields(&data)
	file := NewFile(data.Package)
	file.ImportName("fmt", "fmt")
	file.ImportName("github.com/duxweb/runa/core", "core")
	file.ImportName("github.com/duxweb/runa/crud", "crud")
	file.ImportName("github.com/duxweb/runa/resource", "resource")
	file.ImportName("github.com/duxweb/runa/route", "route")
	file.Type().Id(data.Type + "Model").Struct(outputFieldCodes(data)...)
	file.Type().Id(data.Type + "Query").Struct(Id("ID").String())
	file.Type().Id(data.Type + "Output").Struct(outputFieldCodes(data)...)
	file.Type().Id(data.Var + "Store").Struct(Id("items").Map(String()).Op("*").Id(data.Type + "Model"))
	file.Func().Id("New" + data.Type + "Store").Params().Op("*").Id(data.Var + "Store").Block(
		Return(Op("&").Id(data.Var + "Store").Values(Dict{Id("items"): Map(String()).Op("*").Id(data.Type + "Model").Values()})),
	)
	file.Func().Params(Id("store").Op("*").Id(data.Var+"Store")).Id("Query").Params(Id("ctx").Op("*").Qual("github.com/duxweb/runa/crud", "Context").Types(Id(data.Type+"Model"))).Params(Id(data.Type+"Query"), Error()).Block(
		Return(Id(data.Type+"Query").Values(Dict{Id("ID"): Qual("github.com/duxweb/runa/route", "Param").Types(String()).Call(Id("ctx").Dot("Context"), Lit("id"))}), Nil()),
	)
	file.Func().Params(Id("store").Op("*").Id(data.Var+"Store")).Id("List").Params(Id("ctx").Op("*").Qual("github.com/duxweb/runa/crud", "Context").Types(Id(data.Type+"Model")), Id("query").Id(data.Type+"Query")).Params(Index().Op("*").Id(data.Type+"Model"), Qual("github.com/duxweb/runa/core", "ListMeta"), Error()).Block(
		Id("items").Op(":=").Make(Index().Op("*").Id(data.Type+"Model"), Lit(0), Len(Id("store").Dot("items"))),
		For(List(Id("_"), Id("item")).Op(":=").Range().Id("store").Dot("items")).Block(
			Id("copy").Op(":=").Op("*").Id("item"),
			Id("items").Op("=").Append(Id("items"), Op("&").Id("copy")),
		),
		Return(Id("items"), Qual("github.com/duxweb/runa/core", "PageMeta").Values(Dict{Id("Page"): Lit(1), Id("PageSize"): Len(Id("items")), Id("Total"): Len(Id("items"))}), Nil()),
	)
	file.Func().Params(Id("store").Op("*").Id(data.Var+"Store")).Id("Show").Params(Id("ctx").Op("*").Qual("github.com/duxweb/runa/crud", "Context").Types(Id(data.Type+"Model")), Id("query").Id(data.Type+"Query")).Params(Op("*").Id(data.Type+"Model"), Error()).Block(
		Id("item").Op(":=").Id("store").Dot("items").Index(Id("query").Dot("ID")),
		If(Id("item").Op("==").Nil()).Block(Return(Nil(), Qual("fmt", "Errorf").Call(Lit(data.Name+" %s not found"), Id("query").Dot("ID")))),
		Id("copy").Op(":=").Op("*").Id("item"),
		Return(Op("&").Id("copy"), Nil()),
	)
	file.Func().Params(Id("store").Op("*").Id(data.Var+"Store")).Id("Create").Params(Id("ctx").Op("*").Qual("github.com/duxweb/runa/crud", "Context").Types(Id(data.Type+"Model"))).Params(Op("*").Id(data.Type+"Model"), Error()).Block(
		If(Id("ctx").Dot("Model").Dot("ID").Op("==").Lit("")).Block(Id("ctx").Dot("Model").Dot("ID").Op("=").Qual("fmt", "Sprint").Call(Len(Id("store").Dot("items")).Op("+").Lit(1))),
		Id("copy").Op(":=").Op("*").Id("ctx").Dot("Model"),
		Id("store").Dot("items").Index(Id("copy").Dot("ID")).Op("=").Op("&").Id("copy"),
		Return(Id("ctx").Dot("Model"), Nil()),
	)
	file.Func().Params(Id("store").Op("*").Id(data.Var+"Store")).Id("Edit").Params(Id("ctx").Op("*").Qual("github.com/duxweb/runa/crud", "Context").Types(Id(data.Type+"Model")), Id("query").Id(data.Type+"Query")).Params(Op("*").Id(data.Type+"Model"), Error()).Block(
		Id("ctx").Dot("Model").Dot("ID").Op("=").Id("query").Dot("ID"),
		Id("copy").Op(":=").Op("*").Id("ctx").Dot("Model"),
		Id("store").Dot("items").Index(Id("query").Dot("ID")).Op("=").Op("&").Id("copy"),
		Return(Id("ctx").Dot("Model"), Nil()),
	)
	file.Func().Params(Id("store").Op("*").Id(data.Var+"Store")).Id("Store").Params(Id("ctx").Op("*").Qual("github.com/duxweb/runa/crud", "Context").Types(Id(data.Type+"Model")), Id("query").Id(data.Type+"Query"), Id("fields").Index().String()).Params(Op("*").Id(data.Type+"Model"), Error()).Block(Return(Id("store").Dot("Edit").Call(Id("ctx"), Id("query"))))
	file.Func().Params(Id("store").Op("*").Id(data.Var+"Store")).Id("Delete").Params(Id("ctx").Op("*").Qual("github.com/duxweb/runa/crud", "Context").Types(Id(data.Type+"Model")), Id("query").Id(data.Type+"Query")).Error().Block(Delete(Id("store").Dot("items"), Id("query").Dot("ID")), Return(Nil()))
	file.Func().Params(Id("store").Op("*").Id(data.Var+"Store")).Id("Tx").Params(Id("ctx").Op("*").Qual("github.com/duxweb/runa/crud", "Context").Types(Id(data.Type+"Model")), Id("fn").Func().Params(Id("ctx").Op("*").Qual("github.com/duxweb/runa/crud", "Context").Types(Id(data.Type+"Model"))).Error()).Error().Block(Return(Id("fn").Call(Id("ctx"))))
	file.Comment("Register" + data.Type + "CRUD registers CRUD routes. Replace New" + data.Type + "Store with orostore when using database/oro.")
	file.Func().Id("Register"+data.Type+"CRUD").Params(Id("group").Op("*").Qual("github.com/duxweb/runa/route", "Group"), Id("store").Qual("github.com/duxweb/runa/crud", "Store").Types(Id(data.Type+"Model"), Id(data.Type+"Query"))).Op("*").Qual("github.com/duxweb/runa/crud", "Builder").Types(Id(data.Type+"Model"), Id(data.Type+"Query")).Block(crudRegisterCodes(data)...)
	return goSource(file)
}

func crudTestSource(data genData) string {
	file := NewFile(data.Package)
	file.ImportName("github.com/duxweb/runa/route", "route")
	file.Func().Id("TestRegister"+data.Type+"CRUD").Params(Id("t").Op("*").Qual("testing", "T")).Block(
		Id("registry").Op(":=").Qual("github.com/duxweb/runa/route", "New").Call(),
		Id("group").Op(":=").Qual("github.com/duxweb/runa/route", "NewGroup").Call(Id("registry"), Lit("")),
		Id("builder").Op(":=").Id("Register"+data.Type+"CRUD").Call(Id("group"), Id("New"+data.Type+"Store").Call()),
		If(Id("builder").Op("==").Nil().Op("||").Id("builder").Dot("Route").Call(Lit("list")).Op("==").Nil().Op("||").Id("builder").Dot("Route").Call(Lit("show")).Op("==").Nil()).Block(
			Id("t").Dot("Fatalf").Call(Lit("crud routes were not registered")),
		),
	)
	return goSource(file)
}

func fieldCodes(data genData, form bool) []Code {
	items := make([]Code, 0, len(data.Fields))
	for _, field := range data.Fields {
		tags := map[string]string{"json": field.JSONName}
		if form {
			tags["form"] = field.JSONName
		}
		items = append(items, Id(field.GoName).Id(field.GoType).Tag(tags))
	}
	return items
}

func outputFieldCodes(data genData) []Code {
	items := []Code{Id("ID").String().Tag(map[string]string{"json": "id"})}
	items = append(items, fieldCodes(data, false)...)
	return items
}

func validateCodes(data genData) []Code {
	items := []Code{}
	for _, field := range data.Fields {
		if field.Required {
			items = append(items, Id("v").Dot("Field").Call(Lit(field.JSONName)).Dot("Value").Call(Id("input").Dot(field.GoName)).Dot("Required").Call(Lit(field.JSONName+"不能为空")))
		}
	}
	if len(items) == 0 {
		items = append(items, Id("_").Op("=").Id("v"))
	}
	return items
}

func resourceRegisterCodes(data genData) []Code {
	showValues := Dict{Id("ID"): Id("input").Dot("ID")}
	createValues := Dict{Id("ID"): Lit("new")}
	for _, field := range data.Fields {
		showValues[Id(field.GoName)] = zeroCode(field.GoType)
		createValues[Id(field.GoName)] = Id("input").Dot(field.GoName)
	}
	return []Code{
		Id("res").Op(":=").Qual("github.com/duxweb/runa/resource", "New").Call(Id("group"), Lit(data.RoutePath)).Dot("Name").Call(Lit(data.Name)).Dot("Summary").Call(Lit(data.Type)).Dot("Tags").Call(Lit(data.Type)),
		Id("res").Dot("List").Types(Id(data.Type+"ListInput"), Index().Id(data.Type+"Output")).Call(Func().Params(Id("ctx").Op("*").Qual("github.com/duxweb/runa/route", "Context"), Id("input").Op("*").Id(data.Type+"ListInput")).Params(Op("*").Index().Id(data.Type+"Output"), Error()).Block(Id("items").Op(":=").Index().Id(data.Type+"Output").Values(), Return(Op("&").Id("items"), Nil()))),
		Id("res").Dot("Show").Types(Id(data.Type+"ShowInput"), Id(data.Type+"Output")).Call(Func().Params(Id("ctx").Op("*").Qual("github.com/duxweb/runa/route", "Context"), Id("input").Op("*").Id(data.Type+"ShowInput")).Params(Op("*").Id(data.Type+"Output"), Error()).Block(Return(Op("&").Id(data.Type+"Output").Values(showValues), Nil()))),
		Id("res").Dot("Create").Types(Id(data.Type+"CreateInput"), Id(data.Type+"Output")).Call(Func().Params(Id("ctx").Op("*").Qual("github.com/duxweb/runa/route", "Context"), Id("input").Op("*").Id(data.Type+"CreateInput")).Params(Op("*").Id(data.Type+"Output"), Error()).Block(Return(Op("&").Id(data.Type+"Output").Values(createValues), Nil()))),
		Id("res").Dot("Get").Types(Qual("github.com/duxweb/runa/core", "Empty"), Qual("github.com/duxweb/runa/core", "Map")).Call(Lit("health"), Lit("/health"), Func().Params(Id("ctx").Op("*").Qual("github.com/duxweb/runa/route", "Context"), Id("input").Op("*").Qual("github.com/duxweb/runa/core", "Empty")).Params(Op("*").Qual("github.com/duxweb/runa/core", "Map"), Error()).Block(Id("out").Op(":=").Qual("github.com/duxweb/runa/core", "Map").Values(Dict{Lit("ok"): True()}), Return(Op("&").Id("out"), Nil()))),
		Return(Id("res")),
	}
}

func crudRegisterCodes(data genData) []Code {
	values := Dict{Id("ID"): Id("model").Dot("ID")}
	for _, field := range data.Fields {
		values[Id(field.GoName)] = Id("model").Dot(field.GoName)
	}
	return []Code{
		Id("res").Op(":=").Qual("github.com/duxweb/runa/resource", "New").Call(Id("group"), Lit(data.RoutePath)).Dot("Name").Call(Lit(data.Name)).Dot("Summary").Call(Lit(data.Type)).Dot("Tags").Call(Lit(data.Type)),
		Return(Qual("github.com/duxweb/runa/crud", "New").Types(Id(data.Type+"Model"), Id(data.Type+"Query")).Call(Id("res"), Id("store")).Dot("Transform").Types(Id(data.Type + "Output")).Call(Func().Params(Id("ctx").Op("*").Qual("github.com/duxweb/runa/crud", "Context").Types(Id(data.Type+"Model")), Id("model").Op("*").Id(data.Type+"Model")).Id(data.Type + "Output").Block(Return(Id(data.Type + "Output").Values(values))))),
	}
}

func ensureFields(data *genData) {
	if len(data.Fields) > 0 {
		return
	}
	data.Fields = []genField{{Name: "name", GoName: "Name", GoType: "string", JSONName: "name", Required: true}}
}

func zeroCode(goType string) Code {
	switch strings.TrimSpace(goType) {
	case "string":
		return Lit("")
	case "int", "int64", "int32", "float64", "float32":
		return Lit(0)
	case "bool":
		return False()
	default:
		return Id(goType).Values()
	}
}
