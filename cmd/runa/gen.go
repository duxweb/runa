package main

import (
	"bytes"
	"context"
	"fmt"
	"go/format"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"text/template"
	"unicode"

	"github.com/urfave/cli/v3"
)

type genData struct {
	Name              string
	Package           string
	Module            string
	ModuleType        string
	ModulePath        string
	ModulePkg         string
	ModuleAdminImport string
	Type              string
	Var               string
	Fields            []genField
	Path              string
	Layer             string
	RoutePath         string
	GoModule          string
	Cap               string
	CapPackage        string
	CapImport         string
	Driver            string
	RootReplace       string
	Resources         []genResourceItem
}

type genField struct {
	Name     string
	GoName   string
	GoType   string
	JSONName string
	Required bool
}

type genResourceItem struct {
	Name string
	Type string
	Var  string
	CRUD bool
}

type generatedFile struct {
	Path string
	Body string
	Go   bool
}

func genCommands() []*cli.Command {
	return []*cli.Command{
		{Name: "module", Usage: "Generate a business module", Flags: genFlags("app"), Action: genModule},
		{Name: "resource", Usage: "Generate a route resource", Flags: resourceGenFlags(), Action: genResource},
		{Name: "crud", Usage: "Generate a CRUD resource with a buildable store skeleton", Flags: resourceGenFlags(), Action: genCRUD},
		{Name: "provider", Usage: "Generate a provider skeleton", Flags: genFlags("."), Action: genProvider},
		{Name: "capability", Usage: "Generate a capability package skeleton", Flags: genFlags("."), Action: genCapability},
		{Name: "driver", Usage: "Generate a capability driver submodule", Flags: driverGenFlags(), Action: genDriver},
		{Name: "spec", Usage: "Generate code from a Runa TOML spec", Flags: specGenFlags(), Action: genSpec},
	}
}

func genFlags(base string) []cli.Flag {
	return []cli.Flag{
		&cli.StringFlag{Name: "dir", Value: base, Usage: "Output base directory"},
		&cli.StringFlag{Name: "package", Usage: "Go package name"},
		&cli.StringFlag{Name: "go-module", Usage: "Go module import path for generated business code"},
		&cli.BoolFlag{Name: "force", Usage: "Overwrite existing files"},
	}
}

func driverGenFlags() []cli.Flag {
	flags := genFlags(".")
	flags = append(flags, &cli.StringFlag{Name: "module", Usage: "Module path for generated go.mod"})
	return flags
}

func resourceGenFlags() []cli.Flag {
	flags := genFlags("app")
	flags = append(flags,
		&cli.StringFlag{Name: "module", Usage: "Business module name; output goes to app/<module>/<layer>"},
		&cli.StringFlag{Name: "layer", Value: "admin", Usage: "Module subdirectory for generated route code"},
	)
	return flags
}

func specGenFlags() []cli.Flag {
	return []cli.Flag{
		&cli.StringFlag{Name: "dir", Value: "app", Usage: "Output base directory"},
		&cli.StringFlag{Name: "go-module", Usage: "Go module import path for generated business code"},
		&cli.BoolFlag{Name: "force", Usage: "Overwrite existing files"},
	}
}

func genModule(_ context.Context, cmd *cli.Command) error {
	data, err := oneNameData(cmd)
	if err != nil {
		return err
	}
	return writeGenerated(cmd.Bool("force"), []generatedFile{
		{Path: filepath.Join(data.Path, "module.go"), Body: moduleSource(data), Go: true},
		{Path: filepath.Join(data.Path, "module_test.go"), Body: moduleTestSource(data), Go: true},
		{Path: filepath.Join(data.Path, "admin", "register.go"), Body: adminRegisterSource(data), Go: true},
		{Path: filepath.Join(data.Path, "admin", "register_test.go"), Body: adminRegisterTestSource(), Go: true},
		{Path: filepath.Join(data.Path, "api", ".gitkeep"), Body: ""},
		{Path: filepath.Join(data.Path, "service", ".gitkeep"), Body: ""},
		{Path: filepath.Join(data.Path, "models", ".gitkeep"), Body: ""},
		{Path: filepath.Join(data.Path, "handler", ".gitkeep"), Body: ""},
		{Path: filepath.Join(data.Path, "command", ".gitkeep"), Body: ""},
		{Path: filepath.Join(data.Path, "listener", ".gitkeep"), Body: ""},
		{Path: filepath.Join(data.Path, "queue", ".gitkeep"), Body: ""},
		{Path: filepath.Join(data.Path, "schedule", ".gitkeep"), Body: ""},
		{Path: filepath.Join(data.Path, "middleware", ".gitkeep"), Body: ""},
	})
}

