package task

import (
	"sync"
	"sync/atomic"
)

// Registry stores task handlers.
type Registry struct {
	mu              sync.RWMutex
	entries         map[string][]entry
	queueDispatcher Dispatcher
	frozen          bool
	ids             atomic.Uint64
}

// New creates a registry.
func New() *Registry {
	return &Registry{entries: make(map[string][]entry)}
}
