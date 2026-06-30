package log

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRegistryNamedLoggerAndFallback(t *testing.T) {
	var buffer bytes.Buffer
	registry := New()
	registry.Set(DefaultName, Writer(&buffer, JSON()))
	registry.Get(Queue).Info("queued")
	if !strings.Contains(buffer.String(), `"channel":"queue"`) || !strings.Contains(buffer.String(), `"msg":"queued"`) {
		t.Fatalf("log = %q", buffer.String())
	}
}

func TestRegistryFanoutOutputs(t *testing.T) {
	var first bytes.Buffer
	var second bytes.Buffer
	registry := New()
	registry.Set(HTTP, Writer(&first), Writer(&second))
	registry.Get(HTTP).Info("request")
	if !strings.Contains(first.String(), "request") || !strings.Contains(second.String(), "request") {
		t.Fatalf("first=%q second=%q", first.String(), second.String())
	}
}

func TestPrettyOutputUsesShortLevels(t *testing.T) {
	var buffer bytes.Buffer
	registry := New()
	registry.Set(DefaultName, Writer(&buffer, Pretty()))
	registry.Get(DefaultName).Warn("slow request", "method", "GET")
	body := buffer.String()
	if !strings.Contains(body, "WRN") || !strings.Contains(body, "slow request") || !strings.Contains(body, "method") || !strings.Contains(body, "GET") {
		t.Fatalf("body = %q", body)
	}
}

func TestPrettyOutputCanDisableColor(t *testing.T) {
	var buffer bytes.Buffer
	registry := New()
	registry.Set(DefaultName, Writer(&buffer, Pretty(), Color(false)))
	registry.Get(DefaultName).Error("failed", "err", "broken")
	body := buffer.String()
	if !strings.Contains(body, "ERR") || strings.Contains(body, "\x1b[") {
		t.Fatalf("body = %q", body)
	}
}

func TestFileOutputWritesFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "app.log")
	registry := New()
	registry.Set(DefaultName, File(path, JSON(), Level(slog.LevelInfo)))
	registry.Get(DefaultName).Info("file")
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if !strings.Contains(string(body), `"msg":"file"`) {
		t.Fatalf("body = %q", string(body))
	}
}

type closeBuffer struct {
	bytes.Buffer
	closed int
}

func (buffer *closeBuffer) Close() error {
	buffer.closed++
	return nil
}

func TestRegistryCloseClosesOutputsOnce(t *testing.T) {
	buffer := &closeBuffer{}
	output := newWriterOutput(buffer)
	registry := New()
	registry.Set(DefaultName, output)
	registry.Set(HTTP, output)
	registry.Get(DefaultName).Info("default")
	registry.Get(HTTP).Info("http")
	if err := registry.Close(context.Background()); err != nil {
		t.Fatalf("close: %v", err)
	}
	if buffer.closed != 1 {
		t.Fatalf("close count = %d", buffer.closed)
	}
}

func TestRegistryInfo(t *testing.T) {
	registry := New()
	registry.Set(Redis, Discard())
	infos := registry.Info()
	if len(infos) != 2 {
		t.Fatalf("infos = %#v", infos)
	}
	if infos[0].Name != DefaultName || !infos[0].Default {
		t.Fatalf("default info = %#v", infos)
	}
	if infos[1].Name != Redis || infos[1].Outputs != 1 {
		t.Fatalf("redis info = %#v", infos)
	}
}

func TestRegistryReportsOutputBuildErrors(t *testing.T) {
	previous := os.Stderr
	read, write, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	os.Stderr = write
	defer func() {
		os.Stderr = previous
		_ = read.Close()
		_ = write.Close()
	}()

	registry := New()
	registry.Set(DefaultName, OutputFunc(func(context.Context, string) (slog.Handler, error) {
		return nil, fmt.Errorf("broken output")
	}))
	registry.Get(DefaultName).Info("fallback")

	_ = write.Close()
	body, _ := io.ReadAll(read)
	if !strings.Contains(string(body), "runa log output build failed") || !strings.Contains(string(body), "fallback") {
		t.Fatalf("stderr = %q", string(body))
	}
}
