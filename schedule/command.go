package schedule

import (
	"context"
	"fmt"

	runacommand "github.com/duxweb/runa/command"
	runaprovider "github.com/duxweb/runa/provider"
	"github.com/duxweb/runa/task"
)

type listCommand struct{}

func (listCommand) Name() string    { return "schedule:list" }
func (listCommand) Summary() string { return "List schedules" }
func (listCommand) Flags(flags *runacommand.FlagSet) {
	flags.Bool("json", false, "Output JSON")
}
func (listCommand) Run(_ context.Context, cmd *runacommand.Context) error {
	registry := runaprovider.MustInvoke[*Registry](cmd.App().(runaprovider.Context))
	if cmd.Get[bool]("json") {
		return cmd.JSON(registry.Info())
	}
	rows := [][]string{{"SCHEDULE", "SPEC", "TASK", "MODE", "QUEUE", "ENABLED"}}
	for _, item := range registry.Info() {
		rows = append(rows, []string{item.Name, item.Spec, item.Task, item.Mode, item.Queue, fmt.Sprint(item.Enabled)})
	}
	return cmd.Table(rows)
}

type runCommand struct{}

func (runCommand) Name() string    { return "schedule:run" }
func (runCommand) Summary() string { return "Run schedule worker" }
func (runCommand) Run(ctx context.Context, cmd *runacommand.Context) error {
	providerCtx := cmd.App().(runaprovider.Context)
	registry := runaprovider.MustInvoke[*Registry](providerCtx)
	tasks := runaprovider.MustInvoke[*task.Registry](providerCtx)
	unit := NewUnit(registry, tasks)
	if err := unit.Start(ctx); err != nil {
		return err
	}
	<-ctx.Done()
	return unit.Stop(context.Background())
}

func commands() []runacommand.Command { return []runacommand.Command{listCommand{}, runCommand{}} }
