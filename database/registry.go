package database

import "sync"

type entry struct {
	name   string
	driver Driver
	db     Database
	status string
}

// Registry stores named database drivers and runtimes.
type Registry struct {
	items map[string]*entry
	mu    sync.RWMutex
}

// New creates a registry.
func New() *Registry {
	return &Registry{items: make(map[string]*entry)}
}
