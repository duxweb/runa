package redis

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
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

const (
	jobFieldBody           = "body"
	jobFieldQueue          = "queue"
	jobFieldName           = "name"
	jobFieldUnique         = "unique"
	jobFieldUniqueStrategy = "unique_strategy"
	jobFieldAttempt        = "attempt"
	jobFieldReservedUntil  = "reserved_until"
	jobFieldRunAt          = "run_at"
	jobFieldLastError      = "last_error"
	jobFieldUpdatedAt      = "updated_at"
)

var reserveScript = goredis.NewScript(`
local expired = redis.call('ZRANGEBYSCORE', KEYS[2], '-inf', ARGV[1], 'LIMIT', 0, tonumber(ARGV[4]))
for _, id in ipairs(expired) do
	if redis.call('ZREM', KEYS[2], id) == 1 then
		redis.call('ZADD', KEYS[1], ARGV[1], id)
	end
end
local ids = redis.call('ZRANGEBYSCORE', KEYS[1], '-inf', ARGV[1], 'LIMIT', 0, tonumber(ARGV[2]))
local result = {}
for _, id in ipairs(ids) do
	if redis.call('ZREM', KEYS[1], id) == 1 then
		local jobkey = ARGV[5] .. id
		local vals = redis.call('HMGET', jobkey, 'body', 'queue', 'name', 'unique', 'unique_strategy', 'run_at', 'last_error')
		if vals[1] then
			redis.call('ZADD', KEYS[2], ARGV[3], id)
			local attempt = redis.call('HINCRBY', jobkey, 'attempt', 1)
			redis.call('HSET', jobkey, 'reserved_until', ARGV[3], 'updated_at', ARGV[1])
			table.insert(result, id)
			table.insert(result, vals[1])
			table.insert(result, tostring(attempt))
			table.insert(result, vals[2] or '')
			table.insert(result, vals[3] or '')
			table.insert(result, vals[4] or '')
			table.insert(result, vals[5] or '')
			table.insert(result, vals[6] or '')
			table.insert(result, vals[7] or '')
		end
	end
end
return result
`)

var ackScript = goredis.NewScript(`
local vals = redis.call('HMGET', KEYS[1], 'queue', 'name', 'unique')
local queue = vals[1] or ''
local name = vals[2] or ''
local unique = vals[3] or ''
redis.call('DEL', KEYS[1])
redis.call('ZREM', KEYS[2], ARGV[1])
redis.call('ZREM', KEYS[3], ARGV[1])
redis.call('ZREM', KEYS[4], ARGV[1])
if unique ~= '' then
	local ukey = ARGV[2] .. queue .. ':' .. name .. ':' .. unique
	if redis.call('GET', ukey) == ARGV[1] then
		redis.call('DEL', ukey)
	end
end
return 1
`)

var releaseScript = goredis.NewScript(`
if not redis.call('HGET', KEYS[1], 'body') then
	return 0
end
redis.call('HSET', KEYS[1], 'run_at', ARGV[2], 'last_error', ARGV[3], 'updated_at', ARGV[4], 'reserved_until', '0')
redis.call('ZREM', KEYS[2], ARGV[1])
redis.call('ZADD', KEYS[3], ARGV[2], ARGV[1])
return 1
`)

