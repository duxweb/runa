package ws

import (
	"context"
	"sync"
)

// NewMemoryBroker creates an in-process broker.
func NewMemoryBroker() Broker {
	return &memoryBroker{subscribers: make(map[string][]func(Packet) error)}
}

type memoryBroker struct {
	mu          sync.RWMutex
	subscribers map[string][]func(Packet) error
}

func (broker *memoryBroker) Publish(_ context.Context, channel string, packet Packet) error {
	broker.mu.RLock()
	callbacks := []func(Packet) error{}
	for _, items := range broker.subscribers {
		callbacks = append(callbacks, items...)
	}
	broker.mu.RUnlock()
	for _, callback := range callbacks {
		if callback != nil {
			if err := callback(packet); err != nil {
				return err
			}
		}
	}
	return nil
}

func (broker *memoryBroker) Subscribe(_ context.Context, node string, fn func(Packet) error) error {
	broker.mu.Lock()
	broker.subscribers[node] = append(broker.subscribers[node], fn)
	broker.mu.Unlock()
	return nil
}

func (broker *memoryBroker) Close(context.Context) error { return nil }
