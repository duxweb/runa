package asset

import iregistry "github.com/duxweb/runa/kernel/registry"

// Registry stores named asset domains.
type Registry struct {
	entries iregistry.Entries[*Set]
}

// New creates a registry.
func New() *Registry {
	return &Registry{entries: iregistry.NewEntries[*Set]("")}
}
