package main

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/BurntSushi/toml"
	"github.com/urfave/cli/v3"
)

type specFile struct {
	Module    string         `toml:"module"`
	Resources []specResource `toml:"resources"`
}

type specResource struct {
	Name   string      `toml:"name"`
	Layer  string      `toml:"layer"`
	CRUD   bool        `toml:"crud"`
	Fields []specField `toml:"fields"`
}

type specField struct {
	Name     string `toml:"name"`
	Type     string `toml:"type"`
	Required bool   `toml:"required"`
}

func genSpec(_ context.Context, cmd *cli.Command) error {
	args := cmd.Args().Slice()
	if len(args) == 0 || args[0] == "" {
		return fmt.Errorf("usage: runa gen spec <file>")
	}
	var spec specFile
	if _, err := toml.DecodeFile(args[0], &spec); err != nil {
		return err
	}
	module := normalizeName(spec.Module)
	if module == "" || module == "item" {
		return fmt.Errorf("spec module is required")
	}
	base := cmd.String("dir")
	moduleData, err := genDataForName(base, module, "", cmd.String("go-module"))
	if err != nil {
		return err
	}
	for _, resource := range spec.Resources {
		name := normalizeName(resource.Name)
		if name == "" || name == "item" {
			return fmt.Errorf("resource name is required")
		}
		layer := normalizeName(resource.Layer)
		if layer == "" || layer == "item" {
			layer = "admin"
		}
		if layer == "admin" {
			moduleData.Resources = append(moduleData.Resources, genResourceItem{
				Name: name,
				Type: exportedName(name),
				Var:  lowerCamel(name),
				CRUD: resource.CRUD,
			})
		}
	}
	files := []generatedFile{
		{Path: filepath.Join(moduleData.Path, "module.go"), Body: moduleSource(moduleData), Go: true},
		{Path: filepath.Join(moduleData.Path, "module_test.go"), Body: moduleTestSource(moduleData), Go: true},
		{Path: filepath.Join(moduleData.Path, "admin", "register.go"), Body: adminRegisterSource(moduleData), Go: true},
		{Path: filepath.Join(moduleData.Path, "admin", "register_test.go"), Body: adminRegisterTestSource(), Go: true},
	}
	for _, dir := range []string{"api", "service", "models", "handler", "command", "listener", "queue", "schedule", "middleware"} {
		files = append(files, generatedFile{Path: filepath.Join(moduleData.Path, dir, ".gitkeep"), Body: ""})
	}
	for _, resource := range spec.Resources {
		data, err := specResourceData(base, module, cmd.String("go-module"), resource)
		if err != nil {
			return err
		}
		if resource.CRUD {
			files = append(files,
				generatedFile{Path: filepath.Join(data.Path, data.Var+"_crud.go"), Body: crudSource(data), Go: true},
				generatedFile{Path: filepath.Join(data.Path, data.Var+"_crud_test.go"), Body: crudTestSource(data), Go: true},
			)
			continue
		}
		files = append(files,
			generatedFile{Path: filepath.Join(data.Path, data.Var+"_resource.go"), Body: resourceSource(data), Go: true},
			generatedFile{Path: filepath.Join(data.Path, data.Var+"_resource_test.go"), Body: resourceTestSource(data), Go: true},
		)
	}
	return writeGenerated(cmd.Bool("force"), files)
}

func specResourceData(base string, module string, goModule string, resource specResource) (genData, error) {
	name := normalizeName(resource.Name)
	if name == "" || name == "item" {
		return genData{}, fmt.Errorf("resource name is required")
	}
	layer := normalizeName(resource.Layer)
	if layer == "" || layer == "item" {
		layer = "admin"
	}
	data, err := genDataForName(filepath.Join(base, module), name, layer, goModule)
	if err != nil {
		return genData{}, err
	}
	data.Module = module
	data.ModuleType = exportedName(module)
	data.ModulePkg = packageName(module)
	data.ModulePath = filepath.Join(base, module)
	data.ModuleAdminImport = packageImport(data.GoModule, base, module, "admin")
	data.Path = filepath.Join(base, module, layer)
	data.Layer = layer
	data.Package = packageName(layer)
	data.RoutePath = "/" + module + "/" + pluralName(kebabName(name))
	data.Fields = specFields(resource.Fields)
	return data, nil
}

func specFields(fields []specField) []genField {
	items := make([]genField, 0, len(fields))
	for _, field := range fields {
		name := normalizeName(field.Name)
		if name == "" || name == "item" {
			continue
		}
		items = append(items, genField{
			Name:     name,
			GoName:   exportedName(name),
			GoType:   goFieldType(field.Type),
			JSONName: kebabToSnake(name),
			Required: field.Required,
		})
	}
	return items
}

func goFieldType(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "", "string":
		return "string"
	case "int", "integer":
		return "int"
	case "int64":
		return "int64"
	case "float", "float64", "number":
		return "float64"
	case "bool", "boolean":
		return "bool"
	default:
		return exportedName(value)
	}
}

func kebabToSnake(value string) string {
	return strings.ReplaceAll(normalizeName(value), "-", "_")
}
