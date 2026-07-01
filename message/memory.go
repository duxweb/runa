package message

import (
	"context"
	"sync"
)

func MemoryDriver() Driver {
	return &memoryDriver{subscribers: make(map[string]map[uint64]HandlerFunc)}
}

type memoryDriver struct {
	mu          sync.RWMutex
	subscribers map[string]map[uint64]HandlerFunc
	ids         uint64
	closed      bool
}

func (driver *memoryDriver) Publish(ctx context.Context, topic string, message Envelope) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	driver.mu.RLock()
	if driver.closed {
		driver.mu.RUnlock()
		return nil
	}
	var callbacks []HandlerFunc
	for pattern, handlers := range driver.subscribers {
		if !MatchTopic(pattern, topic) {
			continue
		}
		for _, handler := range handlers {
			callbacks = append(callbacks, handler)
		}
	}
	driver.mu.RUnlock()
	for _, handler := range callbacks {
		if handler == nil {
			continue
		}
		go func(handler HandlerFunc) {
			_ = handler(ctx, message)
		}(handler)
	}
	return nil
}

func (driver *memoryDriver) Subscribe(ctx context.Context, topic string, _ string, handler HandlerFunc) (Subscription, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if topic == "" || handler == nil {
		return noopSubscription{}, nil
	}
	driver.mu.Lock()
	defer driver.mu.Unlock()
	if driver.closed {
		return noopSubscription{}, nil
	}
	driver.ids++
	id := driver.ids
	if driver.subscribers[topic] == nil {
		driver.subscribers[topic] = make(map[uint64]HandlerFunc)
	}
	driver.subscribers[topic][id] = handler
	return memorySubscription{driver: driver, topic: topic, id: id}, nil
}

func (driver *memoryDriver) Close(context.Context) error {
	driver.mu.Lock()
	driver.closed = true
	driver.subscribers = make(map[string]map[uint64]HandlerFunc)
	driver.mu.Unlock()
	return nil
}

type memorySubscription struct {
	driver *memoryDriver
	topic  string
	id     uint64
}

func (subscription memorySubscription) Close(context.Context) error {
	if subscription.driver == nil {
		return nil
	}
	subscription.driver.mu.Lock()
	if items := subscription.driver.subscribers[subscription.topic]; items != nil {
		delete(items, subscription.id)
		if len(items) == 0 {
			delete(subscription.driver.subscribers, subscription.topic)
		}
	}
	subscription.driver.mu.Unlock()
	return nil
}

type noopSubscription struct{}

func (noopSubscription) Close(context.Context) error { return nil }
