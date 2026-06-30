package console

import (
	"sort"
	"sync"
	"time"

	"github.com/duxweb/runa/core"
	"github.com/duxweb/runa/database"
	"github.com/duxweb/runa/middleware/logger"
	runaprovider "github.com/duxweb/runa/provider"
	"github.com/duxweb/runa/route"
)

const defaultMonitorLimit = 500

// MonitorStore stores console metrics and logs.
type MonitorStore interface {
	RecordAccess(AccessLog)
	RecordQueue(QueueSample)
	RecordJob(JobLog)
	RecordMessage(MessageLog)
	RecordRPC(RPCLog)
	RecordWS(WSSample)
	RecordSQL(database.SQLLog)
	AccessLogs(limit int) []AccessLog
	ErrorLogs(limit int) []ErrorLog
	SlowLogs(limit int) []AccessLog
	RouteStats(limit int) []RouteStat
	TrafficSeries(window time.Duration, points int) []MetricPoint
	ErrorSeries(window time.Duration, points int) []MetricPoint
	LatencySeries(window time.Duration, points int) []MetricPoint
	StatusSeries(window time.Duration, points int) []MetricPoint
	QueuePressureSeries(window time.Duration, points int) []MetricPoint
	WorkerThroughputSeries(window time.Duration, points int) []MetricPoint
	JobFailureSeries(window time.Duration, points int) []MetricPoint
	JobLogs(limit int) []JobLog
	JobStats(limit int) []JobStat
	JobLatencySeries(window time.Duration, points int) []MetricPoint
	MessageLogs(limit int) []MessageLog
	MessageStats(limit int) []MessageStat
	MessagePublishSeries(window time.Duration, points int) []MetricPoint
	MessageConsumeSeries(window time.Duration, points int) []MetricPoint
	MessageErrorSeries(window time.Duration, points int) []MetricPoint
	RPCLogs(limit int) []RPCLog
	RPCStats(limit int) []RPCStat
	RPCSeries(window time.Duration, points int) []MetricPoint
	RPCErrorSeries(window time.Duration, points int) []MetricPoint
	RPCLatencySeries(window time.Duration, points int) []MetricPoint
	WSSamples(limit int) []WSSample
	WSMessageInSeries(window time.Duration, points int) []MetricPoint
	WSMessageOutSeries(window time.Duration, points int) []MetricPoint
	SQLLogs(limit int) []database.SQLLog
	SQLStats(limit int) []SQLStat
	SQLSeries(window time.Duration, points int) []MetricPoint
	SQLErrorSeries(window time.Duration, points int) []MetricPoint
	SQLLatencySeries(window time.Duration, points int) []MetricPoint
}

// AccessLog describes one HTTP request captured by the console monitor.
type AccessLog struct {
	Time      time.Time     `json:"time"`
	RequestID string        `json:"request_id"`
	Method    string        `json:"method"`
	Path      string        `json:"path"`
	Route     string        `json:"route"`
	Status    int           `json:"status"`
	IP        string        `json:"ip"`
	Latency   time.Duration `json:"latency"`
	Slow      bool          `json:"slow"`
	Error     string        `json:"error,omitempty"`
	Source    string        `json:"source,omitempty"`
}

// ErrorLog describes one failed request captured by the console monitor.
type ErrorLog struct {
	Time      time.Time     `json:"time"`
	RequestID string        `json:"request_id"`
	Method    string        `json:"method"`
	Path      string        `json:"path"`
	Route     string        `json:"route"`
	Status    int           `json:"status"`
	Latency   time.Duration `json:"latency"`
	Error     string        `json:"error"`
	Source    string        `json:"source,omitempty"`
}

// RouteStat aggregates HTTP request metrics by route.
type RouteStat struct {
	Method    string        `json:"method"`
	Route     string        `json:"route"`
	Path      string        `json:"path"`
	Count     int64         `json:"count"`
	Errors    int64         `json:"errors"`
	Min       time.Duration `json:"min"`
	Max       time.Duration `json:"max"`
	Avg       time.Duration `json:"avg"`
	LastSeen  time.Time     `json:"last_seen"`
	LastState int           `json:"last_status"`
}

// MetricPoint describes one chart point.
type MetricPoint struct {
	Label string  `json:"label"`
	Value float64 `json:"value"`
}

// QueueSample describes one queue/worker aggregate sample.
type QueueSample struct {
	Time      time.Time `json:"time"`
	Pending   int64     `json:"pending"`
	Reserved  int64     `json:"reserved"`
	Delayed   int64     `json:"delayed"`
	Failed    int64     `json:"failed"`
	Processed int64     `json:"processed"`
	Succeeded int64     `json:"succeeded"`
	Retried   int64     `json:"retried"`
	Workers   int       `json:"workers"`
	Instances int       `json:"instances"`
}

// JobLog describes one queue job execution.
type JobLog struct {
	Time    time.Time     `json:"time"`
	Queue   string        `json:"queue"`
	Job     string        `json:"job"`
	Attempt int           `json:"attempt"`
	Latency time.Duration `json:"latency"`
	Bytes   int           `json:"bytes"`
	Error   string        `json:"error,omitempty"`
}

