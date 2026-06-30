package redis

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"time"

	"github.com/duxweb/runa/core"
	"github.com/duxweb/runa/queue"
	goredis "github.com/redis/go-redis/v9"
)

// New creates a Redis-backed queue driver.
func Driver(client *goredis.Client, items ...Option) queue.Driver {
	opts := options{prefix: "runa:queue", now: core.Now}
	for _, item := range items {
		if item != nil {
			item(&opts)
		}
	}
	return &driver{client: client, options: opts}
}

type driver struct {
	client  *goredis.Client
	options options
}

func (driver *driver) Name() string { return "redis" }

var reserveScript = goredis.NewScript(`
local ids = redis.call('ZRANGEBYSCORE', KEYS[1], '-inf', ARGV[1], 'LIMIT', 0, tonumber(ARGV[2]))
local claimed = {}
for _, id in ipairs(ids) do
	if redis.call('ZREM', KEYS[1], id) == 1 then
		redis.call('ZADD', KEYS[2], ARGV[3], id)
		table.insert(claimed, id)
	end
end
return claimed
`)

func (driver *driver) Push(ctx context.Context, queueName string, job *queue.JobMessage) (string, error) {
	if driver.client == nil {
		return "", fmt.Errorf("redis queue client is nil")
	}
	if job == nil {
		return "", fmt.Errorf("job is required")
	}
	item := clone(job)
	item.Queue = queueName
	now := driver.options.now()
	if item.ID == "" {
		item.ID = fmt.Sprintf("redis-%d", now.UnixNano())
	}
	if item.CreatedAt.IsZero() {
		item.CreatedAt = now
	}
	if item.RunAt.IsZero() {
		item.RunAt = now
	}
	item.UpdatedAt = now
	unique := driver.uniqueKey(queueName, item.Name, item.Unique)
	if unique != "" {
		ok, err := driver.client.SetNX(ctx, unique, item.ID, 0).Result()
		if err != nil {
			return "", err
		}
		if !ok {
			return driver.client.Get(ctx, unique).Result()
		}
	}
	body, err := json.Marshal(item)
	if err != nil {
		if unique != "" {
			_ = driver.client.Del(ctx, unique).Err()
		}
		return "", err
	}
	pipe := driver.client.TxPipeline()
	pipe.Set(ctx, driver.jobKey(item.ID), body, 0)
	pipe.ZAdd(ctx, driver.stateKey(queueName, queue.StatePending), goredis.Z{Score: score(item.RunAt), Member: item.ID})
	if _, err := pipe.Exec(ctx); err != nil {
		if unique != "" {
			_ = driver.client.Del(ctx, unique).Err()
		}
		return "", err
	}
	return item.ID, nil
}

