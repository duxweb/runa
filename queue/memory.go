package queue

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/duxweb/runa/core"
)

// MemoryDriver creates an in-process queue driver.
func MemoryDriver() Driver {
	return &memoryDriver{
		pending:   make(map[string]map[string]*JobMessage),
		reserved:  make(map[string]map[string]*JobMessage),
		failed:    make(map[string]map[string]*JobMessage),
		unique:    make(map[string]string),
		instances: make(map[string]WorkerInstance),
	}
}

type memoryDriver struct {
	mu        sync.Mutex
	pending   map[string]map[string]*JobMessage
	reserved  map[string]map[string]*JobMessage
	failed    map[string]map[string]*JobMessage
	unique    map[string]string
	instances map[string]WorkerInstance
	closed    bool
}

func (driver *memoryDriver) Name() string { return "memory" }

func (driver *memoryDriver) Push(ctx context.Context, queueName string, job *JobMessage) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	if queueName == "" {
		return "", fmt.Errorf("queue name is required")
	}
	if job == nil {
		return "", fmt.Errorf("job is required")
	}
	driver.mu.Lock()
	defer driver.mu.Unlock()
	if driver.closed {
		return "", fmt.Errorf("memory queue is closed")
	}
	key := uniqueKey(queueName, job.Name, job.Unique)
	if key != "" {
		if id := driver.unique[key]; id != "" {
			return id, nil
		}
	}
	item := cloneMessage(job)
	item.Queue = queueName
	if item.ID == "" {
		item.ID = fmt.Sprintf("mem-%d", core.Now().UnixNano())
	}
	now := core.Now()
	if item.CreatedAt.IsZero() {
		item.CreatedAt = now
	}
	if item.RunAt.IsZero() {
		item.RunAt = now
	}
	item.UpdatedAt = now
	driver.bucket(driver.pending, queueName)[item.ID] = item
	if key != "" {
		driver.unique[key] = item.ID
	}
	return item.ID, nil
}

func (driver *memoryDriver) Reserve(ctx context.Context, queueName string, limit int, lease time.Duration) ([]*JobMessage, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if limit <= 0 {
		limit = 1
	}
	if lease <= 0 {
		lease = DefaultLease
	}
	driver.mu.Lock()
	defer driver.mu.Unlock()
	if driver.closed {
		return nil, fmt.Errorf("memory queue is closed")
	}
	now := core.Now()
	driver.requeueExpiredLocked(queueName, now)
	candidates := make([]*JobMessage, 0, len(driver.pending[queueName]))
	for _, item := range driver.pending[queueName] {
		if !item.RunAt.After(now) {
			candidates = append(candidates, item)
		}
	}
	sortMessages(candidates)
	if len(candidates) > limit {
		candidates = candidates[:limit]
	}
	results := make([]*JobMessage, 0, len(candidates))
	for _, item := range candidates {
		delete(driver.pending[queueName], item.ID)
		item.Attempt++
		item.ReservedUntil = now.Add(lease)
		item.UpdatedAt = now
		driver.bucket(driver.reserved, queueName)[item.ID] = item
		results = append(results, cloneMessage(item))
	}
	return results, nil
}

func (driver *memoryDriver) Ack(ctx context.Context, queueName string, id string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	driver.mu.Lock()
	defer driver.mu.Unlock()
	item := driver.reserved[queueName][id]
	if item == nil {
		return fmt.Errorf("job %s is not reserved", id)
	}
	delete(driver.reserved[queueName], id)
	driver.deleteUniqueLocked(item)
	return nil
}

func (driver *memoryDriver) Release(ctx context.Context, queueName string, id string, delay time.Duration, reason string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	driver.mu.Lock()
	defer driver.mu.Unlock()
	item := driver.reserved[queueName][id]
	if item == nil {
		return fmt.Errorf("job %s is not reserved", id)
	}
	delete(driver.reserved[queueName], id)
	now := core.Now()
	item.RunAt = now.Add(delay)
	item.ReservedUntil = time.Time{}
	item.LastError = reason
	item.UpdatedAt = now
	driver.bucket(driver.pending, queueName)[id] = item
	return nil
}

func (driver *memoryDriver) Fail(ctx context.Context, queueName string, id string, reason string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	driver.mu.Lock()
	defer driver.mu.Unlock()
	item := driver.reserved[queueName][id]
	if item == nil {
		return fmt.Errorf("job %s is not reserved", id)
	}
	delete(driver.reserved[queueName], id)
	now := core.Now()
	item.ReservedUntil = time.Time{}
	item.LastError = reason
	item.UpdatedAt = now
	driver.bucket(driver.failed, queueName)[id] = item
	return nil
}

func (driver *memoryDriver) Renew(ctx context.Context, queueName string, id string, lease time.Duration) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if lease <= 0 {
		lease = DefaultLease
	}
	driver.mu.Lock()
	defer driver.mu.Unlock()
	item := driver.reserved[queueName][id]
	if item == nil {
		return fmt.Errorf("job %s is not reserved", id)
	}
	now := core.Now()
	item.ReservedUntil = now.Add(lease)
	item.UpdatedAt = now
	return nil
}

func (driver *memoryDriver) Delete(ctx context.Context, queueName string, id string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	driver.mu.Lock()
	defer driver.mu.Unlock()
	for _, store := range []map[string]map[string]*JobMessage{driver.pending, driver.reserved, driver.failed} {
		if item := store[queueName][id]; item != nil {
			delete(store[queueName], id)
			driver.deleteUniqueLocked(item)
			return nil
		}
	}
	return fmt.Errorf("job %s is not found", id)
}

