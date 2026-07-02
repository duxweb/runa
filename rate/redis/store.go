package redis

import (
	"context"
	"strconv"
	"sync/atomic"
	"time"

	"github.com/duxweb/runa/core"
	"github.com/duxweb/runa/rate"
	goredis "github.com/redis/go-redis/v9"
)

type redisDriver struct {
	client     *goredis.Client
	options    rate.DriverOptions
	ownsClient bool
	seq        atomic.Uint64
}

// Driver creates a Redis-backed rate driver.
func Driver(client *goredis.Client, options ...rate.DriverOption) rate.Driver {
	opts := rate.DriverOptions{Name: "redis", Prefix: "runa:rate:"}
	for _, option := range options {
		if option != nil {
			option.ApplyDriver(&opts)
		}
	}
	if opts.Name == "" {
		opts.Name = "redis"
	}
	if opts.Prefix == "" {
		opts.Prefix = "runa:rate:"
	}
	return &redisDriver{client: client, options: opts}
}

func newDriver(client *goredis.Client, opts options, ownsClient bool) rate.Driver {
	return &redisDriver{
		client:     client,
		options:    rate.DriverOptions{Name: opts.driverName, Prefix: opts.prefix},
		ownsClient: ownsClient,
	}
}

func (driver *redisDriver) Name() string { return driver.options.Name }

func (driver *redisDriver) Allow(ctx context.Context, rule rate.Rule, key string) (rate.Result, error) {
	ctx = core.NormalizeContext(ctx)
	now := time.Now()
	redisKey := driver.key(key)
	if rule.Algorithm == rate.AlgorithmSlidingWindow {
		return driver.sliding(ctx, redisKey, rule, now)
	}
	if rule.Algorithm == rate.AlgorithmFixedWindow {
		return driver.fixed(ctx, redisKey, rule, now)
	}
	return driver.bucket(ctx, redisKey, rule, now)
}

func (driver *redisDriver) Reset(ctx context.Context, _ rate.Rule, key string) error {
	return driver.client.Del(core.NormalizeContext(ctx), driver.key(key)).Err()
}

func (driver *redisDriver) Close(context.Context) error {
	if driver.client == nil || !driver.ownsClient {
		return nil
	}
	return driver.client.Close()
}

func (driver *redisDriver) fixed(ctx context.Context, key string, rule rate.Rule, now time.Time) (rate.Result, error) {
	windowMS := rule.Window.Milliseconds()
	values, err := fixedScript.Run(ctx, driver.client, []string{key}, rule.Limit, windowMS, now.UnixMilli()).Slice()
	if err != nil {
		return rate.Result{}, err
	}
	return resultFromRedis(values, rule.Limit, now), nil
}

func (driver *redisDriver) sliding(ctx context.Context, key string, rule rate.Rule, now time.Time) (rate.Result, error) {
	windowMS := rule.Window.Milliseconds()
	member := strconv.FormatInt(now.UnixNano(), 10) + ":" + strconv.FormatUint(driver.seq.Add(1), 10)
	values, err := slidingScript.Run(ctx, driver.client, []string{key}, rule.Limit, windowMS, now.UnixMilli(), member).Slice()
	if err != nil {
		return rate.Result{}, err
	}
	return resultFromRedis(values, rule.Limit, now), nil
}

func (driver *redisDriver) bucket(ctx context.Context, key string, rule rate.Rule, now time.Time) (rate.Result, error) {
	windowMS := rule.Window.Milliseconds()
	values, err := bucketScript.Run(ctx, driver.client, []string{key}, rule.Limit, rule.Burst, windowMS, now.UnixMilli()).Slice()
	if err != nil {
		return rate.Result{}, err
	}
	return resultFromRedis(values, rule.Limit, now), nil
}

func resultFromRedis(values []any, limit int, now time.Time) rate.Result {
	allowed := toInt(values, 0) == 1
	remaining := toInt(values, 1)
	resetMS := int64(toInt(values, 2))
	retryMS := int64(toInt(values, 3))
	if remaining < 0 {
		remaining = 0
	}
	return rate.Result{
		Allowed:    allowed,
		Limit:      limit,
		Remaining:  remaining,
		ResetAt:    core.In(time.UnixMilli(resetMS)),
		RetryAfter: time.Duration(retryMS) * time.Millisecond,
	}
}

func toInt(values []any, index int) int {
	if index >= len(values) {
		return 0
	}
	switch value := values[index].(type) {
	case int64:
		return int(value)
	case int:
		return value
	case string:
		parsed, _ := strconv.Atoi(value)
		return parsed
	case []byte:
		parsed, _ := strconv.Atoi(string(value))
		return parsed
	default:
		return 0
	}
}

func (driver *redisDriver) key(key string) string { return driver.options.Prefix + key }

var fixedScript = goredis.NewScript(`
local key = KEYS[1]
local limit = tonumber(ARGV[1])
local window = tonumber(ARGV[2])
local now = tonumber(ARGV[3])
local current = redis.call('INCR', key)
if current == 1 then
  redis.call('PEXPIRE', key, window)
end
local ttl = redis.call('PTTL', key)
if ttl < 0 then ttl = window end
local allowed = 0
if current <= limit then allowed = 1 end
local remaining = limit - current
if remaining < 0 then remaining = 0 end
local retry = 0
if allowed == 0 then retry = ttl end
return {allowed, remaining, now + ttl, retry}
`)

var slidingScript = goredis.NewScript(`
local key = KEYS[1]
local limit = tonumber(ARGV[1])
local window = tonumber(ARGV[2])
local now = tonumber(ARGV[3])
local member = ARGV[4]
redis.call('ZREMRANGEBYSCORE', key, 0, now - window)
local count = redis.call('ZCARD', key)
local allowed = 0
if count < limit then
  allowed = 1
  redis.call('ZADD', key, now, member)
  count = count + 1
end
redis.call('PEXPIRE', key, window)
local oldest = redis.call('ZRANGE', key, 0, 0, 'WITHSCORES')
local reset = now + window
if oldest[2] ~= nil then reset = tonumber(oldest[2]) + window end
local remaining = limit - count
if remaining < 0 then remaining = 0 end
local retry = 0
if allowed == 0 then retry = reset - now end
if retry < 0 then retry = 0 end
return {allowed, remaining, reset, retry}
`)

var bucketScript = goredis.NewScript(`
local key = KEYS[1]
local limit = tonumber(ARGV[1])
local burst = tonumber(ARGV[2])
local window = tonumber(ARGV[3])
local now = tonumber(ARGV[4])
local data = redis.call('HMGET', key, 'tokens', 'last')
local tokens = tonumber(data[1])
local last = tonumber(data[2])
if tokens == nil then tokens = burst end
if last == nil then last = now end
local elapsed = now - last
local refill = elapsed * limit / window
tokens = tokens + refill
if tokens > burst then tokens = burst end
local allowed = 0
if tokens >= 1 then
  allowed = 1
  tokens = tokens - 1
end
redis.call('HMSET', key, 'tokens', tokens, 'last', now)
redis.call('PEXPIRE', key, window)
local remaining = math.floor(tokens)
local retry = 0
if allowed == 0 then
  retry = math.ceil((1 - tokens) * window / limit)
end
local reset = now + retry
if allowed == 1 then
  reset = now + math.ceil((burst - tokens) * window / limit)
end
return {allowed, remaining, reset, retry}
`)
