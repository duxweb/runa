package redis

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/duxweb/runa"
	"github.com/duxweb/runa/message"
	runaprovider "github.com/duxweb/runa/provider"
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

func TestProviderUsesInjectedClientWithoutClosingIt(t *testing.T) {
	server := miniredis.RunT(t)
	client := goredis.NewClient(&goredis.Options{Addr: server.Addr()})
	app := newMessageTestApp(t, Provider(Client(client), Prefix("provider:message:")))
	ctx := context.Background()
	if err := message.Default().Publish(ctx, message.DefaultBroker, "topic", map[string]int{"id": 1}); err != nil {
		t.Fatalf("publish: %v", err)
	}
	if err := app.Shutdown(ctx); err != nil {
		t.Fatalf("shutdown: %v", err)
	}
	if err := client.Ping(ctx).Err(); err != nil {
		t.Fatalf("injected client should remain open: %v", err)
	}
	_ = client.Close()
}

func TestProviderUsesExplicitOptions(t *testing.T) {
	server := miniredis.RunT(t)
	app := newMessageTestApp(t, Provider(Addr(server.Addr()), Prefix("provider:message:")))
	ctx := context.Background()
	if err := message.Default().Publish(ctx, message.DefaultBroker, "topic", map[string]int{"id": 1}); err != nil {
		t.Fatalf("publish: %v", err)
	}
	if err := app.Shutdown(ctx); err != nil {
		t.Fatalf("shutdown: %v", err)
	}
}

func newMessageTestApp(t *testing.T, provider runaprovider.Provider) *runa.App {
	t.Helper()
	app := runa.New()
	app.Install(
		message.Provider(message.RegisterBroker(message.DefaultBroker, message.Use("redis"))),
		provider,
	)
	if err := app.Freeze(context.Background()); err != nil {
		t.Fatalf("freeze: %v", err)
	}
	return app
}
