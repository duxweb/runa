package prometheus

import (
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/duxweb/runa/cache"
	"github.com/duxweb/runa/database"
	"github.com/duxweb/runa/event"
	"github.com/duxweb/runa/lock"
	"github.com/duxweb/runa/observe"
	runaprovider "github.com/duxweb/runa/provider"
	"github.com/duxweb/runa/queue"
	"github.com/duxweb/runa/rate"
	"github.com/duxweb/runa/route"
	"github.com/duxweb/runa/schedule"
	"github.com/duxweb/runa/session"
	"github.com/duxweb/runa/storage"
	"github.com/duxweb/runa/task"
)

// Exporter returns a Prometheus-compatible text exporter.
func Exporter(values ...any) observe.Exporter {
	if len(values) == 0 {
		return observe.TextMetrics("runa_info 1")
	}
	if app, ok := values[0].(runaprovider.Context); ok {
		return appExporter{app: app, started: time.Now()}
	}
	lines := make([]string, 0, len(values))
	for _, value := range values {
		if line, ok := value.(string); ok && line != "" {
			lines = append(lines, line)
		}
	}
	if len(lines) == 0 {
		lines = []string{"runa_info 1"}
	}
	return observe.TextMetrics(lines...)
}

type appExporter struct {
	app     runaprovider.Context
	started time.Time
}

func (exporter appExporter) Serve(ctx *route.Context) error {
	builder := &textBuilder{}
	builder.Help("runa_info", "Runa application info.")
	builder.Type("runa_info", "gauge")
	builder.Sample("runa_info", labels{"env": appEnv(exporter.app)}, 1)

	builder.Help("runa_uptime_seconds", "Runa process uptime in seconds.")
	builder.Type("runa_uptime_seconds", "gauge")
	builder.Sample("runa_uptime_seconds", nil, time.Since(exporter.started).Seconds())

	var memory runtime.MemStats
	runtime.ReadMemStats(&memory)
	builder.Help("runa_runtime_goroutines", "Number of current goroutines.")
	builder.Type("runa_runtime_goroutines", "gauge")
	builder.Sample("runa_runtime_goroutines", nil, float64(runtime.NumGoroutine()))
	builder.Help("runa_runtime_memory_bytes", "Go runtime memory gauges in bytes.")
	builder.Type("runa_runtime_memory_bytes", "gauge")
	builder.Sample("runa_runtime_memory_bytes", labels{"kind": "alloc"}, float64(memory.Alloc))
	builder.Sample("runa_runtime_memory_bytes", labels{"kind": "sys"}, float64(memory.Sys))
	builder.Sample("runa_runtime_memory_bytes", labels{"kind": "heap_alloc"}, float64(memory.HeapAlloc))
	builder.Sample("runa_runtime_memory_bytes", labels{"kind": "heap_sys"}, float64(memory.HeapSys))

	builder.Help("runa_routes_total", "Number of registered HTTP routes.")
	builder.Type("runa_routes_total", "gauge")
	if routes, err := runaprovider.Invoke[*route.Registry](exporter.app); err == nil && routes != nil {
		builder.Sample("runa_routes_total", nil, float64(len(routes.Routes())))
	} else {
		builder.Sample("runa_routes_total", nil, 0)
	}

	exporter.writeHTTP(builder)
	exporter.writeInfrastructure(builder)
	exporter.writeQueues(ctx, builder)
	exporter.writeSchedules(builder)

	return ctx.Type("text/plain; version=0.0.4; charset=utf-8").Text(builder.String())
}

func (exporter appExporter) writeHTTP(builder *textBuilder) {
	snapshot := httpMetricsOf(exporter.app).Snapshot()
	builder.Help("runa_http_requests_total", "HTTP requests total.")
	builder.Type("runa_http_requests_total", "counter")
	keys := sortedHTTPKeys(snapshot.Requests)
	for _, key := range keys {
		builder.Sample("runa_http_requests_total", labels{"method": key.Method, "path": key.Path, "status": strconv.Itoa(key.Status)}, float64(snapshot.Requests[key]))
	}
	builder.Help("runa_http_errors_total", "HTTP 5xx errors total.")
	builder.Type("runa_http_errors_total", "counter")
	keys = sortedHTTPKeys(snapshot.Errors)
	for _, key := range keys {
		builder.Sample("runa_http_errors_total", labels{"method": key.Method, "path": key.Path, "status": strconv.Itoa(key.Status)}, float64(snapshot.Errors[key]))
	}
	builder.Help("runa_http_request_duration_seconds", "HTTP request duration histogram in seconds.")
	builder.Type("runa_http_request_duration_seconds", "histogram")
	durationKeys := make([]httpKey, 0, len(snapshot.Duration))
	for key := range snapshot.Duration {
		durationKeys = append(durationKeys, key)
	}
	sort.Slice(durationKeys, func(i, j int) bool { return compareHTTPKey(durationKeys[i], durationKeys[j]) < 0 })
	for _, key := range durationKeys {
		hist := snapshot.Duration[key]
		base := labels{"method": key.Method, "path": key.Path, "status": strconv.Itoa(key.Status)}
		for index, bucket := range defaultDurationBuckets {
			builder.Sample("runa_http_request_duration_seconds_bucket", mergeLabels(base, labels{"le": strconv.FormatFloat(bucket, 'f', -1, 64)}), float64(hist.Buckets[index]))
		}
		builder.Sample("runa_http_request_duration_seconds_bucket", mergeLabels(base, labels{"le": "+Inf"}), float64(hist.Count))
		builder.Sample("runa_http_request_duration_seconds_sum", base, hist.Sum)
		builder.Sample("runa_http_request_duration_seconds_count", base, float64(hist.Count))
	}
}

