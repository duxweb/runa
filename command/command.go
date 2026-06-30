package command

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"text/tabwriter"

	"github.com/duxweb/runa/core"
	cli "github.com/urfave/cli/v3"
)

// Command is an application command executed by the project binary.
type Command interface {
	Name() string
	Summary() string
	Run(ctx context.Context, cmd *Context) error
}

// Flags lets a command define flags.
type Flags interface {
	Flags(flags *FlagSet)
}

// Registry stores application commands.
type Registry struct {
	mu       sync.Mutex
	commands map[string]Command
}

// ExecuteConfig configures command execution.
type ExecuteConfig struct {
	Name        string
	Usage       string
	DefaultArgs []string
	Writer      io.Writer
	ErrWriter   io.Writer
}

// New creates a command registry.
func New() *Registry {
	return &Registry{commands: make(map[string]Command)}
}

// Register registers commands by name.
func (registry *Registry) Register(commands ...Command) {
	registry.mu.Lock()
	defer registry.mu.Unlock()
	for _, command := range commands {
		if command == nil {
			continue
		}
		registry.commands[command.Name()] = command
	}
}

// Names returns registered command names.
func (registry *Registry) Names() []string {
	registry.mu.Lock()
	defer registry.mu.Unlock()
	names := make([]string, 0, len(registry.commands))
	for name := range registry.commands {
		names = append(names, name)
	}
	return names
}

// Commands returns CLI command definitions.
func (registry *Registry) Commands(app any) []*cli.Command {
	registry.mu.Lock()
	defer registry.mu.Unlock()

	commands := make([]*cli.Command, 0, len(registry.commands))
	for _, registered := range registry.commands {
		registered := registered
		flags := &FlagSet{}
		if provider, ok := registered.(Flags); ok {
			provider.Flags(flags)
		}
		commands = append(commands, &cli.Command{
			Name:  registered.Name(),
			Usage: registered.Summary(),
			Flags: flags.CLIFlags(),
			Action: func(ctx context.Context, cliCommand *cli.Command) error {
				return registered.Run(ctx, NewContext(app, cliCommand))
			},
		})
	}
	return commands
}

// Execute runs a command set. Empty args use DefaultArgs.
func (registry *Registry) Execute(ctx context.Context, app any, args []string, config ExecuteConfig) error {
	if len(args) == 0 {
		args = append([]string(nil), config.DefaultArgs...)
	}
	name := config.Name
	if name == "" {
		name = "app"
	}
	root := &cli.Command{
		Name:        name,
		Usage:       config.Usage,
		Commands:    registry.Commands(app),
		HideVersion: true,
		Writer:      config.Writer,
		ErrWriter:   config.ErrWriter,
		ExitErrHandler: func(context.Context, *cli.Command, error) {
		},
	}
	return root.Run(ctx, append([]string{name}, args...))
}

// Context is the execution context for application commands.
type Context struct {
	app any
	cmd *cli.Command
}

// NewContext creates a command context.
func NewContext(app any, cmd *cli.Command) *Context {
	return &Context{app: app, cmd: cmd}
}

// App returns the current application object.
func (ctx *Context) App() any { return ctx.app }

// Args returns raw command arguments after the command name.
func (ctx *Context) Args() []string {
	if ctx.cmd == nil {
		return nil
	}
	return ctx.cmd.Args().Slice()
}

// Get reads a flag value and casts it to T.
func (ctx *Context) Get[T any](name string, fallback ...T) T {
	if ctx.cmd == nil {
		return core.Cast[T](nil, fallback...)
	}
	return core.Cast[T](ctx.cmd.Value(name), fallback...)
}

// Print writes values to the command output.
func (ctx *Context) Print(values ...any) error {
	_, err := fmt.Fprint(ctx.output(), values...)
	return err
}

// Println writes values and a newline to the command output.
func (ctx *Context) Println(values ...any) error {
	_, err := fmt.Fprintln(ctx.output(), values...)
	return err
}

// Table writes a simple tab-separated table to the command output.
func (ctx *Context) Table(data any) error {
	writer := tabwriter.NewWriter(ctx.output(), 0, 0, 2, ' ', 0)
	if rows, ok := data.([][]string); ok {
		for _, row := range rows {
			for index, col := range row {
				if index > 0 {
					if _, err := fmt.Fprint(writer, "\t"); err != nil {
						return err
					}
				}
				if _, err := fmt.Fprint(writer, col); err != nil {
					return err
				}
			}
			if _, err := fmt.Fprint(writer, "\n"); err != nil {
				return err
			}
		}
		return writer.Flush()
	}
	body, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return err
	}
	if _, err := writer.Write(body); err != nil {
		return err
	}
	if _, err := fmt.Fprint(writer, "\n"); err != nil {
		return err
	}
	return writer.Flush()
}

// JSON writes indented JSON to the command output.
func (ctx *Context) JSON(data any) error {
	body, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return err
	}
	if _, err := ctx.output().Write(body); err != nil {
		return err
	}
	_, err = fmt.Fprint(ctx.output(), "\n")
	return err
}

func (ctx *Context) output() io.Writer {
	if ctx != nil && ctx.cmd != nil && ctx.cmd.Root().Writer != nil {
		return ctx.cmd.Root().Writer
	}
	return os.Stdout
}

// FormatValue formats a command table value.
func FormatValue(value any) string {
	switch typed := value.(type) {
	case nil:
		return ""
	case string:
		return typed
	case fmt.Stringer:
		return typed.String()
	}
	body, err := json.Marshal(value)
	if err != nil {
		return fmt.Sprint(value)
	}
	return strings.TrimSpace(string(body))
}

// FlagSet is the command flag definition facade.
type FlagSet struct {
	flags []cli.Flag
}

// String defines a string flag.
func (flags *FlagSet) String(name string, value string, usage string) {
	flags.flags = append(flags.flags, &cli.StringFlag{Name: name, Value: value, Usage: usage})
}

// Bool defines a bool flag.
func (flags *FlagSet) Bool(name string, value bool, usage string) {
	flags.flags = append(flags.flags, &cli.BoolFlag{Name: name, Value: value, Usage: usage})
}

// Int defines an int flag.
func (flags *FlagSet) Int(name string, value int, usage string) {
	flags.flags = append(flags.flags, &cli.IntFlag{Name: name, Value: value, Usage: usage})
}

// CLIFlags returns flags for the underlying CLI runtime.
func (flags *FlagSet) CLIFlags() []cli.Flag {
	return append([]cli.Flag(nil), flags.flags...)
}
