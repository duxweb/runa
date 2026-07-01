package message

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"sort"
	"sync"
	"sync/atomic"

	"github.com/duxweb/runa/config"
	"github.com/duxweb/runa/core"
)

type Registry struct {
	mu                 sync.RWMutex
	drivers            map[string]Driver
	brokers            map[string]brokerEntry
	subscriptions      []subscriptionEntry
	active             []Subscription
	middlewares        []Middleware
	publishMiddlewares []PublishMiddleware
	ids                atomic.Uint64
	frozen             bool
}

// New creates a registry.
func New() *Registry {
	registry := &Registry{
		drivers: make(map[string]Driver),
		brokers: make(map[string]brokerEntry),
	}
	registry.RegisterDriver(DefaultDriver, MemoryDriver())
	registry.Broker(DefaultBroker)
	return registry
}

func (registry *Registry) RegisterDriver(name string, driver Driver) {
	if name == "" || driver == nil {
		return
	}
	registry.mu.Lock()
	registry.drivers[name] = driver
	registry.mu.Unlock()
}

func (registry *Registry) Broker(name string, options ...BrokerOption) {
	if name == "" {
		return
	}
	opts := applyBrokerOptions(options...)
	registry.mu.Lock()
	registry.brokers[name] = brokerEntry{name: name, options: opts, code: append([]BrokerOption(nil), options...)}
	registry.mu.Unlock()
}

// Config applies file/env config to already registered brokers.
func (registry *Registry) Config(store *config.Store) {
	registry.mu.Lock()
	defer registry.mu.Unlock()
	for name, entry := range registry.brokers {
		options := append(configOptions(store, name), entry.code...)
		entry.options = applyBrokerOptions(options...)
		registry.brokers[name] = entry
	}
}

func applyBrokerOptions(options ...BrokerOption) BrokerOptions {
	opts := BrokerOptions{Driver: DefaultDriver, Codec: JSONCodec(), Meta: make(core.Map)}
	for _, option := range options {
		if option != nil {
			option.ApplyMessageBroker(&opts)
		}
	}
	if opts.Driver == "" {
		opts.Driver = DefaultDriver
	}
	if opts.Codec == nil {
		opts.Codec = JSONCodec()
	}
	return opts
}

// Use adds broker-level message middleware without replacing existing broker options.
func (registry *Registry) Use(broker string, middlewares ...Middleware) {
	if broker == All {
		registry.mu.Lock()
		for _, middleware := range middlewares {
			if middleware != nil {
				registry.middlewares = append(registry.middlewares, middleware)
			}
		}
		registry.mu.Unlock()
		return
	}
	if broker == "" {
		broker = DefaultBroker
	}
	registry.mu.Lock()
	entry, ok := registry.brokers[broker]
	if ok {
		for _, middleware := range middlewares {
			if middleware != nil {
				entry.options.Middlewares = append(entry.options.Middlewares, middleware)
			}
		}
		registry.brokers[broker] = entry
	}
	registry.mu.Unlock()
}

// UsePublish adds publish middleware without replacing existing broker options.
func (registry *Registry) UsePublish(broker string, middlewares ...PublishMiddleware) {
	if broker == All {
		registry.mu.Lock()
		for _, middleware := range middlewares {
			if middleware != nil {
				registry.publishMiddlewares = append(registry.publishMiddlewares, middleware)
			}
		}
		registry.mu.Unlock()
		return
	}
	if broker == "" {
		broker = DefaultBroker
	}
	registry.mu.Lock()
	entry, ok := registry.brokers[broker]
	if ok {
		for _, middleware := range middlewares {
			if middleware != nil {
				entry.options.PublishMiddlewares = append(entry.options.PublishMiddlewares, middleware)
			}
		}
		registry.brokers[broker] = entry
	}
	registry.mu.Unlock()
}

