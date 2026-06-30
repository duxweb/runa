package console

import (
	"github.com/duxweb/runa/auth"
	"github.com/duxweb/runa/cache"
	"github.com/duxweb/runa/core"
	"github.com/duxweb/runa/event"
	"github.com/duxweb/runa/lock"
	runlog "github.com/duxweb/runa/log"
	"github.com/duxweb/runa/message"
	"github.com/duxweb/runa/queue"
	"github.com/duxweb/runa/rate"
	"github.com/duxweb/runa/schedule"
	"github.com/duxweb/runa/session"
	"github.com/duxweb/runa/storage"
	"github.com/duxweb/runa/task"
	"github.com/duxweb/runa/view"
	"runtime"
	"strings"
	"time"

	"github.com/duxweb/runa/asset"
	"github.com/duxweb/runa/database"
	"github.com/duxweb/runa/ws"
)

func tableRows(columns []string, rows []core.Map) core.Map {
	return core.Map{"columns": columns, "rows": rows}
}

func runtimeRows() core.Map {
	return tableRows([]string{"name", "value"}, []core.Map{
		{"name": "Version", "value": runtime.Version()},
		{"name": "GOOS", "value": runtime.GOOS},
		{"name": "GOARCH", "value": runtime.GOARCH},
		{"name": "GOMAXPROCS", "value": runtime.GOMAXPROCS(0)},
		{"name": "Goroutines", "value": runtime.NumGoroutine()},
	})
}

func memoryRows() core.Map {
	var memory runtime.MemStats
	runtime.ReadMemStats(&memory)
	return tableRows([]string{"name", "value"}, []core.Map{
		{"name": "Alloc", "value": formatBytes(memory.Alloc)},
		{"name": "Total Alloc", "value": formatBytes(memory.TotalAlloc)},
		{"name": "Sys", "value": formatBytes(memory.Sys)},
		{"name": "Heap Alloc", "value": formatBytes(memory.HeapAlloc)},
		{"name": "Heap Sys", "value": formatBytes(memory.HeapSys)},
		{"name": "Heap Objects", "value": memory.HeapObjects},
	})
}

func gcRows() core.Map {
	var memory runtime.MemStats
	runtime.ReadMemStats(&memory)
	return tableRows([]string{"name", "value"}, []core.Map{
		{"name": "GC Runs", "value": memory.NumGC},
		{"name": "Pause Total", "value": formatDuration(time.Duration(memory.PauseTotalNs))},
		{"name": "Next GC", "value": formatBytes(memory.NextGC)},
		{"name": "Last GC", "value": formatUnixNano(memory.LastGC)},
	})
}

func routeRows(items []RouteInfo) core.Map {
	rows := make([]core.Map, 0, len(items))
	for _, item := range items {
		rows = append(rows, core.Map{
			"method":      item.Method,
			"path":        item.Path,
			"name":        item.Name,
			"summary":     item.Summary,
			"status":      item.Status,
			"middlewares": item.Middlewares,
			"security":    joinStrings(item.Security),
			"tags":        joinStrings(item.Tags),
		})
	}
	return tableRows([]string{"method", "path", "name", "summary", "status", "middlewares", "security", "tags"}, rows)
}

func accessRows(items []AccessLog) core.Map {
	rows := make([]core.Map, 0, len(items))
	for _, item := range items {
		rows = append(rows, core.Map{
			"time":       formatClock(item.Time),
			"method":     item.Method,
			"path":       item.Path,
			"route":      item.Route,
			"status":     item.Status,
			"latency_ms": item.Latency.Milliseconds(),
			"ip":         item.IP,
			"request_id": item.RequestID,
		})
	}
	return tableRows([]string{"time", "method", "path", "route", "status", "latency_ms", "ip", "request_id"}, rows)
}

func errorRows(items []ErrorLog) core.Map {
	rows := make([]core.Map, 0, len(items))
	for _, item := range items {
		rows = append(rows, core.Map{
			"time":       formatClock(item.Time),
			"method":     item.Method,
			"path":       item.Path,
			"route":      item.Route,
			"status":     item.Status,
			"latency_ms": item.Latency.Milliseconds(),
			"error":      item.Error,
			"source":     item.Source,
			"request_id": item.RequestID,
		})
	}
	return tableRows([]string{"time", "method", "path", "route", "status", "latency_ms", "error", "source", "request_id"}, rows)
}

