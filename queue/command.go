package queue

import (
	"context"
	"fmt"

	runacommand "github.com/duxweb/runa/command"
	runaprovider "github.com/duxweb/runa/provider"
)

type listCommand struct{}

func (listCommand) Name() string    { return "queue:list" }
func (listCommand) Summary() string { return "List queues and workers" }
func (listCommand) Flags(flags *runacommand.FlagSet) {
	flags.Bool("json", false, "Output JSON")
}
func (listCommand) Run(ctx context.Context, cmd *runacommand.Context) error {
	registry := commandRegistry(cmd)
	if cmd.Get[bool]("json") {
		return cmd.JSON(map[string]any{
			"queues":  registry.QueueInfo(ctx),
			"workers": registry.WorkerInfo(ctx),
			"jobs":    registry.JobInfo(),
		})
	}
	rows := [][]string{{"QUEUE", "DRIVER", "WORKERS", "PENDING", "DELAYED", "RESERVED", "FAILED"}}
	for _, item := range registry.QueueInfo(ctx) {
		rows = append(rows, []string{
			item.Name,
			item.Driver,
			joinStrings(item.Workers),
			fmt.Sprint(item.Pending),
			fmt.Sprint(item.Delayed),
			fmt.Sprint(item.Reserved),
			fmt.Sprint(item.Failed),
		})
	}
	if err := cmd.Table(rows); err != nil {
		return err
	}
	workers := [][]string{{"WORKER", "QUEUES", "CONCURRENCY", "INSTANCES", "PROCESSED", "SUCCEEDED", "FAILED", "RETRIED"}}
	for _, item := range registry.WorkerInfo(ctx) {
		workers = append(workers, []string{
			item.Name,
			joinStrings(item.Queues),
			fmt.Sprint(item.Concurrency),
			fmt.Sprint(item.Instances),
			fmt.Sprint(item.Processed),
			fmt.Sprint(item.Succeeded),
			fmt.Sprint(item.Failed),
			fmt.Sprint(item.Retried),
		})
	}
	return cmd.Table(workers)
}

type workCommand struct{}

func (workCommand) Name() string    { return "queue:work" }
func (workCommand) Summary() string { return "Run queue worker" }
func (workCommand) Run(ctx context.Context, cmd *runacommand.Context) error {
	args := cmd.Args()
	if len(args) == 0 || args[0] == "" {
		return fmt.Errorf("worker name is required")
	}
	return commandRegistry(cmd).Work(ctx, args[0])
}

func commands() []runacommand.Command { return []runacommand.Command{listCommand{}, workCommand{}} }

func commandRegistry(cmd *runacommand.Context) *Registry {
	return runaprovider.MustInvoke[*Registry](cmd.App().(runaprovider.Context))
}

func joinStrings(values []string) string {
	if len(values) == 0 {
		return ""
	}
	output := values[0]
	for _, value := range values[1:] {
		output += "," + value
	}
	return output
}