func genResource(_ context.Context, cmd *cli.Command) error {
	data, err := resourceData(cmd)
	if err != nil {
		return err
	}
	return writeGenerated(cmd.Bool("force"), []generatedFile{
		{Path: filepath.Join(data.Path, data.Var+"_resource.go"), Body: resourceSource(data), Go: true},
		{Path: filepath.Join(data.Path, data.Var+"_resource_test.go"), Body: resourceTestSource(data), Go: true},
	})
}

func genCRUD(_ context.Context, cmd *cli.Command) error {
	data, err := resourceData(cmd)
	if err != nil {
		return err
	}
	return writeGenerated(cmd.Bool("force"), []generatedFile{
		{Path: filepath.Join(data.Path, data.Var+"_crud.go"), Body: crudSource(data), Go: true},
		{Path: filepath.Join(data.Path, data.Var+"_crud_test.go"), Body: crudTestSource(data), Go: true},
	})
}

func genProvider(_ context.Context, cmd *cli.Command) error {
	data, err := oneNameData(cmd)
	if err != nil {
		return err
	}
	return writeGenerated(cmd.Bool("force"), []generatedFile{
		{Path: filepath.Join(data.Path, "provider.go"), Body: renderTemplate(providerTemplate, data), Go: true},
		{Path: filepath.Join(data.Path, "provider_test.go"), Body: renderTemplate(providerTestTemplate, data), Go: true},
	})
}

func genCapability(_ context.Context, cmd *cli.Command) error {
	data, err := oneNameData(cmd)
	if err != nil {
		return err
	}
	return writeGenerated(cmd.Bool("force"), []generatedFile{
		{Path: filepath.Join(data.Path, "go.mod"), Body: renderTemplate(capabilityGoModTemplate, data)},
		{Path: filepath.Join(data.Path, "types.go"), Body: renderTemplate(capabilityTypesTemplate, data), Go: true},
		{Path: filepath.Join(data.Path, "options.go"), Body: renderTemplate(capabilityOptionsTemplate, data), Go: true},
		{Path: filepath.Join(data.Path, "registry.go"), Body: renderTemplate(capabilityRegistryTemplate, data), Go: true},
		{Path: filepath.Join(data.Path, "provider.go"), Body: renderTemplate(capabilityProviderTemplate, data), Go: true},
		{Path: filepath.Join(data.Path, "default.go"), Body: renderTemplate(capabilityDefaultTemplate, data), Go: true},
		{Path: filepath.Join(data.Path, "registry_test.go"), Body: renderTemplate(capabilityTestTemplate, data), Go: true},
	})
}

func genDriver(_ context.Context, cmd *cli.Command) error {
	args := cmd.Args().Slice()
	if len(args) < 2 {
		return fmt.Errorf("usage: runa gen driver <capability> <name>")
	}
	modulePath, moduleRoot, runaReplace, err := currentModuleInfo()
	if err != nil {
		modulePath = "github.com/duxweb/runa"
	}
	if value := cmd.String("module"); value != "" {
		modulePath = value
	}
	capName := normalizeName(args[0])
	driverName := normalizeName(args[1])
	pkg := cmd.String("package")
	if pkg == "" {
		pkg = packageName(driverName)
	}
	base := cmd.String("dir")
	path := filepath.Join(base, capName, driverName)
	data := genData{
		Name:        driverName,
		Package:     pkg,
		Type:        exportedName(driverName),
		Var:         lowerCamel(driverName),
		Path:        path,
		GoModule:    modulePath,
		Cap:         capName,
		CapPackage:  packageName(capName),
		CapImport:   strings.TrimRight(modulePath, "/") + "/" + capName,
		Driver:      packageName(driverName),
		RootReplace: rootReplace(modulePath, moduleRoot, runaReplace, path),
	}
	return writeGenerated(cmd.Bool("force"), []generatedFile{
		{Path: filepath.Join(path, "go.mod"), Body: renderTemplate(driverGoModTemplate, data)},
		{Path: filepath.Join(path, "options.go"), Body: renderTemplate(driverOptionsTemplate, data), Go: true},
		{Path: filepath.Join(path, "driver.go"), Body: renderTemplate(driverTemplate, data), Go: true},
		{Path: filepath.Join(path, "driver_test.go"), Body: renderTemplate(driverTestTemplate, data), Go: true},
	})
}

