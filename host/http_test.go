package host

import (
	"context"
	"io"
	"net/http"
	"testing"
	"time"
)

func TestHTTPServerStartAndStop(t *testing.T) {
	server := NewHTTP(HTTPConfig{
		Addr: ":0",
		Handler: http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			_, _ = writer.Write([]byte("ok"))
		}),
		ShutdownTimeout: time.Second,
	})

	if err := server.Start(context.Background()); err != nil {
		t.Fatalf("start: %v", err)
	}
	if server.Status() != Running {
		t.Fatalf("status = %s, want %s", server.Status(), Running)
	}

	response, err := http.Get("http://" + server.Addr())
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	defer response.Body.Close()
	body, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	if string(body) != "ok" {
		t.Fatalf("body = %q, want ok", body)
	}

	if err := server.Stop(context.Background()); err != nil {
		t.Fatalf("stop: %v", err)
	}
	if server.Status() != Stopped {
		t.Fatalf("status = %s, want %s", server.Status(), Stopped)
	}
}

func TestHTTPServerContextCancelStopsServer(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	server := NewHTTP(HTTPConfig{Addr: ":0", ShutdownTimeout: time.Second})

	if err := server.Start(ctx); err != nil {
		t.Fatalf("start: %v", err)
	}
	cancel()

	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if server.Status() == Stopped {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("status = %s, want %s", server.Status(), Stopped)
}

func TestHTTPServerExplicitStopDoesNotNeedContextCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	server := NewHTTP(HTTPConfig{Addr: ":0", ShutdownTimeout: time.Second})

	if err := server.Start(ctx); err != nil {
		t.Fatalf("start: %v", err)
	}
	if err := server.Stop(context.Background()); err != nil {
		t.Fatalf("stop: %v", err)
	}
	if server.Status() != Stopped {
		t.Fatalf("status = %s, want %s", server.Status(), Stopped)
	}
}
