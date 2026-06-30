package main

import (
	"context"
	"time"

	"github.com/duxweb/runa"
	"github.com/duxweb/runa/observe"
	"github.com/duxweb/runa/provider"
	"github.com/duxweb/runa/queue"
	"github.com/duxweb/runa/route"
	"github.com/duxweb/runa/schedule"
	"github.com/duxweb/runa/task"
	"github.com/duxweb/runa/ws"
)

type Notice struct {
	Text string `json:"text"`
}

func main() {
	app := runa.New()
	hub := ws.New("app", ws.Config{})
	app.Install(route.Provider(route.Addr(":8080")), task.Provider(), schedule.Provider(), queue.Provider(), observe.Provider(observe.Config{Mount: "/debug"}))
	app.Install(ws.Provider(hub), exampleProvider{hub: hub})
	ws.Mount(route.Default().Group("/ws"), hub)
	route.Default().Get("/", func(ctx *route.Context) error {
		_, err := task.Default().Dispatch(ctx.Context(), "notice", Notice{Text: "manual"}, task.Delay(time.Second))
		if err != nil {
			return err
		}
		return ctx.Text("ok")
	})
	if err := app.Run(context.Background()); err != nil {
		panic(err)
	}
}

type exampleProvider struct {
	provider.Base
	hub *ws.Hub
}

func (exampleProvider) Name() string { return "example" }

func (item exampleProvider) Register(ctx provider.Context) error {
	tasks := provider.MustInvoke[*task.Registry](ctx)
	schedules := provider.MustInvoke[*schedule.Registry](ctx)
	tasks.Register[Notice]("notice", func(ctx context.Context, task *task.TaskOf[Notice]) error {
		return item.hub.BroadcastContext(ctx, "notice", task.Payload)
	})
	schedules.Register("notice.every_minute", "@every 1m", "notice", Notice{Text: "hello"}, schedule.Direct())
	return nil
}