// JobStat aggregates queue job execution metrics.
type JobStat struct {
	Queue    string        `json:"queue"`
	Job      string        `json:"job"`
	Count    int64         `json:"count"`
	Errors   int64         `json:"errors"`
	Avg      time.Duration `json:"avg"`
	Max      time.Duration `json:"max"`
	LastSeen time.Time     `json:"last_seen"`
}

type jobStatAccumulator struct {
	stat  JobStat
	total time.Duration
}

// MessageLog describes one message broker operation.
type MessageLog struct {
	Time     time.Time     `json:"time"`
	Broker   string        `json:"broker"`
	Topic    string        `json:"topic"`
	Consumer string        `json:"consumer,omitempty"`
	Action   string        `json:"action"`
	Latency  time.Duration `json:"latency"`
	Bytes    int           `json:"bytes"`
	Error    string        `json:"error,omitempty"`
}

// MessageStat aggregates message operations by broker/topic/action.
type MessageStat struct {
	Broker   string        `json:"broker"`
	Topic    string        `json:"topic"`
	Consumer string        `json:"consumer,omitempty"`
	Action   string        `json:"action"`
	Count    int64         `json:"count"`
	Errors   int64         `json:"errors"`
	Avg      time.Duration `json:"avg"`
	Max      time.Duration `json:"max"`
	LastSeen time.Time     `json:"last_seen"`
}

// RPCLog describes one JSON-RPC method call.
type RPCLog struct {
	Time      time.Time     `json:"time"`
	Transport string        `json:"transport"`
	Method    string        `json:"method"`
	Latency   time.Duration `json:"latency"`
	Error     string        `json:"error,omitempty"`
}

// RPCStat aggregates JSON-RPC method calls.
type RPCStat struct {
	Method   string        `json:"method"`
	Count    int64         `json:"count"`
	Errors   int64         `json:"errors"`
	Avg      time.Duration `json:"avg"`
	Max      time.Duration `json:"max"`
	LastSeen time.Time     `json:"last_seen"`
}

// WSSample describes one websocket hub aggregate sample.
type WSSample struct {
	Time        time.Time `json:"time"`
	Hub         string    `json:"hub"`
	Clients     int       `json:"clients"`
	Channels    int       `json:"channels"`
	MessagesIn  uint64    `json:"messages_in"`
	MessagesOut uint64    `json:"messages_out"`
	BytesIn     uint64    `json:"bytes_in"`
	BytesOut    uint64    `json:"bytes_out"`
}

type messageStatAccumulator struct {
	stat  MessageStat
	total time.Duration
}

type rpcStatAccumulator struct {
	stat  RPCStat
	total time.Duration
}

// SQLStat aggregates SQL execution metrics.
type SQLStat struct {
	Database string        `json:"database"`
	Dialect  string        `json:"dialect"`
	Table    string        `json:"table"`
	Count    int64         `json:"count"`
	Errors   int64         `json:"errors"`
	Slow     int64         `json:"slow"`
	Avg      time.Duration `json:"avg"`
	Max      time.Duration `json:"max"`
	LastSeen time.Time     `json:"last_seen"`
}

type sqlStatAccumulator struct {
	stat  SQLStat
	total time.Duration
}

type routeStatAccumulator struct {
	stat  RouteStat
	total time.Duration
}

// MemoryMonitorStore stores recent monitor data in memory.
type MemoryMonitorStore struct {
	mu      sync.RWMutex
	limit   int
	access  []AccessLog
	queue   []QueueSample
	job     []JobLog
	message []MessageLog
	rpc     []RPCLog
	ws      []WSSample
	sql     []database.SQLLog
}

// NewMemoryMonitorStore creates an in-memory monitor store.
func NewMemoryMonitorStore(limit ...int) *MemoryMonitorStore {
	value := defaultMonitorLimit
	if len(limit) > 0 && limit[0] > 0 {
		value = limit[0]
	}
	return &MemoryMonitorStore{limit: value}
}

func (store *MemoryMonitorStore) RecordAccess(item AccessLog) {
	if store == nil {
		return
	}
	if item.Time.IsZero() {
		item.Time = core.Now()
	}
	store.mu.Lock()
	store.access = append(store.access, item)
	if extra := len(store.access) - store.limit; extra > 0 {
		copy(store.access, store.access[extra:])
		store.access = store.access[:store.limit]
	}
	store.mu.Unlock()
}

func (store *MemoryMonitorStore) RecordQueue(item QueueSample) {
	if store == nil {
		return
	}
	if item.Time.IsZero() {
		item.Time = core.Now()
	}
	store.mu.Lock()
	store.queue = append(store.queue, item)
	if extra := len(store.queue) - store.limit; extra > 0 {
		copy(store.queue, store.queue[extra:])
		store.queue = store.queue[:store.limit]
	}
	store.mu.Unlock()
}

func (store *MemoryMonitorStore) RecordJob(item JobLog) {
	if store == nil {
		return
	}
	if item.Time.IsZero() {
		item.Time = core.Now()
	}
	store.mu.Lock()
	store.job = append(store.job, item)
	store.trimJobLocked()
	store.mu.Unlock()
}

func (store *MemoryMonitorStore) RecordMessage(item MessageLog) {
	if store == nil {
		return
	}
	if item.Time.IsZero() {
		item.Time = core.Now()
	}
	store.mu.Lock()
	store.message = append(store.message, item)
	store.trimMessageLocked()
	store.mu.Unlock()
}

