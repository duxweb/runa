package core

import (
	"sync"
	"time"
)

// Clock returns the current time.
type Clock interface {
	Now() time.Time
}

type systemClock struct{}

func (systemClock) Now() time.Time { return time.Now() }

// FixedClock always returns the same time.
type FixedClock time.Time

// Now returns the fixed time.
func (clock FixedClock) Now() time.Time { return time.Time(clock) }

var defaultClock = struct {
	mu       sync.RWMutex
	clock    Clock
	location *time.Location
}{
	clock:    systemClock{},
	location: time.Local,
}

// Now returns the current wall-clock time in the application timezone.
func Now() time.Time {
	defaultClock.mu.RLock()
	clock := defaultClock.clock
	location := defaultClock.location
	defaultClock.mu.RUnlock()
	if clock == nil {
		clock = systemClock{}
	}
	if location == nil {
		location = time.Local
	}
	return clock.Now().In(location)
}

// Location returns the application timezone.
func Location() *time.Location {
	defaultClock.mu.RLock()
	location := defaultClock.location
	defaultClock.mu.RUnlock()
	if location == nil {
		return time.Local
	}
	return location
}

// SetLocation sets the application timezone.
func SetLocation(location *time.Location) {
	if location == nil {
		location = time.Local
	}
	defaultClock.mu.Lock()
	defaultClock.location = location
	defaultClock.mu.Unlock()
}

// SetClock overrides the application clock. Passing nil restores the system clock.
func SetClock(clock Clock) {
	if clock == nil {
		clock = systemClock{}
	}
	defaultClock.mu.Lock()
	defaultClock.clock = clock
	defaultClock.mu.Unlock()
}

// In converts t to the application timezone.
func In(t time.Time) time.Time {
	return t.In(Location())
}

// Parse parses a time value in the application timezone.
func Parse(layout string, value string) (time.Time, error) {
	return time.ParseInLocation(layout, value, Location())
}
