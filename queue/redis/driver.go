package redis

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"time"

	"github.com/duxweb/runa/queue"
	goredis "github.com/redis/go-redis/v9"
)

// New creates a Redis-backed queue driver.
func Driver(client *goredis.Client, items ...Option) queue.Driver {
	opts := defaultOptions()
	applyOptions(&opts, items...)
	normalizeOptions(&opts)
	return &driver{client: client, options: opts}
}

type driver struct {
	client     *goredis.Client
	options    options
	ownsClient bool
}

func (driver *driver) Name() string { return driver.options.driverName }

const (
	maxReserveLimit = 128
	requeueLimit    = 128
	purgeBatchSize  = 1000
)

var reserveScript = goredis.NewScript(`
local expired = redis.call('ZRANGEBYSCORE', KEYS[2], '-inf', ARGV[1], 'LIMIT', 0, tonumber(ARGV[4]))
for _, id in ipairs(expired) do
	if redis.call('ZREM', KEYS[2], id) == 1 then
		redis.call('ZADD', KEYS[1], ARGV[1], id)
	end
end
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

const deleteUniqueLua = `
if redis.call('GET', KEYS[1]) == ARGV[1] then
	return redis.call('DEL', KEYS[1])
end
return 0
`

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
		suffix, err := randomHex(8)
		if err != nil {
			return "", err
		}
		item.ID = fmt.Sprintf("redis-%d-%s", now.UnixNano(), suffix)
	}
	if item.CreatedAt.IsZero() {
		item.CreatedAt = now
	}
	if item.RunAt.IsZero() {
		item.RunAt = now
	}
	item.UpdatedAt = now
	item.UniqueStrategy = queueUniqueStrategy(item.Unique, item.UniqueStrategy)
	unique := driver.uniqueKey(queueName, item.Name, item.Unique)
	if unique != "" {
		ok, err := driver.client.SetNX(ctx, unique, item.ID, uniqueTTL(item.UniqueTTL)).Result()
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
			_ = driver.deleteUniqueNow(ctx, unique, item.ID)
		}
		return "", err
	}
	pipe := driver.client.TxPipeline()
	pipe.Set(ctx, driver.jobKey(item.ID), body, 0)
	pipe.ZAdd(ctx, driver.stateKey(queueName, queue.StatePending), goredis.Z{Score: score(item.RunAt), Member: item.ID})
	if _, err := pipe.Exec(ctx); err != nil {
		if unique != "" {
			_ = driver.deleteUniqueNow(ctx, unique, item.ID)
		}
		return "", err
	}
	return item.ID, nil
}

func (driver *driver) Reserve(ctx context.Context, queueName string, limit int, lease time.Duration) ([]*queue.JobMessage, error) {
	if limit <= 0 {
		limit = 1
	}
	if limit > maxReserveLimit {
		limit = maxReserveLimit
	}
	if lease <= 0 {
		lease = queue.DefaultLease
	}
	now := driver.options.now()
	reservedUntil := now.Add(lease)
	pendingKey := driver.stateKey(queueName, queue.StatePending)
	reservedKey := driver.stateKey(queueName, queue.StateReserved)
	ids, err := reserveScript.Run(ctx, driver.client,
		[]string{
			pendingKey,
			reservedKey,
		},
		scoreString(now),
		limit,
		score(reservedUntil),
		requeueLimit,
	).StringSlice()
	if err != nil {
		return nil, err
	}
	if len(ids) == 0 {
		return nil, nil
	}
	values, err := driver.client.MGet(ctx, driver.jobKeys(ids)...).Result()
	if err != nil {
		return nil, err
	}
	items := make([]*queue.JobMessage, 0, len(ids))
	missing := make([]string, 0)
	pipe := driver.client.TxPipeline()
	commands := 0
	for index, id := range ids {
		raw := values[index]
		if raw == nil {
			missing = append(missing, id)
			continue
		}
		item, err := decodeJob(raw)
		if err != nil {
			missing = append(missing, id)
			pipe.Del(ctx, driver.jobKey(id))
			commands++
			continue
		}
		item.Attempt++
		item.ReservedUntil = reservedUntil
		item.UpdatedAt = now
		body, err := json.Marshal(item)
		if err != nil {
			return nil, err
		}
		pipe.Set(ctx, driver.jobKey(id), body, 0)
		commands++
		if driver.shouldDeleteUniqueUntilStart(item) {
			driver.deleteUnique(pipe, ctx, driver.uniqueKey(item.Queue, item.Name, item.Unique), item.ID)
			commands++
		}
		items = append(items, item)
	}
	if len(missing) > 0 {
		values := anySlice(missing)
		pipe.ZRem(ctx, pendingKey, values...)
		pipe.ZRem(ctx, reservedKey, values...)
		commands += 2
	}
	if commands > 0 {
		if _, err := pipe.Exec(ctx); err != nil {
			return nil, err
		}
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
			driver.deleteUnique(pipe, ctx, unique, item.ID)
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
	if driver.shouldDeleteUniqueUntilDone(item) {
		driver.deleteUnique(pipe, ctx, driver.uniqueKey(item.Queue, item.Name, item.Unique), item.ID)
	}
	_, err = pipe.Exec(ctx)
	return err
}

func (driver *driver) Renew(ctx context.Context, queueName string, id string, lease time.Duration) error {
	if lease <= 0 {
		lease = queue.DefaultLease
	}
	now := driver.options.now()
	return driver.client.ZAddArgs(ctx, driver.stateKey(queueName, queue.StateReserved), goredis.ZAddArgs{
		XX:      true,
		Members: []goredis.Z{{Score: score(now.Add(lease)), Member: id}},
	}).Err()
}

func (driver *driver) Delete(ctx context.Context, queueName string, id string) error {
	return driver.Ack(ctx, queueName, id)
}

func (driver *driver) Purge(ctx context.Context, queueName string, state queue.JobState, olderThan time.Time) (int64, error) {
	if state != queue.StateFailed {
		return 0, nil
	}
	key := driver.stateKey(queueName, state)
	var total int64
	for {
		ids, err := driver.client.ZRangeByScore(ctx, key, &goredis.ZRangeBy{
			Min:   "-inf",
			Max:   "(" + scoreString(olderThan),
			Count: purgeBatchSize,
		}).Result()
		if err != nil {
			return total, err
		}
		if len(ids) == 0 {
			return total, nil
		}
		values, err := driver.client.MGet(ctx, driver.jobKeys(ids)...).Result()
		if err != nil {
			return total, err
		}
		pipe := driver.client.TxPipeline()
		pipe.ZRem(ctx, key, anySlice(ids)...)
		for index, id := range ids {
			if values[index] != nil {
				if item, err := decodeJob(values[index]); err == nil {
					if unique := driver.uniqueKey(item.Queue, item.Name, item.Unique); unique != "" {
						driver.deleteUnique(pipe, ctx, unique, item.ID)
					}
				}
			}
			pipe.Del(ctx, driver.jobKey(id))
		}
		if _, err := pipe.Exec(ctx); err != nil {
			return total, err
		}
		total += int64(len(ids))
	}
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
	if driver.client == nil || !driver.ownsClient {
		return nil
	}
	return driver.client.Close()
}

func (driver *driver) LockSweep(ctx context.Context, queueName string, ttl time.Duration) (bool, error) {
	if ttl <= 0 {
		ttl = time.Minute
	}
	return driver.client.SetNX(ctx, driver.sweepKey(queueName), "1", ttl).Result()
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

func (driver *driver) jobKeys(ids []string) []string {
	keys := make([]string, len(ids))
	for index, id := range ids {
		keys[index] = driver.jobKey(id)
	}
	return keys
}

func (driver *driver) stateKey(queueName string, state queue.JobState) string {
	return driver.options.prefix + ":queue:" + queueName + ":" + string(state)
}

func (driver *driver) sweepKey(queueName string) string {
	return driver.options.prefix + ":sweep:" + queueName
}

func (driver *driver) uniqueKey(queueName string, jobName string, unique string) string {
	if unique == "" {
		return ""
	}
	return driver.options.prefix + ":unique:" + queueName + ":" + jobName + ":" + unique
}

func (driver *driver) deleteUnique(pipe goredis.Pipeliner, ctx context.Context, key string, id string) {
	if key == "" || id == "" {
		return
	}
	pipe.Eval(ctx, deleteUniqueLua, []string{key}, id)
}

func (driver *driver) deleteUniqueNow(ctx context.Context, key string, id string) error {
	if key == "" || id == "" {
		return nil
	}
	return driver.client.Eval(ctx, deleteUniqueLua, []string{key}, id).Err()
}

func (driver *driver) shouldDeleteUniqueUntilStart(item *queue.JobMessage) bool {
	return item != nil && item.UniqueStrategy == string(queue.UniqueStrategyUntilStart)
}

func (driver *driver) shouldDeleteUniqueUntilDone(item *queue.JobMessage) bool {
	return item != nil && (item.UniqueStrategy == "" || item.UniqueStrategy == string(queue.UniqueStrategyUntilDone))
}

func queueUniqueStrategy(unique string, strategy string) string {
	if unique == "" {
		return ""
	}
	if strategy == string(queue.UniqueStrategyUntilStart) {
		return string(queue.UniqueStrategyUntilStart)
	}
	return string(queue.UniqueStrategyUntilDone)
}

func uniqueTTL(ttl time.Duration) time.Duration {
	if ttl <= 0 {
		return 0
	}
	return ttl
}

func anySlice(values []string) []any {
	items := make([]any, len(values))
	for index, value := range values {
		items[index] = value
	}
	return items
}

func randomHex(byteCount int) (string, error) {
	if byteCount <= 0 {
		byteCount = 8
	}
	body := make([]byte, byteCount)
	if _, err := rand.Read(body); err != nil {
		return "", err
	}
	return hex.EncodeToString(body), nil
}

func decodeJob(raw any) (*queue.JobMessage, error) {
	var body []byte
	switch value := raw.(type) {
	case string:
		body = []byte(value)
	case []byte:
		body = value
	default:
		return nil, fmt.Errorf("unsupported redis job body type %T", raw)
	}
	var item queue.JobMessage
	if err := json.Unmarshal(body, &item); err != nil {
		return nil, err
	}
	return &item, nil
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