func (store *MemoryMonitorStore) RecordRPC(item RPCLog) {
	if store == nil {
		return
	}
	if item.Time.IsZero() {
		item.Time = core.Now()
	}
	store.mu.Lock()
	store.rpc = append(store.rpc, item)
	store.trimRPCLocked()
	store.mu.Unlock()
}

func (store *MemoryMonitorStore) RecordWS(item WSSample) {
	if store == nil {
		return
	}
	if item.Time.IsZero() {
		item.Time = core.Now()
	}
	store.mu.Lock()
	store.ws = append(store.ws, item)
	store.trimWSLocked()
	store.mu.Unlock()
}

func (store *MemoryMonitorStore) RecordSQL(item database.SQLLog) {
	if store == nil {
		return
	}
	if item.Time.IsZero() {
		item.Time = core.Now()
	}
	store.mu.Lock()
	store.sql = append(store.sql, item)
	store.trimSQLLocked()
	store.mu.Unlock()
}

func (store *MemoryMonitorStore) AccessLogs(limit int) []AccessLog {
	items := store.recent(limit, func(item AccessLog) bool { return true })
	return items
}

func (store *MemoryMonitorStore) ErrorLogs(limit int) []ErrorLog {
	access := store.recent(limit, func(item AccessLog) bool { return item.Status >= 500 || item.Error != "" })
	items := make([]ErrorLog, 0, len(access))
	for _, item := range access {
		items = append(items, ErrorLog{
			Time:      item.Time,
			RequestID: item.RequestID,
			Method:    item.Method,
			Path:      item.Path,
			Route:     item.Route,
			Status:    item.Status,
			Latency:   item.Latency,
			Error:     item.Error,
			Source:    item.Source,
		})
	}
	return items
}

func (store *MemoryMonitorStore) SlowLogs(limit int) []AccessLog {
	return store.recent(limit, func(item AccessLog) bool { return item.Slow })
}

func (store *MemoryMonitorStore) RouteStats(limit int) []RouteStat {
	if store == nil {
		return nil
	}
	if limit <= 0 {
		limit = 100
	}
	store.mu.RLock()
	items := append([]AccessLog(nil), store.access...)
	store.mu.RUnlock()
	stats := make(map[string]*routeStatAccumulator)
	for _, item := range items {
		key := routeStatKey(item)
		current := stats[key]
		if current == nil {
			current = &routeStatAccumulator{stat: RouteStat{
				Method: item.Method,
				Route:  routeStatRoute(item),
				Path:   item.Path,
				Min:    item.Latency,
				Max:    item.Latency,
			}}
			stats[key] = current
		}
		current.stat.Count++
		if item.Status >= 500 || item.Error != "" {
			current.stat.Errors++
		}
		if item.Latency < current.stat.Min {
			current.stat.Min = item.Latency
		}
		if item.Latency > current.stat.Max {
			current.stat.Max = item.Latency
		}
		current.total += item.Latency
		if item.Time.After(current.stat.LastSeen) {
			current.stat.LastSeen = item.Time
			current.stat.LastState = item.Status
		}
	}
	out := make([]RouteStat, 0, len(stats))
	for _, item := range stats {
		if item.stat.Count > 0 {
			item.stat.Avg = item.total / time.Duration(item.stat.Count)
		}
		out = append(out, item.stat)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Count != out[j].Count {
			return out[i].Count > out[j].Count
		}
		if out[i].Errors != out[j].Errors {
			return out[i].Errors > out[j].Errors
		}
		if !out[i].LastSeen.Equal(out[j].LastSeen) {
			return out[i].LastSeen.After(out[j].LastSeen)
		}
		return out[i].Method+" "+out[i].Path < out[j].Method+" "+out[j].Path
	})
	if len(out) > limit {
		out = out[:limit]
	}
	return out
}

func routeStatKey(item AccessLog) string {
	return item.Method + "\x00" + routeStatRoute(item) + "\x00" + item.Path
}

func routeStatRoute(item AccessLog) string {
	if item.Route != "" {
		return item.Route
	}
	return item.Path
}

func (store *MemoryMonitorStore) TrafficSeries(window time.Duration, points int) []MetricPoint {
	return store.countSeries(window, points, func(AccessLog) bool { return true })
}

func (store *MemoryMonitorStore) ErrorSeries(window time.Duration, points int) []MetricPoint {
	return store.countSeries(window, points, func(item AccessLog) bool { return item.Status >= 500 || item.Error != "" })
}

func (store *MemoryMonitorStore) LatencySeries(window time.Duration, points int) []MetricPoint {
	return store.avgSeries(window, points, func(item AccessLog) float64 { return float64(item.Latency.Milliseconds()) })
}

func (store *MemoryMonitorStore) StatusSeries(window time.Duration, points int) []MetricPoint {
	store.mu.RLock()
	items := append([]AccessLog(nil), store.access...)
	store.mu.RUnlock()
	counts := map[string]int{"2xx": 0, "3xx": 0, "4xx": 0, "5xx": 0}
	for _, item := range items {
		switch {
		case item.Status >= 500:
			counts["5xx"]++
		case item.Status >= 400:
			counts["4xx"]++
		case item.Status >= 300:
			counts["3xx"]++
		case item.Status >= 200:
			counts["2xx"]++
		}
	}
	return []MetricPoint{{Label: "2xx", Value: float64(counts["2xx"])}, {Label: "3xx", Value: float64(counts["3xx"])}, {Label: "4xx", Value: float64(counts["4xx"])}, {Label: "5xx", Value: float64(counts["5xx"])}}
}

