package task

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	runacommand "github.com/duxweb/runa/command"
	runaprovider "github.com/duxweb/runa/provider"
)

type listCommand struct{}

func (listCommand) Name() string    { return "task:list" }
func (listCommand) Summary() string { return "List tasks" }
func (listCommand) Flags(flags *runacommand.FlagSet) {
	flags.Bool("json", false, "Output JSON")
}
func (listCommand) Run(_ context.Context, cmd *runacommand.Context) error {
	registry := runaprovider.MustInvoke[*Registry](cmd.App().(runaprovider.Context))
	if cmd.Get[bool]("json") {
		return cmd.JSON(registry.Info())
	}
	rows := [][]string{{"TASK", "PAYLOAD", "TIMEOUT", "RETRY"}}
	for _, item := range registry.Info() {
		rows = append(rows, []string{item.Name, item.Payload, durationString(item.Timeout), fmt.Sprint(item.Retry)})
	}
	return cmd.Table(rows)
}

type runCommand struct{}

func (runCommand) Name() string    { return "task:run" }
func (runCommand) Summary() string { return "Run a task once" }
func (runCommand) Flags(flags *runacommand.FlagSet) {
	flags.String("payload", "{}", "JSON payload")
}
func (runCommand) Run(ctx context.Context, cmd *runacommand.Context) error {
	args := cmd.Args()
	if len(args) == 0 || args[0] == "" {
		return fmt.Errorf("task name is required")
	}
	var payload json.RawMessage
	if err := json.Unmarshal([]byte(cmd.Get[string]("payload", "{}")), &payload); err != nil {
		return err
	}
	registry := runaprovider.MustInvoke[*Registry](cmd.App().(runaprovider.Context))
	id, err := registry.DispatchMessage(ctx, Message{Name: args[0], Payload: payload})
	if err != nil {
		return err
	}
	return cmd.Println(id)
}

func commands() []runacommand.Command { return []runacommand.Command{listCommand{}, runCommand{}} }

func durationString(value time.Duration) string {
	if value <= 0 {
		return ""
	}
	return value.String()
}
