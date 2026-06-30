package ws

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
	"github.com/duxweb/runa/core"
	"github.com/duxweb/runa/message"
	"github.com/duxweb/runa/route"
	"github.com/duxweb/runa/runtime"
)

func TestHubSubscribePublishAndStats(t *testing.T) {
	hub := New("admin", Config{Auth: func(ctx *route.Context) (*Identity, error) {
		return &Identity{ID: "u1", Name: "Root", Meta: core.Map{"role": "admin"}}, nil
	}})
	app := runtime.New()
	routes := route.New()
	app.Install(route.Provider(route.UseRegistry(routes)), Provider(hub))
	Mount(routes.Group("/ws"), hub)
	if err := app.Freeze(context.Background()); err != nil {
		t.Fatalf("freeze: %v", err)
	}
	server := httptest.NewServer(routes.Handler())
	defer server.Close()

	conn, _, err := websocket.Dial(context.Background(), wsURL(server.URL, "/ws"), nil)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.CloseNow()

	if err := wsjson.Write(context.Background(), conn, Message{ID: "1", Event: EventSubscribe, Channel: "room:1"}); err != nil {
		t.Fatalf("subscribe write: %v", err)
	}
	var subscribed Response
	if err := wsjson.Read(context.Background(), conn, &subscribed); err != nil {
		t.Fatalf("subscribe read: %v", err)
	}
	if !subscribed.OK || subscribed.ID != "1" {
		t.Fatalf("subscribed = %#v", subscribed)
	}
	if channel := hub.Channel("room:1"); channel.Clients != 1 {
		t.Fatalf("channel = %#v", channel)
	}
	if info, ok := hub.Client("u1"); !ok || info.Name != "Root" || len(info.Channels) != 1 {
		t.Fatalf("client = %#v ok=%v", info, ok)
	}
	if err := hub.Publish("room:1", "notice", map[string]string{"text": "hello"}); err != nil {
		t.Fatalf("publish: %v", err)
	}
	var packet Packet
	if err := wsjson.Read(context.Background(), conn, &packet); err != nil {
		t.Fatalf("packet read: %v", err)
	}
	if packet.Event != "notice" || packet.Channel != "room:1" {
		t.Fatalf("packet = %#v", packet)
	}
	stats := hub.Stats()
	if stats.Clients != 1 || stats.Channels != 1 || stats.MessagesIn == 0 || stats.MessagesOut == 0 {
		t.Fatalf("stats = %#v", stats)
	}
}

func TestHubHandlerAndKick(t *testing.T) {
	hub := New("api", Config{})
	hub.On("echo", func(ctx *Context) error {
		var input struct {
			Text string `json:"text"`
		}
		if err := ctx.Bind(&input); err != nil {
			return err
		}
		return ctx.Reply(map[string]string{"text": input.Text})
	})
	app := runtime.New()
	routes := route.New()
	app.Install(route.Provider(route.UseRegistry(routes)))
	Mount(routes.Group("/ws"), hub)
	if err := app.Freeze(context.Background()); err != nil {
		t.Fatalf("freeze: %v", err)
	}
	server := httptest.NewServer(routes.Handler())
	defer server.Close()
	conn, _, err := websocket.Dial(context.Background(), wsURL(server.URL, "/ws"), nil)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.CloseNow()
	if err := wsjson.Write(context.Background(), conn, Message{ID: "1", Event: "echo", Data: []byte(`{"text":"hi"}`)}); err != nil {
		t.Fatalf("write: %v", err)
	}
	var response Response
	if err := wsjson.Read(context.Background(), conn, &response); err != nil {
		t.Fatalf("read: %v", err)
	}
	if !response.OK || response.ID != "1" {
		t.Fatalf("response = %#v", response)
	}
	clients := hub.Clients()
	if len(clients) != 1 {
		t.Fatalf("clients = %#v", clients)
	}
	if err := hub.Kick(clients[0].ID, CloseReason("test")); err != nil {
		t.Fatalf("kick: %v", err)
	}
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) && len(hub.Clients()) != 0 {
		time.Sleep(10 * time.Millisecond)
	}
	if len(hub.Clients()) != 0 {
		t.Fatalf("clients after kick = %#v", hub.Clients())
	}
}

func TestMemoryPresenceAndBroker(t *testing.T) {
	presence := NewMemoryPresence()
	client := ClientInfo{ID: "1", Channels: []string{"a", "b"}}
	if err := presence.Set(context.Background(), client); err != nil {
		t.Fatalf("set: %v", err)
	}
	clients, err := presence.Clients(context.Background(), ByChannel("a"))
	if err != nil || len(clients) != 1 {
		t.Fatalf("clients=%#v err=%v", clients, err)
	}
	channels, err := presence.Channels(context.Background())
	if err != nil || len(channels) != 2 {
		t.Fatalf("channels=%#v err=%v", channels, err)
	}
	broker := NewMemoryBroker()
	got := make(chan Packet, 1)
	if err := broker.Subscribe(context.Background(), "node", func(packet Packet) error { got <- packet; return nil }); err != nil {
		t.Fatalf("subscribe: %v", err)
	}
	if err := broker.Publish(context.Background(), "a", Packet{Event: "event"}); err != nil {
		t.Fatalf("publish: %v", err)
	}
	select {
	case packet := <-got:
		if packet.Event != "event" {
			t.Fatalf("packet = %#v", packet)
		}
	case <-time.After(time.Second):
		t.Fatal("missing packet")
	}
}

func TestProviderRegistersHostAndCommands(t *testing.T) {
	hub := New("admin", Config{})
	app := runtime.New()
	app.Install(Provider(hub))
	if err := app.Freeze(context.Background()); err != nil {
		t.Fatalf("freeze: %v", err)
	}
	if status := app.HostStatus("ws.admin"); status == "" {
		t.Fatalf("status = %s", status)
	}
}

