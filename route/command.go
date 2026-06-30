package route

import (
	"context"
	"fmt"
	"sort"

	runacommand "github.com/duxweb/runa/command"
)

// ListCommand creates a command that lists registered routes.
func ListCommand(registry *Registry) runacommand.Command {
	return listCommand{registry: registry}
}

type listCommand struct {
	registry *Registry
}

func (listCommand) Name() string    { return "route:list" }
func (listCommand) Summary() string { return "List registered routes" }
func (listCommand) Flags(flags *runacommand.FlagSet) {
	flags.Bool("json", false, "Output JSON")
}
func (command listCommand) Run(_ context.Context, cmd *runacommand.Context) error {
	if cmd.Get[bool]("json") {
		routes := append([]*Route(nil), command.registry.Routes()...)
		sort.SliceStable(routes, func(i, j int) bool {
			if routes[i].Path == routes[j].Path {
				return routes[i].Method < routes[j].Method
			}
			return routes[i].Path < routes[j].Path
		})
		return cmd.JSON(routes)
	}
	return cmd.Table(routeRows(command.registry))
}

func routeRows(registry *Registry) [][]string {
	rows := [][]string{{"METHOD", "PATH", "NAME", "MIDDLEWARE", "META"}}
	routes := append([]*Route(nil), registry.Routes()...)
	sort.SliceStable(routes, func(i, j int) bool {
		if routes[i].Path == routes[j].Path {
			return routes[i].Method < routes[j].Method
		}
		return routes[i].Path < routes[j].Path
	})
	for _, item := range routes {
		rows = append(rows, []string{item.Method, item.Path, item.RouteName, fmt.Sprint(len(item.Middlewares)), runacommand.FormatValue(item.MetaData)})
	}
	return rows
}
