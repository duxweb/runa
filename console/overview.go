package console

import (
	"context"
	"fmt"
	"github.com/duxweb/runa/cache"
	"github.com/duxweb/runa/core"
	"github.com/duxweb/runa/database"
	"github.com/duxweb/runa/lock"
	"github.com/duxweb/runa/queue"
	"github.com/duxweb/runa/rate"
	"github.com/duxweb/runa/session"
	"github.com/duxweb/runa/storage"
	goruntime "runtime"
	"sort"
	"strings"
	"time"

	"github.com/duxweb/runa/host"
)

// BuiltinPanels returns the framework-provided console panels.
func BuiltinPanels() []Panel {
	return []Panel{Overview(), TrafficPanel(), ErrorsPanel(), LogsPanel(), RuntimePanel(), RoutesPanel(), QueuePanel(), MessagePanel(), WebSocketPanel(), RPCPanel(), ORMPanel(), SchedulePanel(), InfrastructurePanel()}
}

// Overview returns the built-in runtime overview panel.
func Overview() Panel {
	return ComponentPanel{
		Name:  "overview",
		Title: "Overview",
		Icon:  "layout-dashboard",
		Order: -1000,
		Components: []Component{
			{Name: "status", Label: "Status", Type: ComponentStatus, Resolve: func(context.Context, AppContext) (any, error) {
				return core.Map{"status": "active", "hint": "application is running"}, nil
			}},
			{Name: "environment", Label: "Environment", Type: ComponentMetric, Resolve: func(_ context.Context, app AppContext) (any, error) {
				return core.Map{"value": appEnv(app), "hint": "current environment"}, nil
			}},
			{Name: "schedules", Label: "Schedules", Type: ComponentMetric, Resolve: func(_ context.Context, app AppContext) (any, error) {
				return scheduleSummary(app), nil
			}},
			{Name: "routes", Label: "Routes", Type: ComponentMetric, Resolve: func(_ context.Context, app AppContext) (any, error) {
				return len(appRoutes(app)), nil
			}},
			{Name: "queues", Label: "Queues", Type: ComponentMetric, Resolve: func(ctx context.Context, app AppContext) (any, error) {
				return len(queueInfos(ctx, app)), nil
			}},
			{Name: "workers", Label: "Workers", Type: ComponentMetric, Resolve: func(ctx context.Context, app AppContext) (any, error) {
				return len(workerInfos(ctx, app)), nil
			}},
			{Name: "goroutines", Label: "Goroutines", Type: ComponentMetric, Resolve: func(context.Context, AppContext) (any, error) {
				return goruntime.NumGoroutine(), nil
			}},
			{Name: "memory", Label: "Memory", Type: ComponentMetric, Resolve: func(context.Context, AppContext) (any, error) {
				state := runtimeState()
				memory, _ := state["memory"].(core.Map)
				return core.Map{"value": formatBytes(numberValue(memory["alloc"])), "hint": "allocated heap"}, nil
			}},
			{Name: "attention", Label: "Attention", Type: ComponentTable, Resolve: func(ctx context.Context, app AppContext) (any, error) {
				return attentionItems(ctx, app), nil
			}},
			{Name: "infrastructure", Label: "Infrastructure Summary", Type: ComponentTable, Resolve: func(ctx context.Context, app AppContext) (any, error) {
				return consoleSummaries(ctx, app), nil
			}},
			{Name: "requests_trend", Label: "Requests Trend", Type: ComponentLine, Resolve: func(ctx context.Context, app AppContext) (any, error) {
				return MonitorStoreOf(app).TrafficSeries(time.Hour, 12), nil
			}},
			{Name: "error_trend", Label: "Error Trend", Type: ComponentLine, Resolve: func(ctx context.Context, app AppContext) (any, error) {
				return MonitorStoreOf(app).ErrorSeries(time.Hour, 12), nil
			}},
			{Name: "latency_trend", Label: "Latency Trend", Type: ComponentLine, Resolve: func(ctx context.Context, app AppContext) (any, error) {
				return MonitorStoreOf(app).LatencySeries(time.Hour, 12), nil
			}},
			{Name: "queue_states", Label: "Queue States", Type: ComponentBar, Resolve: func(ctx context.Context, app AppContext) (any, error) {
				var pending, reserved, delayed, failed int64
				for _, item := range queueInfos(ctx, app) {
					pending += item.Pending
					reserved += item.Reserved
					delayed += item.Delayed
					failed += item.Failed
				}
				return []core.Map{{"label": "pending", "value": pending}, {"label": "reserved", "value": reserved}, {"label": "delayed", "value": delayed}, {"label": "failed", "value": failed}}, nil
			}},
			{Name: "worker_results", Label: "Worker Results", Type: ComponentBar, Resolve: func(ctx context.Context, app AppContext) (any, error) {
				var processed, succeeded, failed, retried int64
				for _, item := range workerInfos(ctx, app) {
					processed += item.Processed
					succeeded += item.Succeeded
					failed += item.Failed
					retried += item.Retried
				}
				return []core.Map{{"label": "processed", "value": processed}, {"label": "succeeded", "value": succeeded}, {"label": "failed", "value": failed}, {"label": "retried", "value": retried}}, nil
			}},
		},
	}
}

