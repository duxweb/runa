package ws

import (
	"context"
	"encoding/json"
	"errors"
	"strconv"
	"sync"
	"sync/atomic"

	"github.com/duxweb/runa/core"
	"github.com/duxweb/runa/message"
)

const defaultMessageTopic = "runa.ws"

// MessageBroker adapts a message broker driver for distributed websocket packets.
func MessageBroker(driver message.Driver, topics ...string) Broker {
	topic := defaultMessageTopic
	if len(topics) > 0 && topics[0] != "" {
		topic = topics[0]
	}
	return &messageBroker{driver: driver, topic: topic}
}

type messageBroker struct {
	driver        message.Driver
	topic         string
	ids           atomic.Uint64
	mu            sync.Mutex
	subscriptions []message.Subscription
}

func (broker *messageBroker) Publish(ctx context.Context, channel string, packet Packet) error {
	if broker.driver == nil {
		return errors.New("websocket message broker driver is nil")
	}
	if packet.Channel == "" {
		packet.Channel = channel
	}
	body, err := json.Marshal(packet)
	if err != nil {
		return err
	}
	return broker.driver.Publish(ctx, broker.topic, message.Envelope{
		ID:        broker.nextID(),
		Topic:     broker.topic,
		Payload:   body,
		CreatedAt: core.Now(),
	})
}

func (broker *messageBroker) Subscribe(ctx context.Context, _ string, fn func(Packet) error) error {
	if broker.driver == nil {
		return errors.New("websocket message broker driver is nil")
	}
	if fn == nil {
		return nil
	}
	subscription, err := broker.driver.Subscribe(ctx, broker.topic, "", func(_ context.Context, item message.Envelope) error {
		var packet Packet
		if len(item.Payload) > 0 {
			if err := json.Unmarshal(item.Payload, &packet); err != nil {
				return err
			}
		}
		return fn(packet)
	})
	if err != nil {
		return err
	}
	if subscription != nil {
		broker.mu.Lock()
		broker.subscriptions = append(broker.subscriptions, subscription)
		broker.mu.Unlock()
	}
	return nil
}

func (broker *messageBroker) Close(ctx context.Context) error {
	broker.mu.Lock()
	subscriptions := append([]message.Subscription(nil), broker.subscriptions...)
	broker.subscriptions = nil
	broker.mu.Unlock()
	var joined error
	for i := len(subscriptions) - 1; i >= 0; i-- {
		joined = errors.Join(joined, subscriptions[i].Close(ctx))
	}
	return joined
}

func (broker *messageBroker) nextID() string {
	return "ws-msg-" + strconv.FormatUint(broker.ids.Add(1), 10)
}
