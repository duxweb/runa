package ws

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sort"
	"sync"
	"sync/atomic"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
	"github.com/duxweb/runa/host"
	"github.com/duxweb/runa/route"
	"github.com/duxweb/runa/scope"
)

// Hub manages websocket clients and channels.
type Hub struct {
	name        string
	config      Config
	clients     map[string]*Client
	channels    map[string]map[string]*Client
	handlers    map[string]Handler
	middlewares []Middleware
	mu          sync.RWMutex
	closed      atomic.Bool
	ids         atomic.Uint64
	in          atomic.Uint64
	out         atomic.Uint64
	bytesIn     atomic.Uint64
	bytesOut    atomic.Uint64
	started     atomic.Bool

	onSubscribe SubscribeHook
	onPublish   PublishHook
}

// New creates a websocket hub.
func New(name string, config Config) *Hub {
	if name == "" {
		name = "default"
	}
	hub := &Hub{
		name:     name,
		config:   normalize(config),
		clients:  make(map[string]*Client),
		channels: make(map[string]map[string]*Client),
		handlers: make(map[string]Handler),
	}
	hub.On(EventPing, func(ctx *Context) error { return ctx.Reply(map[string]string{"event": EventPong}) })
	return hub
}

func (hub *Hub) configure(config Config) {
	hub.mu.Lock()
	defer hub.mu.Unlock()
	merged := hub.config
	if len(config.Origin) > 0 {
		merged.Origin = append([]string(nil), config.Origin...)
	}
	if config.MaxMessageSize > 0 {
		merged.MaxMessageSize = config.MaxMessageSize
	}
	if config.ReadTimeout > 0 {
		merged.ReadTimeout = config.ReadTimeout
	}
	if config.WriteTimeout > 0 {
		merged.WriteTimeout = config.WriteTimeout
	}
	if config.PingInterval > 0 {
		merged.PingInterval = config.PingInterval
	}
	if config.PongTimeout > 0 {
		merged.PongTimeout = config.PongTimeout
	}
	if config.SendBuffer > 0 {
		merged.SendBuffer = config.SendBuffer
	}
	if config.Node != "" {
		merged.Node = config.Node
	}
	if config.Auth != nil {
		merged.Auth = config.Auth
	}
	if config.Broker != nil {
		merged.Broker = config.Broker
	}
	if config.Presence != nil {
		merged.Presence = config.Presence
	}
	hub.config = normalize(merged)
}

// Name returns hub name.
func (hub *Hub) Name() string { return hub.name }

// HostName returns host unit name.
func (hub *Hub) HostName() string { return "ws." + hub.name }

// On registers an event handler.
func (hub *Hub) On(event string, handler Handler) *Hub {
	if event == "" || handler == nil {
		return hub
	}
	hub.mu.Lock()
	hub.handlers[event] = handler
	hub.mu.Unlock()
	return hub
}

// OnSubscribe registers subscription hook.
func (hub *Hub) OnSubscribe(fn SubscribeHook) *Hub { hub.onSubscribe = fn; return hub }

// OnPublish registers publish hook.
func (hub *Hub) OnPublish(fn PublishHook) *Hub { hub.onPublish = fn; return hub }

// Use adds websocket event middleware.
func (hub *Hub) Use(middlewares ...Middleware) *Hub {
	if len(middlewares) == 0 {
		return hub
	}
	hub.mu.Lock()
	for _, middleware := range middlewares {
		if middleware != nil {
			hub.middlewares = append(hub.middlewares, middleware)
		}
	}
	hub.mu.Unlock()
	return hub
}

// Serve upgrades HTTP request to websocket and serves messages.
func (hub *Hub) Serve(ctx *route.Context) error {
	if hub.closed.Load() {
		return ctx.Error(http.StatusServiceUnavailable, "websocket hub is closed")
	}
	identity := &Identity{}
	if hub.config.Auth != nil {
		resolved, err := hub.config.Auth(ctx)
		if err != nil {
			return ctx.Error(http.StatusUnauthorized, err)
		}
		if resolved != nil {
			identity = resolved
		}
	}
	conn, err := websocket.Accept(ctx.Response(), ctx.Request(), &websocket.AcceptOptions{OriginPatterns: hub.config.Origin})
	if err != nil {
		return err
	}
	conn.SetReadLimit(hub.config.MaxMessageSize)
	client := newClient(hub, conn, *identity, ctx.IP(), route.Header[string](ctx, "User-Agent"))
	hub.register(client)
	go client.writeLoop()
	hub.readLoop(client)
	return nil
}

func (hub *Hub) readLoop(client *Client) {
	defer client.close(websocket.StatusNormalClosure, "")
	for {
		var message Message
		ctx := context.Background()
		cancel := func() {}
		if hub.config.ReadTimeout > 0 {
			ctx, cancel = context.WithTimeout(ctx, hub.config.ReadTimeout)
		}
		err := wsjson.Read(ctx, client.conn, &message)
		cancel()
		if err != nil {
			return
		}
		if body, err := json.Marshal(message); err == nil {
			hub.bytesIn.Add(uint64(len(body)))
		}
		hub.in.Add(1)
		client.touch()
		hub.handleMessage(client, message)
	}
}