// TrafficPanel returns HTTP traffic metrics and access logs.
func TrafficPanel() Panel {
	return ComponentPanel{
		Name:  "traffic",
		Title: "Traffic",
		Icon:  "chart-no-axes-combined",
		Order: -950,
		Components: []Component{
			{Name: "requests_total", Label: "Requests", Type: ComponentMetric, Resolve: func(_ context.Context, app AppContext) (any, error) {
				return trafficRequestsMetric(MonitorStoreOf(app)), nil
			}},
			{Name: "errors_total", Label: "Errors", Type: ComponentMetric, Resolve: func(_ context.Context, app AppContext) (any, error) {
				return trafficErrorsMetric(MonitorStoreOf(app)), nil
			}},
			{Name: "latency_avg", Label: "Avg Latency", Type: ComponentMetric, Resolve: func(_ context.Context, app AppContext) (any, error) {
				return trafficLatencyMetric(MonitorStoreOf(app)), nil
			}},
			{Name: "slow_total", Label: "Slow Requests", Type: ComponentMetric, Resolve: func(_ context.Context, app AppContext) (any, error) {
				return trafficSlowMetric(MonitorStoreOf(app)), nil
			}},
			{Name: "requests", Label: "Requests Trend", Type: ComponentLine, Resolve: func(_ context.Context, app AppContext) (any, error) {
				return MonitorStoreOf(app).TrafficSeries(time.Hour, 12), nil
			}},
			{Name: "latency", Label: "Latency Avg (ms)", Type: ComponentLine, Resolve: func(_ context.Context, app AppContext) (any, error) {
				return MonitorStoreOf(app).LatencySeries(time.Hour, 12), nil
			}},
			{Name: "status", Label: "Status Codes", Type: ComponentBar, Resolve: func(_ context.Context, app AppContext) (any, error) {
				return MonitorStoreOf(app).StatusSeries(time.Hour, 12), nil
			}},
			{Name: "routes", Label: "Route Stats", Type: ComponentTable, Resolve: func(_ context.Context, app AppContext) (any, error) {
				return routeStatRows(MonitorStoreOf(app).RouteStats(100)), nil
			}},
			{Name: "slow", Label: "Slow Requests", Type: ComponentTable, Resolve: func(_ context.Context, app AppContext) (any, error) {
				return accessRows(MonitorStoreOf(app).SlowLogs(50)), nil
			}},
			{Name: "access", Label: "Access Logs", Type: ComponentTable, Resolve: func(_ context.Context, app AppContext) (any, error) {
				return accessRows(MonitorStoreOf(app).AccessLogs(100)), nil
			}},
		},
	}
}

// ErrorsPanel returns HTTP error trends and error logs.
func ErrorsPanel() Panel {
	return ComponentPanel{
		Name:  "errors",
		Title: "Errors",
		Icon:  "triangle-alert",
		Order: -940,
		Components: []Component{
			{Name: "total", Label: "Errors", Type: ComponentMetric, Resolve: func(_ context.Context, app AppContext) (any, error) {
				return errorTotalMetric(MonitorStoreOf(app)), nil
			}},
			{Name: "rate", Label: "Error Rate", Type: ComponentMetric, Resolve: func(_ context.Context, app AppContext) (any, error) {
				return errorRateMetric(MonitorStoreOf(app)), nil
			}},
			{Name: "last", Label: "Last Error", Type: ComponentStatus, Resolve: func(_ context.Context, app AppContext) (any, error) {
				return lastErrorStatus(MonitorStoreOf(app)), nil
			}},
			{Name: "errors", Label: "Error Trend", Type: ComponentLine, Resolve: func(_ context.Context, app AppContext) (any, error) {
				return MonitorStoreOf(app).ErrorSeries(time.Hour, 12), nil
			}},
			{Name: "logs", Label: "Error Logs", Type: ComponentTable, Resolve: func(_ context.Context, app AppContext) (any, error) {
				return errorRows(MonitorStoreOf(app).ErrorLogs(100)), nil
			}},
		},
	}
}

// LogsPanel returns captured application monitor logs.
func LogsPanel() Panel {
	return ComponentPanel{
		Name:  "logs",
		Title: "Logs",
		Icon:  "scroll-text",
		Order: -930,
		Components: []Component{
			{Name: "access_count", Label: "Access Entries", Type: ComponentMetric, Resolve: func(_ context.Context, app AppContext) (any, error) {
				return logAccessMetric(MonitorStoreOf(app)), nil
			}},
			{Name: "error_count", Label: "Error Entries", Type: ComponentMetric, Resolve: func(_ context.Context, app AppContext) (any, error) {
				return logErrorMetric(MonitorStoreOf(app)), nil
			}},
			{Name: "access", Label: "Access Logs", Type: ComponentTable, Resolve: func(_ context.Context, app AppContext) (any, error) {
				return accessRows(MonitorStoreOf(app).AccessLogs(200)), nil
			}},
			{Name: "errors", Label: "Error Logs", Type: ComponentTable, Resolve: func(_ context.Context, app AppContext) (any, error) {
				return errorRows(MonitorStoreOf(app).ErrorLogs(200)), nil
			}},
		},
	}
}