func (store *MemoryMonitorStore) QueuePressureSeries(window time.Duration, points int) []MetricPoint {
	return store.queueAvgSeries(window, points, func(item QueueSample) float64 { return float64(item.Pending + item.Reserved + item.Delayed) })
}

func (store *MemoryMonitorStore) WorkerThroughputSeries(window time.Duration, points int) []MetricPoint {
	return store.queueDeltaSeries(window, points, func(item QueueSample) float64 { return float64(item.Processed) })
}

func (store *MemoryMonitorStore) JobFailureSeries(window time.Duration, points int) []MetricPoint {
	return store.queueDeltaSeries(window, points, func(item QueueSample) float64 { return float64(item.Failed) })
}

func (store *MemoryMonitorStore) JobLogs(limit int) []JobLog {
	return store.recentJobs(limit, nil)
}

func (store *MemoryMonitorStore) JobStats(limit int) []JobStat {
	if limit <= 0 {
		limit = 100
	}
	items := store.jobWindowItems(24 * time.Hour)
	stats := make(map[string]*jobStatAccumulator)
	for _, item := range items {
		key := item.Queue + "\x00" + item.Job
		current := stats[key]
		if current == nil {
			current = &jobStatAccumulator{stat: JobStat{Queue: item.Queue, Job: item.Job}}
			stats[key] = current
		}
		current.stat.Count++
		if item.Error != "" {
			current.stat.Errors++
		}
		if item.Latency > current.stat.Max {
			current.stat.Max = item.Latency
		}
		current.total += item.Latency
		if item.Time.After(current.stat.LastSeen) {
			current.stat.LastSeen = item.Time
		}
	}
	out := make([]JobStat, 0, len(stats))
	for _, item := range stats {
		if item.stat.Count > 0 {
			item.stat.Avg = item.total / time.Duration(item.stat.Count)
		}
		out = append(out, item.stat)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Count != out[j].Count {
			return out[i].Count > out[j].Count
		}
		return out[i].Queue+out[i].Job < out[j].Queue+out[j].Job
	})
	if len(out) > limit {
		out = out[:limit]
	}
	return out
}

func (store *MemoryMonitorStore) JobLatencySeries(window time.Duration, points int) []MetricPoint {
	items := store.jobWindowItems(window)
	buckets := timeBuckets(window, points)
	counts := make([]float64, len(buckets))
	for _, item := range items {
		index := bucketIndex(item.Time, window, points)
		if index >= 0 && index < len(buckets) {
			buckets[index].Value += float64(item.Latency.Milliseconds())
			counts[index]++
		}
	}
	for index := range buckets {
		if counts[index] > 0 {
			buckets[index].Value = buckets[index].Value / counts[index]
		}
	}
	return buckets
}

func (store *MemoryMonitorStore) MessageLogs(limit int) []MessageLog {
	return store.recentMessages(limit, nil)
}

func (store *MemoryMonitorStore) MessageStats(limit int) []MessageStat {
	if limit <= 0 {
		limit = 100
	}
	items := store.messageWindowItems(24 * time.Hour)
	stats := make(map[string]*messageStatAccumulator)
	for _, item := range items {
		key := item.Broker + "\x00" + item.Topic + "\x00" + item.Consumer + "\x00" + item.Action
		current := stats[key]
		if current == nil {
			current = &messageStatAccumulator{stat: MessageStat{Broker: item.Broker, Topic: item.Topic, Consumer: item.Consumer, Action: item.Action}}
			stats[key] = current
		}
		current.stat.Count++
		if item.Error != "" {
			current.stat.Errors++
		}
		if item.Latency > current.stat.Max {
			current.stat.Max = item.Latency
		}
		current.total += item.Latency
		if item.Time.After(current.stat.LastSeen) {
			current.stat.LastSeen = item.Time
		}
	}
	out := make([]MessageStat, 0, len(stats))
	for _, item := range stats {
		if item.stat.Count > 0 {
			item.stat.Avg = item.total / time.Duration(item.stat.Count)
		}
		out = append(out, item.stat)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Count != out[j].Count {
			return out[i].Count > out[j].Count
		}
		return out[i].Broker+out[i].Topic+out[i].Action < out[j].Broker+out[j].Topic+out[j].Action
	})
	if len(out) > limit {
		out = out[:limit]
	}
	return out
}

func (store *MemoryMonitorStore) MessagePublishSeries(window time.Duration, points int) []MetricPoint {
	return store.messageCountSeries(window, points, func(item MessageLog) bool { return item.Action == "publish" })
}

func (store *MemoryMonitorStore) MessageConsumeSeries(window time.Duration, points int) []MetricPoint {
	return store.messageCountSeries(window, points, func(item MessageLog) bool { return item.Action == "consume" })
}

