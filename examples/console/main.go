package main

import (
	"context"
	"fmt"
	"math/rand/v2"
	"os"
	goruntime "runtime"
	"sync/atomic"
	"time"

	"github.com/duxweb/runa"
	"github.com/duxweb/runa/console"
	"github.com/duxweb/runa/core"
	"github.com/duxweb/runa/message"
	"github.com/duxweb/runa/observe"
	"github.com/duxweb/runa/openapi"
	"github.com/duxweb/runa/provider"
	"github.com/duxweb/runa/queue"
	"github.com/duxweb/runa/route"
	"github.com/duxweb/runa/schedule"
	"github.com/duxweb/runa/task"
	"github.com/duxweb/runa/ws"
)

type CreateUserInput struct {
	Name string `json:"name"`
}

type UserOutput struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

type Notice struct {
	Text string `json:"text"`
}

type exampleMetrics struct {
	requests atomic.Int64
	jobs     atomic.Int64
}

func main() {
	metrics := &exampleMetrics{}
	app := runa.New()
	hub := ws.New("example", ws.Config{PingInterval: 20 * time.Second, PongTimeout: 5 * time.Second})
	addr := os.Getenv("ADDR")
	if addr == "" {
		addr = ":8080"
	}

	app.Install(route.Provider(route.Addr(addr)), task.Provider(), schedule.Provider(), queue.Provider(), message.Provider(), observe.Provider(observe.Config{Mount: "/debug", Debug: true}))
	app.Install(ws.Provider(hub))
	app.Install(openapi.Provider(openapi.Register("api",
		openapi.Title("Runa Console Example"),
		openapi.Version("dev"),
		openapi.JSON("/openapi.json"),
		openapi.UI("/docs"),
	)))
	app.Install(console.Provider(
		console.MountAt("/__runa"),
		console.Interval(2*time.Second),
		console.Panels(exampleConsolePanel(metrics)),
	))
	app.Install(exampleProvider{hub: hub, metrics: metrics})

	ws.Mount(route.Default().Group("/ws"), hub)
	registerRoutes(metrics)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	fmt.Println("Runa example listening on http://localhost" + addr)
	fmt.Println("Console: http://localhost" + addr + "/__runa")
	fmt.Println("OpenAPI: http://localhost" + addr + "/docs")
	if err := app.Run(ctx); err != nil {
		panic(err)
	}
}

func registerRoutes(metrics *exampleMetrics) {
	route.Default().Get("/", func(ctx *route.Context) error {
		metrics.requests.Add(1)
		return ctx.HTML(`<!doctype html><html><head><meta charset="utf-8"><title>Runa Example</title></head><body style="font-family:system-ui;padding:40px"><h1>Runa Console Example</h1><p><a href="/__runa">Console</a> · <a href="/docs">OpenAPI</a> · <a href="/health">Health</a></p><button onclick="fetch('/jobs/notice',{method:'POST'}).then(()=>alert('queued'))">Push Notice Job</button></body></html>`)
	}).Name("home").Summary("示例首页").Tags("Example")

	route.Default().Get("/health", func(ctx *route.Context) error {
		metrics.requests.Add(1)
		return ctx.JSON(runa.Map{"status": "ok", "time": core.Now().Format(time.RFC3339)})
	}).Name("health").Summary("健康检查").Tags("System")

	route.Get[struct{}, []UserOutput](route.Default(), "/users", func(ctx *route.Context, input *struct{}) (*[]UserOutput, error) {
		metrics.requests.Add(1)
		items := []UserOutput{{ID: 1, Name: "Runa"}, {ID: 2, Name: "Console"}}
		return &items, nil
	}).Name("users.index").Summary("用户列表").Tags("User")

	route.Post[CreateUserInput, UserOutput](route.Default(), "/users", func(ctx *route.Context, input *CreateUserInput) (*UserOutput, error) {
		metrics.requests.Add(1)
		return &UserOutput{ID: rand.IntN(1000) + 10, Name: input.Name}, nil
	}).Name("users.create").Summary("创建用户").Tags("User")

	route.Default().Post("/jobs/notice", func(ctx *route.Context) error {
		metrics.requests.Add(1)
		id, err := queue.Default().Push(ctx.Context(), "default", "notice.send", Notice{Text: "queued notice"})
		if err != nil {
			return err
		}
		return ctx.JSON(runa.Map{"id": id})
	}).Name("jobs.notice").Summary("投递通知任务").Tags("Queue")
}

