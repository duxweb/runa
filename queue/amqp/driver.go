package amqp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/duxweb/runa/core"
	"github.com/duxweb/runa/queue"
	amqp091 "github.com/rabbitmq/amqp091-go"
)

var ErrUnsupported = errors.New("amqp queue driver does not support this operation")

// New creates an AMQP queue driver.
//
// AMQP is intentionally kept in a subpackage so the core queue package does not
// depend on RabbitMQ-specific APIs.
func Driver(conn *amqp091.Connection, items ...Option) queue.Driver {
	opts := options{prefetch: 1}
	for _, item := range items {
		if item != nil {
			item(&opts)
		}
	}
	if opts.prefetch <= 0 {
		opts.prefetch = 1
	}
	return &driver{conn: conn, options: opts, reserved: make(map[string]reserved), consumers: make(map[string]consumer)}
}

type driver struct {
	conn      *amqp091.Connection
	options   options
	mu        sync.Mutex
	reserved  map[string]reserved
	consumers map[string]consumer
	publishMu sync.Mutex
	publish   *amqp091.Channel
}

func (driver *driver) Name() string { return "amqp" }

func (driver *driver) Push(ctx context.Context, queueName string, job *queue.JobMessage) (string, error) {
	if driver.conn == nil {
		return "", fmt.Errorf("amqp connection is nil")
	}
	if job == nil {
		return "", fmt.Errorf("job is required")
	}
	if job.ID == "" {
		job.ID = fmt.Sprintf("amqp-%d", core.Now().UnixNano())
	}
	driver.publishMu.Lock()
	defer driver.publishMu.Unlock()
	ch, err := driver.publishChannel()
	if err != nil {
		return "", err
	}
	name := driver.queueName(queueName)
	if _, err := ch.QueueDeclare(name, true, false, false, false, nil); err != nil {
		driver.resetPublish(ch)
		return "", err
	}
	body, err := json.Marshal(job)
	if err != nil {
		return "", err
	}
	publish := amqp091.Publishing{
		ContentType:  "application/json",
		DeliveryMode: amqp091.Persistent,
		MessageId:    job.ID,
		Timestamp:    core.Now(),
		Body:         body,
	}
	if driver.options.exchange == "" {
		err = ch.PublishWithContext(ctx, "", name, false, false, publish)
	} else {
		err = ch.PublishWithContext(ctx, driver.options.exchange, name, false, false, publish)
	}
	if err != nil {
		driver.resetPublish(ch)
		return "", err
	}
	return job.ID, nil
}

func (driver *driver) Reserve(ctx context.Context, queueName string, limit int, lease time.Duration) ([]*queue.JobMessage, error) {
	if limit <= 0 {
		limit = 1
	}
	consumer, err := driver.consumer(queueName)
	if err != nil {
		return nil, err
	}
	items := make([]*queue.JobMessage, 0, limit)
	timer := time.NewTimer(50 * time.Millisecond)
	defer timer.Stop()
	for len(items) < limit {
		select {
		case delivery, ok := <-consumer.deliveries:
			if !ok {
				return items, nil
			}
			var item queue.JobMessage
			if err := json.Unmarshal(delivery.Body, &item); err != nil {
				_ = delivery.Nack(false, false)
				continue
			}
			if item.ID == "" {
				item.ID = delivery.MessageId
			}
			if item.ID == "" {
				item.ID = fmt.Sprintf("amqp-%d", core.Now().UnixNano())
			}
			if item.RunAt.After(core.Now()) {
				_ = delivery.Nack(false, true)
				continue
			}
			item.Queue = queueName
			item.Attempt++
			if lease > 0 {
				item.ReservedUntil = core.Now().Add(lease)
			}
			driver.mu.Lock()
			driver.reserved[item.ID] = reserved{queue: queueName, delivery: delivery, message: &item}
			driver.mu.Unlock()
			items = append(items, &item)
		case <-timer.C:
			return items, nil
		case <-ctx.Done():
			return items, ctx.Err()
		}
	}
	return items, nil
}