func (hub *Hub) handleMessage(client *Client, message Message) {
	messageScope := scope.New(context.Background(), scope.WS)
	defer messageScope.Close()
	ctx := &Context{hub: hub, client: client, message: message, scope: messageScope}
	call := hub.eventHandler(message.Event)
	err := call(ctx)
	if message.ID == "" {
		return
	}
	if err != nil {
		_ = ctx.Error("error", err.Error())
		return
	}
	_ = client.write(Response{ID: message.ID, Event: message.Event, OK: true})
}

func (hub *Hub) eventHandler(event string) Handler {
	hub.mu.RLock()
	handler := hub.handlers[event]
	middlewares := append([]Middleware(nil), hub.middlewares...)
	hub.mu.RUnlock()
	call := Handler(func(ctx *Context) error {
		switch event {
		case EventSubscribe:
			return ctx.Subscribe(ctx.message.Channel)
		case EventUnsubscribe:
			return ctx.Unsubscribe(ctx.message.Channel)
		default:
			if handler == nil {
				return fmt.Errorf("websocket handler %s is not registered", event)
			}
			return handler(ctx)
		}
	})
	for i := len(middlewares) - 1; i >= 0; i-- {
		call = middlewares[i](call)
	}
	return call
}

func (hub *Hub) register(client *Client) {
	hub.mu.Lock()
	hub.clients[client.id] = client
	hub.mu.Unlock()
	_ = hub.config.Presence.Set(context.Background(), client.info())
}

func (hub *Hub) unregister(client *Client) {
	hub.mu.Lock()
	delete(hub.clients, client.id)
	for channel := range client.channels {
		if clients := hub.channels[channel]; clients != nil {
			delete(clients, client.id)
			if len(clients) == 0 {
				delete(hub.channels, channel)
			}
		}
	}
	hub.mu.Unlock()
	_ = hub.config.Presence.Remove(context.Background(), client.id)
}

func (hub *Hub) subscribe(ctx *Context, client *Client, channel string) error {
	if channel == "" {
		return errors.New("channel is required")
	}
	if ctx != nil && hub.onSubscribe != nil {
		if err := hub.onSubscribe(ctx, channel); err != nil {
			return err
		}
	}
	hub.mu.Lock()
	if hub.channels[channel] == nil {
		hub.channels[channel] = make(map[string]*Client)
	}
	hub.channels[channel][client.id] = client
	client.mu.Lock()
	client.channels[channel] = struct{}{}
	client.mu.Unlock()
	hub.mu.Unlock()
	return hub.config.Presence.Set(context.Background(), client.info())
}

func (hub *Hub) unsubscribe(client *Client, channel string) error {
	hub.mu.Lock()
	if clients := hub.channels[channel]; clients != nil {
		delete(clients, client.id)
		if len(clients) == 0 {
			delete(hub.channels, channel)
		}
	}
	client.mu.Lock()
	delete(client.channels, channel)
	client.mu.Unlock()
	hub.mu.Unlock()
	return hub.config.Presence.Set(context.Background(), client.info())
}

// Publish publishes an event to a channel.
func (hub *Hub) Publish(channel string, event string, data any) error {
	return hub.PublishContext(context.Background(), channel, event, data)
}

// PublishContext publishes an event to a channel.
func (hub *Hub) PublishContext(ctx context.Context, channel string, event string, data any) error {
	if hub.onPublish != nil {
		if err := hub.onPublish(ctx, channel, event, data); err != nil {
			return err
		}
	}
	packet := Packet{Channel: channel, Event: event, Data: data, Origin: hub.config.Node}
	localErr := hub.publishLocal(channel, packet)
	brokerErr := hub.config.Broker.Publish(ctx, channel, packet)
	return errors.Join(localErr, brokerErr)
}

func (hub *Hub) publishLocal(channel string, packet Packet) error {
	hub.mu.RLock()
	clients := make([]*Client, 0, len(hub.channels[channel]))
	for _, client := range hub.channels[channel] {
		clients = append(clients, client)
	}
	hub.mu.RUnlock()
	var joined error
	for _, client := range clients {
		if err := client.write(packet); err != nil {
			joined = errors.Join(joined, err)
		}
	}
	return joined
}

// Broadcast sends an event to all connected clients.
func (hub *Hub) Broadcast(event string, data any) error {
	return hub.BroadcastContext(context.Background(), event, data)
}

// BroadcastContext sends an event to all connected clients.
func (hub *Hub) BroadcastContext(ctx context.Context, event string, data any) error {
	packet := Packet{Event: event, Data: data, Origin: hub.config.Node}
	localErr := hub.broadcastLocal(packet)
	brokerErr := hub.config.Broker.Publish(ctx, "", packet)
	return errors.Join(localErr, brokerErr)
}

func (hub *Hub) broadcastLocal(packet Packet) error {
	hub.mu.RLock()
	clients := make([]*Client, 0, len(hub.clients))
	for _, client := range hub.clients {
		clients = append(clients, client)
	}
	hub.mu.RUnlock()
	var joined error
	for _, client := range clients {
		if err := client.write(packet); err != nil {
			joined = errors.Join(joined, err)
		}
	}
	return joined
}

