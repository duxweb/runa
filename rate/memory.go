package rate

import (
	"context"
	"math"
	"sync"
	"time"

	"github.com/duxweb/runa/core"
)

type memoryDriver struct {
	options DriverOptions
	items   map[string]*memoryItem
	nextGC  time.Time
	mu      sync.Mutex
}

type memoryItem struct {
	count       int
	windowStart time.Time
	previous    int
	tokens      float64
	last        time.Time
	resetAt     time.Time
	expiresAt   time.Time
}

// MemoryDriver creates a single-process rate driver.
func MemoryDriver(options ...DriverOption) Driver {
	opts := applyDriverOptions(options...)
	opts.Name = DefaultDriver
	return &memoryDriver{options: opts, items: make(map[string]*memoryItem)}
}

func (driver *memoryDriver) Name() string { return driver.options.Name }

func (driver *memoryDriver) Allow(_ context.Context, rule Rule, key string) (Result, error) {
	rule = normalizeRule(rule)
	now := time.Now()
	driver.mu.Lock()
	defer driver.mu.Unlock()
	driver.cleanupLocked(now)
	item := driver.items[driver.key(key)]
	if item == nil {
		item = &memoryItem{tokens: float64(rule.Burst), last: now, windowStart: now, resetAt: now.Add(rule.Window)}
		driver.items[driver.key(key)] = item
	}
	var result Result
	switch rule.Algorithm {
	case AlgorithmFixedWindow:
		result = driver.fixedWindow(rule, item, now)
	case AlgorithmSlidingWindow:
		result = driver.slidingWindow(rule, item, now)
	default:
		result = driver.tokenBucket(rule, item, now)
	}
	item.expiresAt = now.Add(memoryItemTTL(rule))
	return result, nil
}

func (driver *memoryDriver) Reset(_ context.Context, _ Rule, key string) error {
	driver.mu.Lock()
	delete(driver.items, driver.key(key))
	driver.mu.Unlock()
	return nil
}

func (driver *memoryDriver) Close(context.Context) error { return nil }

func (driver *memoryDriver) fixedWindow(rule Rule, item *memoryItem, now time.Time) Result {
	if item.windowStart.IsZero() || now.Sub(item.windowStart) >= rule.Window {
		item.windowStart = now
		item.count = 0
		item.resetAt = now.Add(rule.Window)
	}
	allowed := item.count < rule.Limit
	if allowed {
		item.count++
	}
	remaining := rule.Limit - item.count
	if remaining < 0 {
		remaining = 0
	}
	return Result{Allowed: allowed, Limit: rule.Limit, Remaining: remaining, ResetAt: core.In(item.resetAt), RetryAfter: retryAfter(allowed, item.resetAt, now)}
}

func (driver *memoryDriver) slidingWindow(rule Rule, item *memoryItem, now time.Time) Result {
	if item.windowStart.IsZero() {
		item.windowStart = now
		item.resetAt = now.Add(rule.Window)
	}
	elapsed := now.Sub(item.windowStart)
	if elapsed >= rule.Window {
		windows := int(elapsed / rule.Window)
		if windows == 1 {
			item.previous = item.count
		} else {
			item.previous = 0
		}
		item.count = 0
		item.windowStart = item.windowStart.Add(time.Duration(windows) * rule.Window)
		item.resetAt = item.windowStart.Add(rule.Window)
		elapsed = now.Sub(item.windowStart)
	}
	weight := 1 - float64(elapsed)/float64(rule.Window)
	estimated := int(math.Ceil(float64(item.previous)*weight)) + item.count
	allowed := estimated < rule.Limit
	if allowed {
		item.count++
		estimated++
	}
	remaining := rule.Limit - estimated
	if remaining < 0 {
		remaining = 0
	}
	return Result{Allowed: allowed, Limit: rule.Limit, Remaining: remaining, ResetAt: core.In(item.resetAt), RetryAfter: retryAfter(allowed, item.resetAt, now)}
}

func (driver *memoryDriver) tokenBucket(rule Rule, item *memoryItem, now time.Time) Result {
	if item.last.IsZero() {
		item.last = now
		item.tokens = float64(rule.Burst)
	}
	ratePerNano := float64(rule.Limit) / float64(rule.Window)
	item.tokens += float64(now.Sub(item.last)) * ratePerNano
	if item.tokens > float64(rule.Burst) {
		item.tokens = float64(rule.Burst)
	}
	item.last = now
	allowed := item.tokens >= 1
	if allowed {
		item.tokens--
	}
	remaining := int(math.Floor(item.tokens))
	if remaining < 0 {
		remaining = 0
	}
	missing := 1 - item.tokens
	wait := time.Duration(0)
	if !allowed && ratePerNano > 0 {
		wait = time.Duration(missing / ratePerNano)
	}
	resetAt := now.Add(wait)
	if allowed {
		resetAt = now.Add(time.Duration((float64(rule.Burst) - item.tokens) / ratePerNano))
	}
	return Result{Allowed: allowed, Limit: rule.Limit, Remaining: remaining, ResetAt: core.In(resetAt), RetryAfter: retryAfter(allowed, resetAt, now)}
}

func retryAfter(allowed bool, resetAt time.Time, now time.Time) time.Duration {
	if allowed {
		return 0
	}
	if resetAt.Before(now) {
		return 0
	}
	return resetAt.Sub(now)
}

func (driver *memoryDriver) key(key string) string { return driver.options.Prefix + key }

func (driver *memoryDriver) cleanupLocked(now time.Time) {
	if !driver.nextGC.IsZero() && now.Before(driver.nextGC) {
		return
	}
	driver.nextGC = now.Add(time.Minute)
	for key, item := range driver.items {
		if item == nil {
			delete(driver.items, key)
			continue
		}
		if !item.expiresAt.IsZero() && now.After(item.expiresAt) {
			delete(driver.items, key)
		}
	}
}

func memoryItemTTL(rule Rule) time.Duration {
	ttl := rule.Window * 2
	if ttl < time.Minute {
		return time.Minute
	}
	return ttl
}