func routeStatRows(items []RouteStat) core.Map {
	rows := make([]core.Map, 0, len(items))
	for _, item := range items {
		rows = append(rows, core.Map{
			"method":      item.Method,
			"route":       item.Route,
			"path":        item.Path,
			"count":       item.Count,
			"errors":      item.Errors,
			"min_ms":      item.Min.Milliseconds(),
			"avg_ms":      item.Avg.Milliseconds(),
			"max_ms":      item.Max.Milliseconds(),
			"last_status": item.LastState,
			"last_seen":   formatClock(item.LastSeen),
		})
	}
	return tableRows([]string{"method", "route", "path", "count", "errors", "min_ms", "avg_ms", "max_ms", "last_status", "last_seen"}, rows)
}

func failedQueueRows(items []queue.QueueInfo) core.Map {
	rows := make([]core.Map, 0)
	for _, item := range items {
		if item.Failed == 0 {
			continue
		}
		rows = append(rows, core.Map{
			"name":     item.Name,
			"driver":   item.Driver,
			"pending":  item.Pending,
			"reserved": item.Reserved,
			"delayed":  item.Delayed,
			"failed":   item.Failed,
		})
	}
	return tableRows([]string{"name", "driver", "pending", "reserved", "delayed", "failed"}, rows)
}

func queueRows(items []queue.QueueInfo) core.Map {
	rows := make([]core.Map, 0, len(items))
	for _, item := range items {
		rows = append(rows, core.Map{
			"name":     item.Name,
			"driver":   item.Driver,
			"workers":  joinStrings(item.Workers),
			"pending":  item.Pending,
			"reserved": item.Reserved,
			"delayed":  item.Delayed,
			"failed":   item.Failed,
		})
	}
	return tableRows([]string{"name", "driver", "workers", "pending", "reserved", "delayed", "failed"}, rows)
}

func workerRows(items []queue.WorkerInfo) core.Map {
	rows := make([]core.Map, 0, len(items))
	for _, item := range items {
		rows = append(rows, core.Map{
			"name":        item.Name,
			"queues":      joinStrings(item.Queues),
			"concurrency": item.Concurrency,
			"instances":   item.Instances,
			"status":      item.Status,
			"processed":   item.Processed,
			"succeeded":   item.Succeeded,
			"failed":      item.Failed,
			"retried":     item.Retried,
		})
	}
	return tableRows([]string{"name", "queues", "concurrency", "instances", "status", "processed", "succeeded", "failed", "retried"}, rows)
}

func jobRows(items []queue.JobInfo) core.Map {
	rows := make([]core.Map, 0, len(items))
	for _, item := range items {
		rows = append(rows, core.Map{"name": item.Name, "payload": item.Payload, "source": item.Source})
	}
	return tableRows([]string{"name", "payload", "source"}, rows)
}

func scheduleRows(items []schedule.Info) core.Map {
	rows := make([]core.Map, 0, len(items))
	for _, item := range items {
		status := "disabled"
		if item.Enabled {
			status = "enabled"
		}
		rows = append(rows, core.Map{
			"name":     item.Name,
			"spec":     item.Spec,
			"task":     item.Task,
			"mode":     item.Mode,
			"queue":    item.Queue,
			"timezone": item.Timezone,
			"status":   status,
		})
	}
	return tableRows([]string{"name", "spec", "task", "mode", "queue", "timezone", "status"}, rows)
}

func taskRows(items []task.Info) core.Map {
	rows := make([]core.Map, 0, len(items))
	for _, item := range items {
		rows = append(rows, core.Map{
			"name":    item.Name,
			"payload": item.Payload,
			"timeout": formatDuration(item.Timeout),
			"retry":   item.Retry,
			"source":  item.Source,
		})
	}
	return tableRows([]string{"name", "payload", "timeout", "retry", "source"}, rows)
}

