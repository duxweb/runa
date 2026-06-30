package lock

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"
)

func TestMemoryLockTryReleaseAndFencing(t *testing.T) {
	locker := New().MustOf(DefaultName)
	ctx := context.Background()
	lease, ok, err := locker.Try(ctx, "job", TTL(time.Second))
	if err != nil || !ok {
		t.Fatalf("try = %v %v", ok, err)
	}
	if lease.Key() != "job" || lease.Token() == "" || lease.Fencing() == 0 {
		t.Fatalf("lease = %#v", lease)
	}
	if _, ok, err := locker.Try(ctx, "job", TTL(time.Second)); err != nil || ok {
		t.Fatalf("second try ok=%v err=%v", ok, err)
	}
	if err := lease.Release(ctx); err != nil {
		t.Fatalf("release: %v", err)
	}
	lease2, ok, err := locker.Try(ctx, "job", TTL(time.Second))
	if err != nil || !ok {
		t.Fatalf("try after release = %v %v", ok, err)
	}
	if lease2.Fencing() <= lease.Fencing() {
		t.Fatalf("fencing did not increase")
	}
}

func TestWaitBlocksCurrentGoroutineUntilRelease(t *testing.T) {
	locker := New().MustOf(DefaultName)
	ctx := context.Background()
	lease, ok, err := locker.Try(ctx, "same", TTL(time.Second))
	if err != nil || !ok {
		t.Fatalf("try: %v %v", ok, err)
	}
	done := make(chan error, 1)
	go func() {
		waited, err := locker.Wait(ctx, "same", TTL(time.Second), Wait(time.Second), RetryInterval(10*time.Millisecond))
		if err == nil {
			err = waited.Release(ctx)
		}
		done <- err
	}()
	time.Sleep(40 * time.Millisecond)
	select {
	case err := <-done:
		t.Fatalf("wait returned too early: %v", err)
	default:
	}
	if err := lease.Release(ctx); err != nil {
		t.Fatalf("release: %v", err)
	}
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("wait err: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("wait timed out")
	}
}

func TestWaitTimeout(t *testing.T) {
	locker := New().MustOf(DefaultName)
	ctx := context.Background()
	lease, ok, err := locker.Try(ctx, "same", TTL(time.Second))
	if err != nil || !ok {
		t.Fatalf("try: %v %v", ok, err)
	}
	defer lease.Release(ctx)
	_, err = locker.Wait(ctx, "same", TTL(time.Second), Wait(30*time.Millisecond), RetryInterval(5*time.Millisecond))
	if !errors.Is(err, ErrTimeout) {
		t.Fatalf("err = %v", err)
	}
}

func TestWithReleasesAndAutoRenews(t *testing.T) {
	locker := New().MustOf(DefaultName)
	ctx := context.Background()
	var called atomic.Int64
	if err := locker.With(ctx, "renew", func(context.Context) error {
		called.Add(1)
		time.Sleep(170 * time.Millisecond)
		return nil
	}, TTL(80*time.Millisecond), Wait(time.Second), RetryInterval(10*time.Millisecond), AutoRenew(true)); err != nil {
		t.Fatalf("with: %v", err)
	}
	if called.Load() != 1 {
		t.Fatalf("called = %d", called.Load())
	}
	lease, ok, err := locker.Try(ctx, "renew", TTL(time.Second))
	if err != nil || !ok {
		t.Fatalf("try after with = %v %v", ok, err)
	}
	_ = lease.Release(ctx)
}