// OnPublish adds broker-level publish hooks without replacing existing broker options.
func (registry *Registry) OnPublish(broker string, hooks ...PublishHook) {
	if broker == "" {
		broker = DefaultBroker
	}
	registry.mu.Lock()
	entry, ok := registry.brokers[broker]
	if ok {
		for _, hook := range hooks {
			if hook != nil {
				entry.options.OnPublish = append(entry.options.OnPublish, hook)
			}
		}
		registry.brokers[broker] = entry
	}
	registry.mu.Unlock()
}

func (registry *Registry) Subscribe[T any](broker string, topic string, handler Handler[T], options ...SubscribeOption) {
	if topic == "" || handler == nil {
		return
	}
	if broker == "" {
		broker = DefaultBroker
	}
	opts := SubscribeOptions{Meta: make(core.Map)}
	for _, option := range options {
		if option != nil {
			option.ApplyMessageSubscribe(&opts)
		}
	}
	codec := opts.Codec
	payloadType := core.TypeOf[T]()
	entry := subscriptionEntry{
		broker:      broker,
		topic:       topic,
		consumer:    opts.Consumer,
		meta:        core.CloneMap(opts.Meta),
		codec:       codec,
		payloadType: payloadType,
		payloadName: core.TypeName(payloadType),
		call: func(ctx context.Context, message Envelope, codec Codec) error {
			var payload T
			if len(message.Payload) > 0 {
				resolved := codec
				if resolved == nil {
					resolved = JSONCodec()
				}
				if err := resolved.Unmarshal(message.Payload, &payload); err != nil {
					return err
				}
			}
			return handler(ctx, &MessageOf[T]{
				ID:        message.ID,
				Topic:     message.Topic,
				Payload:   payload,
				Headers:   core.CloneMap(message.Headers),
				CreatedAt: message.CreatedAt,
			})
		},
	}
	if entry.consumer == "" {
		entry.consumer = defaultConsumerName(payloadType, len(registry.subscriptions)+1)
	}
	registry.mu.Lock()
	registry.subscriptions = append(registry.subscriptions, entry)
	registry.mu.Unlock()
}

func (registry *Registry) Publish[T any](ctx context.Context, broker string, topic string, payload T, options ...PublishOption) error {
	opts := PublishOptions{Headers: make(core.Map)}
	for _, option := range options {
		if option != nil {
			option.ApplyMessagePublish(&opts)
		}
	}
	return registry.PublishValue(ctx, broker, topic, payload, opts)
}

func (registry *Registry) PublishValue(ctx context.Context, broker string, topic string, payload any, options PublishOptions) error {
	ctx = core.NormalizeContext(ctx)
	if broker == "" {
		broker = DefaultBroker
	}
	if topic == "" {
		return fmt.Errorf("message topic is required")
	}
	entry, driver, err := registry.broker(broker)
	if err != nil {
		return err
	}
	codec := options.Codec
	if codec == nil {
		codec = entry.options.Codec
	}
	if codec == nil {
		codec = JSONCodec()
	}
	body, err := codec.Marshal(payload)
	if err != nil {
		return err
	}
	options.Codec = codec
	return registry.publishMessage(ctx, entry.name, entry.options, driver, topic, body, options)
}

func (registry *Registry) PublishMessage(ctx context.Context, broker string, topic string, payload []byte, options PublishOptions) error {
	ctx = core.NormalizeContext(ctx)
	if broker == "" {
		broker = DefaultBroker
	}
	if topic == "" {
		return fmt.Errorf("message topic is required")
	}
	entry, driver, err := registry.broker(broker)
	if err != nil {
		return err
	}
	if options.Codec == nil {
		options.Codec = entry.options.Codec
	}
	return registry.publishMessage(ctx, entry.name, entry.options, driver, topic, payload, options)
}

