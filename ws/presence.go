package ws

import (
	"context"
	"sort"
	"sync"
)

// NewMemoryPresence creates in-process presence storage.
func NewMemoryPresence() Presence { return &memoryPresence{clients: make(map[string]ClientInfo)} }

type memoryPresence struct {
	mu      sync.RWMutex
	clients map[string]ClientInfo
}

func (presence *memoryPresence) Set(_ context.Context, client ClientInfo) error {
	presence.mu.Lock()
	presence.clients[client.ID] = client
	presence.mu.Unlock()
	return nil
}

func (presence *memoryPresence) Remove(_ context.Context, clientID string) error {
	presence.mu.Lock()
	delete(presence.clients, clientID)
	presence.mu.Unlock()
	return nil
}

func (presence *memoryPresence) Clients(_ context.Context, filters ...Filter) ([]ClientInfo, error) {
	presence.mu.RLock()
	items := make([]ClientInfo, 0, len(presence.clients))
	for _, client := range presence.clients {
		if matchClient(client, filters...) {
			items = append(items, client)
		}
	}
	presence.mu.RUnlock()
	sort.SliceStable(items, func(i, j int) bool { return items[i].ID < items[j].ID })
	return items, nil
}

func (presence *memoryPresence) Channels(context.Context) ([]ChannelInfo, error) {
	presence.mu.RLock()
	counts := make(map[string]int)
	for _, client := range presence.clients {
		for _, channel := range client.Channels {
			counts[channel]++
		}
	}
	presence.mu.RUnlock()
	items := make([]ChannelInfo, 0, len(counts))
	for name, count := range counts {
		items = append(items, ChannelInfo{Name: name, Clients: count})
	}
	sort.SliceStable(items, func(i, j int) bool { return items[i].Name < items[j].Name })
	return items, nil
}

func (presence *memoryPresence) Close(context.Context) error { return nil }

func matchClient(client ClientInfo, filters ...Filter) bool {
	for _, filter := range filters {
		if filter != nil && !filter(client) {
			return false
		}
	}
	return true
}