func eventRows(items []event.Info) core.Map {
	rows := make([]core.Map, 0, len(items))
	for _, item := range items {
		rows = append(rows, core.Map{
			"name":     item.Name,
			"payload":  item.Payload,
			"listener": item.Listener,
			"priority": item.Priority,
			"queue":    item.Queue,
			"async":    boolText(item.Async),
			"source":   item.Source,
		})
	}
	return tableRows([]string{"name", "payload", "listener", "priority", "queue", "async", "source"}, rows)
}

func databaseRows(items []database.Info) core.Map {
	rows := make([]core.Map, 0, len(items))
	for _, item := range items {
		rows = append(rows, core.Map{"name": item.Name, "kind": item.Kind, "dialect": item.Dialect, "status": item.Status})
	}
	return tableRows([]string{"name", "kind", "dialect", "status"}, rows)
}

func cacheRows(items []cache.Info) core.Map {
	rows := make([]core.Map, 0, len(items))
	for _, item := range items {
		rows = append(rows, core.Map{"name": item.Name, "driver": item.Driver, "prefix": item.Prefix, "ttl": formatDuration(item.TTL)})
	}
	return tableRows([]string{"name", "driver", "prefix", "ttl"}, rows)
}

func storageRows(items []storage.Info) core.Map {
	rows := make([]core.Map, 0, len(items))
	for _, item := range items {
		rows = append(rows, core.Map{"name": item.Name, "driver": item.Driver, "prefix": item.Prefix, "public": boolText(item.Public), "default": boolText(item.Default)})
	}
	return tableRows([]string{"name", "driver", "prefix", "public", "default"}, rows)
}

func sessionRows(items []session.Info) core.Map {
	rows := make([]core.Map, 0, len(items))
	for _, item := range items {
		rows = append(rows, core.Map{
			"name":        item.Name,
			"driver":      item.Driver,
			"cookie_name": item.CookieName,
			"ttl":         formatDuration(item.TTL),
			"shared":      boolText(item.Shared),
			"default":     boolText(item.Default),
		})
	}
	return tableRows([]string{"name", "driver", "cookie_name", "ttl", "shared", "default"}, rows)
}

func rateRows(items []rate.Info) core.Map {
	rows := make([]core.Map, 0, len(items))
	for _, item := range items {
		rows = append(rows, core.Map{
			"name":      item.Name,
			"driver":    item.Driver,
			"algorithm": item.Algorithm,
			"limit":     item.Limit,
			"window":    formatDuration(item.Window),
			"burst":     item.Burst,
			"default":   boolText(item.Default),
		})
	}
	return tableRows([]string{"name", "driver", "algorithm", "limit", "window", "burst", "default"}, rows)
}

func lockRows(items []lock.Info) core.Map {
	rows := make([]core.Map, 0, len(items))
	for _, item := range items {
		rows = append(rows, core.Map{
			"name":           item.Name,
			"driver":         item.Driver,
			"prefix":         item.Prefix,
			"ttl":            formatDuration(item.TTL),
			"wait":           formatDuration(item.Wait),
			"retry_interval": formatDuration(item.RetryInterval),
			"auto_renew":     boolText(item.AutoRenew),
		})
	}
	return tableRows([]string{"name", "driver", "prefix", "ttl", "wait", "retry_interval", "auto_renew"}, rows)
}

func viewRows(items []view.Info) core.Map {
	rows := make([]core.Map, 0, len(items))
	for _, item := range items {
		rows = append(rows, core.Map{"name": item.Name, "default": boolText(item.Default)})
	}
	return tableRows([]string{"name", "default"}, rows)
}

func assetRows(items []asset.Info) core.Map {
	rows := make([]core.Map, 0, len(items))
	for _, item := range items {
		rows = append(rows, core.Map{"name": item.Name, "files": item.Files, "default": boolText(item.Default)})
	}
	return tableRows([]string{"name", "files", "default"}, rows)
}

func logRows(items []runlog.Info) core.Map {
	rows := make([]core.Map, 0, len(items))
	for _, item := range items {
		rows = append(rows, core.Map{"name": item.Name, "outputs": item.Outputs, "default": boolText(item.Default)})
	}
	return tableRows([]string{"name", "outputs", "default"}, rows)
}

func namesTable(names []string) core.Map {
	rows := make([]core.Map, 0, len(names))
	for _, name := range names {
		rows = append(rows, core.Map{"name": name})
	}
	return tableRows([]string{"name"}, rows)
}