// RuntimePanel returns the built-in Go runtime panel.
func RuntimePanel() Panel {
	return ComponentPanel{
		Name:  "runtime",
		Title: "Runtime",
		Icon:  "activity",
		Order: -900,
		Components: []Component{
			{Name: "version", Label: "Go Version", Type: ComponentMetric, Resolve: func(context.Context, AppContext) (any, error) {
				return core.Map{"value": goruntime.Version(), "hint": goruntime.GOOS + "/" + goruntime.GOARCH}, nil
			}},
			{Name: "goroutines", Label: "Goroutines", Type: ComponentMetric, Resolve: func(context.Context, AppContext) (any, error) {
				return goruntime.NumGoroutine(), nil
			}},
			{Name: "heap", Label: "Heap", Type: ComponentMetric, Resolve: func(context.Context, AppContext) (any, error) {
				return heapMetric(), nil
			}},
			{Name: "gc_runs", Label: "GC Runs", Type: ComponentMetric, Resolve: func(context.Context, AppContext) (any, error) {
				return gcMetric(), nil
			}},
			{Name: "go", Label: "Go", Type: ComponentTable, Resolve: func(context.Context, AppContext) (any, error) {
				return runtimeRows(), nil
			}},
			{Name: "memory", Label: "Memory", Type: ComponentTable, Resolve: func(context.Context, AppContext) (any, error) {
				return memoryRows(), nil
			}},
			{Name: "gc", Label: "GC", Type: ComponentTable, Resolve: func(context.Context, AppContext) (any, error) {
				return gcRows(), nil
			}},
		},
	}
}

// RoutesPanel returns the built-in route snapshot panel.
func RoutesPanel() Panel {
	return ComponentPanel{
		Name:  "routes",
		Title: "Routes",
		Icon:  "route",
		Order: -800,
		Components: []Component{
			{Name: "total", Label: "Routes", Type: ComponentMetric, Resolve: func(_ context.Context, app AppContext) (any, error) {
				return routeTotalMetric(PublicRoutes(app)), nil
			}},
			{Name: "secured", Label: "Secured", Type: ComponentMetric, Resolve: func(_ context.Context, app AppContext) (any, error) {
				return routeSecuredMetric(PublicRoutes(app)), nil
			}},
			{Name: "middlewares", Label: "Middlewares", Type: ComponentMetric, Resolve: func(_ context.Context, app AppContext) (any, error) {
				return routeMiddlewareMetric(PublicRoutes(app)), nil
			}},
			{Name: "routes", Label: "Routes", Type: ComponentTable, Resolve: func(_ context.Context, app AppContext) (any, error) { return routeRows(PublicRoutes(app)), nil }},
		},
	}
}

// QueuePanel returns the built-in queue and worker panel.
func QueuePanel() Panel {
	return ComponentPanel{
		Name:  "jobs",
		Title: "Jobs",
		Icon:  "list-tree",
		Order: -700,
		Components: []Component{
			{Name: "backlog", Label: "Backlog", Type: ComponentMetric, Resolve: func(ctx context.Context, app AppContext) (any, error) {
				return queueBacklogMetric(queueInfos(ctx, app)), nil
			}},
			{Name: "failed_total", Label: "Failed", Type: ComponentMetric, Resolve: func(ctx context.Context, app AppContext) (any, error) {
				return queueFailedMetric(queueInfos(ctx, app)), nil
			}},
			{Name: "worker_status", Label: "Workers", Type: ComponentMetric, Resolve: func(ctx context.Context, app AppContext) (any, error) {
				return workerStatusMetric(workerInfos(ctx, app)), nil
			}},
			{Name: "handlers", Label: "Handlers", Type: ComponentMetric, Resolve: func(_ context.Context, app AppContext) (any, error) {
				return len(jobInfos(app)), nil
			}},
			{Name: "pressure", Label: "Queue Pressure", Type: ComponentLine, Resolve: func(_ context.Context, app AppContext) (any, error) {
				return MonitorStoreOf(app).QueuePressureSeries(time.Hour, 12), nil
			}},
			{Name: "throughput", Label: "Worker Throughput", Type: ComponentBar, Resolve: func(_ context.Context, app AppContext) (any, error) {
				return MonitorStoreOf(app).WorkerThroughputSeries(time.Hour, 12), nil
			}},
			{Name: "failures", Label: "Job Failures", Type: ComponentBar, Resolve: func(_ context.Context, app AppContext) (any, error) {
				return MonitorStoreOf(app).JobFailureSeries(time.Hour, 12), nil
			}},
			{Name: "job_latency", Label: "Job Latency Avg (ms)", Type: ComponentLine, Resolve: func(_ context.Context, app AppContext) (any, error) {
				return MonitorStoreOf(app).JobLatencySeries(time.Hour, 12), nil
			}},
			{Name: "failed", Label: "Failed Queues", Type: ComponentTable, Resolve: func(ctx context.Context, app AppContext) (any, error) {
				return failedQueueRows(queueInfos(ctx, app)), nil
			}},
			{Name: "queues", Label: "Queues", Type: ComponentTable, Resolve: func(ctx context.Context, app AppContext) (any, error) {
				return queueRows(queueInfos(ctx, app)), nil
			}},
			{Name: "workers", Label: "Workers", Type: ComponentTable, Resolve: func(ctx context.Context, app AppContext) (any, error) {
				return workerRows(workerInfos(ctx, app)), nil
			}},
			{Name: "jobs", Label: "Jobs", Type: ComponentTable, Resolve: func(_ context.Context, app AppContext) (any, error) { return jobRows(jobInfos(app)), nil }},
			{Name: "job_stats", Label: "Job Stats", Type: ComponentTable, Resolve: func(_ context.Context, app AppContext) (any, error) {
				return jobStatRows(MonitorStoreOf(app).JobStats(100)), nil
			}},
			{Name: "job_logs", Label: "Job Logs", Type: ComponentTable, Resolve: func(_ context.Context, app AppContext) (any, error) {
				return jobLogRows(MonitorStoreOf(app).JobLogs(100)), nil
			}},
		},
	}
}

