package mqtt

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"time"

	"github.com/duxweb/runa/message"
	paho "github.com/eclipse/paho.mqtt.golang"
)

func Driver(client paho.Client, options ...Option) message.Driver {
	opts := defaultOptions()
	for _, option := range options {
		if option != nil {
			option(&opts)
		}
	}
	return &driver{client: client, options: opts}
}

type driver struct {
	client  paho.Client
	options options
	mu      sync.Mutex
	topics  map[string]struct{}
}

func (driver *driver) Publish(ctx context.Context, topic string, item message.Envelope) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if driver.client == nil {
		return errors.New("mqtt client is nil")
	}
	body, err := json.Marshal(item)
	if err != nil {
		return err
	}
	return wait(ctx, driver.client.Publish(driver.topic(topic), driver.options.qos, driver.options.retained, body), driver.options.timeout)
}

func (driver *driver) Subscribe(ctx context.Context, topic string, _ string, handler message.HandlerFunc) (message.Subscription, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if driver.client == nil {
		return nil, errors.New("mqtt client is nil")
	}
	if handler == nil {
		return noopSubscription{}, nil
	}
	resolved := driver.topic(topic)
	token := driver.client.Subscribe(resolved, driver.options.qos, func(_ paho.Client, msg paho.Message) {
		var item message.Envelope
		if err := json.Unmarshal(msg.Payload(), &item); err == nil {
			_ = handler(ctx, item)
		}
	})
	if err := wait(ctx, token, driver.options.timeout); err != nil {
		return nil, err
	}
	driver.mu.Lock()
	if driver.topics == nil {
		driver.topics = make(map[string]struct{})
	}
	driver.topics[resolved] = struct{}{}
	driver.mu.Unlock()
	return driverSubscription{driver: driver, topic: resolved, timeout: driver.options.timeout}, nil
}

func (driver *driver) Close(ctx context.Context) error {
	driver.mu.Lock()
	topics := make([]string, 0, len(driver.topics))
	for topic := range driver.topics {
		topics = append(topics, topic)
	}
	driver.topics = nil
	driver.mu.Unlock()
	var joined error
	if len(topics) > 0 && driver.client != nil {
		joined = errors.Join(joined, wait(ctx, driver.client.Unsubscribe(topics...), driver.options.timeout))
	}
	if driver.client != nil && driver.client.IsConnected() {
		driver.client.Disconnect(driver.options.disconnect)
	}
	return joined
}

func (driver *driver) topic(topic string) string {
	resolved := strings.NewReplacer("**", "#", ".", "/", ":", "/", "*", "+").Replace(topic)
	return driver.options.prefix + resolved
}

func (driver *driver) remove(topic string) {
	driver.mu.Lock()
	delete(driver.topics, topic)
	driver.mu.Unlock()
}

type driverSubscription struct {
	driver  *driver
	topic   string
	timeout time.Duration
}

func (subscription driverSubscription) Close(ctx context.Context) error {
	if subscription.driver == nil || subscription.driver.client == nil || subscription.topic == "" {
		return nil
	}
	subscription.driver.remove(subscription.topic)
	return wait(ctx, subscription.driver.client.Unsubscribe(subscription.topic), subscription.timeout)
}

func wait(ctx context.Context, token paho.Token, timeout time.Duration) error {
	if token == nil {
		return nil
	}
	if timeout > 0 {
		if !token.WaitTimeout(timeout) {
			return context.DeadlineExceeded
		}
	} else {
		token.Wait()
	}
	if err := token.Error(); err != nil {
		return err
	}
	return ctx.Err()
}

type noopSubscription struct{}

func (noopSubscription) Close(context.Context) error { return nil }
