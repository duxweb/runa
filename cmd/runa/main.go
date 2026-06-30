package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime/debug"
	"strings"

	"github.com/duxweb/runa/devtools"
	"github.com/duxweb/runa/kernel/embedgen"
	"github.com/urfave/cli/v3"
)

func main() {
	root := &cli.Command{
		Name:        "runa",
		Usage:       "Runa development tool",
		HideVersion: true,
		Commands: []*cli.Command{
			{Name: "version", Usage: "Show version", Action: version},
			{Name: "build", Usage: "Build Go application", Action: build},
			{Name: "dev", Usage: "Run application with rebuild/restart on file changes", Flags: devFlags(), Action: dev},
			{Name: "gen", Usage: "Generate Runa code", Commands: genCommands()},
			{Name: "llms", Usage: "Generate llms.txt", Flags: llmsFlags(), Action: llms},
			{Name: "doctor", Usage: "Check Runa project conventions", Flags: doctorFlags(), Action: doctor},
			{Name: "check", Usage: "Alias of doctor", Flags: doctorFlags(), Action: doctor},
			{Name: "mcp", Usage: "Run Runa MCP stdio server", Flags: mcpFlags(), Action: mcp},
			{Name: "embed", Usage: "Generate precise go:embed file", Flags: embedFlags(), Action: embed},
			{Name: "embed:view", Usage: "Generate precise view go:embed file", Flags: embedFlags(), Action: embed},
			{Name: "new", Usage: "Create a minimal Runa project", Action: scaffold},
		},
	}
	if err := root.Run(context.Background(), os.Args); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func version(_ context.Context, _ *cli.Command) error {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		fmt.Println("runa dev")
		return nil
	}
	moduleVersion := info.Main.Version
	if moduleVersion == "" || moduleVersion == "(devel)" {
		moduleVersion = "dev"
	}
	revision := ""
	modified := false
	for _, setting := range info.Settings {
		switch setting.Key {
		case "vcs.revision":
			revision = setting.Value
		case "vcs.modified":
			modified = setting.Value == "true"
		}
	}
	if revision != "" {
		if len(revision) > 12 {
			revision = revision[:12]
		}
		if modified {
			revision += "+dirty"
		}
		fmt.Printf("runa %s (%s)\n", moduleVersion, revision)
		return nil
	}
	fmt.Printf("runa %s\n", moduleVersion)
	return nil
}

func build(ctx context.Context, cmd *cli.Command) error {
	args := cmd.Args().Slice()
	if len(args) == 0 {
		args = []string{"./..."}
	}
	command := exec.CommandContext(ctx, "go", append([]string{"build"}, args...)...)
	command.Stdout = os.Stdout
	command.Stderr = os.Stderr
	return command.Run()
}

func embedFlags() []cli.Flag {
	return []cli.Flag{
		&cli.StringFlag{Name: "root", Value: "views", Usage: "Embed root"},
		&cli.StringFlag{Name: "pattern", Value: "**/*.{html,tmpl,tpl}", Usage: "Glob pattern"},
		&cli.StringFlag{Name: "out", Value: "internal/embed/view.go", Usage: "Output file"},
		&cli.StringFlag{Name: "name", Value: "ViewFS", Usage: "Embed variable name"},
		&cli.StringFlag{Name: "package", Value: "embed", Usage: "Go package name"},
	}
}

func embed(ctx context.Context, cmd *cli.Command) error {
	_ = ctx
	root := cmd.String("root")
	pattern := cmd.String("pattern")
	out := cmd.String("out")
	name := cmd.String("name")
	pkg := cmd.String("package")
	rootPath := root
	if !filepath.IsAbs(rootPath) {
		cwd, err := os.Getwd()
		if err != nil {
			return err
		}
		rootPath = filepath.Join(cwd, rootPath)
	}
	body, err := embedgen.Generate(embedgen.Config{Name: name, Root: rootPath, Patterns: []string{pattern}, Package: pkg})
	if err != nil {
		return err
	}
	body = []byte(strings.ReplaceAll(string(body), filepath.ToSlash(rootPath)+"/", filepath.ToSlash(root)+"/"))
	if err := os.MkdirAll(filepath.Dir(out), 0o755); err != nil {
		return err
	}
	return os.WriteFile(out, body, 0o644)
}

func scaffold(ctx context.Context, cmd *cli.Command) error {
	_ = ctx
	args := cmd.Args().Slice()
	dir := "."
	module := ""
	if len(args) > 0 {
		dir = args[0]
	}
	if len(args) > 1 {
		module = args[1]
	}
	return devtools.Scaffold(dir, module)
}