// MessagePanel returns the built-in message broker panel.
func MessagePanel() Panel {
	return ComponentPanel{
		Name:  "messages",
		Title: "Messages",
		Icon:  "radio",
		Order: -690,
		Components: []Component{
			{Name: "brokers_total", Label: "Brokers", Type: ComponentMetric, Resolve: func(_ context.Context, app AppContext) (any, error) {
				return core.Map{"value": len(messageInfos(app)), "hint": "registered brokers"}, nil
			}},
			{Name: "subscriptions_total", Label: "Subscriptions", Type: ComponentMetric, Resolve: func(_ context.Context, app AppContext) (any, error) {
				return core.Map{"value": len(messageSubscriptionInfos(app)), "hint": "registered consumers"}, nil
			}},
			{Name: "publish", Label: "Published", Type: ComponentBar, Resolve: func(_ context.Context, app AppContext) (any, error) {
				return MonitorStoreOf(app).MessagePublishSeries(time.Hour, 12), nil
			}},
			{Name: "consume", Label: "Consumed", Type: ComponentBar, Resolve: func(_ context.Context, app AppContext) (any, error) {
				return MonitorStoreOf(app).MessageConsumeSeries(time.Hour, 12), nil
			}},
			{Name: "errors", Label: "Message Errors", Type: ComponentLine, Resolve: func(_ context.Context, app AppContext) (any, error) {
				return MonitorStoreOf(app).MessageErrorSeries(time.Hour, 12), nil
			}},
			{Name: "brokers", Label: "Brokers", Type: ComponentTable, Resolve: func(_ context.Context, app AppContext) (any, error) {
				return messageBrokerRows(messageInfos(app)), nil
			}},
			{Name: "subscriptions", Label: "Subscriptions", Type: ComponentTable, Resolve: func(_ context.Context, app AppContext) (any, error) {
				return messageSubscriptionRows(messageSubscriptionInfos(app)), nil
			}},
			{Name: "stats", Label: "Topic Stats", Type: ComponentTable, Resolve: func(_ context.Context, app AppContext) (any, error) {
				return messageStatRows(MonitorStoreOf(app).MessageStats(100)), nil
			}},
			{Name: "logs", Label: "Message Logs", Type: ComponentTable, Resolve: func(_ context.Context, app AppContext) (any, error) {
				return messageLogRows(MonitorStoreOf(app).MessageLogs(100)), nil
			}},
		},
	}
}

// WebSocketPanel returns the built-in websocket panel.
func WebSocketPanel() Panel {
	return ComponentPanel{
		Name:  "websocket",
		Title: "WebSocket",
		Icon:  "waypoints",
		Order: -680,
		Components: []Component{
			{Name: "hubs_total", Label: "Hubs", Type: ComponentMetric, Resolve: func(_ context.Context, app AppContext) (any, error) {
				return core.Map{"value": len(websocketHubs(app)), "hint": "registered hubs"}, nil
			}},
			{Name: "clients_total", Label: "Clients", Type: ComponentMetric, Resolve: func(_ context.Context, app AppContext) (any, error) {
				total := 0
				for _, hub := range websocketHubs(app) {
					total += hub.Stats().Clients
				}
				return core.Map{"value": total, "hint": "online clients"}, nil
			}},
			{Name: "in", Label: "Messages In", Type: ComponentBar, Resolve: func(_ context.Context, app AppContext) (any, error) {
				return MonitorStoreOf(app).WSMessageInSeries(time.Hour, 12), nil
			}},
			{Name: "out", Label: "Messages Out", Type: ComponentBar, Resolve: func(_ context.Context, app AppContext) (any, error) {
				return MonitorStoreOf(app).WSMessageOutSeries(time.Hour, 12), nil
			}},
			{Name: "hubs", Label: "Hubs", Type: ComponentTable, Resolve: func(_ context.Context, app AppContext) (any, error) {
				return wsHubRows(websocketHubs(app)), nil
			}},
			{Name: "samples", Label: "Samples", Type: ComponentTable, Resolve: func(_ context.Context, app AppContext) (any, error) {
				return wsSampleRows(MonitorStoreOf(app).WSSamples(100)), nil
			}},
		},
	}
}