func (driver *memoryDriver) Count(ctx context.Context, queueName string, state JobState) (int64, error) {
	if err := ctx.Err(); err != nil {
		return 0, err
	}
	driver.mu.Lock()
	defer driver.mu.Unlock()
	now := core.Now()
	switch state {
	case StatePending:
		return int64(countMessages(driver.pending[queueName], func(item *JobMessage) bool { return !item.RunAt.After(now) })), nil
	case StateDelayed:
		return int64(countMessages(driver.pending[queueName], func(item *JobMessage) bool { return item.RunAt.After(now) })), nil
	case StateReserved:
		return int64(len(driver.reserved[queueName])), nil
	case StateFailed:
		return int64(len(driver.failed[queueName])), nil
	default:
		return 0, nil
	}
}

func (driver *memoryDriver) List(ctx context.Context, queueName string, query JobQuery) ([]*JobMessage, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	driver.mu.Lock()
	defer driver.mu.Unlock()
	now := core.Now()
	var source map[string]*JobMessage
	var filter func(*JobMessage) bool
	switch query.State {
	case StateDelayed:
		source = driver.pending[queueName]
		filter = func(item *JobMessage) bool { return item.RunAt.After(now) }
	case StateReserved:
		source = driver.reserved[queueName]
	case StateFailed:
		source = driver.failed[queueName]
	default:
		source = driver.pending[queueName]
		filter = func(item *JobMessage) bool { return !item.RunAt.After(now) }
	}
	items := make([]*JobMessage, 0, len(source))
	for _, item := range source {
		if filter != nil && !filter(item) {
			continue
		}
		items = append(items, cloneMessage(item))
	}
	sortMessages(items)
	return paginateMessages(items, query.Page, query.Limit), nil
}

func (driver *memoryDriver) Close(context.Context) error {
	driver.mu.Lock()
	defer driver.mu.Unlock()
	driver.closed = true
	return nil
}

func (driver *memoryDriver) Register(ctx context.Context, worker WorkerInstance) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	driver.mu.Lock()
	defer driver.mu.Unlock()
	now := core.Now()
	if worker.StartedAt.IsZero() {
		worker.StartedAt = now
	}
	worker.HeartbeatAt = now
	worker.Status = "running"
	driver.instances[worker.ID] = worker
	return nil
}

func (driver *memoryDriver) Heartbeat(ctx context.Context, id string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	driver.mu.Lock()
	defer driver.mu.Unlock()
	instance := driver.instances[id]
	if instance.ID == "" {
		return fmt.Errorf("worker instance %s is not registered", id)
	}
	instance.HeartbeatAt = core.Now()
	instance.Status = "running"
	driver.instances[id] = instance
	return nil
}

func (driver *memoryDriver) Unregister(ctx context.Context, id string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	driver.mu.Lock()
	defer driver.mu.Unlock()
	delete(driver.instances, id)
	return nil
}

func (driver *memoryDriver) Instances(ctx context.Context, worker string) ([]WorkerInstance, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	driver.mu.Lock()
	defer driver.mu.Unlock()
	now := core.Now()
	items := make([]WorkerInstance, 0)
	for _, instance := range driver.instances {
		if worker != "" && instance.Worker != worker {
			continue
		}
		if now.Sub(instance.HeartbeatAt) > 3*DefaultLease {
			continue
		}
		items = append(items, instance)
	}
	sort.SliceStable(items, func(i, j int) bool { return items[i].ID < items[j].ID })
	return items, nil
}

func (driver *memoryDriver) bucket(store map[string]map[string]*JobMessage, queueName string) map[string]*JobMessage {
	if store[queueName] == nil {
		store[queueName] = make(map[string]*JobMessage)
	}
	return store[queueName]
}

func (driver *memoryDriver) requeueExpiredLocked(queueName string, now time.Time) {
	for id, item := range driver.reserved[queueName] {
		if item.ReservedUntil.IsZero() || item.ReservedUntil.After(now) {
			continue
		}
		delete(driver.reserved[queueName], id)
		item.ReservedUntil = time.Time{}
		item.RunAt = now
		item.UpdatedAt = now
		driver.bucket(driver.pending, queueName)[id] = item
	}
}

func (driver *memoryDriver) deleteUniqueLocked(item *JobMessage) {
	key := uniqueKey(item.Queue, item.Name, item.Unique)
	if key != "" {
		delete(driver.unique, key)
	}
}

func uniqueKey(queueName string, jobName string, value string) string {
	if value == "" {
		return ""
	}
	return queueName + "\x00" + jobName + "\x00" + value
}

func cloneMessage(input *JobMessage) *JobMessage {
	if input == nil {
		return nil
	}
	output := *input
	output.Payload = append([]byte(nil), input.Payload...)
	output.Meta = core.CloneMap(input.Meta)
	return &output
}

func sortMessages(items []*JobMessage) {
	sort.SliceStable(items, func(i, j int) bool {
		if items[i].RunAt.Equal(items[j].RunAt) {
			if items[i].CreatedAt.Equal(items[j].CreatedAt) {
				return items[i].ID < items[j].ID
			}
			return items[i].CreatedAt.Before(items[j].CreatedAt)
		}
		return items[i].RunAt.Before(items[j].RunAt)
	})
}

func countMessages(items map[string]*JobMessage, fn func(*JobMessage) bool) int {
	count := 0
	for _, item := range items {
		if fn == nil || fn(item) {
			count++
		}
	}
	return count
}

func paginateMessages(items []*JobMessage, page int, limit int) []*JobMessage {
	if limit <= 0 {
		return items
	}
	if page <= 0 {
		page = 1
	}
	start := (page - 1) * limit
	if start >= len(items) {
		return nil
	}
	end := start + limit
	if end > len(items) {
		end = len(items)
	}
	return items[start:end]
}
