package devtools

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Scaffold creates a minimal Runa project.
func Scaffold(dir string, module string) error {
	if dir == "" {
		dir = "."
	}
	if module == "" {
		module = filepath.Base(filepath.Clean(dir))
		if module == "." || module == string(filepath.Separator) || module == "" {
			module = "runa-app"
		}
	}
	if err := os.MkdirAll(filepath.Join(dir, "cmd", "app"), 0o755); err != nil {
		return err
	}
	if err := writeNew(filepath.Join(dir, "go.mod"), "module "+module+"\n\ngo 1.27\n\nrequire (\n\tgithub.com/duxweb/runa v0.0.0\n\tgithub.com/duxweb/runa/route v0.0.0\n)\n"); err != nil {
		return err
	}
	main := `package main

import (
	"context"

	"github.com/duxweb/runa"
	"github.com/duxweb/runa/route"
)

func main() {
	app := runa.New()
	app.Install(route.Provider(route.Addr(":8080")))
	route.Default().Get("/", func(ctx *route.Context) error {
		return ctx.Text("Hello Runa")
	})
	if err := app.Run(context.Background()); err != nil {
		panic(err)
	}
}
`
	if err := writeNew(filepath.Join(dir, "cmd", "app", "main.go"), main); err != nil {
		return err
	}
	return writeNew(filepath.Join(dir, "CLAUDE.md"), claudeGuide(module))
}

func claudeGuide(module string) string {
	return fmt.Sprintf("# %s Runa Guide\n\n"+
		"This project uses Runa, a microkernel Go web framework.\n\n"+
		"## Core rules\n\n"+
		"- Keep the core app small: install only the capabilities this project uses.\n"+
		"- Register business code through modules based on provider.ModuleBase.\n"+
		"- Keep each module root small: only module.go should live at app/<module>/; put admin/api/service/models/handler/command/listener/queue/schedule/middleware code in subdirectories.\n"+
		"- Use app.Install(...) for framework capabilities and drivers.\n"+
		"- Use Default() helpers only after the app has installed the related provider.\n"+
		"- Keep long-running workers as separate commands unless this project intentionally registers them as Host units.\n\n"+
		"## Common commands\n\n"+
		"- go run ./cmd/app serve: run the application.\n"+
		"- go run ./cmd/app route:list: inspect registered routes.\n"+
		"- runa dev --cmd serve: rebuild and restart on Go/config changes.\n"+
		"- runa gen module <name>: generate a Dux-style business module.\n"+
		"- runa gen resource <name> --module <module>: generate route resource handlers under a module layer.\n"+
		"- runa gen crud <name> --module <module>: generate CRUD skeleton under a module layer.\n"+
		"- runa gen spec <file>: generate modules and resources from TOML specs.\n\n"+
		"## Naming rules\n\n"+
		"- Driver interfaces are named Driver inside their package.\n"+
		"- Driver selection options are named Use(name).\n"+
		"- Provider options use RegisterDriver(name, driver) and RegisterXxx(name, options...).\n"+
		"- Capability packages expose New(), Provider(...), Default(), and Registry.\n"+
		"- External driver packages expose a Driver(...) factory.\n\n"+
		"## Implementation preference\n\n"+
		"Prefer small modules, explicit installation, typed route inputs, and context-aware handlers. Do not add dependencies to the app unless the feature is actually used.\n", module)
}

func writeNew(path string, body string) error {
	if _, err := os.Stat(path); err == nil {
		return fmt.Errorf("%s already exists", path)
	} else if err != nil && !os.IsNotExist(err) {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(strings.TrimLeft(body, "\n")), 0o644)
}
