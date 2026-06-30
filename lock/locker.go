package lock

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/duxweb/runa/core"
)

type locker struct {
	name    string
	store   Driver
	options Options
}

func newLocker(name string, store Driver, options Options) Locker {
	return &locker{name: name, store: store, options: normalizeOptions(options)}
}

func (locker *locker) Try(ctx context.Context, key string, options ...LockOption) (Lease, bool, error) {
	ctx = core.NormalizeContext(ctx)
	opts := locker.merge(options...)
	token, err := token()
	if err != nil {
		return nil, false, err
	}
	storeKey := locker.key(key)
	state, ok, err := locker.store.Try(ctx, storeKey, token, opts.TTL)
	if err != nil || !ok {
		return nil, ok, err
	}
	return &lease{
		key:      key,
		storeKey: storeKey,
		token:    state.Token,
		fencing:  state.Fencing,
		store:    locker.store,
		ttl:      opts.TTL,
	}, true, nil
}

func (locker *locker) Wait(ctx context.Context, key string, options ...LockOption) (Lease, error) {
	ctx = core.NormalizeContext(ctx)
	opts := locker.merge(options...)
	waitCtx, cancel := context.WithTimeout(ctx, opts.Wait)
	defer cancel()
	for {
		lease, ok, err := locker.Try(waitCtx, key, options...)
		if err != nil {
			return nil, err
		}
		if ok {
			return lease, nil
		}
		timer := time.NewTimer(opts.RetryInterval)
		select {
		case <-waitCtx.Done():
			timer.Stop()
			return nil, ErrTimeout
		case <-timer.C:
		}
	}
}

func (locker *locker) With(ctx context.Context, key string, fn func(context.Context) error, options ...LockOption) error {
	ctx = core.NormalizeContext(ctx)
	opts := locker.merge(options...)
	lease, err := locker.Wait(ctx, key, options...)
	if err != nil {
		return err
	}

	runCtx := ctx
	var cancel context.CancelFunc
	var stop chan struct{}
	renewErr := make(chan error, 1)
	if opts.AutoRenew {
		runCtx, cancel = context.WithCancel(ctx)
		stop = make(chan struct{})
		go autoRenew(runCtx, lease, opts.TTL, stop, renewErr, cancel)
	}

	runErr := fn(runCtx)
	if cancel != nil {
		cancel()
	}
	if stop != nil {
		close(stop)
	}
	releaseErr := lease.Release(core.DefaultContext())
	select {
	case err := <-renewErr:
		if runErr == nil {
			runErr = err
		}
	default:
	}
	return errors.Join(runErr, releaseErr)
}

func (locker *locker) merge(options ...LockOption) Options {
	opts := locker.options
	opts.Meta = core.CloneMap(locker.options.Meta)
	for _, option := range options {
		if option != nil {
			option.ApplyLock(&opts)
		}
	}
	return normalizeOptions(opts)
}

func (locker *locker) key(key string) string { return locker.options.Prefix + key }

func autoRenew(ctx context.Context, lease Lease, ttl time.Duration, stop <-chan struct{}, renewErr chan<- error, cancel context.CancelFunc) {
	interval := ttl / 2
	if interval <= 0 {
		interval = DefaultTTL / 2
	}
	if interval < 10*time.Millisecond {
		interval = 10 * time.Millisecond
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-stop:
			return
		case <-ticker.C:
			if err := lease.Renew(ctx, ttl); err != nil {
				select {
				case renewErr <- err:
				default:
				}
				cancel()
				return
			}
		}
	}
}

type lease struct {
	key      string
	storeKey string
	token    string
	fencing  uint64
	store    Driver
	ttl      time.Duration
}

func (lease *lease) Key() string     { return lease.key }
func (lease *lease) Token() string   { return lease.token }
func (lease *lease) Fencing() uint64 { return lease.fencing }
func (lease *lease) Renew(ctx context.Context, ttl time.Duration) error {
	if ttl <= 0 {
		ttl = lease.ttl
	}
	return lease.store.Renew(core.NormalizeContext(ctx), lease.storeKey, lease.token, ttl)
}
func (lease *lease) Release(ctx context.Context) error {
	return lease.store.Release(core.NormalizeContext(ctx), lease.storeKey, lease.token)
}

func token() (string, error) {
	var body [16]byte
	if _, err := rand.Read(body[:]); err != nil {
		return "", fmt.Errorf("generate lock token: %w", err)
	}
	return hex.EncodeToString(body[:]), nil
}