func (store *MemoryMonitorStore) MessageErrorSeries(window time.Duration, points int) []MetricPoint {
	return store.messageCountSeries(window, points, func(item MessageLog) bool { return item.Error != "" })
}

func (store *MemoryMonitorStore) RPCLogs(limit int) []RPCLog {
	return store.recentRPC(limit, nil)
}

func (store *MemoryMonitorStore) RPCStats(limit int) []RPCStat {
	if limit <= 0 {
		limit = 100
	}
	items := store.rpcWindowItems(24 * time.Hour)
	stats := make(map[string]*rpcStatAccumulator)
	for _, item := range items {
		current := stats[item.Method]
		if current == nil {
			current = &rpcStatAccumulator{stat: RPCStat{Method: item.Method}}
			stats[item.Method] = current
		}
		current.stat.Count++
		if item.Error != "" {
			current.stat.Errors++
		}
		if item.Latency > current.stat.Max {
			current.stat.Max = item.Latency
		}
		current.total += item.Latency
		if item.Time.After(current.stat.LastSeen) {
			current.stat.LastSeen = item.Time
		}
	}
	out := make([]RPCStat, 0, len(stats))
	for _, item := range stats {
		if item.stat.Count > 0 {
			item.stat.Avg = item.total / time.Duration(item.stat.Count)
		}
		out = append(out, item.stat)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Count != out[j].Count {
			return out[i].Count > out[j].Count
		}
		return out[i].Method < out[j].Method
	})
	if len(out) > limit {
		out = out[:limit]
	}
	return out
}

func (store *MemoryMonitorStore) RPCSeries(window time.Duration, points int) []MetricPoint {
	return store.rpcCountSeries(window, points, func(RPCLog) bool { return true })
}

func (store *MemoryMonitorStore) RPCErrorSeries(window time.Duration, points int) []MetricPoint {
	return store.rpcCountSeries(window, points, func(item RPCLog) bool { return item.Error != "" })
}

func (store *MemoryMonitorStore) RPCLatencySeries(window time.Duration, points int) []MetricPoint {
	items := store.rpcWindowItems(window)
	buckets := timeBuckets(window, points)
	counts := make([]float64, len(buckets))
	for _, item := range items {
		index := bucketIndex(item.Time, window, points)
		if index >= 0 && index < len(buckets) {
			buckets[index].Value += float64(item.Latency.Milliseconds())
			counts[index]++
		}
	}
	for index := range buckets {
		if counts[index] > 0 {
			buckets[index].Value = buckets[index].Value / counts[index]
		}
	}
	return buckets
}

func (store *MemoryMonitorStore) WSSamples(limit int) []WSSample {
	if store == nil {
		return nil
	}
	if limit <= 0 {
		limit = 50
	}
	store.mu.RLock()
	items := append([]WSSample(nil), store.ws...)
	store.mu.RUnlock()
	out := make([]WSSample, 0, min(limit, len(items)))
	for i := len(items) - 1; i >= 0 && len(out) < limit; i-- {
		out = append(out, items[i])
	}
	return out
}

func (store *MemoryMonitorStore) WSMessageInSeries(window time.Duration, points int) []MetricPoint {
	return store.wsDeltaSeries(window, points, func(item WSSample) float64 { return float64(item.MessagesIn) })
}

func (store *MemoryMonitorStore) WSMessageOutSeries(window time.Duration, points int) []MetricPoint {
	return store.wsDeltaSeries(window, points, func(item WSSample) float64 { return float64(item.MessagesOut) })
}

func (store *MemoryMonitorStore) SQLLogs(limit int) []database.SQLLog {
	return store.recentSQL(limit, nil)
}

func (store *MemoryMonitorStore) SQLStats(limit int) []SQLStat {
	if limit <= 0 {
		limit = 100
	}
	items := store.sqlWindowItems(24 * time.Hour)
	stats := make(map[string]*sqlStatAccumulator)
	for _, item := range items {
		key := item.Database + "\x00" + item.Dialect + "\x00" + item.Table
		current := stats[key]
		if current == nil {
			current = &sqlStatAccumulator{stat: SQLStat{Database: item.Database, Dialect: item.Dialect, Table: item.Table}}
			stats[key] = current
		}
		current.stat.Count++
		if item.Error != "" {
			current.stat.Errors++
		}
		if item.Slow {
			current.stat.Slow++
		}
		if item.Latency > current.stat.Max {
			current.stat.Max = item.Latency
		}
		current.total += item.Latency
		if item.Time.After(current.stat.LastSeen) {
			current.stat.LastSeen = item.Time
		}
	}
	out := make([]SQLStat, 0, len(stats))
	for _, item := range stats {
		if item.stat.Count > 0 {
			item.stat.Avg = item.total / time.Duration(item.stat.Count)
		}
		out = append(out, item.stat)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Count != out[j].Count {
			return out[i].Count > out[j].Count
		}
		return out[i].Database+out[i].Table < out[j].Database+out[j].Table
	})
	if len(out) > limit {
		out = out[:limit]
	}
	return out
}

func (store *MemoryMonitorStore) SQLSeries(window time.Duration, points int) []MetricPoint {
	return store.sqlCountSeries(window, points, func(database.SQLLog) bool { return true })
}

func (store *MemoryMonitorStore) SQLErrorSeries(window time.Duration, points int) []MetricPoint {
	return store.sqlCountSeries(window, points, func(item database.SQLLog) bool { return item.Error != "" })
}