// RPCPanel returns the built-in JSON-RPC panel.
func RPCPanel() Panel {
	return ComponentPanel{
		Name:  "rpc",
		Title: "JSON-RPC",
		Icon:  "braces",
		Order: -670,
		Components: []Component{
			{Name: "calls_total", Label: "Calls", Type: ComponentMetric, Resolve: func(_ context.Context, app AppContext) (any, error) {
				return core.Map{"value": len(MonitorStoreOf(app).RPCLogs(defaultMonitorLimit)), "hint": "recent calls"}, nil
			}},
			{Name: "errors_total", Label: "Errors", Type: ComponentMetric, Resolve: func(_ context.Context, app AppContext) (any, error) {
				count := 0
				for _, item := range MonitorStoreOf(app).RPCLogs(defaultMonitorLimit) {
					if item.Error != "" {
						count++
					}
				}
				return core.Map{"value": count, "hint": "recent errors"}, nil
			}},
			{Name: "calls", Label: "Calls", Type: ComponentLine, Resolve: func(_ context.Context, app AppContext) (any, error) {
				return MonitorStoreOf(app).RPCSeries(time.Hour, 12), nil
			}},
			{Name: "latency", Label: "Latency Avg (ms)", Type: ComponentLine, Resolve: func(_ context.Context, app AppContext) (any, error) {
				return MonitorStoreOf(app).RPCLatencySeries(time.Hour, 12), nil
			}},
			{Name: "errors", Label: "Errors", Type: ComponentBar, Resolve: func(_ context.Context, app AppContext) (any, error) {
				return MonitorStoreOf(app).RPCErrorSeries(time.Hour, 12), nil
			}},
			{Name: "methods", Label: "Method Stats", Type: ComponentTable, Resolve: func(_ context.Context, app AppContext) (any, error) {
				return rpcStatRows(MonitorStoreOf(app).RPCStats(100)), nil
			}},
			{Name: "logs", Label: "RPC Logs", Type: ComponentTable, Resolve: func(_ context.Context, app AppContext) (any, error) {
				return rpcLogRows(MonitorStoreOf(app).RPCLogs(100)), nil
			}},
		},
	}
}

// ORMPanel returns the built-in ORM and SQL execution panel.
func ORMPanel() Panel {
	return ComponentPanel{
		Name:  "orm",
		Title: "ORM",
		Icon:  "database-zap",
		Order: -660,
		Components: []Component{
			{Name: "queries_total", Label: "Queries", Type: ComponentMetric, Resolve: func(_ context.Context, app AppContext) (any, error) {
				return core.Map{"value": len(MonitorStoreOf(app).SQLLogs(defaultMonitorLimit)), "hint": "recent SQL executions"}, nil
			}},
			{Name: "errors_total", Label: "Errors", Type: ComponentMetric, Resolve: func(_ context.Context, app AppContext) (any, error) {
				count := 0
				for _, item := range MonitorStoreOf(app).SQLLogs(defaultMonitorLimit) {
					if item.Error != "" {
						count++
					}
				}
				return core.Map{"value": count, "hint": "recent SQL errors"}, nil
			}},
			{Name: "slow_total", Label: "Slow SQL", Type: ComponentMetric, Resolve: func(_ context.Context, app AppContext) (any, error) {
				count := 0
				for _, item := range MonitorStoreOf(app).SQLLogs(defaultMonitorLimit) {
					if item.Slow {
						count++
					}
				}
				return core.Map{"value": count, "hint": "marked by ORM logger"}, nil
			}},
			{Name: "latency_avg", Label: "Avg Latency", Type: ComponentMetric, Resolve: func(_ context.Context, app AppContext) (any, error) {
				items := MonitorStoreOf(app).SQLLogs(defaultMonitorLimit)
				if len(items) == 0 {
					return core.Map{"value": "-", "hint": "no SQL yet"}, nil
				}
				var total time.Duration
				for _, item := range items {
					total += item.Latency
				}
				return core.Map{"value": formatDuration(total / time.Duration(len(items))), "hint": "recent average"}, nil
			}},
			{Name: "queries", Label: "Queries", Type: ComponentBar, Resolve: func(_ context.Context, app AppContext) (any, error) {
				return MonitorStoreOf(app).SQLSeries(time.Hour, 12), nil
			}},
			{Name: "latency", Label: "Latency Avg (ms)", Type: ComponentLine, Resolve: func(_ context.Context, app AppContext) (any, error) {
				return MonitorStoreOf(app).SQLLatencySeries(time.Hour, 12), nil
			}},
			{Name: "errors", Label: "Errors", Type: ComponentBar, Resolve: func(_ context.Context, app AppContext) (any, error) {
				return MonitorStoreOf(app).SQLErrorSeries(time.Hour, 12), nil
			}},
			{Name: "tables", Label: "Table Stats", Type: ComponentTable, Resolve: func(_ context.Context, app AppContext) (any, error) {
				return sqlStatRows(MonitorStoreOf(app).SQLStats(100)), nil
			}},
			{Name: "logs", Label: "SQL Logs", Type: ComponentTable, Resolve: func(_ context.Context, app AppContext) (any, error) {
				return sqlLogRows(MonitorStoreOf(app).SQLLogs(100)), nil
			}},
		},
	}
}