func permissionRows(items []auth.PermissionInfo) core.Map {
	rows := make([]core.Map, 0, len(items))
	for _, item := range items {
		rows = append(rows, core.Map{
			"id":     item.ID,
			"name":   item.Name,
			"label":  item.Label,
			"group":  item.Group,
			"method": item.Method,
			"path":   item.Path,
		})
	}
	return tableRows([]string{"id", "name", "label", "group", "method", "path"}, rows)
}

func scheduleStateRows(items []schedule.Info) []core.Map {
	enabled := 0
	disabled := 0
	for _, item := range items {
		if item.Enabled {
			enabled++
		} else {
			disabled++
		}
	}
	return []core.Map{{"label": "enabled", "value": enabled}, {"label": "disabled", "value": disabled}}
}

func formatClock(value time.Time) string {
	if value.IsZero() {
		return "-"
	}
	return value.Format("15:04:05")
}

func formatUnixNano(value uint64) string {
	if value == 0 {
		return "-"
	}
	return time.Unix(0, int64(value)).Format("15:04:05")
}

func formatDuration(value time.Duration) string {
	if value <= 0 {
		return "-"
	}
	text := value.Round(time.Millisecond).String()
	if strings.Contains(text, "m") && strings.HasSuffix(text, "0s") {
		text = strings.TrimSuffix(text, "0s")
	}
	if strings.HasSuffix(text, "h0m") {
		text = strings.TrimSuffix(text, "0m")
	}
	return text
}

func boolText(value bool) string {
	if value {
		return "yes"
	}
	return "-"
}

func compactSQL(value string) string {
	value = strings.Join(strings.Fields(value), " ")
	if len(value) <= 180 {
		return value
	}
	return value[:177] + "..."
}

func joinStrings(values []string) string {
	if len(values) == 0 {
		return "-"
	}
	return strings.Join(values, ", ")
}

func messageBrokerRows(items []message.BrokerInfo) core.Map {
	rows := make([]core.Map, 0, len(items))
	for _, item := range items {
		rows = append(rows, core.Map{"name": item.Name, "driver": item.Driver, "codec": item.Codec, "subscribers": item.Subscribers})
	}
	return tableRows([]string{"name", "driver", "codec", "subscribers"}, rows)
}

func messageSubscriptionRows(items []message.SubscriptionInfo) core.Map {
	rows := make([]core.Map, 0, len(items))
	for _, item := range items {
		rows = append(rows, core.Map{"broker": item.Broker, "topic": item.Topic, "consumer": item.Consumer, "payload": item.Payload, "codec": item.Codec})
	}
	return tableRows([]string{"broker", "topic", "consumer", "payload", "codec"}, rows)
}

func messageStatRows(items []MessageStat) core.Map {
	rows := make([]core.Map, 0, len(items))
	for _, item := range items {
		rows = append(rows, core.Map{"broker": valueOrDash(item.Broker), "topic": valueOrDash(item.Topic), "consumer": valueOrDash(item.Consumer), "action": item.Action, "count": item.Count, "errors": item.Errors, "avg": formatDuration(item.Avg), "max": formatDuration(item.Max), "last_seen": formatClock(item.LastSeen)})
	}
	return tableRows([]string{"broker", "topic", "consumer", "action", "count", "errors", "avg", "max", "last_seen"}, rows)
}

func messageLogRows(items []MessageLog) core.Map {
	rows := make([]core.Map, 0, len(items))
	for _, item := range items {
		rows = append(rows, core.Map{"time": formatClock(item.Time), "broker": valueOrDash(item.Broker), "topic": valueOrDash(item.Topic), "consumer": valueOrDash(item.Consumer), "action": item.Action, "latency": formatDuration(item.Latency), "bytes": item.Bytes, "error": valueOrDash(item.Error)})
	}
	return tableRows([]string{"time", "broker", "topic", "consumer", "action", "latency", "bytes", "error"}, rows)
}

func rpcStatRows(items []RPCStat) core.Map {
	rows := make([]core.Map, 0, len(items))
	for _, item := range items {
		rows = append(rows, core.Map{"method": item.Method, "count": item.Count, "errors": item.Errors, "avg": formatDuration(item.Avg), "max": formatDuration(item.Max), "last_seen": formatClock(item.LastSeen)})
	}
	return tableRows([]string{"method", "count", "errors", "avg", "max", "last_seen"}, rows)
}

