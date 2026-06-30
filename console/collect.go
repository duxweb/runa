package console

import (
	"context"
	"time"

	"github.com/duxweb/runa/core"
	"github.com/duxweb/runa/jsonrpc"
	"github.com/duxweb/runa/message"
	"github.com/duxweb/runa/queue"
	"github.com/duxweb/runa/ws"
)

func installExecutionMonitors(app AppContext, store MonitorStore) {
	if app == nil || store == nil {
		return
	}
	if registry := queueRegistry(app); registry != nil {
		for _, worker := range registry.WorkerInfo(context.Background()) {
			registry.Use(worker.Name, queueMonitor(store))
		}
	}
	if registry := messageRegistry(app); registry != nil {
		for _, broker := range registry.Info() {
			registry.Use(broker.Name, messageConsumeMonitor(store, broker.Name))
			registry.OnPublish(broker.Name, messagePublishMonitor(store))
		}
	}
	for _, server := range jsonrpcServers(app) {
		server.Use(rpcMonitor(store))
	}
	for _, hub := range websocketHubs(app) {
		hub.Use(wsMonitor(store))
	}
}

func queueMonitor(store MonitorStore) queue.Middleware {
	return func(next queue.HandlerFunc) queue.HandlerFunc {
		return func(ctx context.Context, job *queue.JobMessage) error {
			start := time.Now()
			err := next(ctx, job)
			item := JobLog{Time: core.Now(), Queue: job.Queue, Job: job.Name, Attempt: job.Attempt, Latency: time.Since(start), Bytes: len(job.Payload)}
			if err != nil {
				item.Error = err.Error()
			}
			store.RecordJob(item)
			return err
		}
	}
}

func messagePublishMonitor(store MonitorStore) message.PublishHook {
	return func(_ context.Context, event message.PublishEvent) {
		item := MessageLog{Time: event.Envelope.CreatedAt, Broker: event.Broker, Topic: event.Topic, Action: "publish", Bytes: len(event.Envelope.Payload)}
		if item.Time.IsZero() {
			item.Time = core.Now()
		}
		if event.Err != nil {
			item.Error = event.Err.Error()
		}
		store.RecordMessage(item)
	}
}

func messageConsumeMonitor(store MonitorStore, broker string) message.Middleware {
	return func(next message.HandlerFunc) message.HandlerFunc {
		return func(ctx context.Context, item message.Envelope) error {
			start := time.Now()
			err := next(ctx, item)
			log := MessageLog{Time: core.Now(), Broker: broker, Topic: item.Topic, Action: "consume", Latency: time.Since(start), Bytes: len(item.Payload)}
			if err != nil {
				log.Error = err.Error()
			}
			store.RecordMessage(log)
			return err
		}
	}
}

func rpcMonitor(store MonitorStore) jsonrpc.Middleware {
	return func(next jsonrpc.Handler) jsonrpc.Handler {
		return func(ctx *jsonrpc.Context) (any, error) {
			start := time.Now()
			result, err := next(ctx)
			item := RPCLog{Time: core.Now(), Transport: ctx.Transport(), Method: ctx.Method(), Latency: time.Since(start)}
			if err != nil {
				item.Error = err.Error()
			}
			store.RecordRPC(item)
			return result, err
		}
	}
}

func wsMonitor(store MonitorStore) ws.Middleware {
	return func(next ws.Handler) ws.Handler {
		return func(ctx *ws.Context) error {
			start := time.Now()
			err := next(ctx)
			message := ctx.Message()
			item := MessageLog{Time: core.Now(), Broker: "ws", Topic: message.Channel, Consumer: message.Event, Action: "event", Latency: time.Since(start), Bytes: len(message.Data)}
			if err != nil {
				item.Error = err.Error()
			}
			store.RecordMessage(item)
			return err
		}
	}
}

func sampleWS(app AppContext, store MonitorStore) {
	if app == nil || store == nil {
		return
	}
	for _, hub := range websocketHubs(app) {
		stats := hub.Stats()
		store.RecordWS(WSSample{Time: core.Now(), Hub: hub.Name(), Clients: stats.Clients, Channels: stats.Channels, MessagesIn: stats.MessagesIn, MessagesOut: stats.MessagesOut, BytesIn: stats.BytesIn, BytesOut: stats.BytesOut})
	}
}
