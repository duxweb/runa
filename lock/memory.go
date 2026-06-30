package lock

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	"github.com/duxweb/runa/core"
)

// MemoryDriver creates a single-process lock driver.
func MemoryDriver(options ...DriverOption) Driver {
	opts := normalizeDriverOptions(DriverOptions{Name: DefaultDriver})
	for _, option := range options {
		if option != nil {
			option.ApplyDriver(&opts)
		}
	}
	return &memoryStore{options: opts, locks: make(map[string]State)}
}

type memoryStore struct {
	mu      sync.Mutex
	options DriverOptions
	locks   map[string]State
	seq     atomic.Uint64
}

func (store *memoryStore) Name() string {
	if store.options.Name != "" {
		return store.options.Name
	}
	return DefaultDriver
}

func (store *memoryStore) Try(ctx context.Context, key string, token string, ttl time.Duration) (State, bool, error) {
	ctx = core.NormalizeContext(ctx)
	select {
	case <-ctx.Done():
		return State{}, false, ctx.Err()
	default:
	}
	now := core.Now()
	store.mu.Lock()
	defer store.mu.Unlock()
	if state, ok := store.locks[key]; ok && state.ExpiresAt.After(now) {
		return State{}, false, nil
	}
	state := State{
		Key:       key,
		Token:     token,
		Fencing:   store.seq.Add(1),
		ExpiresAt: now.Add(ttl),
	}
	store.locks[key] = state
	return state, true, nil
}

func (store *memoryStore) Renew(ctx context.Context, key string, token string, ttl time.Duration) error {
	ctx = core.NormalizeContext(ctx)
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}
	now := core.Now()
	store.mu.Lock()
	defer store.mu.Unlock()
	state, ok := store.locks[key]
	if !ok || state.Token != token || !state.ExpiresAt.After(now) {
		return ErrNotHeld
	}
	state.ExpiresAt = now.Add(ttl)
	store.locks[key] = state
	return nil
}

func (store *memoryStore) Release(ctx context.Context, key string, token string) error {
	ctx = core.NormalizeContext(ctx)
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}
	store.mu.Lock()
	defer store.mu.Unlock()
	state, ok := store.locks[key]
	if !ok || state.Token != token {
		return ErrNotHeld
	}
	delete(store.locks, key)
	return nil
}

func (store *memoryStore) Close(context.Context) error {
	store.mu.Lock()
	store.locks = make(map[string]State)
	store.mu.Unlock()
	return nil
}
