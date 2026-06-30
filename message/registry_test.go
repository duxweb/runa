package message

import (
	"context"
	"errors"
	"testing"
	"time"
)

type registryPayload struct {
	Value string `json:"value"`
}

func TestRegistryMemoryPublishSubscribe(t *testing.T) {
	registry := New()
	got := make(chan string, 1)
	registry.Subscribe("default", "topic", func(ctx context.Context, msg *MessageOf[registryPayload]) error {
		got <- msg.Payload.Value
		return nil
	}, Consumer("consumer"))
	if err := registry.Freeze(context.Background()); err != nil {
		t.Fatalf("freeze: %v", err)
	}
	if err := registry.Publish(context.Background(), "default", "topic", registryPayload{Value: "ok"}); err != nil {
		t.Fatalf("publish: %v", err)
	}
	select {
	case value := <-got:
		if value != "ok" {
			t.Fatalf("value = %q", value)
		}
	case <-time.After(time.Second):
		t.Fatal("message not received")
	}
}

func TestRegistryValidatesBrokerDriver(t *testing.T) {
	registry := New()
	registry.Broker("bad", Use("missing"))
	if err := registry.Freeze(context.Background()); err == nil {
		t.Fatal("expected driver error")
	}
}

func TestRegistryTopicPattern(t *testing.T) {
	registry := New()
	got := make(chan string, 1)
	registry.Subscribe("default", "user.*", func(ctx context.Context, msg *MessageOf[registryPayload]) error {
		got <- msg.Topic + ":" + msg.Payload.Value
		return nil
	}, Consumer("patterns"))
	if err := registry.Freeze(context.Background()); err != nil {
		t.Fatalf("freeze: %v", err)
	}
	if err := registry.Publish(context.Background(), "default", "user.created", registryPayload{Value: "ok"}); err != nil {
		t.Fatalf("publish: %v", err)
	}
	select {
	case value := <-got:
		if value != "user.created:ok" {
			t.Fatalf("value = %q", value)
		}
	case <-time.After(time.Second):
		t.Fatal("message not received")
	}
}

func TestRegistryRawCodec(t *testing.T) {
	registry := New()
	registry.Broker("raw", CodecOption(RawCodec()))
	got := make(chan string, 1)
	registry.Subscribe("raw", "topic", func(ctx context.Context, msg *MessageOf[string]) error {
		if msg.Headers[HeaderContentType] != ContentTypeRaw {
			t.Fatalf("headers = %#v", msg.Headers)
		}
		got <- msg.Payload
		return nil
	}, Consumer("raw"))
	if err := registry.Freeze(context.Background()); err != nil {
		t.Fatalf("freeze: %v", err)
	}
	if err := registry.Publish(context.Background(), "raw", "topic", "plain"); err != nil {
		t.Fatalf("publish: %v", err)
	}
	select {
	case value := <-got:
		if value != "plain" {
			t.Fatalf("value = %q", value)
		}
	case <-time.After(time.Second):
		t.Fatal("message not received")
	}
}

func TestRegistryMiddlewareAndErrorHook(t *testing.T) {
	registry := New()
	calls := make(chan string, 2)
	fail := errors.New("boom")
	registry.Broker("events",
		UseMiddleware(func(next HandlerFunc) HandlerFunc {
			return func(ctx context.Context, msg Envelope) error {
				calls <- "middleware:" + msg.Topic
				return next(ctx, msg)
			}
		}),
		OnError(func(ctx context.Context, err Error) {
			if !errors.Is(err.Err, fail) || err.Broker != "events" || err.Topic != "topic" || err.Consumer != "consumer" {
				t.Fatalf("error = %#v", err)
			}
			calls <- "error"
		}),
	)
	registry.Subscribe("events", "topic", func(ctx context.Context, msg *MessageOf[registryPayload]) error {
		return fail
	}, Consumer("consumer"))
	if err := registry.Freeze(context.Background()); err != nil {
		t.Fatalf("freeze: %v", err)
	}
	if err := registry.Publish(context.Background(), "events", "topic", registryPayload{Value: "ok"}); err != nil {
		t.Fatalf("publish: %v", err)
	}
	want := []string{"middleware:topic", "error"}
	for _, expected := range want {
		select {
		case value := <-calls:
			if value != expected {
				t.Fatalf("value = %q expected %q", value, expected)
			}
		case <-time.After(time.Second):
			t.Fatalf("missing call %q", expected)
		}
	}
}
