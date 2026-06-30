package observe

import (
	"context"
	"fmt"

	"github.com/duxweb/runa/cache"
	"github.com/duxweb/runa/core"
	"github.com/duxweb/runa/database"
	"github.com/duxweb/runa/host"
	runaprovider "github.com/duxweb/runa/provider"
	"github.com/duxweb/runa/queue"
	"github.com/duxweb/runa/storage"
)

// Self returns a process liveness checker.
func Self() Checker { return selfChecker{} }

type selfChecker struct{}

func (selfChecker) Name() string { return "self" }
func (selfChecker) Check(context.Context) Result {
	return ok("self", "ok", nil)
}

// Host returns a checker that reads a host unit status.
func Host(app runaprovider.Context, name string) Checker {
	return hostChecker{app: app, name: name}
}

type hostChecker struct {
	app  runaprovider.Context
	name string
}

func (checker hostChecker) Name() string {
	if checker.name == "" {
		return "host"
	}
	return "host:" + checker.name
}

func (checker hostChecker) Check(context.Context) Result {
	if checker.app == nil {
		return failed(checker.Name(), fmt.Errorf("app is nil"), nil)
	}
	app, available := checker.app.App().(interface{ HostStatus(string) host.Status })
	if !available {
		return failed(checker.Name(), fmt.Errorf("host runtime is not available"), nil)
	}
	status := app.HostStatus(checker.name)
	meta := core.Map{"status": string(status)}
	switch status {
	case host.Running:
		return ok(checker.Name(), "running", meta)
	case host.Created, host.Starting, host.Draining, host.Stopping:
		return Result{Name: checker.Name(), Status: Warn, Message: string(status), Meta: meta}
	case host.Stopped:
		return failed(checker.Name(), fmt.Errorf("stopped"), meta)
	default:
		return failed(checker.Name(), fmt.Errorf("%s", status), meta)
	}
}

// Database returns a checker for a named database runtime.
func Database(app runaprovider.Context, name string) Checker {
	return databaseChecker{app: app, name: name}
}

type databaseChecker struct {
	app  runaprovider.Context
	name string
}

func (checker databaseChecker) Name() string {
	if checker.name == "" {
		return "database:default"
	}
	return "database:" + checker.name
}

func (checker databaseChecker) Check(ctx context.Context) Result {
	if checker.app == nil {
		return failed(checker.Name(), fmt.Errorf("app is nil"), nil)
	}
	name := checker.name
	if name == "" {
		name = "default"
	}
	databases, err := runaprovider.Invoke[*database.Registry](checker.app)
	if err != nil {
		return failed(checker.Name(), err, nil)
	}
	found := false
	for _, info := range databases.Info() {
		if info.Name == name {
			found = true
			break
		}
	}
	if !found {
		return failed(checker.Name(), fmt.Errorf("database is not registered"), nil)
	}
	db := databases.MustGet(name)
	if db == nil {
		return failed(checker.Name(), fmt.Errorf("database is not open"), nil)
	}
	if err := db.Ping(ctx); err != nil {
		return failed(checker.Name(), err, nil)
	}
	return ok(checker.Name(), "ok", nil)
}

// Cache returns a checker for a named cache pool.
func Cache(app runaprovider.Context, name string) Checker {
	return cacheChecker{app: app, name: name}
}

type cacheChecker struct {
	app  runaprovider.Context
	name string
}

func (checker cacheChecker) Name() string {
	if checker.name == "" {
		return "cache:default"
	}
	return "cache:" + checker.name
}

func (checker cacheChecker) Check(ctx context.Context) Result {
	if checker.app == nil {
		return failed(checker.Name(), fmt.Errorf("app is nil"), nil)
	}
	name := checker.name
	if name == "" {
		name = "default"
	}
	caches, err := runaprovider.Invoke[*cache.Registry](checker.app)
	if err != nil {
		return failed(checker.Name(), err, nil)
	}
	found := false
	for _, info := range caches.Info() {
		if info.Name == name {
			found = true
			break
		}
	}
	if !found {
		return failed(checker.Name(), fmt.Errorf("cache %s is not configured", checker.name), nil)
	}
	pool := caches.MustOf[[]byte](name)
	stats := pool.Stats(ctx)
	return ok(checker.Name(), "ok", core.Map{"driver": stats.Driver, "hit": stats.Hit, "miss": stats.Miss})
}

// Queue returns a checker for a named queue.
func Queue(app runaprovider.Context, name string) Checker {
	return queueChecker{app: app, name: name}
}

type queueChecker struct {
	app  runaprovider.Context
	name string
}

func (checker queueChecker) Name() string {
	if checker.name == "" {
		return "queue:default"
	}
	return "queue:" + checker.name
}

func (checker queueChecker) Check(ctx context.Context) Result {
	if checker.app == nil {
		return failed(checker.Name(), fmt.Errorf("app is nil"), nil)
	}
	queues, err := runaprovider.Invoke[*queue.Registry](checker.app)
	if err != nil {
		return failed(checker.Name(), err, nil)
	}
	for _, info := range queues.QueueInfo(ctx) {
		if info.Name == checker.name || checker.name == "" && info.Name == "default" {
			return ok(checker.Name(), "ok", core.Map{"pending": info.Pending, "delayed": info.Delayed, "reserved": info.Reserved, "failed": info.Failed})
		}
	}
	return failed(checker.Name(), fmt.Errorf("queue is not registered"), nil)
}

// Storage returns a checker for a named storage disk.
func Storage(app runaprovider.Context, name string) Checker {
	return storageChecker{app: app, name: name}
}

type storageChecker struct {
	app  runaprovider.Context
	name string
}

func (checker storageChecker) Name() string {
	if checker.name == "" {
		return "storage:local"
	}
	return "storage:" + checker.name
}

func (checker storageChecker) Check(context.Context) Result {
	if checker.app == nil {
		return failed(checker.Name(), fmt.Errorf("app is nil"), nil)
	}
	storages, err := runaprovider.Invoke[*storage.Registry](checker.app)
	if err != nil {
		return failed(checker.Name(), err, nil)
	}
	for _, info := range storages.Info() {
		if info.Name == checker.name || checker.name == "" && info.Default {
			return ok(checker.Name(), "ok", core.Map{"driver": info.Driver, "prefix": info.Prefix, "public": info.Public})
		}
	}
	return failed(checker.Name(), fmt.Errorf("storage disk is not registered"), nil)
}
