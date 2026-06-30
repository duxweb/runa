package devtools

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	runacommand "github.com/duxweb/runa/command"
	"github.com/duxweb/runa/kernel/embedgen"
)

type pathResolver interface {
	BasePath(paths ...string) string
}

type embedViewCommand struct{ config Config }

func (command embedViewCommand) Name() string    { return "devtools:embed" }
func (command embedViewCommand) Summary() string { return "Generate view embed file" }
func (command embedViewCommand) Flags(flags *runacommand.FlagSet) {
	flags.String("root", command.config.EmbedRoot, "Embed root")
	flags.String("out", command.config.EmbedOut, "Output file")
	flags.String("name", command.config.EmbedName, "Embed variable name")
	flags.String("package", command.config.EmbedPackage, "Go package name")
}
func (command embedViewCommand) Run(ctx context.Context, cmd *runacommand.Context) error {
	_ = ctx
	app := cmd.App().(pathResolver)
	root := cmd.Get[string]("root", command.config.EmbedRoot)
	rootPath := root
	if !filepath.IsAbs(rootPath) {
		rootPath = app.BasePath(rootPath)
	}
	out := cmd.Get[string]("out", command.config.EmbedOut)
	outPath := out
	if !filepath.IsAbs(outPath) {
		outPath = app.BasePath(outPath)
	}
	name := cmd.Get[string]("name", command.config.EmbedName)
	pkg := cmd.Get[string]("package", command.config.EmbedPackage)
	body, err := embedgen.Generate(embedgen.Config{Name: name, Root: rootPath, Patterns: command.config.EmbedPatterns, Package: pkg})
	if err != nil {
		return err
	}
	body = []byte(strings.ReplaceAll(string(body), filepath.ToSlash(rootPath)+"/", filepath.ToSlash(root)+"/"))
	if err := os.MkdirAll(filepath.Dir(outPath), 0o755); err != nil {
		return err
	}
	return os.WriteFile(outPath, body, 0o644)
}

type buildCommand struct{}

func (buildCommand) Name() string    { return "devtools:build" }
func (buildCommand) Summary() string { return "Build Go application" }
func (buildCommand) Run(ctx context.Context, cmd *runacommand.Context) error {
	args := cmd.Args()
	if len(args) == 0 {
		args = []string{"./..."}
	}
	command := exec.CommandContext(ctx, "go", append([]string{"build"}, args...)...)
	command.Stdout = os.Stdout
	command.Stderr = os.Stderr
	return command.Run()
}

type newCommand struct{}

func (newCommand) Name() string    { return "devtools:new" }
func (newCommand) Summary() string { return "Create a minimal Runa project scaffold" }
func (newCommand) Run(_ context.Context, cmd *runacommand.Context) error {
	args := cmd.Args()
	dir := "."
	module := ""
	if len(args) > 0 {
		dir = args[0]
	}
	if len(args) > 1 {
		module = args[1]
	}
	return Scaffold(dir, module)
}
