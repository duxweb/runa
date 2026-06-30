package redis

import (
	messageredis "github.com/duxweb/runa/message/redis"
	"github.com/duxweb/runa/ws"
	goredis "github.com/redis/go-redis/v9"
)

const defaultChannel = "runa:ws"

// Broker creates a Redis Pub/Sub websocket broker.
func Broker(client *goredis.Client, channels ...string) ws.Broker {
	channel := defaultChannel
	if len(channels) > 0 && channels[0] != "" {
		channel = channels[0]
	}
	return ws.MessageBroker(messageredis.Driver(client, messageredis.Prefix("")), channel)
}
