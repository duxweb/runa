package event

import (
	"context"
	"fmt"

	runacommand "github.com/duxweb/runa/command"
	runaprovider "github.com/duxweb/runa/provider"
)

type listCommand struct{}

func (listCommand) Name() string    { return "event:list" }
func (listCommand) Summary() string { return "List events" }
func (listCommand) Flags(flags *runacommand.FlagSet) {
	flags.Bool("json", false, "Output JSON")
}
func (listCommand) Run(_ context.Context, cmd *runacommand.Context) error {
	if cmd.Get[bool]("json") {
		return cmd.JSON(commandRegistry(cmd).Info())
	}
	return cmd.Table(listRows(commandRegistry(cmd)))
}

func commands() []runacommand.Command { return []runacommand.Command{listCommand{}} }

func listRows(registry *Registry) [][]string {
	rows := [][]string{{"EVENT", "PAYLOAD", "LISTENER", "PRIORITY", "QUEUE"}}
	for _, item := range registry.Info() {
		rows = append(rows, []string{item.Name, item.Payload, item.Listener, fmt.Sprint(item.Priority), item.Queue})
	}
	return rows
}

func commandRegistry(cmd *runacommand.Context) *Registry {
	return runaprovider.MustInvoke[*Registry](cmd.App().(runaprovider.Context))
}