func oneNameData(cmd *cli.Command) (genData, error) {
	args := cmd.Args().Slice()
	if len(args) == 0 || args[0] == "" {
		return genData{}, fmt.Errorf("name is required")
	}
	return genDataForName(cmd.String("dir"), args[0], cmd.String("package"), cmd.String("go-module"))
}

func genDataForName(base string, rawName string, rawPackage string, rawGoModule string) (genData, error) {
	name := normalizeName(rawName)
	pkg := rawPackage
	if pkg == "" {
		pkg = packageName(name)
	} else {
		pkg = packageName(pkg)
	}
	path := filepath.Join(base, name)
	if base == "." {
		path = name
	}
	modulePath, moduleRoot, runaReplace, err := moduleInfoForTarget(path)
	if err != nil {
		modulePath = "github.com/duxweb/runa"
	}
	if rawGoModule != "" {
		modulePath = strings.TrimRight(rawGoModule, "/")
	}
	return genData{
		Name:              name,
		Package:           pkg,
		Module:            name,
		ModuleType:        exportedName(name),
		ModulePath:        path,
		ModulePkg:         pkg,
		ModuleAdminImport: packageImport(modulePath, base, name, "admin"),
		Type:              exportedName(name),
		Var:               lowerCamel(name),
		Path:              path,
		Layer:             "",
		RoutePath:         "/" + pluralName(kebabName(name)),
		GoModule:          modulePath,
		RootReplace:       rootReplace(modulePath, moduleRoot, runaReplace, path),
	}, nil
}

func resourceData(cmd *cli.Command) (genData, error) {
	data, err := oneNameData(cmd)
	if err != nil {
		return genData{}, err
	}
	layer := normalizeName(cmd.String("layer"))
	if layer == "" || layer == "item" {
		layer = "admin"
	}
	module := normalizeName(cmd.String("module"))
	if module == "" || module == "item" {
		data.Layer = layer
		return data, nil
	}
	base := cmd.String("dir")
	modulePath := filepath.Join(base, module)
	layerPath := filepath.Join(modulePath, layer)
	data.Module = module
	data.ModuleType = exportedName(module)
	data.ModulePath = modulePath
	data.ModulePkg = packageName(module)
	data.ModuleAdminImport = packageImport(data.GoModule, base, module, "admin")
	data.Layer = layer
	data.Package = packageName(layer)
	if pkg := cmd.String("package"); pkg != "" {
		data.Package = packageName(pkg)
	}
	data.Path = layerPath
	data.RoutePath = "/" + module + "/" + pluralName(kebabName(data.Name))
	data.RootReplace = rootReplace(data.GoModule, filepath.Dir(modulePath), "", layerPath)
	return data, nil
}

func writeGenerated(force bool, files []generatedFile) error {
	for _, file := range files {
		if !force {
			if _, err := os.Stat(file.Path); err == nil {
				return fmt.Errorf("%s already exists", file.Path)
			} else if err != nil && !os.IsNotExist(err) {
				return err
			}
		}
	}
	for _, file := range files {
		body := strings.TrimLeft(file.Body, "\n")
		if file.Go {
			formatted, err := format.Source([]byte(body))
			if err != nil {
				return fmt.Errorf("format %s: %w", file.Path, err)
			}
			body = string(formatted)
		}
		if err := os.MkdirAll(filepath.Dir(file.Path), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(file.Path, []byte(body), 0o644); err != nil {
			return err
		}
		fmt.Println("created", file.Path)
	}
	return nil
}

func renderTemplate(body string, data genData) string {
	tpl := template.Must(template.New("gen").Parse(body))
	var out bytes.Buffer
	if err := tpl.Execute(&out, data); err != nil {
		panic(err)
	}
	return out.String()
}

func currentModuleInfo() (string, string, string, error) {
	cmd := exec.Command("go", "list", "-m")
	cmd.Env = append(os.Environ(), "GOWORK=off")
	body, err := cmd.Output()
	if err != nil {
		return "", "", "", err
	}
	gomod := exec.Command("go", "env", "GOMOD")
	gomod.Env = append(os.Environ(), "GOWORK=off")
	modFile, err := gomod.Output()
	if err != nil {
		return strings.TrimSpace(string(body)), "", "", nil
	}
	moduleRoot := filepath.Dir(strings.TrimSpace(string(modFile)))
	return strings.TrimSpace(string(body)), moduleRoot, runaReplaceDir(moduleRoot), nil
}

func moduleInfoForTarget(targetPath string) (string, string, string, error) {
	abs, err := filepath.Abs(targetPath)
	if err != nil {
		return currentModuleInfo()
	}
	root, err := findGoModUp(abs)
	if err != nil {
		return currentModuleInfo()
	}
	modulePath, err := modulePathFromGoMod(filepath.Join(root, "go.mod"))
	if err != nil {
		return currentModuleInfo()
	}
	return modulePath, root, runaReplaceDir(root), nil
}

func modulePathFromGoMod(path string) (string, error) {
	body, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	for _, line := range strings.Split(string(body), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "//") {
			continue
		}
		if strings.HasPrefix(line, "module ") {
			modulePath := strings.TrimSpace(strings.TrimPrefix(line, "module "))
			modulePath = strings.Trim(modulePath, `"'`)
			if modulePath != "" {
				return modulePath, nil
			}
		}
	}
	return "", fmt.Errorf("module directive not found in %s", path)
}

