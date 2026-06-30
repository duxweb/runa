package rbac

import (
	"context"
	"sort"
	"sync"
)

// MemoryStore stores RBAC data in memory.
type MemoryStore struct {
	mu                 sync.RWMutex
	roles              map[string]map[string]struct{}
	rolePermissions    map[string]map[string]struct{}
	subjectPermissions map[string]map[string]struct{}
}

// NewMemoryStore creates an in-memory RBAC store.
func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		roles:              make(map[string]map[string]struct{}),
		rolePermissions:    make(map[string]map[string]struct{}),
		subjectPermissions: make(map[string]map[string]struct{}),
	}
}

// AssignRole grants a role to a subject.
func (store *MemoryStore) AssignRole(subject string, roles ...string) *MemoryStore {
	if store == nil || subject == "" {
		return store
	}
	store.mu.Lock()
	defer store.mu.Unlock()
	values := ensureSet(store.roles, subject)
	for _, role := range roles {
		if role != "" {
			values[role] = struct{}{}
		}
	}
	return store
}

// RevokeRole removes a role from a subject.
func (store *MemoryStore) RevokeRole(subject string, roles ...string) *MemoryStore {
	if store == nil || subject == "" {
		return store
	}
	store.mu.Lock()
	defer store.mu.Unlock()
	removeSetValues(store.roles, subject, roles)
	return store
}

// Grant grants permissions to a role.
func (store *MemoryStore) Grant(role string, permissions ...string) *MemoryStore {
	if store == nil || role == "" {
		return store
	}
	store.mu.Lock()
	defer store.mu.Unlock()
	values := ensureSet(store.rolePermissions, role)
	for _, permission := range permissions {
		if permission != "" {
			values[permission] = struct{}{}
		}
	}
	return store
}

// Revoke removes permissions from a role.
func (store *MemoryStore) Revoke(role string, permissions ...string) *MemoryStore {
	if store == nil || role == "" {
		return store
	}
	store.mu.Lock()
	defer store.mu.Unlock()
	removeSetValues(store.rolePermissions, role, permissions)
	return store
}

// GrantSubject grants direct permissions to a subject.
func (store *MemoryStore) GrantSubject(subject string, permissions ...string) *MemoryStore {
	if store == nil || subject == "" {
		return store
	}
	store.mu.Lock()
	defer store.mu.Unlock()
	values := ensureSet(store.subjectPermissions, subject)
	for _, permission := range permissions {
		if permission != "" {
			values[permission] = struct{}{}
		}
	}
	return store
}

// RevokeSubject removes direct permissions from a subject.
func (store *MemoryStore) RevokeSubject(subject string, permissions ...string) *MemoryStore {
	if store == nil || subject == "" {
		return store
	}
	store.mu.Lock()
	defer store.mu.Unlock()
	removeSetValues(store.subjectPermissions, subject, permissions)
	return store
}

// Roles returns roles assigned to a subject.
func (store *MemoryStore) Roles(_ context.Context, subject string) ([]string, error) {
	if store == nil || subject == "" {
		return nil, nil
	}
	store.mu.RLock()
	defer store.mu.RUnlock()
	return setValues(store.roles[subject]), nil
}

// RolePermissions returns permissions granted to roles.
func (store *MemoryStore) RolePermissions(_ context.Context, roles []string) ([]string, error) {
	if store == nil || len(roles) == 0 {
		return nil, nil
	}
	store.mu.RLock()
	defer store.mu.RUnlock()
	values := make(map[string]struct{})
	for _, role := range roles {
		for permission := range store.rolePermissions[role] {
			values[permission] = struct{}{}
		}
	}
	return setValues(values), nil
}

// SubjectPermissions returns direct permissions granted to a subject.
func (store *MemoryStore) SubjectPermissions(_ context.Context, subject string) ([]string, error) {
	if store == nil || subject == "" {
		return nil, nil
	}
	store.mu.RLock()
	defer store.mu.RUnlock()
	return setValues(store.subjectPermissions[subject]), nil
}

func ensureSet(items map[string]map[string]struct{}, key string) map[string]struct{} {
	values := items[key]
	if values == nil {
		values = make(map[string]struct{})
		items[key] = values
	}
	return values
}

func removeSetValues(items map[string]map[string]struct{}, key string, values []string) {
	set := items[key]
	if set == nil {
		return
	}
	for _, value := range values {
		delete(set, value)
	}
	if len(set) == 0 {
		delete(items, key)
	}
}

func setValues(values map[string]struct{}) []string {
	if len(values) == 0 {
		return nil
	}
	items := make([]string, 0, len(values))
	for value := range values {
		items = append(items, value)
	}
	sort.Strings(items)
	return items
}
