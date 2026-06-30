package message

import "github.com/duxweb/runa/core"

type BrokerOption interface {
	ApplyMessageBroker(*BrokerOptions)
}

type PublishOption interface {
	ApplyMessagePublish(*PublishOptions)
}

type SubscribeOption interface {
	ApplyMessageSubscribe(*SubscribeOptions)
}

type BrokerOptions struct {
	Driver             string
	Codec              Codec
	Middlewares        []Middleware
	PublishMiddlewares []PublishMiddleware
	OnError            ErrorHandler
	OnPublish          []PublishHook
	Meta               core.Map
}

type PublishOptions struct {
	Headers core.Map
	Codec   Codec
}

type SubscribeOptions struct {
	Consumer string
	Codec    Codec
	Meta     core.Map
}

type brokerOptionFunc func(*BrokerOptions)

func (fn brokerOptionFunc) ApplyMessageBroker(options *BrokerOptions) { fn(options) }

type publishOptionFunc func(*PublishOptions)

func (fn publishOptionFunc) ApplyMessagePublish(options *PublishOptions) { fn(options) }

type subscribeOptionFunc func(*SubscribeOptions)

func (fn subscribeOptionFunc) ApplyMessageSubscribe(options *SubscribeOptions) { fn(options) }

func Use(name string) BrokerOption {
	return brokerOptionFunc(func(options *BrokerOptions) {
		if name != "" {
			options.Driver = name
		}
	})
}

func Header(key string, value any) PublishOption {
	return publishOptionFunc(func(options *PublishOptions) {
		if key == "" {
			return
		}
		if options.Headers == nil {
			options.Headers = make(core.Map)
		}
		options.Headers[key] = value
	})
}

func PublishCodec(codec Codec) PublishOption {
	return publishOptionFunc(func(options *PublishOptions) {
		if codec != nil {
			options.Codec = codec
		}
	})
}

func Consumer(name string) SubscribeOption {
	return subscribeOptionFunc(func(options *SubscribeOptions) {
		if name != "" {
			options.Consumer = name
		}
	})
}

func SubscribeCodec(codec Codec) SubscribeOption {
	return subscribeOptionFunc(func(options *SubscribeOptions) {
		if codec != nil {
			options.Codec = codec
		}
	})
}

func CodecOption(codec Codec) BrokerOption {
	return brokerOptionFunc(func(options *BrokerOptions) {
		if codec != nil {
			options.Codec = codec
		}
	})
}

func UseMiddleware(middlewares ...Middleware) BrokerOption {
	return brokerOptionFunc(func(options *BrokerOptions) {
		for _, middleware := range middlewares {
			if middleware != nil {
				options.Middlewares = append(options.Middlewares, middleware)
			}
		}
	})
}

// UsePublish adds publish middleware.
func UsePublish(middlewares ...PublishMiddleware) BrokerOption {
	return brokerOptionFunc(func(options *BrokerOptions) {
		for _, middleware := range middlewares {
			if middleware != nil {
				options.PublishMiddlewares = append(options.PublishMiddlewares, middleware)
			}
		}
	})
}

func OnError(handler ErrorHandler) BrokerOption {
	return brokerOptionFunc(func(options *BrokerOptions) {
		if handler != nil {
			options.OnError = handler
		}
	})
}

func OnPublish(handler PublishHook) BrokerOption {
	return brokerOptionFunc(func(options *BrokerOptions) {
		if handler != nil {
			options.OnPublish = append(options.OnPublish, handler)
		}
	})
}

func Meta(key string, value any) BrokerOption {
	return brokerOptionFunc(func(options *BrokerOptions) {
		if key == "" {
			return
		}
		if options.Meta == nil {
			options.Meta = make(core.Map)
		}
		options.Meta[key] = value
	})
}

func SubscribeMeta(key string, value any) SubscribeOption {
	return subscribeOptionFunc(func(options *SubscribeOptions) {
		if key == "" {
			return
		}
		if options.Meta == nil {
			options.Meta = make(core.Map)
		}
		options.Meta[key] = value
	})
}
