package console

import (
	"context"
	"fmt"
	"github.com/duxweb/runa/core"
	"github.com/duxweb/runa/queue"
	"github.com/duxweb/runa/schedule"
	goruntime "runtime"
	"strings"
)

func trafficRequestsMetric(store MonitorStore) core.Map {
	total := len(store.AccessLogs(defaultMonitorLimit))
	return core.Map{"value": total, "hint": "recent captured requests"}
}

func trafficErrorsMetric(store MonitorStore) core.Map {
	total := len(store.ErrorLogs(defaultMonitorLimit))
	return core.Map{"value": total, "hint": "5xx or captured errors"}
}

func trafficLatencyMetric(store MonitorStore) core.Map {
	items := store.AccessLogs(defaultMonitorLimit)
	if len(items) == 0 {
		return core.Map{"value": "-", "hint": "no traffic yet"}
	}
	var total int64
	for _, item := range items {
		total += item.Latency.Milliseconds()
	}
	return core.Map{"value": fmt.Sprintf("%d ms", total/int64(len(items))), "hint": "recent average"}
}

func trafficSlowMetric(store MonitorStore) core.Map {
	return core.Map{"value": len(store.SlowLogs(defaultMonitorLimit)), "hint": "above slow threshold"}
}

func errorTotalMetric(store MonitorStore) core.Map {
	return core.Map{"value": len(store.ErrorLogs(defaultMonitorLimit)), "hint": "recent error entries"}
}

func errorRateMetric(store MonitorStore) core.Map {
	requests := len(store.AccessLogs(defaultMonitorLimit))
	if requests == 0 {
		return core.Map{"value": "0%", "hint": "no traffic yet"}
	}
	errors := len(store.ErrorLogs(defaultMonitorLimit))
	return core.Map{"value": fmt.Sprintf("%.1f%%", float64(errors)*100/float64(requests)), "hint": fmt.Sprintf("%d of %d", errors, requests)}
}

func lastErrorStatus(store MonitorStore) core.Map {
	items := store.ErrorLogs(1)
	if len(items) == 0 {
		return core.Map{"status": "ok", "hint": "no recent errors"}
	}
	item := items[0]
	return core.Map{"status": "error", "hint": fmt.Sprintf("%s %s at %s", item.Method, item.Path, formatClock(item.Time))}
}

func logAccessMetric(store MonitorStore) core.Map {
	return core.Map{"value": len(store.AccessLogs(defaultMonitorLimit)), "hint": "recent access logs"}
}

func logErrorMetric(store MonitorStore) core.Map {
	return core.Map{"value": len(store.ErrorLogs(defaultMonitorLimit)), "hint": "recent error logs"}
}

func heapMetric() core.Map {
	var memory goruntime.MemStats
	goruntime.ReadMemStats(&memory)
	return core.Map{"value": formatBytes(memory.HeapAlloc), "hint": fmt.Sprintf("%d objects", memory.HeapObjects)}
}

func gcMetric() core.Map {
	var memory goruntime.MemStats
	goruntime.ReadMemStats(&memory)
	return core.Map{"value": memory.NumGC, "hint": "runtime GC cycles"}
}

func routeTotalMetric(items []RouteInfo) core.Map {
	methods := map[string]struct{}{}
	for _, item := range items {
		if item.Method != "" {
			methods[item.Method] = struct{}{}
		}
	}
	return core.Map{"value": len(items), "hint": fmt.Sprintf("%d methods", len(methods))}
}

func routeSecuredMetric(items []RouteInfo) core.Map {
	total := 0
	for _, item := range items {
		if len(item.Security) > 0 {
			total++
		}
	}
	return core.Map{"value": total, "hint": "routes with security metadata"}
}

func routeMiddlewareMetric(items []RouteInfo) core.Map {
	total := 0
	for _, item := range items {
		total += item.Middlewares
	}
	return core.Map{"value": total, "hint": "route middleware bindings"}
}

func queueBacklogMetric(items []queue.QueueInfo) core.Map {
	var pending, reserved, delayed int64
	for _, item := range items {
		pending += item.Pending
		reserved += item.Reserved
		delayed += item.Delayed
	}
	return core.Map{"value": pending + reserved + delayed, "hint": fmt.Sprintf("%d pending", pending)}
}

func queueFailedMetric(items []queue.QueueInfo) core.Map {
	var failed int64
	for _, item := range items {
		failed += item.Failed
	}
	return core.Map{"value": failed, "hint": "failed jobs"}
}

func workerStatusMetric(items []queue.WorkerInfo) core.Map {
	instances := 0
	running := 0
	for _, item := range items {
		instances += item.Instances
		if item.Status == "running" || item.Status == "idle" {
			running++
		}
	}
	return core.Map{"value": len(items), "hint": fmt.Sprintf("%d groups healthy, %d instances", running, instances)}
}

func scheduleTotalMetric(items []schedule.Info) core.Map {
	return core.Map{"value": len(items), "hint": "registered schedules"}
}

func scheduleEnabledMetric(items []schedule.Info) core.Map {
	total := 0
	for _, item := range items {
		if item.Enabled {
			total++
		}
	}
	return core.Map{"value": total, "hint": "active schedules"}
}

func scheduleQueuedMetric(items []schedule.Info) core.Map {
	total := 0
	for _, item := range items {
		if item.Queue != "" || item.Mode == "queue" {
			total++
		}
	}
	return core.Map{"value": total, "hint": "queue-backed schedules"}
}

func infrastructureModulesMetric(ctx context.Context, app AppContext) core.Map {
	return core.Map{"value": len(consoleSummaries(ctx, app)), "hint": "configured modules"}
}

func infrastructureDriversMetric(ctx context.Context, app AppContext) core.Map {
	drivers := map[string]struct{}{}
	for _, summary := range consoleSummaries(ctx, app) {
		if summary.Summary == "" {
			continue
		}
		for _, item := range stringsSplit(summary.Summary) {
			drivers[item] = struct{}{}
		}
	}
	return core.Map{"value": len(drivers), "hint": "unique drivers"}
}

func infrastructureDefaultsMetric(ctx context.Context, app AppContext) core.Map {
	total := 0
	for _, summary := range consoleSummaries(ctx, app) {
		if summary.Default != "" {
			total++
		}
	}
	return core.Map{"value": total, "hint": "modules with defaults"}
}

func stringsSplit(value string) []string {
	if value == "" {
		return nil
	}
	items := strings.Split(value, "/")
	out := make([]string, 0, len(items))
	for _, item := range items {
		if item != "" {
			out = append(out, item)
		}
	}
	return out
}