// SchedulePanel returns the built-in schedule/task/event panel.
func SchedulePanel() Panel {
	return ComponentPanel{
		Name:  "schedule",
		Title: "Schedule",
		Icon:  "calendar-clock",
		Order: -600,
		Components: []Component{
			{Name: "total", Label: "Schedules", Type: ComponentMetric, Resolve: func(_ context.Context, app AppContext) (any, error) {
				return scheduleTotalMetric(scheduleInfos(app)), nil
			}},
			{Name: "enabled", Label: "Enabled", Type: ComponentMetric, Resolve: func(_ context.Context, app AppContext) (any, error) {
				return scheduleEnabledMetric(scheduleInfos(app)), nil
			}},
			{Name: "queued", Label: "Queued", Type: ComponentMetric, Resolve: func(_ context.Context, app AppContext) (any, error) {
				return scheduleQueuedMetric(scheduleInfos(app)), nil
			}},
			{Name: "handlers", Label: "Handlers", Type: ComponentMetric, Resolve: func(_ context.Context, app AppContext) (any, error) {
				return core.Map{"value": len(taskInfos(app)), "hint": fmt.Sprintf("%d events", len(eventInfos(app)))}, nil
			}},
			{Name: "state", Label: "Schedule State", Type: ComponentBar, Resolve: func(_ context.Context, app AppContext) (any, error) {
				return scheduleStateRows(scheduleInfos(app)), nil
			}},
			{Name: "schedule", Label: "Schedule", Type: ComponentTable, Resolve: func(_ context.Context, app AppContext) (any, error) {
				return scheduleRows(scheduleInfos(app)), nil
			}},
			{Name: "tasks", Label: "Tasks", Type: ComponentTable, Resolve: func(_ context.Context, app AppContext) (any, error) { return taskRows(taskInfos(app)), nil }},
			{Name: "events", Label: "Events", Type: ComponentTable, Resolve: func(_ context.Context, app AppContext) (any, error) { return eventRows(eventInfos(app)), nil }},
		},
	}
}

// InfrastructurePanel returns built-in infrastructure snapshots.
func InfrastructurePanel() Panel {
	return ComponentPanel{
		Name:  "infrastructure",
		Title: "Infrastructure",
		Icon:  "server",
		Order: -500,
		Components: []Component{
			{Name: "modules", Label: "Modules", Type: ComponentMetric, Resolve: func(ctx context.Context, app AppContext) (any, error) {
				return infrastructureModulesMetric(ctx, app), nil
			}},
			{Name: "drivers", Label: "Drivers", Type: ComponentMetric, Resolve: func(ctx context.Context, app AppContext) (any, error) {
				return infrastructureDriversMetric(ctx, app), nil
			}},
			{Name: "defaults", Label: "Defaults", Type: ComponentMetric, Resolve: func(ctx context.Context, app AppContext) (any, error) {
				return infrastructureDefaultsMetric(ctx, app), nil
			}},
			{Name: "database", Label: "Database", Type: ComponentTable, Resolve: func(_ context.Context, app AppContext) (any, error) {
				return databaseRows(databaseInfos(app)), nil
			}},
			{Name: "cache", Label: "Cache", Type: ComponentTable, Resolve: func(_ context.Context, app AppContext) (any, error) { return cacheRows(cacheInfos(app)), nil }},
			{Name: "storage", Label: "Storage", Type: ComponentTable, Resolve: func(_ context.Context, app AppContext) (any, error) { return storageRows(storageInfos(app)), nil }},
			{Name: "session", Label: "Session", Type: ComponentTable, Resolve: func(_ context.Context, app AppContext) (any, error) { return sessionRows(sessionInfos(app)), nil }},
			{Name: "rate", Label: "Rate", Type: ComponentTable, Resolve: func(_ context.Context, app AppContext) (any, error) { return rateRows(rateInfos(app)), nil }},
			{Name: "lock", Label: "Lock", Type: ComponentTable, Resolve: func(_ context.Context, app AppContext) (any, error) { return lockRows(lockInfos(app)), nil }},
			{Name: "view", Label: "View", Type: ComponentTable, Resolve: func(_ context.Context, app AppContext) (any, error) { return viewRows(viewInfos(app)), nil }},
			{Name: "asset", Label: "Asset", Type: ComponentTable, Resolve: func(_ context.Context, app AppContext) (any, error) { return assetRows(assetInfos(app)), nil }},
			{Name: "log", Label: "Log", Type: ComponentTable, Resolve: func(_ context.Context, app AppContext) (any, error) { return logRows(logInfos(app)), nil }},
			{Name: "auth", Label: "Auth", Type: ComponentTable, Resolve: func(_ context.Context, app AppContext) (any, error) { return namesTable(authNames(app)), nil }},
			{Name: "permission", Label: "Permission", Type: ComponentTable, Resolve: func(_ context.Context, app AppContext) (any, error) {
				return permissionRows(permissionInfos(app)), nil
			}},
		},
	}
}