func (exporter appExporter) writeInfrastructure(builder *textBuilder) {
	builder.Help("runa_databases", "Configured databases by kind and status.")
	builder.Type("runa_databases", "gauge")
	if registry, err := runaprovider.Invoke[*database.Registry](exporter.app); err == nil && registry != nil {
		for _, item := range registry.Info() {
			builder.Sample("runa_databases", labels{"name": item.Name, "kind": item.Kind, "status": item.Status}, 1)
		}
	}
	builder.Help("runa_cache_pools", "Configured cache pools by driver.")
	builder.Type("runa_cache_pools", "gauge")
	if registry, err := runaprovider.Invoke[*cache.Registry](exporter.app); err == nil && registry != nil {
		for _, item := range registry.Info() {
			builder.Sample("runa_cache_pools", labels{"name": item.Name, "driver": item.Driver}, 1)
		}
	}
	builder.Help("runa_storage_disks", "Configured storage disks by driver.")
	builder.Type("runa_storage_disks", "gauge")
	if registry, err := runaprovider.Invoke[*storage.Registry](exporter.app); err == nil && registry != nil {
		for _, item := range registry.Info() {
			builder.Sample("runa_storage_disks", labels{"name": item.Name, "driver": item.Driver, "default": strconv.FormatBool(item.Default)}, 1)
		}
	}
	builder.Help("runa_session_pools", "Configured session pools by driver.")
	builder.Type("runa_session_pools", "gauge")
	if registry, err := runaprovider.Invoke[*session.Registry](exporter.app); err == nil && registry != nil {
		for _, item := range registry.Info() {
			builder.Sample("runa_session_pools", labels{"name": item.Name, "driver": item.Driver, "default": strconv.FormatBool(item.Default)}, 1)
		}
	}
	builder.Help("runa_rate_limiters", "Configured rate limiters by driver.")
	builder.Type("runa_rate_limiters", "gauge")
	if registry, err := runaprovider.Invoke[*rate.Registry](exporter.app); err == nil && registry != nil {
		for _, item := range registry.Info() {
			builder.Sample("runa_rate_limiters", labels{"name": item.Name, "driver": item.Driver, "algorithm": string(item.Algorithm), "default": strconv.FormatBool(item.Default)}, 1)
		}
	}
	builder.Help("runa_lock_pools", "Configured lock pools by driver.")
	builder.Type("runa_lock_pools", "gauge")
	if registry, err := runaprovider.Invoke[*lock.Registry](exporter.app); err == nil && registry != nil {
		for _, item := range registry.Info() {
			builder.Sample("runa_lock_pools", labels{"name": item.Name, "driver": item.Driver}, 1)
		}
	}
}

