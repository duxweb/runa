package schedule

import "sync"

// Registry stores schedules.
type Registry struct {
	mu      sync.RWMutex
	entries map[string][]entry
	frozen  bool
}

// New creates a registry.
func New() *Registry {
	return &Registry{entries: make(map[string][]entry)}
}