func rpcLogRows(items []RPCLog) core.Map {
	rows := make([]core.Map, 0, len(items))
	for _, item := range items {
		rows = append(rows, core.Map{"time": formatClock(item.Time), "transport": item.Transport, "method": item.Method, "latency": formatDuration(item.Latency), "error": valueOrDash(item.Error)})
	}
	return tableRows([]string{"time", "transport", "method", "latency", "error"}, rows)
}

func sqlStatRows(items []SQLStat) core.Map {
	rows := make([]core.Map, 0, len(items))
	for _, item := range items {
		rows = append(rows, core.Map{"database": item.Database, "dialect": item.Dialect, "table": valueOrDash(item.Table), "count": item.Count, "errors": item.Errors, "slow": item.Slow, "avg": formatDuration(item.Avg), "max": formatDuration(item.Max), "last_seen": formatClock(item.LastSeen)})
	}
	return tableRows([]string{"database", "dialect", "table", "count", "errors", "slow", "avg", "max", "last_seen"}, rows)
}

func sqlLogRows(items []database.SQLLog) core.Map {
	rows := make([]core.Map, 0, len(items))
	for _, item := range items {
		rows = append(rows, core.Map{
			"time":      formatClock(item.Time),
			"database":  item.Database,
			"table":     valueOrDash(item.Table),
			"operation": valueOrDash(item.Operation),
			"latency":   formatDuration(item.Latency),
			"rows":      item.Rows,
			"slow":      boolText(item.Slow),
			"error":     valueOrDash(item.Error),
			"sql":       compactSQL(item.SQL),
		})
	}
	return tableRows([]string{"time", "database", "table", "operation", "latency", "rows", "slow", "error", "sql"}, rows)
}

func wsSampleRows(items []WSSample) core.Map {
	rows := make([]core.Map, 0, len(items))
	for _, item := range items {
		rows = append(rows, core.Map{"time": formatClock(item.Time), "hub": item.Hub, "clients": item.Clients, "channels": item.Channels, "messages_in": item.MessagesIn, "messages_out": item.MessagesOut, "bytes_in": formatBytes(item.BytesIn), "bytes_out": formatBytes(item.BytesOut)})
	}
	return tableRows([]string{"time", "hub", "clients", "channels", "messages_in", "messages_out", "bytes_in", "bytes_out"}, rows)
}

func wsHubRows(items []*ws.Hub) core.Map {
	rows := make([]core.Map, 0, len(items))
	for _, hub := range items {
		stats := hub.Stats()
		rows = append(rows, core.Map{"name": hub.Name(), "clients": stats.Clients, "channels": stats.Channels, "messages_in": stats.MessagesIn, "messages_out": stats.MessagesOut, "bytes_in": formatBytes(stats.BytesIn), "bytes_out": formatBytes(stats.BytesOut)})
	}
	return tableRows([]string{"name", "clients", "channels", "messages_in", "messages_out", "bytes_in", "bytes_out"}, rows)
}

func valueOrDash(value string) string {
	if value == "" {
		return "-"
	}
	return value
}

func jobStatRows(items []JobStat) core.Map {
	rows := make([]core.Map, 0, len(items))
	for _, item := range items {
		rows = append(rows, core.Map{"queue": item.Queue, "job": item.Job, "count": item.Count, "errors": item.Errors, "avg": formatDuration(item.Avg), "max": formatDuration(item.Max), "last_seen": formatClock(item.LastSeen)})
	}
	return tableRows([]string{"queue", "job", "count", "errors", "avg", "max", "last_seen"}, rows)
}

func jobLogRows(items []JobLog) core.Map {
	rows := make([]core.Map, 0, len(items))
	for _, item := range items {
		rows = append(rows, core.Map{"time": formatClock(item.Time), "queue": item.Queue, "job": item.Job, "attempt": item.Attempt, "latency": formatDuration(item.Latency), "bytes": item.Bytes, "error": valueOrDash(item.Error)})
	}
	return tableRows([]string{"time", "queue", "job", "attempt", "latency", "bytes", "error"}, rows)
}