func (exporter appExporter) writeQueues(ctx *route.Context, builder *textBuilder) {
	builder.Help("runa_queue_jobs", "Queue jobs by state.")
	builder.Type("runa_queue_jobs", "gauge")
	if registry, err := runaprovider.Invoke[*queue.Registry](exporter.app); err == nil && registry != nil {
		for _, item := range registry.QueueInfo(ctx.Context()) {
			base := labels{"queue": item.Name, "driver": item.Driver}
			builder.Sample("runa_queue_jobs", mergeLabels(base, labels{"state": "pending"}), float64(item.Pending))
			builder.Sample("runa_queue_jobs", mergeLabels(base, labels{"state": "reserved"}), float64(item.Reserved))
			builder.Sample("runa_queue_jobs", mergeLabels(base, labels{"state": "delayed"}), float64(item.Delayed))
			builder.Sample("runa_queue_jobs", mergeLabels(base, labels{"state": "failed"}), float64(item.Failed))
		}
	}
	builder.Help("runa_worker_instances", "Queue worker instances.")
	builder.Type("runa_worker_instances", "gauge")
	builder.Help("runa_worker_processed_total", "Queue worker processed jobs total.")
	builder.Type("runa_worker_processed_total", "counter")
	builder.Help("runa_worker_succeeded_total", "Queue worker succeeded jobs total.")
	builder.Type("runa_worker_succeeded_total", "counter")
	builder.Help("runa_worker_failed_total", "Queue worker failed jobs total.")
	builder.Type("runa_worker_failed_total", "counter")
	builder.Help("runa_worker_retried_total", "Queue worker retried jobs total.")
	builder.Type("runa_worker_retried_total", "counter")
	if registry, err := runaprovider.Invoke[*queue.Registry](exporter.app); err == nil && registry != nil {
		for _, item := range registry.WorkerInfo(ctx.Context()) {
			workerLabels := labels{"worker": item.Name, "status": item.Status}
			builder.Sample("runa_worker_instances", workerLabels, float64(item.Instances))
			builder.Sample("runa_worker_processed_total", labels{"worker": item.Name}, float64(item.Processed))
			builder.Sample("runa_worker_succeeded_total", labels{"worker": item.Name}, float64(item.Succeeded))
			builder.Sample("runa_worker_failed_total", labels{"worker": item.Name}, float64(item.Failed))
			builder.Sample("runa_worker_retried_total", labels{"worker": item.Name}, float64(item.Retried))
		}
	}
}

func (exporter appExporter) writeSchedules(builder *textBuilder) {
	builder.Help("runa_schedules", "Registered schedules by enabled state.")
	builder.Type("runa_schedules", "gauge")
	if registry, err := runaprovider.Invoke[*schedule.Registry](exporter.app); err == nil && registry != nil {
		for _, item := range registry.Info() {
			builder.Sample("runa_schedules", labels{"name": item.Name, "task": item.Task, "enabled": strconv.FormatBool(item.Enabled)}, 1)
		}
	}
	builder.Help("runa_tasks_total", "Registered task handlers.")
	builder.Type("runa_tasks_total", "gauge")
	if registry, err := runaprovider.Invoke[*task.Registry](exporter.app); err == nil && registry != nil {
		builder.Sample("runa_tasks_total", nil, float64(len(registry.Info())))
	}
	builder.Help("runa_events_total", "Registered event listeners.")
	builder.Type("runa_events_total", "gauge")
	if registry, err := runaprovider.Invoke[*event.Registry](exporter.app); err == nil && registry != nil {
		builder.Sample("runa_events_total", nil, float64(len(registry.Info())))
	}
}

type labels map[string]string

type textBuilder struct{ lines []string }

func (builder *textBuilder) Help(name string, text string) {
	builder.lines = append(builder.lines, "# HELP "+name+" "+sanitizeHelp(text))
}

func (builder *textBuilder) Type(name string, kind string) {
	builder.lines = append(builder.lines, "# TYPE "+name+" "+kind)
}

func (builder *textBuilder) Sample(name string, labels labels, value float64) {
	builder.lines = append(builder.lines, name+formatLabels(labels)+" "+strconv.FormatFloat(value, 'f', -1, 64))
}

func (builder *textBuilder) String() string {
	return strings.Join(builder.lines, "\n") + "\n"
}

func formatLabels(values labels) string {
	if len(values) == 0 {
		return ""
	}
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, key := range keys {
		parts = append(parts, key+"=\""+escapeLabel(values[key])+"\"")
	}
	return "{" + strings.Join(parts, ",") + "}"
}

func mergeLabels(left labels, right labels) labels {
	merged := make(labels, len(left)+len(right))
	for key, value := range left {
		merged[key] = value
	}
	for key, value := range right {
		merged[key] = value
	}
	return merged
}

func sortedHTTPKeys(values map[httpKey]uint64) []httpKey {
	keys := make([]httpKey, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Slice(keys, func(i, j int) bool { return compareHTTPKey(keys[i], keys[j]) < 0 })
	return keys
}

func compareHTTPKey(left httpKey, right httpKey) int {
	if left.Method != right.Method {
		return strings.Compare(left.Method, right.Method)
	}
	if left.Path != right.Path {
		return strings.Compare(left.Path, right.Path)
	}
	if left.Status < right.Status {
		return -1
	}
	if left.Status > right.Status {
		return 1
	}
	return 0
}

func sanitizeHelp(value string) string {
	return strings.ReplaceAll(strings.ReplaceAll(value, "\n", " "), "\\", "\\\\")
}

func escapeLabel(value string) string {
	value = strings.ReplaceAll(value, "\\", "\\\\")
	value = strings.ReplaceAll(value, "\n", "\\n")
	value = strings.ReplaceAll(value, "\"", "\\\"")
	return value
}

func appEnv(ctx runaprovider.Context) string {
	if ctx == nil {
		return ""
	}
	if app, ok := ctx.App().(interface{ Env() string }); ok {
		return app.Env()
	}
	return ""
}