// State returns current app runtime snapshots.
func State(app AppContext) core.Map {
	if app == nil {
		return core.Map{}
	}
	return core.Map{
		"env":        appEnv(app),
		"version":    "dev",
		"checked_at": core.Now().Format(time.RFC3339Nano),
		"runtime":    runtimeState(),
		"routes":     Routes(app),
		"database":   databaseInfos(app),
		"cache":      cacheInfos(app),
		"queue":      queueInfos(context.Background(), app),
		"worker":     workerInfos(context.Background(), app),
		"job":        jobInfos(app),
		"event":      eventInfos(app),
		"task":       taskInfos(app),
		"schedule":   scheduleInfos(app),
		"storage":    storageInfos(app),
		"session":    sessionInfos(app),
		"rate":       rateInfos(app),
		"lock":       lockInfos(app),
		"view":       viewInfos(app),
		"asset":      assetInfos(app),
		"log":        logInfos(app),
		"auth":       authNames(app),
		"permission": permissionInfos(app),
	}
}

// Runtime returns current app runtime snapshots.
func Runtime(app AppContext) core.Map { return State(app) }

func runtimeState() core.Map {
	var memory goruntime.MemStats
	goruntime.ReadMemStats(&memory)
	return core.Map{
		"goos":       goruntime.GOOS,
		"goarch":     goruntime.GOARCH,
		"gomaxprocs": goruntime.GOMAXPROCS(0),
		"goroutine":  goruntime.NumGoroutine(),
		"memory": core.Map{
			"alloc":       memory.Alloc,
			"total_alloc": memory.TotalAlloc,
			"sys":         memory.Sys,
			"heap_alloc":  memory.HeapAlloc,
			"heap_sys":    memory.HeapSys,
		},
	}
}

func numberValue(value any) uint64 {
	switch typed := value.(type) {
	case uint64:
		return typed
	case uint:
		return uint64(typed)
	case int64:
		return uint64(typed)
	case int:
		return uint64(typed)
	default:
		return 0
	}
}

func formatBytes(value uint64) string {
	const unit = 1024
	if value < unit {
		return fmt.Sprintf("%d B", value)
	}
	units := []string{"KB", "MB", "GB", "TB"}
	amount := float64(value)
	index := -1
	for amount >= unit && index < len(units)-1 {
		amount /= unit
		index++
	}
	return fmt.Sprintf("%.1f %s", amount, units[index])
}

func scheduleSummary(app AppContext) core.Map {
	items := scheduleInfos(app)
	enabled := 0
	for _, item := range items {
		if item.Enabled {
			enabled++
		}
	}
	if len(items) == 0 {
		return core.Map{"value": 0, "hint": "not configured"}
	}
	return core.Map{"value": len(items), "hint": fmt.Sprintf("%d enabled", enabled)}
}

func attentionItems(ctx context.Context, app AppContext) []core.Map {
	items := make([]core.Map, 0)
	for _, item := range appHosts(app) {
		status := string(item.Status)
		if status == "failed" || status == "unhealthy" {
			items = append(items, core.Map{"level": "error", "source": "host", "message": fmt.Sprintf("%s is %s", item.Name, status)})
		}
	}
	for _, item := range queueInfos(ctx, app) {
		if item.Failed > 0 {
			items = append(items, core.Map{"level": "error", "source": "queue", "message": fmt.Sprintf("%s has %d failed jobs", item.Name, item.Failed)})
		}
		if item.Pending > 0 && len(item.Workers) == 0 {
			items = append(items, core.Map{"level": "warning", "source": "queue", "message": fmt.Sprintf("%s has pending jobs but no workers", item.Name)})
		}
	}
	for _, item := range workerInfos(ctx, app) {
		if item.Status != "" && item.Status != "running" && item.Status != "idle" {
			items = append(items, core.Map{"level": "warning", "source": "worker", "message": fmt.Sprintf("%s is %s", item.Name, item.Status)})
		}
		if item.Failed > 0 {
			items = append(items, core.Map{"level": "warning", "source": "worker", "message": fmt.Sprintf("%s failed %d jobs", item.Name, item.Failed)})
		}
	}
	for _, item := range databaseInfos(app) {
		if item.Status != "" && item.Status != "open" && item.Status != "ready" {
			items = append(items, core.Map{"level": "warning", "source": "database", "message": fmt.Sprintf("%s is %s", item.Name, item.Status)})
		}
	}
	return items
}

func BuiltinSummaries() []SummaryResolver {
	return []SummaryResolver{
		HostSummary,
		DatabaseSummary,
		CacheSummary,
		QueueSummary,
		StorageSummary,
		SessionSummary,
		RateSummary,
		LockSummary,
	}
}

func HostSummary(_ context.Context, app AppContext) []Summary {
	items := appHosts(app)
	if len(items) == 0 {
		return nil
	}
	return []Summary{{Module: "Host", Default: defaultHostSummary(items)}}
}

