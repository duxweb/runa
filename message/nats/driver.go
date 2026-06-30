package nats

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"time"

	"github.com/duxweb/runa/message"
	natsgo "github.com/nats-io/nats.go"
)

func Driver(conn *natsgo.Conn, options ...Option) message.BrokerDriver {
	opts := defaultOptions()
	for _, option := range options {
		if option != nil {
			option(&opts)
		}
	}
	return &driver{conn: conn, options: opts}
}

type driver struct {
	conn          *natsgo.Conn
	options       options
	mu            sync.Mutex
	subscriptions []*natsgo.Subscription
}

func (driver *driver) Publish(ctx context.Context, topic string, item message.Envelope) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if driver.conn == nil {
		return errors.New("nats connection is nil")
	}
	body, err := json.Marshal(item)
	if err != nil {
		return err
	}
	return driver.conn.Publish(driver.subject(topic), body)
}

func (driver *driver) Subscribe(ctx context.Context, topic string, consumer string, handler message.HandlerFunc) (message.Subscription, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if driver.conn == nil {
		return nil, errors.New("nats connection is nil")
	}
	if handler == nil {
		return noopSubscription{}, nil
	}
	callback := func(msg *natsgo.Msg) {
		var item message.Envelope
		if err := json.Unmarshal(msg.Data, &item); err == nil {
			_ = handler(ctx, item)
		}
	}
	subject := driver.subject(topic)
	var subscription *natsgo.Subscription
	var err error
	if consumer != "" {
		subscription, err = driver.conn.QueueSubscribe(subject, consumer, callback)
	} else {
		subscription, err = driver.conn.Subscribe(subject, callback)
	}
	if err != nil {
		return nil, err
	}
	driver.mu.Lock()
	driver.subscriptions = append(driver.subscriptions, subscription)
	driver.mu.Unlock()
	return driverSubscription{driver: driver, subscription: subscription, timeout: driver.options.drainTimeout}, nil
}

func (driver *driver) Close(context.Context) error {
	driver.mu.Lock()
	subscriptions := append([]*natsgo.Subscription(nil), driver.subscriptions...)
	driver.subscriptions = nil
	driver.mu.Unlock()
	var joined error
	for _, subscription := range subscriptions {
		if subscription != nil {
			joined = errors.Join(joined, drain(subscription, driver.options.drainTimeout))
		}
	}
	return joined
}

func (driver *driver) subject(topic string) string {
	return driver.options.prefix + strings.NewReplacer("**", ">", "#", ">", "+", "*", "/", ".", ":", ".").Replace(topic)
}

func (driver *driver) remove(subscription *natsgo.Subscription) {
	driver.mu.Lock()
	for index, item := range driver.subscriptions {
		if item == subscription {
			driver.subscriptions = append(driver.subscriptions[:index], driver.subscriptions[index+1:]...)
			break
		}
	}
	driver.mu.Unlock()
}

type driverSubscription struct {
	driver       *driver
	subscription *natsgo.Subscription
	timeout      time.Duration
}

func (subscription driverSubscription) Close(context.Context) error {
	if subscription.driver != nil && subscription.subscription != nil {
		subscription.driver.remove(subscription.subscription)
	}
	return drain(subscription.subscription, subscription.timeout)
}

func drain(subscription *natsgo.Subscription, timeout time.Duration) error {
	if subscription == nil {
		return nil
	}
	if timeout <= 0 {
		return subscription.Unsubscribe()
	}
	done := make(chan error, 1)
	go func() { done <- subscription.Drain() }()
	select {
	case err := <-done:
		return err
	case <-time.After(timeout):
		return subscription.Unsubscribe()
	}
}

type noopSubscription struct{}

func (noopSubscription) Close(context.Context) error { return nil }
