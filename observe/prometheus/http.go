package prometheus

import (
	"context"
	"sync"
	"time"

	"github.com/duxweb/runa/observe"
	runaprovider "github.com/duxweb/runa/provider"
	"github.com/duxweb/runa/route"
)

var defaultDurationBuckets = []float64{0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10}

// HTTPCollector installs HTTP request metrics for Prometheus exporter.
func HTTPCollector() observe.Installer {
	return observe.InstallerFunc(func(ctx runaprovider.Context) error {
		metrics := newHTTPMetrics()
		runaprovider.ProvideValueOnce(ctx, metrics)
		return ctx.RegisterService(&httpCollectorService{metrics: metrics})
	})
}

type httpCollectorService struct {
	runaprovider.ServiceBase
	metrics *httpMetrics
}

func (service *httpCollectorService) Name() string { return "prometheus.http" }

func (service *httpCollectorService) Boot(_ context.Context, app runaprovider.Context) error {
	routes, err := runaprovider.Invoke[*route.Registry](app)
	if err != nil {
		return err
	}
	middleware := httpMiddleware(service.metrics)
	for _, item := range routes.Routes() {
		if item == nil || item.MetaAs[bool]("observe") {
			continue
		}
		item.Use(middleware)
	}
	return nil
}

type httpMetrics struct {
	mu       sync.RWMutex
	requests map[httpKey]uint64
	errors   map[httpKey]uint64
	duration map[httpKey]*histogram
}

type httpKey struct {
	Method string
	Path   string
	Status int
}

type histogram struct {
	Buckets []uint64
	Count   uint64
	Sum     float64
}

func newHTTPMetrics() *httpMetrics {
	return &httpMetrics{
		requests: make(map[httpKey]uint64),
		errors:   make(map[httpKey]uint64),
		duration: make(map[httpKey]*histogram),
	}
}

func (metrics *httpMetrics) Record(method string, path string, status int, duration time.Duration) {
	if metrics == nil {
		return
	}
	key := httpKey{Method: method, Path: path, Status: status}
	seconds := duration.Seconds()
	metrics.mu.Lock()
	metrics.requests[key]++
	if status >= 500 {
		metrics.errors[key]++
	}
	hist := metrics.duration[key]
	if hist == nil {
		hist = &histogram{Buckets: make([]uint64, len(defaultDurationBuckets))}
		metrics.duration[key] = hist
	}
	for index, bucket := range defaultDurationBuckets {
		if seconds <= bucket {
			hist.Buckets[index]++
		}
	}
	hist.Count++
	hist.Sum += seconds
	metrics.mu.Unlock()
}

func (metrics *httpMetrics) Snapshot() httpSnapshot {
	if metrics == nil {
		return httpSnapshot{}
	}
	metrics.mu.RLock()
	defer metrics.mu.RUnlock()
	snapshot := httpSnapshot{
		Requests: make(map[httpKey]uint64, len(metrics.requests)),
		Errors:   make(map[httpKey]uint64, len(metrics.errors)),
		Duration: make(map[httpKey]histogram, len(metrics.duration)),
	}
	for key, value := range metrics.requests {
		snapshot.Requests[key] = value
	}
	for key, value := range metrics.errors {
		snapshot.Errors[key] = value
	}
	for key, value := range metrics.duration {
		snapshot.Duration[key] = histogram{Buckets: append([]uint64(nil), value.Buckets...), Count: value.Count, Sum: value.Sum}
	}
	return snapshot
}

type httpSnapshot struct {
	Requests map[httpKey]uint64
	Errors   map[httpKey]uint64
	Duration map[httpKey]histogram
}

func httpMetricsOf(app runaprovider.Context) *httpMetrics {
	metrics, err := runaprovider.Invoke[*httpMetrics](app)
	if err == nil && metrics != nil {
		return metrics
	}
	return newHTTPMetrics()
}

func httpMiddleware(metrics *httpMetrics) route.Middleware {
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
			path := ctx.Request().URL.Path
			if ctx.Route() != nil && ctx.Route().Path != "" {
				path = ctx.Route().Path
			}
			metrics.Record(ctx.Request().Method, path, status, time.Since(start))
			return err
		}
	}
}
