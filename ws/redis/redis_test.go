package redis

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/duxweb/runa/ws"
	goredis "github.com/redis/go-redis/v9"
)

func TestRedisBroker(t *testing.T) {
	server := miniredis.RunT(t)
	client := goredis.NewClient(&goredis.Options{Addr: server.Addr()})
	defer client.Close()
	broker := Broker(client, "test:ws")
	got := make(chan ws.Packet, 1)
	if err := broker.Subscribe(context.Background(), "node", func(packet ws.Packet) error { got <- packet; return nil }); err != nil {
		t.Fatalf("subscribe: %v", err)
	}
	if err := broker.Publish(context.Background(), "room", ws.Packet{Channel: "room", Event: "notice"}); err != nil {
		t.Fatalf("publish: %v", err)
	}
	select {
	case packet := <-got:
		if packet.Event != "notice" || packet.Channel != "room" {
			t.Fatalf("packet = %#v", packet)
		}
	case <-time.After(time.Second):
		t.Fatal("missing packet")
	}
}

func TestRedisPresence(t *testing.T) {
	server := miniredis.RunT(t)
	client := goredis.NewClient(&goredis.Options{Addr: server.Addr()})
	defer client.Close()
	presence := Presence(client, "test:presence:")
	if err := presence.Set(context.Background(), ws.ClientInfo{ID: "1", Channels: []string{"room"}}); err != nil {
		t.Fatalf("set: %v", err)
	}
	clients, err := presence.Clients(context.Background(), ws.ByChannel("room"))
	if err != nil || len(clients) != 1 {
		t.Fatalf("clients=%#v err=%v", clients, err)
	}
	channels, err := presence.Channels(context.Background())
	if err != nil || len(channels) != 1 || channels[0].Name != "room" {
		t.Fatalf("channels=%#v err=%v", channels, err)
	}
	if err := presence.Remove(context.Background(), "1"); err != nil {
		t.Fatalf("remove: %v", err)
	}
	clients, err = presence.Clients(context.Background())
	if err != nil || len(clients) != 0 {
		t.Fatalf("clients after remove=%#v err=%v", clients, err)
	}
}
