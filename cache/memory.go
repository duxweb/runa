package cache

import (
	"container/list"
	"context"
	"sync"
	"sync/atomic"
	"time"

	"github.com/duxweb/runa/core"
)

// MemoryDriver creates an in-process cache driver.
func MemoryDriver(options ...DriverOption) Driver {
	opts := normalizeDriverOptions(DriverOptions{Name: DefaultDriver})
	for _, option := range options {
		if option != nil {
			option.ApplyDriver(&opts)
		}
	}
	return &memoryStore{options: opts, items: make(map[string]*list.Element), order: list.New()}
}

type memoryStore struct {
	options DriverOptions
	items   map[string]*list.Element
	order   *list.List
	size    int
	mu      sync.Mutex
	hit     atomic.Uint64
	miss    atomic.Uint64
	set     atomic.Uint64
	delete  atomic.Uint64
}

type memoryItem struct {
	key       string
	value     []byte
	size      int
	expiresAt time.Time
}

func (store *memoryStore) Name() string {
	if store.options.Name != "" {
		return store.options.Name
	}
	return DefaultDriver
}

func (store *memoryStore) Get(_ context.Context, key string) ([]byte, bool, error) {
	fullKey := store.key(key)
	now := core.Now()

	store.mu.Lock()
	defer store.mu.Unlock()

	element := store.items[fullKey]
	if element == nil {
		store.miss.Add(1)
		return nil, false, nil
	}
	item := element.Value.(*memoryItem)
	if expired(item, now) {
		store.remove(element)
		store.miss.Add(1)
		return nil, false, nil
	}
	store.order.MoveToFront(element)
	store.hit.Add(1)
	return append([]byte(nil), item.value...), true, nil
}

func (store *memoryStore) GetMany(ctx context.Context, keys []string) (map[string][]byte, []string, error) {
	values := make(map[string][]byte)
	missing := make([]string, 0)
	for _, key := range keys {
		value, ok, err := store.Get(ctx, key)
		if err != nil {
			return nil, nil, err
		}
		if ok {
			values[key] = value
		} else {
			missing = append(missing, key)
		}
	}
	return values, missing, nil
}

func (store *memoryStore) Set(_ context.Context, key string, value []byte, ttl time.Duration) error {
	if ttl <= 0 {
		ttl = store.options.TTL
	}
	fullKey := store.key(key)
	item := &memoryItem{key: fullKey, value: append([]byte(nil), value...), size: len(fullKey) + len(value)}
	if ttl > 0 {
		item.expiresAt = core.Now().Add(ttl)
	}

	store.mu.Lock()
	if element := store.items[fullKey]; element != nil {
		store.remove(element)
	}
	store.items[fullKey] = store.order.PushFront(item)
	store.size += item.size
	store.evict()
	store.mu.Unlock()

	store.set.Add(1)
	return nil
}

func (store *memoryStore) SetMany(ctx context.Context, values map[string][]byte, ttl time.Duration) error {
	for key, value := range values {
		if err := store.Set(ctx, key, value, ttl); err != nil {
			return err
		}
	}
	return nil
}

func (store *memoryStore) Has(ctx context.Context, key string) (bool, error) {
	_, ok, err := store.Get(ctx, key)
	return ok, err
}

func (store *memoryStore) Delete(_ context.Context, keys ...string) error {
	var deleted uint64
	store.mu.Lock()
	for _, key := range keys {
		if element := store.items[store.key(key)]; element != nil {
			store.remove(element)
			deleted++
		}
	}
	store.mu.Unlock()
	if deleted > 0 {
		store.delete.Add(deleted)
	}
	return nil
}

func (store *memoryStore) Purge(context.Context) error {
	store.mu.Lock()
	store.items = make(map[string]*list.Element)
	store.order.Init()
	store.size = 0
	store.mu.Unlock()
	return nil
}

func (store *memoryStore) Close(context.Context) error { return nil }

func (store *memoryStore) Stats(context.Context) Stats {
	store.mu.Lock()
	store.purgeExpired(core.Now())
	size := store.size
	count := store.order.Len()
	store.mu.Unlock()

	meta := core.CloneMap(store.options.Meta)
	if meta == nil {
		meta = make(core.Map)
	}
	meta["bytes"] = size

	return Stats{
		Name:     store.Name(),
		Driver:   store.Name(),
		Hit:      store.hit.Load(),
		Miss:     store.miss.Load(),
		Set:      store.set.Load(),
		Delete:   store.delete.Load(),
		Size:     int64(count),
		Capacity: int64(store.options.Capacity),
		Meta:     meta,
	}
}

func (store *memoryStore) evict() {
	if store.options.Capacity <= 0 {
		return
	}
	for store.size > store.options.Capacity {
		element := store.order.Back()
		if element == nil {
			return
		}
		store.remove(element)
	}
}

func (store *memoryStore) purgeExpired(now time.Time) {
	for element := store.order.Back(); element != nil; {
		previous := element.Prev()
		if expired(element.Value.(*memoryItem), now) {
			store.remove(element)
		}
		element = previous
	}
}

func (store *memoryStore) remove(element *list.Element) {
	item := element.Value.(*memoryItem)
	delete(store.items, item.key)
	store.order.Remove(element)
	store.size -= item.size
}

func expired(item *memoryItem, now time.Time) bool {
	return !item.expiresAt.IsZero() && !now.Before(item.expiresAt)
}

func (store *memoryStore) key(key string) string { return store.options.Prefix + key }
