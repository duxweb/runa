package main

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/urfave/cli/v3"
)

func llmsFlags() []cli.Flag {
	return []cli.Flag{
		&cli.StringFlag{Name: "docs", Value: filepath.Join("docs", "src", "content", "docs"), Usage: "Documentation directory"},
		&cli.StringSliceFlag{Name: "out", Value: []string{"llms.txt", filepath.Join("docs", "public", "llms.txt")}, Usage: "Output file; can be repeated"},
	}
}

func llms(_ context.Context, cmd *cli.Command) error {
	body, err := buildLLMSText(cmd.String("docs"))
	if err != nil {
		return err
	}
	for _, out := range cmd.StringSlice("out") {
		if out == "" {
			continue
		}
		if err := os.MkdirAll(filepath.Dir(out), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(out, []byte(body), 0o644); err != nil {
			return err
		}
		fmt.Println("created", out)
	}
	return nil
}

func buildLLMSText(root string) (string, error) {
	pages, err := collectDocPages(root)
	if err != nil {
		return "", err
	}
	var out strings.Builder
	out.WriteString("# Runa\n\n")
	out.WriteString("Runa is a microkernel Go web framework for business applications. Core handles app startup, dependency injection, config, commands, host units, modules, lifecycle, and application time. HTTP route, cache, queue, database, storage, auth, sessions, views, i18n, observability, and drivers are installed as independent capability modules.\n\n")
	out.WriteString("## Architecture\n\n")
	out.WriteString("- Core: `github.com/duxweb/runa`; no HTTP router, cache, queue, database, or storage by default.\n")
	out.WriteString("- Transports: `route`, `jsonrpc`, `ws`, and future `grpc` connect as host/capability modules.\n")
	out.WriteString("- Capabilities: packages such as `cache`, `queue`, `database`, `storage`, `view`, `lang`, `task`, `event`, and `schedule` install with `app.Install(...)`.\n")
	out.WriteString("- Drivers: external dependencies live in submodules such as `queue/redis`, `storage/s3`, and `database/oro`.\n")
	out.WriteString("- Business code is organized with `provider.ModuleBase` modules. A module directory keeps only `module.go` at the root; admin/api/service/models/handler/command/listener/queue/schedule/middleware code belongs in subdirectories, matching the Dux module style.\n\n")
	out.WriteString("## Naming rules\n\n")
	out.WriteString("- Driver interfaces are named `Driver` inside their package.\n")
	out.WriteString("- Driver selection options are named `Use(name)`.\n")
	out.WriteString("- Provider options use `RegisterDriver(name, driver)` and `RegisterXxx(name, options...)`.\n")
	out.WriteString("- Capability packages expose `New()`, `Provider(...)`, `Default()`, `Registry`, `types.go`, `options.go`, and `registry.go`.\n")
	out.WriteString("- External driver subpackages expose a `Driver(...)` factory.\n")
	out.WriteString("- Generated code should follow the naming conventions in `docs/src/content/docs/contributing/naming.mdx`.\n\n")
	out.WriteString("## Common commands\n\n")
	out.WriteString("- `runa dev --cmd serve`: rebuild and restart on Go/config changes.\n")
	out.WriteString("- `runa gen module <name>`: generate a Dux-style business module with root `module.go` and subdirectories.\n")
	out.WriteString("- `runa gen resource <name> --module <module>`: generate route resource handlers under `app/<module>/admin` by default.\n")
	out.WriteString("- `runa gen crud <name> --module <module>`: generate CRUD builder and store skeleton under `app/<module>/admin` by default.\n")
	out.WriteString("- `runa gen spec <file>`: generate modules and resources from a TOML spec.\n")
	out.WriteString("- `runa gen provider <name>`: generate provider skeleton.\n")
	out.WriteString("- `runa gen capability <name>`: generate a capability package.\n")
	out.WriteString("- `runa gen driver <cap> <name>`: generate a capability driver submodule.\n\n")
	out.WriteString("## Documentation map\n\n")
	for _, page := range pages {
		out.WriteString("- ")
		out.WriteString(page.Path)
		if page.Title != "" {
			out.WriteString(": ")
			out.WriteString(page.Title)
		}
		if len(page.Headings) > 0 {
			out.WriteString(" — ")
			out.WriteString(strings.Join(page.Headings, "; "))
		}
		out.WriteString("\n")
	}
	return out.String(), nil
}

type docPage struct {
	Path     string
	Title    string
	Headings []string
}

func collectDocPages(root string) ([]docPage, error) {
	base := filepath.Join(root)
	preferred := base
	if _, err := os.Stat(filepath.Join(base, "index.mdx")); err != nil {
		preferred = filepath.Join(base)
	}
	var pages []docPage
	if err := filepath.WalkDir(preferred, func(path string, entry os.DirEntry, err error) error {
		if err != nil || entry.IsDir() || filepath.Ext(path) != ".mdx" {
			return nil
		}
		if strings.Contains(filepath.ToSlash(path), "/zh-cn/") {
			return nil
		}
		page, err := parseDocPage(base, path)
		if err != nil {
			return err
		}
		pages = append(pages, page)
		return nil
	}); err != nil {
		return nil, err
	}
	sort.SliceStable(pages, func(i, j int) bool { return pages[i].Path < pages[j].Path })
	return pages, nil
}

func parseDocPage(root string, path string) (docPage, error) {
	file, err := os.Open(path)
	if err != nil {
		return docPage{}, err
	}
	defer file.Close()
	rel, _ := filepath.Rel(root, path)
	page := docPage{Path: filepath.ToSlash(strings.TrimSuffix(rel, filepath.Ext(rel)))}
	scanner := bufio.NewScanner(file)
	inFrontMatter := false
	frontMatterSeen := false
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "---" && !frontMatterSeen {
			inFrontMatter = true
			frontMatterSeen = true
			continue
		}
		if line == "---" && inFrontMatter {
			inFrontMatter = false
			continue
		}
		if inFrontMatter {
			if strings.HasPrefix(line, "title:") {
				page.Title = cleanFrontMatterValue(strings.TrimPrefix(line, "title:"))
			}
			continue
		}
		if strings.HasPrefix(line, "## ") && len(page.Headings) < 6 {
			page.Headings = append(page.Headings, strings.TrimSpace(strings.TrimPrefix(line, "## ")))
		}
	}
	return page, scanner.Err()
}

func cleanFrontMatterValue(value string) string {
	value = strings.TrimSpace(value)
	value = strings.Trim(value, `"'`)
	return value
}
