package database

import (
	"context"
	"fmt"

	runacommand "github.com/duxweb/runa/command"
	runaprovider "github.com/duxweb/runa/provider"
)

type listCommand struct{}

func (listCommand) Name() string    { return "database:list" }
func (listCommand) Summary() string { return "List registered databases" }
func (listCommand) Flags(flags *runacommand.FlagSet) {
	flags.Bool("json", false, "Output JSON")
}
func (listCommand) Run(_ context.Context, cmd *runacommand.Context) error {
	registry := runaprovider.MustInvoke[*Registry](providerCommandContext(cmd))
	if cmd.Get[bool]("json") {
		return cmd.JSON(registry.Info())
	}
	rows := [][]string{{"NAME", "KIND", "DIALECT", "STATUS"}}
	for _, info := range registry.Info() {
		rows = append(rows, []string{info.Name, info.Kind, info.Dialect, info.Status})
	}
	return cmd.Table(rows)
}

type pingCommand struct{}

func (pingCommand) Name() string    { return "database:ping" }
func (pingCommand) Summary() string { return "Ping registered databases" }
func (pingCommand) Run(ctx context.Context, cmd *runacommand.Context) error {
	registry := runaprovider.MustInvoke[*Registry](providerCommandContext(cmd))
	var failed error
	rows := [][]string{{"NAME", "KIND", "DIALECT", "STATUS"}}
	for _, info := range registry.Info() {
		status := "ok"
		if err := registry.Ping(ctx, info.Name); err != nil {
			status = "error"
			failed = fmt.Errorf("%s ping failed: %w", info.Name, err)
		}
		rows = append(rows, []string{info.Name, info.Kind, info.Dialect, status})
	}
	if err := cmd.Table(rows); err != nil {
		return err
	}
	return failed
}

func commands() []runacommand.Command {
	return []runacommand.Command{listCommand{}, pingCommand{}}
}

func providerCommandContext(cmd *runacommand.Context) runaprovider.Context {
	return cmd.App().(runaprovider.Context)
}
