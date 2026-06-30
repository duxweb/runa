package amqp

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/duxweb/runa/message"
	amqp091 "github.com/rabbitmq/amqp091-go"
)

func Driver(conn *amqp091.Connection, options ...Option) message.BrokerDriver {
	opts := defaultOptions()
	for _, option := range options {
		if option != nil {
			option(&opts)
		}
	}
	return &driver{conn: conn, options: opts}
}

type driver struct {
	conn     *amqp091.Connection
	options  options
	mu       sync.Mutex
	publish  *amqp091.Channel
	channels []*amqp091.Channel
}

func (driver *driver) Publish(ctx context.Context, topic string, item message.Envelope) error {
	if driver.conn == nil {
		return fmt.Errorf("amqp connection is nil")
	}
	ch, err := driver.publishChannel()
	if err != nil {
		return err
	}
	if err := ch.ExchangeDeclare(driver.options.exchange, "topic", true, false, false, false, nil); err != nil {
		return err
	}
	body, err := json.Marshal(item)
	if err != nil {
		return err
	}
	return ch.PublishWithContext(ctx, driver.options.exchange, driver.topic(topic), false, false, amqp091.Publishing{ContentType: "application/json", DeliveryMode: amqp091.Persistent, MessageId: item.ID, Timestamp: item.CreatedAt, Body: body})
}

func (driver *driver) Subscribe(ctx context.Context, topic string, consumer string, handler message.HandlerFunc) (message.Subscription, error) {
	if driver.conn == nil {
		return nil, fmt.Errorf("amqp connection is nil")
	}
	if handler == nil {
		return noopSubscription{}, nil
	}
	ch, err := driver.subscribeChannel()
	if err != nil {
		return nil, err
	}
	if err := ch.ExchangeDeclare(driver.options.exchange, "topic", true, false, false, false, nil); err != nil {
		return nil, err
	}
	queue, err := ch.QueueDeclare(consumer, true, consumer == "", false, false, nil)
	if err != nil {
		return nil, err
	}
	if err := ch.QueueBind(queue.Name, driver.topic(topic), driver.options.exchange, false, nil); err != nil {
		return nil, err
	}
	deliveries, err := ch.Consume(queue.Name, "", false, false, false, false, nil)
	if err != nil {
		return nil, err
	}
	go func() {
		for delivery := range deliveries {
			var item message.Envelope
			if err := json.Unmarshal(delivery.Body, &item); err != nil {
				_ = delivery.Nack(false, false)
				continue
			}
			if err := handler(ctx, item); err != nil {
				_ = delivery.Nack(false, true)
				continue
			}
			_ = delivery.Ack(false)
		}
	}()
	return subscription{channel: ch, driver: driver}, nil
}

func (driver *driver) Close(context.Context) error {
	driver.mu.Lock()
	publish := driver.publish
	driver.publish = nil
	channels := append([]*amqp091.Channel(nil), driver.channels...)
	driver.channels = nil
	driver.mu.Unlock()
	if publish != nil {
		_ = publish.Close()
	}
	for _, ch := range channels {
		if ch != nil {
			_ = ch.Close()
		}
	}
	if driver.conn != nil {
		return driver.conn.Close()
	}
	return nil
}

func (driver *driver) publishChannel() (*amqp091.Channel, error) {
	driver.mu.Lock()
	defer driver.mu.Unlock()
	if driver.publish != nil {
		return driver.publish, nil
	}
	if driver.conn == nil {
		return nil, fmt.Errorf("amqp connection is nil")
	}
	ch, err := driver.conn.Channel()
	if err != nil {
		return nil, err
	}
	driver.publish = ch
	return ch, nil
}

func (driver *driver) subscribeChannel() (*amqp091.Channel, error) {
	driver.mu.Lock()
	defer driver.mu.Unlock()
	if driver.conn == nil {
		return nil, fmt.Errorf("amqp connection is nil")
	}
	ch, err := driver.conn.Channel()
	if err != nil {
		return nil, err
	}
	driver.channels = append(driver.channels, ch)
	return ch, nil
}

func (driver *driver) remove(channel *amqp091.Channel) {
	driver.mu.Lock()
	for index, item := range driver.channels {
		if item == channel {
			driver.channels = append(driver.channels[:index], driver.channels[index+1:]...)
			break
		}
	}
	driver.mu.Unlock()
}

func (driver *driver) topic(topic string) string {
	if driver.options.prefix == "" {
		return topic
	}
	return driver.options.prefix + topic
}

type subscription struct {
	channel *amqp091.Channel
	driver  *driver
}

func (subscription subscription) Close(context.Context) error {
	if subscription.driver != nil && subscription.channel != nil {
		subscription.driver.remove(subscription.channel)
	}
	if subscription.channel != nil {
		return subscription.channel.Close()
	}
	return nil
}

type noopSubscription struct{}

func (noopSubscription) Close(context.Context) error { return nil }