func (registry *Registry) publishMessage(ctx context.Context, broker string, brokerOptions BrokerOptions, driver Driver, topic string, payload []byte, options PublishOptions) error {
	headers := core.CloneMap(options.Headers)
	if headers == nil {
		headers = make(core.Map)
	}
	codec := options.Codec
	if codec != nil && headers[HeaderContentType] == nil {
		headers[HeaderContentType] = codec.Name()
	}
	item := Envelope{ID: registry.nextID(), Topic: topic, Payload: append([]byte(nil), payload...), Headers: headers, CreatedAt: core.Now()}
	call := PublishHandlerFunc(func(ctx context.Context, topic string, message Envelope) error {
		return driver.Publish(ctx, topic, message)
	})
	for i := len(brokerOptions.PublishMiddlewares) - 1; i >= 0; i-- {
		call = brokerOptions.PublishMiddlewares[i](call)
	}
	middlewares := registry.publishMiddlewareSnapshot()
	for i := len(middlewares) - 1; i >= 0; i-- {
		call = middlewares[i](call)
	}
	err := call(ctx, topic, item)
	for _, hook := range brokerOptions.OnPublish {
		if hook != nil {
			hook(ctx, PublishEvent{Broker: broker, Topic: topic, Envelope: item, Err: err})
		}
	}
	return err
}

func (registry *Registry) Freeze(ctx context.Context) error {
	ctx = core.NormalizeContext(ctx)
	registry.mu.Lock()
	defer registry.mu.Unlock()
	for name, entry := range registry.brokers {
		if registry.drivers[entry.options.Driver] == nil {
			return fmt.Errorf("message broker %s driver %s is not registered", name, entry.options.Driver)
		}
	}
	seen := make(map[string]struct{}, len(registry.subscriptions))
	for _, entry := range registry.subscriptions {
		broker, ok := registry.brokers[entry.broker]
		if !ok {
			closeSubscriptions(ctx, registry.active)
			registry.active = nil
			return fmt.Errorf("message broker %s is not registered", entry.broker)
		}
		driver := registry.drivers[broker.options.Driver]
		key := entry.broker + "\x00" + entry.topic + "\x00" + entry.consumer
		if _, ok := seen[key]; ok {
			closeSubscriptions(ctx, registry.active)
			registry.active = nil
			return fmt.Errorf("message subscription %s:%s:%s already registered", entry.broker, entry.topic, entry.consumer)
		}
		seen[key] = struct{}{}
		codec := entry.codec
		if codec == nil {
			codec = broker.options.Codec
		}
		call := registry.wrap(entry, broker.options, codec)
		subscription, err := driver.Subscribe(ctx, entry.topic, entry.consumer, call)
		if err != nil {
			closeSubscriptions(ctx, registry.active)
			registry.active = nil
			return err
		}
		if subscription != nil {
			registry.active = append(registry.active, subscription)
		}
	}
	registry.frozen = true
	return nil
}

func closeSubscriptions(ctx context.Context, subscriptions []Subscription) error {
	var err error
	for i := len(subscriptions) - 1; i >= 0; i-- {
		err = errors.Join(err, subscriptions[i].Close(ctx))
	}
	return err
}

func (registry *Registry) Close(ctx context.Context) error {
	ctx = core.NormalizeContext(ctx)
	registry.mu.Lock()
	active := append([]Subscription(nil), registry.active...)
	registry.active = nil
	drivers := make(map[string]Driver, len(registry.drivers))
	for name, driver := range registry.drivers {
		drivers[name] = driver
	}
	registry.mu.Unlock()
	err := closeSubscriptions(ctx, active)
	seen := map[Driver]struct{}{}
	for _, driver := range drivers {
		if driver == nil {
			continue
		}
		if _, ok := seen[driver]; ok {
			continue
		}
		seen[driver] = struct{}{}
		err = errors.Join(err, driver.Close(ctx))
	}
	return err
}

// Shutdown closes subscriptions and broker drivers when managed by DI.
func (registry *Registry) Shutdown(ctx context.Context) error {
	return registry.Close(ctx)
}