func (store *MemoryMonitorStore) SQLLatencySeries(window time.Duration, points int) []MetricPoint {
	items := store.sqlWindowItems(window)
	buckets := timeBuckets(window, points)
	counts := make([]float64, len(buckets))
	for _, item := range items {
		index := bucketIndex(item.Time, window, points)
		if index >= 0 && index < len(buckets) {
			buckets[index].Value += float64(item.Latency.Milliseconds())
			counts[index]++
		}
	}
	for index := range buckets {
		if counts[index] > 0 {
			buckets[index].Value = buckets[index].Value / counts[index]
		}
	}
	return buckets
}

func (store *MemoryMonitorStore) trimJobLocked() {
	if extra := len(store.job) - store.limit; extra > 0 {
		copy(store.job, store.job[extra:])
		store.job = store.job[:store.limit]
	}
}

func (store *MemoryMonitorStore) trimMessageLocked() {
	if extra := len(store.message) - store.limit; extra > 0 {
		copy(store.message, store.message[extra:])
		store.message = store.message[:store.limit]
	}
}

func (store *MemoryMonitorStore) trimRPCLocked() {
	if extra := len(store.rpc) - store.limit; extra > 0 {
		copy(store.rpc, store.rpc[extra:])
		store.rpc = store.rpc[:store.limit]
	}
}

func (store *MemoryMonitorStore) trimWSLocked() {
	if extra := len(store.ws) - store.limit; extra > 0 {
		copy(store.ws, store.ws[extra:])
		store.ws = store.ws[:store.limit]
	}
}

func (store *MemoryMonitorStore) trimSQLLocked() {
	if extra := len(store.sql) - store.limit; extra > 0 {
		copy(store.sql, store.sql[extra:])
		store.sql = store.sql[:store.limit]
	}
}

func (store *MemoryMonitorStore) recent(limit int, filter func(AccessLog) bool) []AccessLog {
	if store == nil {
		return nil
	}
	if limit <= 0 {
		limit = 50
	}
	store.mu.RLock()
	items := append([]AccessLog(nil), store.access...)
	store.mu.RUnlock()
	out := make([]AccessLog, 0, min(limit, len(items)))
	for i := len(items) - 1; i >= 0 && len(out) < limit; i-- {
		if filter == nil || filter(items[i]) {
			out = append(out, items[i])
		}
	}
	return out
}

func (store *MemoryMonitorStore) countSeries(window time.Duration, points int, filter func(AccessLog) bool) []MetricPoint {
	items := store.windowItems(window)
	buckets := timeBuckets(window, points)
	for _, item := range items {
		if filter != nil && !filter(item) {
			continue
		}
		index := bucketIndex(item.Time, window, points)
		if index >= 0 && index < len(buckets) {
			buckets[index].Value++
		}
	}
	return buckets
}

func (store *MemoryMonitorStore) avgSeries(window time.Duration, points int, value func(AccessLog) float64) []MetricPoint {
	items := store.windowItems(window)
	buckets := timeBuckets(window, points)
	counts := make([]float64, len(buckets))
	for _, item := range items {
		index := bucketIndex(item.Time, window, points)
		if index >= 0 && index < len(buckets) {
			buckets[index].Value += value(item)
			counts[index]++
		}
	}
	for index := range buckets {
		if counts[index] > 0 {
			buckets[index].Value = buckets[index].Value / counts[index]
		}
	}
	return buckets
}

func (store *MemoryMonitorStore) windowItems(window time.Duration) []AccessLog {
	if store == nil {
		return nil
	}
	if window <= 0 {
		window = time.Hour
	}
	cutoff := core.Now().Add(-window)
	store.mu.RLock()
	items := append([]AccessLog(nil), store.access...)
	store.mu.RUnlock()
	index := sort.Search(len(items), func(i int) bool { return !items[i].Time.Before(cutoff) })
	return items[index:]
}

func (store *MemoryMonitorStore) queueWindowItems(window time.Duration) []QueueSample {
	if store == nil {
		return nil
	}
	if window <= 0 {
		window = time.Hour
	}
	cutoff := core.Now().Add(-window)
	store.mu.RLock()
	items := append([]QueueSample(nil), store.queue...)
	store.mu.RUnlock()
	index := sort.Search(len(items), func(i int) bool { return !items[i].Time.Before(cutoff) })
	return items[index:]
}

func (store *MemoryMonitorStore) queueAvgSeries(window time.Duration, points int, value func(QueueSample) float64) []MetricPoint {
	items := store.queueWindowItems(window)
	buckets := timeBuckets(window, points)
	counts := make([]float64, len(buckets))
	for _, item := range items {
		index := bucketIndex(item.Time, window, points)
		if index >= 0 && index < len(buckets) {
			buckets[index].Value += value(item)
			counts[index]++
		}
	}
	for index := range buckets {
		if counts[index] > 0 {
			buckets[index].Value = buckets[index].Value / counts[index]
		}
	}
	return buckets
}