func DatabaseSummary(_ context.Context, app AppContext) []Summary {
	items := databaseInfos(app)
	if len(items) == 0 {
		return nil
	}
	return []Summary{{Module: "Database", Summary: databaseDriverSummary(items), Default: defaultDatabaseSummary(items)}}
}

func CacheSummary(_ context.Context, app AppContext) []Summary {
	items := cacheInfos(app)
	if len(items) == 0 {
		return nil
	}
	return []Summary{{Module: "Cache", Summary: cacheDriverSummary(items), Default: defaultCacheSummary(items)}}
}

func QueueSummary(ctx context.Context, app AppContext) []Summary {
	items := queueInfos(ctx, app)
	if len(items) == 0 {
		return nil
	}
	return []Summary{{Module: "Queue", Summary: queueDriverSummary(items), Default: defaultQueueSummary(items)}}
}

func StorageSummary(_ context.Context, app AppContext) []Summary {
	items := storageInfos(app)
	if len(items) == 0 {
		return nil
	}
	return []Summary{{Module: "Storage", Summary: storageDriverSummary(items), Default: defaultStorageSummary(items)}}
}

func SessionSummary(_ context.Context, app AppContext) []Summary {
	items := sessionInfos(app)
	if len(items) == 0 {
		return nil
	}
	return []Summary{{Module: "Session", Summary: sessionDriverSummary(items), Default: defaultSessionSummary(items)}}
}

func RateSummary(_ context.Context, app AppContext) []Summary {
	items := rateInfos(app)
	if len(items) == 0 {
		return nil
	}
	return []Summary{{Module: "Rate", Summary: rateDriverSummary(items), Default: defaultRateSummary(items)}}
}

func LockSummary(_ context.Context, app AppContext) []Summary {
	items := lockInfos(app)
	if len(items) == 0 {
		return nil
	}
	return []Summary{{Module: "Lock", Summary: lockDriverSummary(items), Default: defaultLockSummary(items)}}
}

func defaultHostSummary(items []host.Info) string {
	return uniqueSummary(items, func(item host.Info) string { return item.Addr })
}

func databaseDriverSummary(items []database.Info) string {
	return uniqueSummary(items, func(item database.Info) string { return item.Kind })
}

func defaultDatabaseSummary(items []database.Info) string {
	return defaultByName(items, func(item database.Info) string { return item.Name }, func(item database.Info) string {
		if item.Kind != "" {
			return item.Kind
		}
		return item.Dialect
	})
}

func cacheDriverSummary(items []cache.Info) string {
	return uniqueSummary(items, func(item cache.Info) string { return item.Driver })
}

func queueDriverSummary(items []queue.QueueInfo) string {
	return uniqueSummary(items, func(item queue.QueueInfo) string { return item.Driver })
}

func storageDriverSummary(items []storage.Info) string {
	return uniqueSummary(items, func(item storage.Info) string { return item.Driver })
}

func sessionDriverSummary(items []session.Info) string {
	return uniqueSummary(items, func(item session.Info) string { return item.Driver })
}

func rateDriverSummary(items []rate.Info) string {
	return uniqueSummary(items, func(item rate.Info) string { return item.Driver })
}

func lockDriverSummary(items []lock.Info) string {
	return uniqueSummary(items, func(item lock.Info) string { return item.Driver })
}

func defaultCacheSummary(items []cache.Info) string {
	return defaultByName(items, func(item cache.Info) string { return item.Name }, func(item cache.Info) string { return item.Driver })
}

func defaultQueueSummary(items []queue.QueueInfo) string {
	return defaultByName(items, func(item queue.QueueInfo) string { return item.Name }, func(item queue.QueueInfo) string { return item.Driver })
}

func defaultStorageSummary(items []storage.Info) string {
	for _, item := range items {
		if item.Default {
			return item.Driver
		}
	}
	return ""
}

func defaultSessionSummary(items []session.Info) string {
	for _, item := range items {
		if item.Default {
			return item.Driver
		}
	}
	return ""
}

func defaultRateSummary(items []rate.Info) string {
	for _, item := range items {
		if item.Default {
			return item.Driver
		}
	}
	return ""
}

func defaultLockSummary(items []lock.Info) string {
	return defaultByName(items, func(item lock.Info) string { return item.Name }, func(item lock.Info) string { return item.Driver })
}

func uniqueSummary[T any](items []T, value func(T) string) string {
	values := make(map[string]struct{})
	for _, item := range items {
		name := value(item)
		if name != "" {
			values[name] = struct{}{}
		}
	}
	if len(values) == 0 {
		return ""
	}
	names := make([]string, 0, len(values))
	for name := range values {
		names = append(names, name)
	}
	sort.Strings(names)
	return strings.Join(names, "/")
}

func defaultByName[T any](items []T, name func(T) string, value func(T) string) string {
	if len(items) == 0 {
		return ""
	}
	for _, item := range items {
		if name(item) == "default" {
			return value(item)
		}
	}
	if len(items) == 1 {
		return value(items[0])
	}
	return ""
}