func TestProviderReadsHubConfig(t *testing.T) {
	root := t.TempDir()
	writeConfig(t, root, "ws.toml", `[hubs.admin]
node = "node-a"
send_buffer = 64
`)
	hub := New("admin", Config{})
	app := runtime.New(runtime.BasePath(root))
	app.Install(Provider(hub))
	if err := app.Freeze(context.Background()); err != nil {
		t.Fatalf("freeze: %v", err)
	}
	if hub.config.Node != "node-a" || hub.config.SendBuffer != 64 {
		t.Fatalf("hub config = %#v", hub.config)
	}
}

func writeConfig(t *testing.T, root string, name string, body string) {
	t.Helper()
	dir := filepath.Join(root, "config")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir config: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}
}

func TestPublishLocalContinuesAfterClientWriteError(t *testing.T) {
	hub := New("test", Config{})
	closed := newClient(hub, nil, Identity{ID: "closed"}, "", "")
	close(closed.done)
	open := newClient(hub, nil, Identity{ID: "open"}, "", "")
	hub.channels["room"] = map[string]*Client{"closed": closed, "open": open}

	err := hub.publishLocal("room", Packet{Channel: "room", Event: "notice"})
	if err == nil {
		t.Fatal("expected joined error")
	}
	select {
	case value := <-open.send:
		packet, ok := value.(Packet)
		if !ok || packet.Event != "notice" {
			t.Fatalf("value = %#v", value)
		}
	default:
		t.Fatal("open client did not receive packet")
	}
}

func TestPublishContextPublishesBrokerWhenLocalFails(t *testing.T) {
	broker := &captureBroker{}
	hub := New("test", Config{Broker: broker})
	closed := newClient(hub, nil, Identity{ID: "closed"}, "", "")
	close(closed.done)
	hub.channels["room"] = map[string]*Client{"closed": closed}

	err := hub.PublishContext(context.Background(), "room", "notice", nil)
	if err == nil {
		t.Fatal("expected local error")
	}
	if broker.count != 1 {
		t.Fatalf("broker count = %d", broker.count)
	}
}

func TestMessageBrokerDeliversPacketsAcrossNodes(t *testing.T) {
	driver := message.MemoryDriver()
	nodeA := MessageBroker(driver, "ws.test")
	nodeB := MessageBroker(driver, "ws.test")
	t.Cleanup(func() {
		_ = nodeA.Close(context.Background())
		_ = nodeB.Close(context.Background())
	})
	got := make(chan Packet, 1)
	if err := nodeB.Subscribe(context.Background(), "node-b", func(packet Packet) error {
		got <- packet
		return nil
	}); err != nil {
		t.Fatalf("subscribe: %v", err)
	}
	if err := nodeA.Publish(context.Background(), "room", Packet{Channel: "room", Event: "notice", Origin: "node-a"}); err != nil {
		t.Fatalf("publish: %v", err)
	}
	select {
	case packet := <-got:
		if packet.Channel != "room" || packet.Event != "notice" || packet.Origin != "node-a" {
			t.Fatalf("packet = %#v", packet)
		}
	case <-time.After(time.Second):
		t.Fatal("missing packet")
	}
}

func TestHubBroadcastPublishesToBroker(t *testing.T) {
	broker := &captureBroker{}
	hub := New("test", Config{Broker: broker, Node: "node-a"})
	if err := hub.BroadcastContext(context.Background(), "notice", nil); err != nil {
		t.Fatalf("broadcast: %v", err)
	}
	if broker.count != 1 {
		t.Fatalf("broker count = %d", broker.count)
	}
	if broker.last.Channel != "" || broker.last.Event != "notice" || broker.last.Origin != "node-a" {
		t.Fatalf("broker packet = %#v", broker.last)
	}
}

func TestHubPublishesAcrossMessageBrokerNodes(t *testing.T) {
	driver := message.MemoryDriver()
	hubA := New("a", Config{Node: "node-a", Broker: MessageBroker(driver, "ws.test")})
	hubB := New("b", Config{Node: "node-b", Broker: MessageBroker(driver, "ws.test")})
	if err := hubA.Start(context.Background()); err != nil {
		t.Fatalf("start hub a: %v", err)
	}
	if err := hubB.Start(context.Background()); err != nil {
		t.Fatalf("start hub b: %v", err)
	}
	t.Cleanup(func() {
		_ = hubA.Stop(context.Background())
		_ = hubB.Stop(context.Background())
	})
	client := newClient(hubB, nil, Identity{ID: "remote"}, "", "")
	if err := client.Subscribe("room"); err != nil {
		t.Fatalf("subscribe client: %v", err)
	}
	if err := hubA.PublishContext(context.Background(), "room", "notice", map[string]string{"text": "hello"}); err != nil {
		t.Fatalf("publish: %v", err)
	}
	select {
	case value := <-client.send:
		packet, ok := value.(Packet)
		if !ok || packet.Channel != "room" || packet.Event != "notice" || packet.Origin != "node-a" {
			t.Fatalf("value = %#v", value)
		}
	case <-time.After(time.Second):
		t.Fatal("remote client did not receive packet")
	}
}

type captureBroker struct {
	count int
	last  Packet
}

func (broker *captureBroker) Publish(_ context.Context, _ string, packet Packet) error {
	broker.count++
	broker.last = packet
	return nil
}

func (broker *captureBroker) Subscribe(context.Context, string, func(Packet) error) error {
	return nil
}

func (broker *captureBroker) Close(context.Context) error { return nil }

func wsURL(base string, path string) string {
	return "ws" + strings.TrimPrefix(base, "http") + path
}

var _ = http.MethodGet