func (driver *driver) Reserve(ctx context.Context, queueName string, limit int, lease time.Duration) ([]*queue.JobMessage, error) {
	if limit <= 0 {
		limit = 1
	}
	if lease <= 0 {
		lease = queue.DefaultLease
	}
	now := driver.options.now()
	if err := driver.requeueExpired(ctx, queueName, now); err != nil {
		return nil, err
	}
	reservedUntil := now.Add(lease)
	ids, err := reserveScript.Run(ctx, driver.client,
		[]string{
			driver.stateKey(queueName, queue.StatePending),
			driver.stateKey(queueName, queue.StateReserved),
		},
		scoreString(now),
		limit,
		score(reservedUntil),
	).StringSlice()
	if err != nil {
		return nil, err
	}
	items := make([]*queue.JobMessage, 0, len(ids))
	for _, id := range ids {
		item, err := driver.get(ctx, id)
		if err != nil {
			continue
		}
		item.Attempt++
		item.ReservedUntil = reservedUntil
		item.UpdatedAt = now
		body, err := json.Marshal(item)
		if err != nil {
			return nil, err
		}
		if err := driver.client.Set(ctx, driver.jobKey(id), body, 0).Err(); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, nil
}

func (driver *driver) Ack(ctx context.Context, queueName string, id string) error {
	item, _ := driver.get(ctx, id)
	pipe := driver.client.TxPipeline()
	pipe.Del(ctx, driver.jobKey(id))
	pipe.ZRem(ctx, driver.stateKey(queueName, queue.StatePending), id)
	pipe.ZRem(ctx, driver.stateKey(queueName, queue.StateReserved), id)
	pipe.ZRem(ctx, driver.stateKey(queueName, queue.StateFailed), id)
	if item != nil {
		if unique := driver.uniqueKey(item.Queue, item.Name, item.Unique); unique != "" {
			pipe.Del(ctx, unique)
		}
	}
	_, err := pipe.Exec(ctx)
	return err
}

func (driver *driver) Release(ctx context.Context, queueName string, id string, delay time.Duration, reason string) error {
	item, err := driver.get(ctx, id)
	if err != nil {
		return err
	}
	now := driver.options.now()
	item.RunAt = now.Add(delay)
	item.ReservedUntil = time.Time{}
	item.LastError = reason
	item.UpdatedAt = now
	body, err := json.Marshal(item)
	if err != nil {
		return err
	}
	pipe := driver.client.TxPipeline()
	pipe.ZRem(ctx, driver.stateKey(queueName, queue.StateReserved), id)
	pipe.ZAdd(ctx, driver.stateKey(queueName, queue.StatePending), goredis.Z{Score: score(item.RunAt), Member: id})
	pipe.Set(ctx, driver.jobKey(id), body, 0)
	_, err = pipe.Exec(ctx)
	return err
}

func (driver *driver) Fail(ctx context.Context, queueName string, id string, reason string) error {
	item, err := driver.get(ctx, id)
	if err != nil {
		return err
	}
	now := driver.options.now()
	item.ReservedUntil = time.Time{}
	item.LastError = reason
	item.UpdatedAt = now
	body, err := json.Marshal(item)
	if err != nil {
		return err
	}
	pipe := driver.client.TxPipeline()
	pipe.ZRem(ctx, driver.stateKey(queueName, queue.StateReserved), id)
	pipe.ZAdd(ctx, driver.stateKey(queueName, queue.StateFailed), goredis.Z{Score: score(now), Member: id})
	pipe.Set(ctx, driver.jobKey(id), body, 0)
	_, err = pipe.Exec(ctx)
	return err
}

func (driver *driver) Renew(ctx context.Context, queueName string, id string, lease time.Duration) error {
	if lease <= 0 {
		lease = queue.DefaultLease
	}
	item, err := driver.get(ctx, id)
	if err != nil {
		return err
	}
	now := driver.options.now()
	item.ReservedUntil = now.Add(lease)
	item.UpdatedAt = now
	body, err := json.Marshal(item)
	if err != nil {
		return err
	}
	pipe := driver.client.TxPipeline()
	pipe.ZAdd(ctx, driver.stateKey(queueName, queue.StateReserved), goredis.Z{Score: score(item.ReservedUntil), Member: id})
	pipe.Set(ctx, driver.jobKey(id), body, 0)
	_, err = pipe.Exec(ctx)
	return err
}

func (driver *driver) Delete(ctx context.Context, queueName string, id string) error {
	return driver.Ack(ctx, queueName, id)
}

func (driver *driver) Count(ctx context.Context, queueName string, state queue.JobState) (int64, error) {
	now := driver.options.now()
	switch state {
	case queue.StatePending:
		return driver.client.ZCount(ctx, driver.stateKey(queueName, queue.StatePending), "-inf", scoreString(now)).Result()
	case queue.StateDelayed:
		return driver.client.ZCount(ctx, driver.stateKey(queueName, queue.StatePending), "("+scoreString(now), "+inf").Result()
	case queue.StateReserved, queue.StateFailed:
		return driver.client.ZCard(ctx, driver.stateKey(queueName, state)).Result()
	default:
		return 0, nil
	}
}

func (driver *driver) List(ctx context.Context, queueName string, query queue.JobQuery) ([]*queue.JobMessage, error) {
	now := driver.options.now()
	key := driver.stateKey(queueName, queue.StatePending)
	min := "-inf"
	max := scoreString(now)
	if query.State == queue.StateDelayed {
		min = "(" + scoreString(now)
		max = "+inf"
	} else if query.State == queue.StateReserved || query.State == queue.StateFailed {
		key = driver.stateKey(queueName, query.State)
		min = "-inf"
		max = "+inf"
	}
	offset, count := int64(0), int64(query.Limit)
	if query.Page > 1 && query.Limit > 0 {
		offset = int64((query.Page - 1) * query.Limit)
	}
	rangeBy := &goredis.ZRangeBy{Min: min, Max: max, Offset: offset}
	if count > 0 {
		rangeBy.Count = count
	}
	ids, err := driver.client.ZRangeByScore(ctx, key, rangeBy).Result()
	if err != nil {
		return nil, err
	}
	items := make([]*queue.JobMessage, 0, len(ids))
	for _, id := range ids {
		item, err := driver.get(ctx, id)
		if err == nil {
			items = append(items, item)
		}
	}
	sort.SliceStable(items, func(i, j int) bool { return items[i].RunAt.Before(items[j].RunAt) })
	return items, nil
}

func (driver *driver) Close(context.Context) error {
	if driver.client == nil {
		return nil
	}
	return driver.client.Close()
}

func (driver *driver) Register(ctx context.Context, worker queue.WorkerInstance) error {
	now := driver.options.now()
	if worker.StartedAt.IsZero() {
		worker.StartedAt = now
	}
	worker.HeartbeatAt = now
	worker.Status = "running"
	body, err := json.Marshal(worker)
	if err != nil {
		return err
	}
	return driver.client.HSet(ctx, driver.workersKey(), worker.ID, body).Err()
}

func (driver *driver) Heartbeat(ctx context.Context, id string) error {
	raw, err := driver.client.HGet(ctx, driver.workersKey(), id).Bytes()
	if err != nil {
		return err
	}
	var worker queue.WorkerInstance
	if err := json.Unmarshal(raw, &worker); err != nil {
		return err
	}
	worker.HeartbeatAt = driver.options.now()
	worker.Status = "running"
	body, err := json.Marshal(worker)
	if err != nil {
		return err
	}
	return driver.client.HSet(ctx, driver.workersKey(), id, body).Err()
}

func (driver *driver) Unregister(ctx context.Context, id string) error {
	return driver.client.HDel(ctx, driver.workersKey(), id).Err()
}

func (driver *driver) Instances(ctx context.Context, workerName string) ([]queue.WorkerInstance, error) {
	values, err := driver.client.HGetAll(ctx, driver.workersKey()).Result()
	if err != nil {
		return nil, err
	}
	now := driver.options.now()
	items := make([]queue.WorkerInstance, 0, len(values))
	for _, raw := range values {
		var item queue.WorkerInstance
		if err := json.Unmarshal([]byte(raw), &item); err != nil {
			continue
		}
		if workerName != "" && item.Worker != workerName {
			continue
		}
		if now.Sub(item.HeartbeatAt) > 3*queue.DefaultLease {
			continue
		}
		items = append(items, item)
	}
	sort.SliceStable(items, func(i, j int) bool { return items[i].ID < items[j].ID })
	return items, nil
}

func (driver *driver) requeueExpired(ctx context.Context, queueName string, now time.Time) error {
	ids, err := driver.client.ZRangeByScore(ctx, driver.stateKey(queueName, queue.StateReserved), &goredis.ZRangeBy{
		Min: "-inf",
		Max: scoreString(now),
	}).Result()
	if err != nil {
		return err
	}
	for _, id := range ids {
		item, err := driver.get(ctx, id)
		if err != nil {
			continue
		}
		item.ReservedUntil = time.Time{}
		item.RunAt = now
		item.UpdatedAt = now
		body, err := json.Marshal(item)
		if err != nil {
			return err
		}
		pipe := driver.client.TxPipeline()
		pipe.ZRem(ctx, driver.stateKey(queueName, queue.StateReserved), id)
		pipe.ZAdd(ctx, driver.stateKey(queueName, queue.StatePending), goredis.Z{Score: score(now), Member: id})
		pipe.Set(ctx, driver.jobKey(id), body, 0)
		if _, err := pipe.Exec(ctx); err != nil {
			return err
		}
	}
	return nil
}

func (driver *driver) get(ctx context.Context, id string) (*queue.JobMessage, error) {
	raw, err := driver.client.Get(ctx, driver.jobKey(id)).Bytes()
	if err != nil {
		return nil, err
	}
	var item queue.JobMessage
	if err := json.Unmarshal(raw, &item); err != nil {
		return nil, err
	}
	return &item, nil
}

func (driver *driver) jobKey(id string) string {
	return driver.options.prefix + ":job:" + id
}

func (driver *driver) stateKey(queueName string, state queue.JobState) string {
	return driver.options.prefix + ":queue:" + queueName + ":" + string(state)
}

func (driver *driver) uniqueKey(queueName string, jobName string, unique string) string {
	if unique == "" {
		return ""
	}
	return driver.options.prefix + ":unique:" + queueName + ":" + jobName + ":" + unique
}

func (driver *driver) workersKey() string {
	return driver.options.prefix + ":workers"
}

func score(value time.Time) float64 {
	return float64(value.UnixMilli())
}

func scoreString(value time.Time) string {
	return fmt.Sprintf("%d", value.UnixMilli())
}

func clone(input *queue.JobMessage) *queue.JobMessage {
	if input == nil {
		return nil
	}
	output := *input
	output.Payload = append([]byte(nil), input.Payload...)
	if input.Meta != nil {
		output.Meta = make(map[string]any, len(input.Meta))
		for key, value := range input.Meta {
			output.Meta[key] = value
		}
	}
	return &output
}