func (registry *Registry) Info() []BrokerInfo {
	registry.mu.RLock()
	defer registry.mu.RUnlock()
	counts := make(map[string]int)
	for _, subscription := range registry.subscriptions {
		counts[subscription.broker]++
	}
	items := make([]BrokerInfo, 0, len(registry.brokers))
	for name, entry := range registry.brokers {
		codecName := ""
		if entry.options.Codec != nil {
			codecName = entry.options.Codec.Name()
		}
		items = append(items, BrokerInfo{Name: name, Driver: entry.options.Driver, Codec: codecName, Subscribers: counts[name], Meta: core.CloneMap(entry.options.Meta)})
	}
	sort.SliceStable(items, func(i, j int) bool { return items[i].Name < items[j].Name })
	return items
}

func (registry *Registry) SubscriptionInfo() []SubscriptionInfo {
	registry.mu.RLock()
	defer registry.mu.RUnlock()
	items := make([]SubscriptionInfo, 0, len(registry.subscriptions))
	for _, entry := range registry.subscriptions {
		codecName := ""
		if entry.codec != nil {
			codecName = entry.codec.Name()
		}
		items = append(items, SubscriptionInfo{Broker: entry.broker, Topic: entry.topic, Consumer: entry.consumer, Payload: entry.payloadName, Codec: codecName, Handler: entry.consumer, Meta: core.CloneMap(entry.meta)})
	}
	sort.SliceStable(items, func(i, j int) bool {
		if items[i].Broker == items[j].Broker {
			if items[i].Topic == items[j].Topic {
				return items[i].Consumer < items[j].Consumer
			}
			return items[i].Topic < items[j].Topic
		}
		return items[i].Broker < items[j].Broker
	})
	return items
}

func (registry *Registry) wrap(entry subscriptionEntry, options BrokerOptions, codec Codec) HandlerFunc {
	call := func(ctx context.Context, message Envelope) error {
		if codec != nil {
			message.Headers = core.CloneMap(message.Headers)
			message.Headers[HeaderContentType] = codec.Name()
		}
		return entry.call(ctx, message, codec)
	}
	for i := len(options.Middlewares) - 1; i >= 0; i-- {
		call = options.Middlewares[i](call)
	}
	middlewares := append([]Middleware(nil), registry.middlewares...)
	for i := len(middlewares) - 1; i >= 0; i-- {
		call = middlewares[i](call)
	}
	if options.OnError == nil {
		return call
	}
	return func(ctx context.Context, message Envelope) error {
		err := call(ctx, message)
		if err != nil {
			options.OnError(ctx, Error{Broker: entry.broker, Topic: entry.topic, Consumer: entry.consumer, Envelope: message, Err: err})
		}
		return err
	}
}

func (registry *Registry) broker(name string) (brokerEntry, Driver, error) {
	registry.mu.RLock()
	defer registry.mu.RUnlock()
	entry, ok := registry.brokers[name]
	if !ok {
		return brokerEntry{}, nil, fmt.Errorf("message broker %s is not registered", name)
	}
	driver := registry.drivers[entry.options.Driver]
	if driver == nil {
		return brokerEntry{}, nil, fmt.Errorf("message broker %s driver %s is not registered", name, entry.options.Driver)
	}
	return entry, driver, nil
}

func (registry *Registry) nextID() string {
	return fmt.Sprintf("msg-%d", registry.ids.Add(1))
}

func defaultConsumerName(payload reflect.Type, index int) string {
	name := core.TypeName(payload)
	if name == "" {
		name = "message"
	}
	return fmt.Sprintf("%s#%d", name, index)
}

func (registry *Registry) publishMiddlewareSnapshot() []PublishMiddleware {
	registry.mu.RLock()
	defer registry.mu.RUnlock()
	return append([]PublishMiddleware(nil), registry.publishMiddlewares...)
}