// Send sends an event to one client.
func (hub *Hub) Send(clientID string, event string, data any) error {
	return hub.SendContext(context.Background(), clientID, event, data)
}

// SendContext sends an event to one client.
func (hub *Hub) SendContext(ctx context.Context, clientID string, event string, data any) error {
	_ = ctx
	hub.mu.RLock()
	client := hub.clients[clientID]
	hub.mu.RUnlock()
	if client == nil {
		return fmt.Errorf("client %s not found", clientID)
	}
	return client.Send(event, data)
}

// SendMany sends an event to many clients.
func (hub *Hub) SendMany(clientIDs []string, event string, data any) error {
	var joined error
	for _, id := range clientIDs {
		if err := hub.Send(id, event, data); err != nil {
			joined = errors.Join(joined, err)
		}
	}
	return joined
}

// Client returns one client info.
func (hub *Hub) Client(id string) (*ClientInfo, bool) {
	hub.mu.RLock()
	client := hub.clients[id]
	hub.mu.RUnlock()
	if client == nil {
		return nil, false
	}
	info := client.info()
	return &info, true
}

// Clients returns online clients.
func (hub *Hub) Clients(filters ...Filter) []ClientInfo {
	items, _ := hub.config.Presence.Clients(context.Background(), filters...)
	return items
}

// Channels returns channel stats.
func (hub *Hub) Channels() []ChannelInfo {
	items, _ := hub.config.Presence.Channels(context.Background())
	return items
}

// Channel returns one channel stats.
func (hub *Hub) Channel(name string) ChannelInfo {
	for _, channel := range hub.Channels() {
		if channel.Name == name {
			return channel
		}
	}
	return ChannelInfo{Name: name}
}

// Stats returns hub runtime stats.
func (hub *Hub) Stats() Stats {
	channels := hub.Channels()
	return Stats{Clients: len(hub.Clients()), Channels: len(channels), MessagesIn: hub.in.Load(), MessagesOut: hub.out.Load(), BytesIn: hub.bytesIn.Load(), BytesOut: hub.bytesOut.Load()}
}

// Kick closes one client.
func (hub *Hub) Kick(clientID string, reason CloseReason) error {
	hub.mu.RLock()
	client := hub.clients[clientID]
	hub.mu.RUnlock()
	if client == nil {
		return nil
	}
	return client.Close(reason)
}

// KickMany closes many clients.
func (hub *Hub) KickMany(clientIDs []string, reason CloseReason) error {
	var joined error
	for _, id := range clientIDs {
		if err := hub.Kick(id, reason); err != nil {
			joined = errors.Join(joined, err)
		}
	}
	return joined
}

// KickChannel closes all clients subscribed to a channel.
func (hub *Hub) KickChannel(channel string, reason CloseReason) error {
	infos := hub.Clients(ByChannel(channel))
	ids := make([]string, 0, len(infos))
	for _, info := range infos {
		ids = append(ids, info.ID)
	}
	return hub.KickMany(ids, reason)
}

// Start subscribes the broker and marks hub active.
func (hub *Hub) Start(ctx context.Context) error {
	hub.closed.Store(false)
	hub.started.Store(true)
	return hub.config.Broker.Subscribe(ctx, hub.config.Node, func(packet Packet) error {
		if packet.Origin == hub.config.Node {
			return nil
		}
		if packet.Channel == "" {
			return hub.broadcastLocal(packet)
		}
		return hub.publishLocal(packet.Channel, packet)
	})
}

// Stop closes all clients and dependencies.
func (hub *Hub) Stop(ctx context.Context) error {
	hub.closed.Store(true)
	hub.started.Store(false)
	hub.mu.RLock()
	clients := make([]*Client, 0, len(hub.clients))
	for _, client := range hub.clients {
		clients = append(clients, client)
	}
	hub.mu.RUnlock()
	for _, client := range clients {
		_ = client.close(websocket.StatusGoingAway, "server shutdown")
	}
	return errors.Join(hub.config.Broker.Close(ctx), hub.config.Presence.Close(ctx))
}

// Status returns host status.
func (hub *Hub) Status() host.Status {
	if !hub.started.Load() {
		return host.Created
	}
	if hub.closed.Load() {
		return host.Stopped
	}
	return host.Running
}

// Check returns host health.
func (hub *Hub) Check(context.Context) host.Health {
	return host.Health{Status: hub.Status(), Message: "ok"}
}

func (hub *Hub) nextID() string {
	return fmt.Sprintf("%s-%d", hub.name, hub.ids.Add(1))
}

func (hub *Hub) statsOut(value any) {
	hub.out.Add(1)
	if body, err := json.Marshal(value); err == nil {
		hub.bytesOut.Add(uint64(len(body)))
	}
}

func sortedChannels(channels map[string]struct{}) []string {
	items := make([]string, 0, len(channels))
	for channel := range channels {
		items = append(items, channel)
	}
	sort.Strings(items)
	return items
}
