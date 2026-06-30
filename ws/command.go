package ws

import (
	"context"
	"fmt"

	runacommand "github.com/duxweb/runa/command"
)

type listCommand struct{ hub *Hub }

func (command listCommand) Name() string    { return "ws:list" }
func (command listCommand) Summary() string { return "List websocket clients" }
func (command listCommand) Flags(flags *runacommand.FlagSet) {
	flags.Bool("json", false, "Output JSON")
}
func (command listCommand) Run(_ context.Context, cmd *runacommand.Context) error {
	if cmd.Get[bool]("json") {
		return cmd.JSON(command.hub.Clients())
	}
	rows := [][]string{{"ID", "NAME", "NODE", "CHANNELS", "IP", "CONNECTED"}}
	for _, client := range command.hub.Clients() {
		rows = append(rows, []string{client.ID, client.Name, client.Node, join(client.Channels), client.IP, client.Connected.Format("2006-01-02 15:04:05")})
	}
	return cmd.Table(rows)
}

type channelsCommand struct{ hub *Hub }

func (command channelsCommand) Name() string    { return "ws:channels" }
func (command channelsCommand) Summary() string { return "List websocket channels" }
func (command channelsCommand) Flags(flags *runacommand.FlagSet) {
	flags.Bool("json", false, "Output JSON")
}
func (command channelsCommand) Run(_ context.Context, cmd *runacommand.Context) error {
	if cmd.Get[bool]("json") {
		return cmd.JSON(command.hub.Channels())
	}
	rows := [][]string{{"CHANNEL", "CLIENTS"}}
	for _, channel := range command.hub.Channels() {
		rows = append(rows, []string{channel.Name, fmt.Sprint(channel.Clients)})
	}
	return cmd.Table(rows)
}

type statsCommand struct{ hub *Hub }

func (command statsCommand) Name() string    { return "ws:stats" }
func (command statsCommand) Summary() string { return "Show websocket stats" }
func (command statsCommand) Flags(flags *runacommand.FlagSet) {
	flags.Bool("json", false, "Output JSON")
}
func (command statsCommand) Run(_ context.Context, cmd *runacommand.Context) error {
	if cmd.Get[bool]("json") {
		return cmd.JSON(command.hub.Stats())
	}
	return cmd.Table(command.hub.Stats())
}

type kickCommand struct{ hub *Hub }

func (command kickCommand) Name() string    { return "ws:kick" }
func (command kickCommand) Summary() string { return "Kick websocket client" }
func (command kickCommand) Run(_ context.Context, cmd *runacommand.Context) error {
	args := cmd.Args()
	if len(args) == 0 || args[0] == "" {
		return fmt.Errorf("client id is required")
	}
	return command.hub.Kick(args[0], CloseReason("kicked"))
}

func join(values []string) string {
	if len(values) == 0 {
		return ""
	}
	out := values[0]
	for _, value := range values[1:] {
		out += "," + value
	}
	return out
}
