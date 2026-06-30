package ws

import (
	"context"
	"encoding/json"

	"github.com/duxweb/runa/errs"
	"github.com/duxweb/runa/scope"
)

// Context is one websocket message handling context.
type Context struct {
	hub     *Hub
	client  *Client
	message Message
	scope   *scope.Scope
}

// Hub returns current hub.
func (ctx *Context) Hub() *Hub { return ctx.hub }

// Client returns current client.
func (ctx *Context) Client() *Client { return ctx.client }

// Message returns current message.
func (ctx *Context) Message() Message { return ctx.message }

// Scope returns current message scope.
func (ctx *Context) Scope() *scope.Scope { return ctx.scope }

// Context returns current message context.
func (ctx *Context) Context() context.Context {
	if ctx == nil || ctx.scope == nil {
		return context.Background()
	}
	return ctx.scope.Context()
}

// SetContext replaces current message context.
func (ctx *Context) SetContext(value context.Context) {
	if ctx == nil || ctx.scope == nil {
		return
	}
	ctx.scope.SetContext(value)
}

// Identity returns current client identity.
func (ctx *Context) Identity() Identity { return ctx.client.Identity() }

// Bind decodes message data into output.
func (ctx *Context) Bind(output any) error {
	if len(ctx.message.Data) == 0 || output == nil {
		return nil
	}
	return json.Unmarshal(ctx.message.Data, output)
}

// Subscribe subscribes current client to a channel.
func (ctx *Context) Subscribe(channel string) error {
	return ctx.hub.subscribe(ctx, ctx.client, channel)
}

// Unsubscribe unsubscribes current client from a channel.
func (ctx *Context) Unsubscribe(channel string) error {
	return ctx.hub.unsubscribe(ctx.client, channel)
}

// Publish publishes a packet to a channel.
func (ctx *Context) Publish(channel string, event string, data any) error {
	return ctx.hub.PublishContext(ctx.scope.Context(), channel, event, data)
}

// Send sends an event to current client.
func (ctx *Context) Send(event string, data any) error {
	return ctx.client.Send(event, data)
}

// Can checks metadata permission flag.
func (ctx *Context) Can(name string) error {
	if name == "" {
		return nil
	}
	if ctx.client == nil {
		return errs.New("client is nil")
	}
	permissions, ok := ctx.client.identity.Meta["permissions"].([]string)
	if !ok {
		return nil
	}
	for _, item := range permissions {
		if item == name {
			return nil
		}
	}
	return errs.New("permission denied")
}

// Reply writes a response for current message.
func (ctx *Context) Reply(data any) error {
	return ctx.client.write(Response{ID: ctx.message.ID, Event: ctx.message.Event, OK: true, Data: data})
}

// Error writes an error response for current message.
func (ctx *Context) Error(code string, message string) error {
	return ctx.client.write(Response{ID: ctx.message.ID, Event: ctx.message.Event, OK: false, Code: code, Message: message})
}
