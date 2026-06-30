package auth

import (
	"fmt"
	"sort"
	"sync"
)

// Registry stores named authenticators and default permission checker.
type Registry struct {
	mu      sync.RWMutex
	items   map[string]Authenticator
	checker PermissionChecker
}

// New creates a registry.
func New() *Registry {
	return &Registry{items: make(map[string]Authenticator), checker: DefaultPermissionChecker()}
}

func (registry *Registry) Auth(name string, authenticator Authenticator) {
	if name == "" || authenticator == nil {
		return
	}
	registry.mu.Lock()
	registry.items[name] = authenticator
	registry.mu.Unlock()
}

func (registry *Registry) Of(name string) Authenticator {
	registry.mu.RLock()
	defer registry.mu.RUnlock()
	return registry.items[name]
}

func (registry *Registry) MustOf(name string) Authenticator {
	authenticator := registry.Of(name)
	if authenticator == nil {
		panic(fmt.Errorf("auth %s is not registered", name))
	}
	return authenticator
}

func (registry *Registry) Names() []string {
	registry.mu.RLock()
	defer registry.mu.RUnlock()
	names := make([]string, 0, len(registry.items))
	for name := range registry.items {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func (registry *Registry) Permission(checker PermissionChecker) {
	if checker == nil {
		checker = DefaultPermissionChecker()
	}
	registry.mu.Lock()
	registry.checker = checker
	registry.mu.Unlock()
}

func (registry *Registry) Checker() PermissionChecker {
	registry.mu.RLock()
	defer registry.mu.RUnlock()
	if registry.checker == nil {
		return DefaultPermissionChecker()
	}
	return registry.checker
}
