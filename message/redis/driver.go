package redis

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"

	"github.com/duxweb/runa/message"
	goredis "github.com/redis/go-redis/v9"
)

func Driver(client *goredis.Client, options ...Option) message.Driver {
	opts := defaultOptions()
	for _, option := range options {
		if option != nil {
			option(&opts)
		}
	}
	return &driver{client: client, options: opts}
}

type driver struct {
	client  *goredis.Client
	options options
	mu      sync.Mutex
	pubsubs []*goredis.PubSub
}

func (driver *driver) Publish(ctx context.Context, topic string, item message.Envelope) error {
	if driver.client == nil {
		return fmt.Errorf("redis message client is nil")
	}
	body, err := json.Marshal(item)
	if err != nil {
		return err
	}
	return driver.client.Publish(ctx, driver.topic(topic), body).Err()
}

func (driver *driver) Subscribe(ctx context.Context, topic string, _ string, handler message.HandlerFunc) (message.Subscription, error) {
	if driver.client == nil {
		return nil, fmt.Errorf("redis message client is nil")
	}
	if handler == nil {
		return noopSubscription{}, nil
	}
	ctx, cancel := context.WithCancel(ctx)
	pubsub := driver.subscribe(ctx, topic)
	driver.mu.Lock()
	driver.pubsubs = append(driver.pubsubs, pubsub)
	driver.mu.Unlock()
	go func() {
		defer pubsub.Close()
		for item := range pubsub.Channel() {
			var msg message.Envelope
			if err := json.Unmarshal([]byte(item.Payload), &msg); err == nil {
				_ = handler(ctx, msg)
			}
		}
	}()
	return subscription{cancel: cancel, pubsub: pubsub, driver: driver}, nil
}

func (driver *driver) Close(context.Context) error {
	driver.mu.Lock()
	pubsubs := append([]*goredis.PubSub(nil), driver.pubsubs...)
	driver.pubsubs = nil
	driver.mu.Unlock()
	var err error
	for _, pubsub := range pubsubs {
		if pubsub != nil {
			err = errors.Join(err, pubsub.Close())
		}
	}
	return err
}

func (driver *driver) subscribe(ctx context.Context, topic string) *goredis.PubSub {
	resolved := driver.topic(topic)
	if strings.ContainsAny(topic, "*+#") {
		return driver.client.PSubscribe(ctx, resolved)
	}
	return driver.client.Subscribe(ctx, resolved)
}

func (driver *driver) topic(topic string) string {
	if driver.options.prefix == "" {
		return topic
	}
	return driver.options.prefix + topic
}

func (driver *driver) remove(pubsub *goredis.PubSub) {
	driver.mu.Lock()
	for index, item := range driver.pubsubs {
		if item == pubsub {
			driver.pubsubs = append(driver.pubsubs[:index], driver.pubsubs[index+1:]...)
			break
		}
	}
	driver.mu.Unlock()
}

type subscription struct {
	cancel context.CancelFunc
	pubsub *goredis.PubSub
	driver *driver
}

func (subscription subscription) Close(context.Context) error {
	if subscription.cancel != nil {
		subscription.cancel()
	}
	if subscription.driver != nil && subscription.pubsub != nil {
		subscription.driver.remove(subscription.pubsub)
	}
	if subscription.pubsub != nil {
		if err := subscription.pubsub.Close(); err != nil && !strings.Contains(strings.ToLower(err.Error()), "closed") {
			return err
		}
	}
	return nil
}

type noopSubscription struct{}

func (noopSubscription) Close(context.Context) error { return nil }