func (store *MemoryMonitorStore) queueDeltaSeries(window time.Duration, points int, value func(QueueSample) float64) []MetricPoint {
	items := store.queueWindowItems(window)
	buckets := timeBuckets(window, points)
	if len(items) < 2 {
		return buckets
	}
	for index := 1; index < len(items); index++ {
		delta := value(items[index]) - value(items[index-1])
		if delta < 0 {
			delta = 0
		}
		bucket := bucketIndex(items[index].Time, window, points)
		if bucket >= 0 && bucket < len(buckets) {
			buckets[bucket].Value += delta
		}
	}
	return buckets
}

func (store *MemoryMonitorStore) recentJobs(limit int, filter func(JobLog) bool) []JobLog {
	if store == nil {
		return nil
	}
	if limit <= 0 {
		limit = 50
	}
	store.mu.RLock()
	items := append([]JobLog(nil), store.job...)
	store.mu.RUnlock()
	out := make([]JobLog, 0, min(limit, len(items)))
	for i := len(items) - 1; i >= 0 && len(out) < limit; i-- {
		if filter == nil || filter(items[i]) {
			out = append(out, items[i])
		}
	}
	return out
}

func (store *MemoryMonitorStore) jobWindowItems(window time.Duration) []JobLog {
	if store == nil {
		return nil
	}
	if window <= 0 {
		window = time.Hour
	}
	cutoff := core.Now().Add(-window)
	store.mu.RLock()
	items := append([]JobLog(nil), store.job...)
	store.mu.RUnlock()
	index := sort.Search(len(items), func(i int) bool { return !items[i].Time.Before(cutoff) })
	return items[index:]
}

func (store *MemoryMonitorStore) recentMessages(limit int, filter func(MessageLog) bool) []MessageLog {
	if store == nil {
		return nil
	}
	if limit <= 0 {
		limit = 50
	}
	store.mu.RLock()
	items := append([]MessageLog(nil), store.message...)
	store.mu.RUnlock()
	out := make([]MessageLog, 0, min(limit, len(items)))
	for i := len(items) - 1; i >= 0 && len(out) < limit; i-- {
		if filter == nil || filter(items[i]) {
			out = append(out, items[i])
		}
	}
	return out
}

func (store *MemoryMonitorStore) recentRPC(limit int, filter func(RPCLog) bool) []RPCLog {
	if store == nil {
		return nil
	}
	if limit <= 0 {
		limit = 50
	}
	store.mu.RLock()
	items := append([]RPCLog(nil), store.rpc...)
	store.mu.RUnlock()
	out := make([]RPCLog, 0, min(limit, len(items)))
	for i := len(items) - 1; i >= 0 && len(out) < limit; i-- {
		if filter == nil || filter(items[i]) {
			out = append(out, items[i])
		}
	}
	return out
}

func (store *MemoryMonitorStore) recentSQL(limit int, filter func(database.SQLLog) bool) []database.SQLLog {
	if store == nil {
		return nil
	}
	if limit <= 0 {
		limit = 50
	}
	store.mu.RLock()
	items := append([]database.SQLLog(nil), store.sql...)
	store.mu.RUnlock()
	out := make([]database.SQLLog, 0, min(limit, len(items)))
	for i := len(items) - 1; i >= 0 && len(out) < limit; i-- {
		if filter == nil || filter(items[i]) {
			out = append(out, items[i])
		}
	}
	return out
}

func (store *MemoryMonitorStore) messageWindowItems(window time.Duration) []MessageLog {
	if store == nil {
		return nil
	}
	if window <= 0 {
		window = time.Hour
	}
	cutoff := core.Now().Add(-window)
	store.mu.RLock()
	items := append([]MessageLog(nil), store.message...)
	store.mu.RUnlock()
	index := sort.Search(len(items), func(i int) bool { return !items[i].Time.Before(cutoff) })
	return items[index:]
}

func (store *MemoryMonitorStore) rpcWindowItems(window time.Duration) []RPCLog {
	if store == nil {
		return nil
	}
	if window <= 0 {
		window = time.Hour
	}
	cutoff := core.Now().Add(-window)
	store.mu.RLock()
	items := append([]RPCLog(nil), store.rpc...)
	store.mu.RUnlock()
	index := sort.Search(len(items), func(i int) bool { return !items[i].Time.Before(cutoff) })
	return items[index:]
}

func (store *MemoryMonitorStore) wsWindowItems(window time.Duration) []WSSample {
	if store == nil {
		return nil
	}
	if window <= 0 {
		window = time.Hour
	}
	cutoff := core.Now().Add(-window)
	store.mu.RLock()
	items := append([]WSSample(nil), store.ws...)
	store.mu.RUnlock()
	index := sort.Search(len(items), func(i int) bool { return !items[i].Time.Before(cutoff) })
	return items[index:]
}

func (store *MemoryMonitorStore) sqlWindowItems(window time.Duration) []database.SQLLog {
	if store == nil {
		return nil
	}
	if window <= 0 {
		window = time.Hour
	}
	cutoff := core.Now().Add(-window)
	store.mu.RLock()
	items := append([]database.SQLLog(nil), store.sql...)
	store.mu.RUnlock()
	index := sort.Search(len(items), func(i int) bool { return !items[i].Time.Before(cutoff) })
	return items[index:]
}

