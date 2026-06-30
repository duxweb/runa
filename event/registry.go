package event

import "sync"

// Registry stores event listeners.
type Registry struct {
	mu         sync.RWMutex
	listeners  map[string][]listenerEntry
	dispatcher AsyncDispatcher
	frozen     bool
}

// New creates a registry.
func New() *Registry {
	return &Registry{listeners: make(map[string][]listenerEntry)}
}
