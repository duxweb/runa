package redis

import (
	"context"
	"encoding/json"
	"sort"
	"strings"
	"time"

	"github.com/duxweb/runa/ws"
	goredis "github.com/redis/go-redis/v9"
)

const defaultPresencePrefix = "runa:ws:presence:"

// Presence creates Redis-backed websocket presence storage.
func Presence(client *goredis.Client, prefixes ...string) ws.Presence {
	prefix := defaultPresencePrefix
	if len(prefixes) > 0 && prefixes[0] != "" {
		prefix = prefixes[0]
	}
	return &presence{client: client, prefix: prefix, ttl: 90 * time.Second}
}

type presence struct {
	client *goredis.Client
	prefix string
	ttl    time.Duration
}

func (presence *presence) Set(ctx context.Context, client ws.ClientInfo) error {
	if presence.client == nil {
		return nil
	}
	body, err := json.Marshal(client)
	if err != nil {
		return err
	}
	return presence.client.Set(ctx, presence.prefix+client.ID, body, presence.ttl).Err()
}

func (presence *presence) Remove(ctx context.Context, clientID string) error {
	if presence.client == nil {
		return nil
	}
	return presence.client.Del(ctx, presence.prefix+clientID).Err()
}

func (presence *presence) Clients(ctx context.Context, filters ...ws.Filter) ([]ws.ClientInfo, error) {
	if presence.client == nil {
		return nil, nil
	}
	keys, err := presence.client.Keys(ctx, presence.prefix+"*").Result()
	if err != nil {
		return nil, err
	}
	items := make([]ws.ClientInfo, 0, len(keys))
	for _, key := range keys {
		body, err := presence.client.Get(ctx, key).Bytes()
		if err != nil {
			continue
		}
		var client ws.ClientInfo
		if err := json.Unmarshal(body, &client); err != nil {
			continue
		}
		if match(client, filters...) {
			items = append(items, client)
		}
	}
	sort.SliceStable(items, func(i, j int) bool { return items[i].ID < items[j].ID })
	return items, nil
}

func (presence *presence) Channels(ctx context.Context) ([]ws.ChannelInfo, error) {
	clients, err := presence.Clients(ctx)
	if err != nil {
		return nil, err
	}
	counts := make(map[string]int)
	for _, client := range clients {
		for _, channel := range client.Channels {
			counts[channel]++
		}
	}
	items := make([]ws.ChannelInfo, 0, len(counts))
	for name, count := range counts {
		items = append(items, ws.ChannelInfo{Name: name, Clients: count})
	}
	sort.SliceStable(items, func(i, j int) bool { return strings.Compare(items[i].Name, items[j].Name) < 0 })
	return items, nil
}

func (presence *presence) Close(context.Context) error { return nil }

func match(client ws.ClientInfo, filters ...ws.Filter) bool {
	for _, filter := range filters {
		if filter != nil && !filter(client) {
			return false
		}
	}
	return true
}
