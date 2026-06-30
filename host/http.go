package host

import (
	"context"
	"errors"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/duxweb/runa/core"
)

// HTTPConfig configures an HTTP host unit.
type HTTPConfig struct {
	Name            string
	Addr            string
	Handler         http.Handler
	ReadTimeout     time.Duration
	WriteTimeout    time.Duration
	IdleTimeout     time.Duration
	ShutdownTimeout time.Duration
}

// HTTPServer is a net/http based host unit.
type HTTPServer struct {
	name            string
	addr            string
	handler         http.Handler
	readTimeout     time.Duration
	writeTimeout    time.Duration
	idleTimeout     time.Duration
	shutdownTimeout time.Duration

	mu       sync.Mutex
	server   *http.Server
	listener net.Listener
	status   Status
	err      error
	done     chan struct{}
	watch    chan struct{}
}

// NewHTTP creates an HTTP host unit.
func NewHTTP(config HTTPConfig) *HTTPServer {
	name := config.Name
	if name == "" {
		name = "http"
	}
	addr := config.Addr
	if addr == "" {
		addr = ":8080"
	}
	handler := config.Handler
	if handler == nil {
		handler = http.NotFoundHandler()
	}
	shutdownTimeout := config.ShutdownTimeout
	if shutdownTimeout <= 0 {
		shutdownTimeout = 15 * time.Second
	}
	return &HTTPServer{
		name:            name,
		addr:            addr,
		handler:         handler,
		readTimeout:     config.ReadTimeout,
		writeTimeout:    config.WriteTimeout,
		idleTimeout:     config.IdleTimeout,
		shutdownTimeout: shutdownTimeout,
		status:          Created,
	}
}

// Name returns the host name.
func (server *HTTPServer) Name() string { return server.name }

// Addr returns the listening address.
func (server *HTTPServer) Addr() string { return server.addr }

// Start starts the HTTP server.
func (server *HTTPServer) Start(ctx context.Context) error {
	ctx = core.NormalizeContext(ctx)

	server.mu.Lock()
	if server.status == Running {
		server.mu.Unlock()
		return nil
	}
	server.status = Starting
	listener, err := net.Listen("tcp", server.addr)
	if err != nil {
		server.status = Failed
		server.err = err
		server.mu.Unlock()
		return err
	}
	server.listener = listener
	server.addr = listener.Addr().String()
	server.server = &http.Server{
		Addr:         server.addr,
		Handler:      server.handler,
		ReadTimeout:  server.readTimeout,
		WriteTimeout: server.writeTimeout,
		IdleTimeout:  server.idleTimeout,
	}
	server.done = make(chan struct{})
	server.watch = make(chan struct{})
	server.status = Running
	httpServer := server.server
	done := server.done
	watch := server.watch
	server.mu.Unlock()

	go func() {
		defer close(done)
		err := httpServer.Serve(listener)
		server.mu.Lock()
		defer server.mu.Unlock()
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			server.status = Failed
			server.err = err
			return
		}
		if server.status != Stopped {
			server.status = Stopped
		}
	}()

	if done := ctx.Done(); done != nil {
		go func() {
			select {
			case <-done:
				_ = server.Stop(core.DefaultContext())
			case <-watch:
			}
		}()
	}

	return nil
}

// Drain marks the HTTP server as draining.
func (server *HTTPServer) Drain(context.Context) error {
	server.mu.Lock()
	defer server.mu.Unlock()
	if server.status == Running {
		server.status = Draining
	}
	return nil
}

// Stop gracefully shuts down the HTTP server.
func (server *HTTPServer) Stop(ctx context.Context) error {
	ctx = core.NormalizeContext(ctx)

	server.mu.Lock()
	if server.status == Stopped || server.status == Created {
		server.status = Stopped
		server.mu.Unlock()
		return nil
	}
	server.status = Stopping
	httpServer := server.server
	done := server.done
	watch := server.watch
	shutdownTimeout := server.shutdownTimeout
	server.watch = nil
	server.mu.Unlock()

	if watch != nil {
		close(watch)
	}
	if httpServer == nil {
		server.mu.Lock()
		server.status = Stopped
		server.mu.Unlock()
		return nil
	}

	shutdownCtx := ctx
	cancel := func() {}
	if _, ok := ctx.Deadline(); !ok && shutdownTimeout > 0 {
		shutdownCtx, cancel = context.WithTimeout(ctx, shutdownTimeout)
	}
	defer cancel()

	err := httpServer.Shutdown(shutdownCtx)
	if done != nil {
		<-done
	}

	server.mu.Lock()
	defer server.mu.Unlock()
	if err != nil {
		server.status = Failed
		server.err = err
		return err
	}
	server.status = Stopped
	return nil
}

// Status returns the current HTTP host status.
func (server *HTTPServer) Status() Status {
	server.mu.Lock()
	defer server.mu.Unlock()
	return server.status
}

// Error returns the last server error.
func (server *HTTPServer) Error() error {
	server.mu.Lock()
	defer server.mu.Unlock()
	return server.err
}

// Check returns HTTP host health.
func (server *HTTPServer) Check(context.Context) Health {
	server.mu.Lock()
	defer server.mu.Unlock()
	message := "ok"
	if server.err != nil {
		message = server.err.Error()
	}
	return Health{
		Status:  server.status,
		Message: message,
		Details: map[string]any{
			"addr": server.addr,
		},
	}
}