var failScript = goredis.NewScript(`
if not redis.call('HGET', KEYS[1], 'body') then
	return 0
end
local vals = redis.call('HMGET', KEYS[1], 'queue', 'name', 'unique', 'unique_strategy')
local queue = vals[1] or ''
local name = vals[2] or ''
local unique = vals[3] or ''
local strategy = vals[4] or ''
redis.call('HSET', KEYS[1], 'reserved_until', '0', 'last_error', ARGV[2], 'updated_at', ARGV[3])
redis.call('ZREM', KEYS[2], ARGV[1])
redis.call('ZADD', KEYS[3], ARGV[3], ARGV[1])
if unique ~= '' and (strategy == '' or strategy == 'until-done') then
	local ukey = ARGV[4] .. queue .. ':' .. name .. ':' .. unique
	if redis.call('GET', ukey) == ARGV[1] then
		redis.call('DEL', ukey)
	end
end
return 1
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
	fields, err := jobHashFields(item)
	if err != nil {
		if unique != "" {
			_ = driver.deleteUniqueNow(ctx, unique, item.ID)
		}
		return "", err
	}
	pipe := driver.client.TxPipeline()
	pipe.HSet(ctx, driver.jobKey(item.ID), fields)
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
	raw, err := reserveScript.Run(ctx, driver.client,
		[]string{
			pendingKey,
			reservedKey,
		},
		scoreString(now),
		limit,
		scoreString(reservedUntil),
		requeueLimit,
		driver.jobKeyPrefix(),
	).Result()
	if err != nil {
		return nil, err
	}
	records, err := parseReserveRecords(raw, now, reservedUntil)
	if err != nil {
		return nil, err
	}
	if len(records) == 0 {
		return nil, nil
	}
	items := make([]*queue.JobMessage, 0, len(records))
	missing := make([]string, 0)
	pipe := driver.client.TxPipeline()
	commands := 0
	for _, record := range records {
		item, err := composeJob(record.fields)
		if err != nil {
			missing = append(missing, record.id)
			pipe.Del(ctx, driver.jobKey(record.id))
			commands++
			continue
		}
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
	return ackScript.Run(ctx, driver.client,
		[]string{
			driver.jobKey(id),
			driver.stateKey(queueName, queue.StatePending),
			driver.stateKey(queueName, queue.StateReserved),
			driver.stateKey(queueName, queue.StateFailed),
		},
		id,
		driver.uniqueKeyPrefix(),
	).Err()
}

func (driver *driver) Release(ctx context.Context, queueName string, id string, delay time.Duration, reason string) error {
	now := driver.options.now()
	runAt := now.Add(delay)
	result, err := releaseScript.Run(ctx, driver.client,
		[]string{
			driver.jobKey(id),
			driver.stateKey(queueName, queue.StateReserved),
			driver.stateKey(queueName, queue.StatePending),
		},
		id,
		scoreString(runAt),
		reason,
		scoreString(now),
	).Int()
	if err != nil {
		return err
	}
	if result == 0 {
		return goredis.Nil
	}
	return nil
}

func (driver *driver) Fail(ctx context.Context, queueName string, id string, reason string) error {
	now := driver.options.now()
	result, err := failScript.Run(ctx, driver.client,
		[]string{
			driver.jobKey(id),
			driver.stateKey(queueName, queue.StateReserved),
			driver.stateKey(queueName, queue.StateFailed),
		},
		id,
		reason,
		scoreString(now),
		driver.uniqueKeyPrefix(),
	).Int()
	if err != nil {
		return err
	}
	if result == 0 {
		return goredis.Nil
	}
	return nil
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
		readPipe := driver.client.Pipeline()
		commands := make([]*goredis.SliceCmd, len(ids))
		for index, id := range ids {
			commands[index] = readPipe.HMGet(ctx, driver.jobKey(id), jobFieldQueue, jobFieldName, jobFieldUnique)
		}
		if _, err := readPipe.Exec(ctx); err != nil && err != goredis.Nil {
			return total, err
		}
		pipe := driver.client.TxPipeline()
		pipe.ZRem(ctx, key, anySlice(ids)...)
		for index, id := range ids {
			if values, err := commands[index].Result(); err == nil && len(values) >= 3 {
				queueName, _ := redisString(values[0])
				jobName, _ := redisString(values[1])
				uniqueValue, _ := redisString(values[2])
				if unique := driver.uniqueKey(queueName, jobName, uniqueValue); unique != "" {
					driver.deleteUnique(pipe, ctx, unique, id)
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
	pipe := driver.client.Pipeline()
	commands := make([]*goredis.MapStringStringCmd, len(ids))
	for index, id := range ids {
		commands[index] = pipe.HGetAll(ctx, driver.jobKey(id))
	}
	if _, err := pipe.Exec(ctx); err != nil && err != goredis.Nil {
		return nil, err
	}
	for _, command := range commands {
		fields, err := command.Result()
		if err != nil || len(fields) == 0 {
			continue
		}
		item, err := composeJob(fields)
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
	fields, err := driver.client.HGetAll(ctx, driver.jobKey(id)).Result()
	if err != nil {
		return nil, err
	}
	if len(fields) == 0 {
		return nil, goredis.Nil
	}
	return composeJob(fields)
}

func (driver *driver) jobKey(id string) string {
	return driver.jobKeyPrefix() + id
}

func (driver *driver) jobKeyPrefix() string {
	return driver.options.prefix + ":job:"
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
	return driver.uniqueKeyPrefix() + queueName + ":" + jobName + ":" + unique
}

func (driver *driver) uniqueKeyPrefix() string {
	return driver.options.prefix + ":unique:"
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

type reserveRecord struct {
	id     string
	fields map[string]string
}

func parseReserveRecords(raw any, now time.Time, reservedUntil time.Time) ([]reserveRecord, error) {
	values, err := redisSlice(raw)
	if err != nil {
		return nil, err
	}
	const width = 9
	if len(values)%width != 0 {
		return nil, fmt.Errorf("invalid redis reserve result width %d", len(values))
	}
	records := make([]reserveRecord, 0, len(values)/width)
	for offset := 0; offset < len(values); offset += width {
		id, err := redisString(values[offset])
		if err != nil {
			return nil, err
		}
		body, err := redisString(values[offset+1])
		if err != nil {
			return nil, err
		}
		attempt, err := redisString(values[offset+2])
		if err != nil {
			return nil, err
		}
		queueName, _ := redisString(values[offset+3])
		name, _ := redisString(values[offset+4])
		unique, _ := redisString(values[offset+5])
		uniqueStrategy, _ := redisString(values[offset+6])
		runAt, _ := redisString(values[offset+7])
		lastError, _ := redisString(values[offset+8])
		records = append(records, reserveRecord{
			id: id,
			fields: map[string]string{
				jobFieldBody:           body,
				jobFieldQueue:          queueName,
				jobFieldName:           name,
				jobFieldUnique:         unique,
				jobFieldUniqueStrategy: uniqueStrategy,
				jobFieldRunAt:          runAt,
				jobFieldLastError:      lastError,
				jobFieldAttempt:        attempt,
				jobFieldReservedUntil:  scoreString(reservedUntil),
				jobFieldUpdatedAt:      scoreString(now),
			},
		})
	}
	return records, nil
}

func jobHashFields(item *queue.JobMessage) (map[string]any, error) {
	body, err := json.Marshal(item)
	if err != nil {
		return nil, err
	}
	return map[string]any{
		jobFieldBody:           body,
		jobFieldQueue:          item.Queue,
		jobFieldName:           item.Name,
		jobFieldUnique:         item.Unique,
		jobFieldUniqueStrategy: item.UniqueStrategy,
		jobFieldAttempt:        item.Attempt,
		jobFieldReservedUntil:  timeField(item.ReservedUntil),
		jobFieldRunAt:          timeField(item.RunAt),
		jobFieldLastError:      item.LastError,
		jobFieldUpdatedAt:      timeField(item.UpdatedAt),
	}, nil
}

func composeJob(fields map[string]string) (*queue.JobMessage, error) {
	body, ok := fields[jobFieldBody]
	if !ok || body == "" {
		return nil, goredis.Nil
	}
	item, err := decodeJob(body)
	if err != nil {
		return nil, err
	}
	if value, ok := fields[jobFieldQueue]; ok {
		item.Queue = value
	}
	if value, ok := fields[jobFieldName]; ok {
		item.Name = value
	}
	if value, ok := fields[jobFieldUnique]; ok {
		item.Unique = value
	}
	if value, ok := fields[jobFieldUniqueStrategy]; ok {
		item.UniqueStrategy = value
	}
	if value, ok := fields[jobFieldAttempt]; ok && value != "" {
		attempt, err := strconv.Atoi(value)
		if err != nil {
			return nil, err
		}
		item.Attempt = attempt
	}
	if value, ok := fields[jobFieldRunAt]; ok && value != "" {
		item.RunAt = parseTimeField(value)
	}
	if value, ok := fields[jobFieldReservedUntil]; ok && value != "" {
		item.ReservedUntil = parseTimeField(value)
	}
	if value, ok := fields[jobFieldLastError]; ok {
		item.LastError = value
	}
	if value, ok := fields[jobFieldUpdatedAt]; ok && value != "" {
		item.UpdatedAt = parseTimeField(value)
	}
	return item, nil
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

func redisSlice(raw any) ([]any, error) {
	switch value := raw.(type) {
	case []any:
		return value, nil
	case []string:
		items := make([]any, len(value))
		for index, item := range value {
			items[index] = item
		}
		return items, nil
	case nil:
		return nil, nil
	default:
		return nil, fmt.Errorf("unsupported redis result type %T", raw)
	}
}

func redisString(raw any) (string, error) {
	switch value := raw.(type) {
	case nil:
		return "", nil
	case string:
		return value, nil
	case []byte:
		return string(value), nil
	case int:
		return strconv.Itoa(value), nil
	case int64:
		return strconv.FormatInt(value, 10), nil
	case uint64:
		return strconv.FormatUint(value, 10), nil
	default:
		return "", fmt.Errorf("unsupported redis value type %T", raw)
	}
}

func timeField(value time.Time) string {
	if value.IsZero() {
		return "0"
	}
	return scoreString(value)
}

func parseTimeField(value string) time.Time {
	if value == "" || value == "0" {
		return time.Time{}
	}
	millis, err := strconv.ParseInt(value, 10, 64)
	if err != nil || millis <= 0 {
		return time.Time{}
	}
	return time.UnixMilli(millis)
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
