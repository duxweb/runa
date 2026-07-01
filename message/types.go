package message

import (
	"context"
	"time"

	"github.com/duxweb/runa/core"
)

const (
	DefaultDriver = "memory"
	DefaultBroker = "default"
	All           = "*"

	HeaderContentType = "content-type"
	ContentTypeJSON   = "application/json"
	ContentTypeRaw    = "application/octet-stream"
)

type Envelope struct {
	ID        string    `json:"id"`
	Topic     string    `json:"topic"`
	Payload   []byte    `json:"payload"`
	Headers   core.Map  `json:"headers,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

type MessageOf[T any] struct {
	ID        string
	Topic     string
	Payload   T
	Headers   core.Map
	CreatedAt time.Time
}

type Handler[T any] func(ctx context.Context, message *MessageOf[T]) error

type HandlerFunc func(ctx context.Context, message Envelope) error

type Middleware func(HandlerFunc) HandlerFunc

// PublishHandlerFunc publishes one serialized envelope.
type PublishHandlerFunc func(ctx context.Context, topic string, message Envelope) error

// PublishMiddleware wraps message publishing.
type PublishMiddleware func(PublishHandlerFunc) PublishHandlerFunc

type ErrorHandler func(ctx context.Context, err Error)

// PublishHook runs after a publish attempt.
type PublishHook func(ctx context.Context, event PublishEvent)

// PublishEvent describes one publish attempt.
type PublishEvent struct {
	Broker   string
	Topic    string
	Envelope Envelope
	Err      error
}

type Error struct {
	Broker   string
	Topic    string
	Consumer string
	Envelope Envelope
	Err      error
}

type Codec interface {
	Name() string
	Marshal(value any) ([]byte, error)
	Unmarshal(data []byte, output any) error
}

type Driver interface {
	Publish(ctx context.Context, topic string, message Envelope) error
	Subscribe(ctx context.Context, topic string, consumer string, handler HandlerFunc) (Subscription, error)
	Close(ctx context.Context) error
}

type Subscription interface {
	Close(ctx context.Context) error
}

type BrokerInfo struct {
	Name        string
	Driver      string
	Codec       string
	Subscribers int
	Meta        core.Map
}

type SubscriptionInfo struct {
	Broker   string
	Topic    string
	Consumer string
	Payload  string
	Codec    string
	Handler  string
	Meta     core.Map
}