func (store *MemoryMonitorStore) messageCountSeries(window time.Duration, points int, filter func(MessageLog) bool) []MetricPoint {
	items := store.messageWindowItems(window)
	buckets := timeBuckets(window, points)
	for _, item := range items {
		if filter != nil && !filter(item) {
			continue
		}
		index := bucketIndex(item.Time, window, points)
		if index >= 0 && index < len(buckets) {
			buckets[index].Value++
		}
	}
	return buckets
}

func (store *MemoryMonitorStore) rpcCountSeries(window time.Duration, points int, filter func(RPCLog) bool) []MetricPoint {
	items := store.rpcWindowItems(window)
	buckets := timeBuckets(window, points)
	for _, item := range items {
		if filter != nil && !filter(item) {
			continue
		}
		index := bucketIndex(item.Time, window, points)
		if index >= 0 && index < len(buckets) {
			buckets[index].Value++
		}
	}
	return buckets
}

func (store *MemoryMonitorStore) sqlCountSeries(window time.Duration, points int, filter func(database.SQLLog) bool) []MetricPoint {
	items := store.sqlWindowItems(window)
	buckets := timeBuckets(window, points)
	for _, item := range items {
		if filter != nil && !filter(item) {
			continue
		}
		index := bucketIndex(item.Time, window, points)
		if index >= 0 && index < len(buckets) {
			buckets[index].Value++
		}
	}
	return buckets
}

func (store *MemoryMonitorStore) wsDeltaSeries(window time.Duration, points int, value func(WSSample) float64) []MetricPoint {
	items := store.wsWindowItems(window)
	buckets := timeBuckets(window, points)
	lastByHub := map[string]WSSample{}
	for _, item := range items {
		last, ok := lastByHub[item.Hub]
		lastByHub[item.Hub] = item
		if !ok {
			continue
		}
		delta := value(item) - value(last)
		if delta < 0 {
			delta = 0
		}
		index := bucketIndex(item.Time, window, points)
		if index >= 0 && index < len(buckets) {
			buckets[index].Value += delta
		}
	}
	return buckets
}

func timeBuckets(window time.Duration, points int) []MetricPoint {
	if points <= 0 {
		points = 12
	}
	if window <= 0 {
		window = time.Hour
	}
	step := window / time.Duration(points)
	if step <= 0 {
		step = time.Minute
	}
	start := core.Now().Add(-window).Truncate(step)
	items := make([]MetricPoint, points)
	for i := range items {
		items[i] = MetricPoint{Label: start.Add(time.Duration(i) * step).Format("15:04"), Value: 0}
	}
	return items
}

func bucketIndex(value time.Time, window time.Duration, points int) int {
	if points <= 0 {
		points = 12
	}
	if window <= 0 {
		window = time.Hour
	}
	start := core.Now().Add(-window)
	if value.Before(start) {
		return -1
	}
	step := window / time.Duration(points)
	if step <= 0 {
		step = time.Minute
	}
	return int(value.Sub(start) / step)
}

// MonitorStoreOf returns the app-scoped monitor store.
func MonitorStoreOf(app AppContext) MonitorStore {
	store, err := runaprovider.Invoke[MonitorStore](app)
	if err == nil && store != nil {
		return store
	}
	return NewMemoryMonitorStore()
}

// MonitorStoreProvider registers a custom monitor store.
func MonitorStoreProvider(store MonitorStore) runaprovider.Provider {
	return monitorStoreProvider{store: store}
}

type monitorStoreProvider struct {
	runaprovider.Base
	store MonitorStore
}

func (provider monitorStoreProvider) Name() string { return "console.monitor" }
func (item monitorStoreProvider) Register(ctx runaprovider.Context) error {
	if item.store == nil {
		return nil
	}
	app := AppContext(ctx)
	provideMonitorStore(app, item.store)
	provideSQLRecorder(app, item.store)
	return nil
}

func monitorMiddleware(store MonitorStore, slow time.Duration) route.Middleware {
	return func(next route.Handler) route.Handler {
		return func(ctx *route.Context) error {
			start := time.Now()
			writer := ctx.Response()
			recorder := route.NewStatusRecorder(ctx.Response())
			ctx.SetResponse(recorder)
			defer ctx.SetResponse(writer)
			err := next(ctx)
			status := recorder.Status()
			if err != nil && !recorder.Written() {
				status = route.ErrorStatus(err)
			}
			latency := time.Since(start)
			item := AccessLog{
				Time:      core.Now(),
				RequestID: ctx.RequestID(),
				Method:    ctx.Request().Method,
				Path:      ctx.Request().URL.Path,
				Status:    status,
				IP:        ctx.IP(),
				Latency:   latency,
				Slow:      slow > 0 && latency >= slow,
			}
			if ctx.Route() != nil {
				item.Route = ctx.Route().RouteName
			}
			if err != nil {
				item.Error = err.Error()
				if source := errorSource(err); source != "" {
					item.Source = source
				}
			}
			if shouldRecordAccess(item) {
				store.RecordAccess(item)
			}
			return err
		}
	}
}

func shouldRecordAccess(item AccessLog) bool {
	if logger.MatchPath(item.Path, "/favicon.ico") {
		return false
	}
	return true
}

func errorSource(err error) string {
	if err == nil {
		return ""
	}
	if withSource, ok := err.(interface{ Source() string }); ok {
		return withSource.Source()
	}
	return ""
}