func findGoModUp(path string) (string, error) {
	info, err := os.Stat(path)
	if err == nil && !info.IsDir() {
		path = filepath.Dir(path)
	}
	for {
		if _, err := os.Stat(filepath.Join(path, "go.mod")); err == nil {
			return path, nil
		}
		parent := filepath.Dir(path)
		if parent == path {
			return "", os.ErrNotExist
		}
		path = parent
	}
}

func runaReplaceDir(moduleRoot string) string {
	cmd := exec.Command("go", "list", "-m", "-json", "github.com/duxweb/runa")
	cmd.Env = append(os.Environ(), "GOWORK=off")
	cmd.Dir = moduleRoot
	body, err := cmd.Output()
	if err != nil {
		return ""
	}
	const marker = "\"Dir\": \""
	text := string(body)
	index := strings.LastIndex(text, marker)
	if index < 0 {
		return ""
	}
	value := text[index+len(marker):]
	end := strings.Index(value, "\"")
	if end < 0 {
		return ""
	}
	return value[:end]
}

func rootReplace(modulePath string, moduleRoot string, runaReplace string, targetPath string) string {
	root := runaReplace
	if root == "" && modulePath == "github.com/duxweb/runa" {
		root = moduleRoot
	}
	if root == "" {
		return ""
	}
	absTarget, err := filepath.Abs(targetPath)
	if err != nil {
		return root
	}
	rel, err := filepath.Rel(absTarget, root)
	if err != nil {
		return root
	}
	return filepath.ToSlash(rel)
}

var nonIdent = regexp.MustCompile(`[^a-zA-Z0-9]+`)

func normalizeName(name string) string {
	name = strings.TrimSpace(name)
	name = strings.Trim(name, `/\`)
	name = strings.ReplaceAll(name, "_", "-")
	name = nonIdent.ReplaceAllString(name, "-")
	name = strings.Trim(name, "-")
	if name == "" {
		return "item"
	}
	return strings.ToLower(name)
}

func packageName(name string) string {
	name = strings.ReplaceAll(normalizeName(name), "-", "")
	if name == "" {
		return "app"
	}
	if unicode.IsDigit(rune(name[0])) {
		name = "pkg" + name
	}
	return name
}

func exportedName(name string) string {
	parts := strings.Split(normalizeName(name), "-")
	var out string
	for _, part := range parts {
		if part == "" {
			continue
		}
		runes := []rune(part)
		runes[0] = unicode.ToUpper(runes[0])
		out += string(runes)
	}
	if out == "" {
		return "Item"
	}
	if unicode.IsDigit([]rune(out)[0]) {
		out = "Item" + out
	}
	return out
}

func lowerCamel(name string) string {
	out := exportedName(name)
	runes := []rune(out)
	if len(runes) == 0 {
		return "item"
	}
	runes[0] = unicode.ToLower(runes[0])
	return string(runes)
}

func kebabName(name string) string { return normalizeName(name) }

func pluralName(name string) string {
	if strings.HasSuffix(name, "s") {
		return name
	}
	return name + "s"
}

func packageImport(goModule string, parts ...string) string {
	items := []string{strings.TrimRight(goModule, "/")}
	for _, part := range parts {
		part = importPathPart(part)
		part = strings.Trim(part, "/")
		if part == "" || part == "." {
			continue
		}
		items = append(items, part)
	}
	return strings.Join(items, "/")
}

func importPathPart(part string) string {
	part = filepath.ToSlash(filepath.Clean(part))
	chunks := strings.Split(part, "/")
	for index, chunk := range chunks {
		if chunk == "app" {
			return strings.Join(chunks[index:], "/")
		}
	}
	if filepath.IsAbs(part) {
		return filepath.Base(part)
	}
	part = strings.TrimPrefix(part, "./")
	for strings.HasPrefix(part, "../") {
		part = strings.TrimPrefix(part, "../")
	}
	return part
}