func (driver *driver) Ack(ctx context.Context, queueName string, id string) error {
	item, ok := driver.takeReserved(id)
	if !ok {
		return fmt.Errorf("job %s is not reserved", id)
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	return item.delivery.Ack(false)
}

func (driver *driver) Release(ctx context.Context, queueName string, id string, delay time.Duration, reason string) error {
	item, ok := driver.takeReserved(id)
	if !ok {
		return fmt.Errorf("job %s is not reserved", id)
	}
	message := item.message
	message.RunAt = core.Now().Add(delay)
	message.LastError = reason
	message.ReservedUntil = time.Time{}
	message.UpdatedAt = core.Now()
	if err := item.delivery.Ack(false); err != nil {
		return err
	}
	_, err := driver.Push(ctx, queueName, message)
	return err
}

func (driver *driver) Fail(ctx context.Context, queueName string, id string, reason string) error {
	item, ok := driver.takeReserved(id)
	if !ok {
		return fmt.Errorf("job %s is not reserved", id)
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	return item.delivery.Ack(false)
}

func (driver *driver) Renew(context.Context, string, string, time.Duration) error {
	return ErrUnsupported
}

func (driver *driver) Delete(context.Context, string, string) error {
	return ErrUnsupported
}

func (driver *driver) Count(context.Context, string, queue.JobState) (int64, error) {
	return 0, ErrUnsupported
}

func (driver *driver) List(context.Context, string, queue.JobQuery) ([]*queue.JobMessage, error) {
	return nil, ErrUnsupported
}

func (driver *driver) Close(context.Context) error {
	driver.publishMu.Lock()
	driver.mu.Lock()
	publish := driver.publish
	driver.publish = nil
	consumers := make([]consumer, 0, len(driver.consumers))
	for _, item := range driver.consumers {
		consumers = append(consumers, item)
	}
	driver.consumers = make(map[string]consumer)
	driver.mu.Unlock()
	if publish != nil {
		_ = publish.Close()
	}
	driver.publishMu.Unlock()
	for _, item := range consumers {
		_ = item.channel.Close()
	}
	if driver.conn == nil {
		return nil
	}
	return driver.conn.Close()
}

func (driver *driver) consumer(queueName string) (consumer, error) {
	driver.mu.Lock()
	if item, ok := driver.consumers[queueName]; ok {
		driver.mu.Unlock()
		return item, nil
	}
	if driver.conn == nil {
		driver.mu.Unlock()
		return consumer{}, fmt.Errorf("amqp connection is nil")
	}
	ch, err := driver.conn.Channel()
	if err != nil {
		driver.mu.Unlock()
		return consumer{}, err
	}
	name := driver.queueName(queueName)
	if _, err := ch.QueueDeclare(name, true, false, false, false, nil); err != nil {
		_ = ch.Close()
		driver.mu.Unlock()
		return consumer{}, err
	}
	if err := ch.Qos(driver.options.prefetch, 0, false); err != nil {
		_ = ch.Close()
		driver.mu.Unlock()
		return consumer{}, err
	}
	deliveries, err := ch.Consume(name, "", false, false, false, false, nil)
	if err != nil {
		_ = ch.Close()
		driver.mu.Unlock()
		return consumer{}, err
	}
	item := consumer{channel: ch, deliveries: deliveries}
	driver.consumers[queueName] = item
	driver.mu.Unlock()
	return item, nil
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

func (driver *driver) resetPublish(ch *amqp091.Channel) {
	driver.mu.Lock()
	if driver.publish == ch {
		driver.publish = nil
	}
	driver.mu.Unlock()
	if ch != nil {
		_ = ch.Close()
	}
}

func (driver *driver) takeReserved(id string) (reserved, bool) {
	driver.mu.Lock()
	defer driver.mu.Unlock()
	item, ok := driver.reserved[id]
	if ok {
		delete(driver.reserved, id)
	}
	return item, ok
}

func (driver *driver) queueName(name string) string {
	if driver.options.prefix == "" {
		return name
	}
	return driver.options.prefix + "." + name
}

type consumer struct {
	channel    *amqp091.Channel
	deliveries <-chan amqp091.Delivery
}

type reserved struct {
	queue    string
	delivery amqp091.Delivery
	message  *queue.JobMessage
}
