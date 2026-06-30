package redis

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/duxweb/runa/message"
	goredis "github.com/redis/go-redis/v9"
)

func TestRedisDriverPublishSubscribe(t *testing.T) {
	server := miniredis.RunT(t)
	client := goredis.NewClient(&goredis.Options{Addr: server.Addr()})
	driver := Driver(client, Prefix("test:message:"))
	got := make(chan message.Envelope, 1)
	subscription, err := driver.Subscribe(context.Background(), "topic", "consumer", func(ctx context.Context, msg message.Envelope) error {
		got <- msg
		return nil
	})
	if err != nil {
		t.Fatalf("subscribe: %v", err)
	}
	defer subscription.Close(context.Background())
	if err := driver.Publish(context.Background(), "topic", message.Envelope{ID: "1", Topic: "topic", Payload: []byte(`{"ok":true}`)}); err != nil {
		t.Fatalf("publish: %v", err)
	}
	select {
	case msg := <-got:
		if msg.ID != "1" || msg.Topic != "topic" {
			t.Fatalf("msg = %#v", msg)
		}
	case <-time.After(time.Second):
		t.Fatal("message not received")
	}
}

func TestRedisDriverPatternSubscribe(t *testing.T) {
	server := miniredis.RunT(t)
	client := goredis.NewClient(&goredis.Options{Addr: server.Addr()})
	driver := Driver(client, Prefix("test:message:"))
	got := make(chan message.Envelope, 1)
	subscription, err := driver.Subscribe(context.Background(), "user.*", "consumer", func(ctx context.Context, msg message.Envelope) error {
		got <- msg
		return nil
	})
	if err != nil {
		t.Fatalf("subscribe: %v", err)
	}
	defer subscription.Close(context.Background())
	if err := driver.Publish(context.Background(), "user.created", message.Envelope{ID: "1", Topic: "user.created", Payload: []byte(`{"ok":true}`)}); err != nil {
		t.Fatalf("publish: %v", err)
	}
	select {
	case msg := <-got:
		if msg.ID != "1" || msg.Topic != "user.created" {
			t.Fatalf("msg = %#v", msg)
		}
	case <-time.After(time.Second):
		t.Fatal("message not received")
	}
}
