package log

import (
	"bytes"
	"log/slog"
	"strings"
	"testing"

	runaprovider "github.com/duxweb/runa/provider"
)

func TestChannelUsesRegistry(t *testing.T) {
	var buffer bytes.Buffer
	registry := New()
	registry.Set(Queue, Writer(&buffer, JSON()))

	Channel(registry, Queue).Info("queued")

	body := buffer.String()
	if !strings.Contains(body, `"channel":"queue"`) || !strings.Contains(body, `"msg":"queued"`) {
		t.Fatalf("log = %q", body)
	}
}

func TestChannelFallsBackToDefaultSlog(t *testing.T) {
	previousLogger := slog.Default()
	previousInjector := runaprovider.DefaultInjector()
	defer slog.SetDefault(previousLogger)
	defer runaprovider.SetDefaultInjector(previousInjector)

	var buffer bytes.Buffer
	runaprovider.SetDefaultInjector(nil)
	slog.SetDefault(slog.New(slog.NewTextHandler(&buffer, nil)))

	Channel(nil, Queue).Info("fallback")

	body := buffer.String()
	if strings.Contains(body, "channel=") || !strings.Contains(body, "msg=fallback") {
		t.Fatalf("log = %q", body)
	}
}

func TestChannelFallbackDoesNotDuplicateChannel(t *testing.T) {
	previousLogger := slog.Default()
	previousInjector := runaprovider.DefaultInjector()
	defer slog.SetDefault(previousLogger)
	defer runaprovider.SetDefaultInjector(previousInjector)

	var buffer bytes.Buffer
	runaprovider.SetDefaultInjector(nil)
	slog.SetDefault(slog.New(slog.NewTextHandler(&buffer, nil)).With("channel", DefaultName))

	Channel(struct{}{}, Queue).Info("fallback")

	body := buffer.String()
	if strings.Count(body, "channel=") != 1 || !strings.Contains(body, "channel=default") {
		t.Fatalf("log = %q", body)
	}
}