type exampleProvider struct {
	provider.Base
	hub     *ws.Hub
	metrics *exampleMetrics
}

func (exampleProvider) Name() string { return "example" }

func (item exampleProvider) Register(ctx provider.Context) error {
	queues := provider.MustInvoke[*queue.Registry](ctx)
	tasks := provider.MustInvoke[*task.Registry](ctx)
	schedules := provider.MustInvoke[*schedule.Registry](ctx)

	queues.Queue("default", queue.Workers("default"), queue.Retry(1), queue.RetryDelay(time.Second))
	queues.Worker("default", queue.Concurrency(2), queue.PollInterval(100*time.Millisecond))
	queues.Job[Notice]("notice.send", func(ctx context.Context, job *queue.Job[Notice]) error {
		item.metrics.jobs.Add(1)
		return item.hub.BroadcastContext(ctx, "notice", job.Payload)
	})
	tasks.Register[Notice]("notice.direct", func(ctx context.Context, task *task.TaskOf[Notice]) error {
		item.metrics.jobs.Add(1)
		return item.hub.BroadcastContext(ctx, "notice", task.Payload)
	})
	schedules.Register("notice.tick", "@every 30s", "notice.direct", Notice{Text: "scheduled notice"}, schedule.Direct(), schedule.SkipIfRunning())
	return ctx.RegisterHost(queue.NewUnit(queues, "default"))
}

func exampleConsolePanel(metrics *exampleMetrics) console.Panel {
	return console.ComponentPanel{
		Name:  "example",
		Title: "Example Service",
		Icon:  "◆",
		Order: 10,
		Components: []console.Component{
			{Name: "health", Label: "Health", Type: console.ComponentStatus, Resolve: func(context.Context, console.AppContext) (any, error) {
				return runa.Map{"status": "ok", "hint": "example service ready"}, nil
			}},
			{Name: "requests", Label: "Requests", Type: console.ComponentMetric, Resolve: func(context.Context, console.AppContext) (any, error) {
				return runa.Map{"value": metrics.requests.Load(), "hint": "handled HTTP requests"}, nil
			}},
			{Name: "jobs", Label: "Jobs", Type: console.ComponentMetric, Resolve: func(context.Context, console.AppContext) (any, error) {
				return runa.Map{"value": metrics.jobs.Load(), "hint": "executed jobs/tasks"}, nil
			}},
			{Name: "runtime", Label: "Runtime", Type: console.ComponentTable, Resolve: func(context.Context, console.AppContext) (any, error) {
				return []runa.Map{
					{"name": "goos", "value": goruntime.GOOS},
					{"name": "goarch", "value": goruntime.GOARCH},
					{"name": "goroutines", "value": goruntime.NumGoroutine()},
				}, nil
			}},
			{Name: "traffic", Label: "Traffic", Type: console.ComponentLine, Resolve: func(context.Context, console.AppContext) (any, error) {
				base := metrics.requests.Load()
				return []runa.Map{{"label": "-4m", "value": max64(base-4, 0)}, {"label": "-3m", "value": max64(base-3, 0)}, {"label": "-2m", "value": max64(base-2, 0)}, {"label": "-1m", "value": max64(base-1, 0)}, {"label": "now", "value": base}}, nil
			}},
			{Name: "queue", Label: "Queue States", Type: console.ComponentBar, Resolve: func(ctx context.Context, app console.AppContext) (any, error) {
				infos := queue.Default().QueueInfo(ctx)
				if len(infos) == 0 {
					return nil, nil
				}
				item := infos[0]
				return []runa.Map{{"label": "pending", "value": item.Pending}, {"label": "reserved", "value": item.Reserved}, {"label": "delayed", "value": item.Delayed}, {"label": "failed", "value": item.Failed}}, nil
			}},
		},
	}
}

func max64(a int64, b int64) int64 {
	if a > b {
		return a
	}
	return b
}
