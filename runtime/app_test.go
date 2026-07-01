package runtime

import (
	"bytes"
	"context"
	"errors"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	runacommand "github.com/duxweb/runa/command"
	"github.com/duxweb/runa/host"
	runaprovider "github.com/duxweb/runa/provider"
)

func mustFreeze(t *testing.T, app *App) {
	t.Helper()
	if err := app.Freeze(app.Context()); err != nil {
		t.Fatalf("freeze: %v", err)
	}
}

type lifecycleRecorder struct {
	name  string
	calls *[]string
}

func (r lifecycleRecorder) Name() string { return r.name }
func (r lifecycleRecorder) Init(context.Context, runaprovider.Context) error {
	*r.calls = append(*r.calls, r.name+":init")
	return nil
}
func (r lifecycleRecorder) Register(context.Context, runaprovider.Context) error {
	*r.calls = append(*r.calls, r.name+":register")
	return nil
}
func (r lifecycleRecorder) Boot(context.Context, runaprovider.Context) error {
	*r.calls = append(*r.calls, r.name+":boot")
	return nil
}
func (r lifecycleRecorder) Shutdown(context.Context, runaprovider.Context) error {
	*r.calls = append(*r.calls, r.name+":shutdown")
	return nil
}

type failingLifecycle struct {
	lifecycleRecorder
	bootErr error
}

func (recorder failingLifecycle) Boot(context.Context, runaprovider.Context) error {
	if recorder.calls != nil {
		*recorder.calls = append(*recorder.calls, recorder.name+":boot")
	}
	return recorder.bootErr
}

type failingRegisterLifecycle struct {
	lifecycleRecorder
	registerErr error
}

func (recorder failingRegisterLifecycle) Register(context.Context, runaprovider.Context) error {
	if recorder.calls != nil {
		*recorder.calls = append(*recorder.calls, recorder.name+":register")
	}
	return recorder.registerErr
}

type blockingLifecycle struct {
	name    string
	started chan struct{}
	release chan struct{}
	count   *atomic.Int32
}

func (recorder blockingLifecycle) Name() string { return recorder.name }
func (recorder blockingLifecycle) Init(context.Context, runaprovider.Context) error {
	return nil
}
func (recorder blockingLifecycle) Register(context.Context, runaprovider.Context) error {
	return nil
}
func (recorder blockingLifecycle) Boot(context.Context, runaprovider.Context) error {
	recorder.count.Add(1)
	close(recorder.started)
	<-recorder.release
	return nil
}
func (recorder blockingLifecycle) Shutdown(context.Context, runaprovider.Context) error {
	return nil
}

type diShutdownRecorder struct {
	calls *[]string
}

func (recorder *diShutdownRecorder) Shutdown(context.Context) error {
	*recorder.calls = append(*recorder.calls, "di:shutdown")
	return nil
}

type providerFunc struct {
	runaprovider.Base
	name string
	fn   func(app *App) error
}

func (p providerFunc) Name() string { return p.name }
func (p providerFunc) Register(ctx runaprovider.Context) error {
	return p.fn(ctx.App().(*App))
}

type providerLifecycleRecorder struct {
	runaprovider.Base
	name     string
	priority int
	calls    *[]string
}

func (recorder providerLifecycleRecorder) Name() string { return recorder.name }
func (recorder providerLifecycleRecorder) Priority() int {
	return recorder.priority
}
func (recorder providerLifecycleRecorder) Init(context.Context, runaprovider.Context) error {
	*recorder.calls = append(*recorder.calls, recorder.name+":init")
	return nil
}
func (recorder providerLifecycleRecorder) Register(runaprovider.Context) error {
	*recorder.calls = append(*recorder.calls, recorder.name+":register")
	return nil
}
func (recorder providerLifecycleRecorder) Boot(context.Context, runaprovider.Context) error {
	*recorder.calls = append(*recorder.calls, recorder.name+":boot")
	return nil
}
func (recorder providerLifecycleRecorder) Shutdown(context.Context, runaprovider.Context) error {
	*recorder.calls = append(*recorder.calls, recorder.name+":shutdown")
	return nil
}

func TestAppLifecycle(t *testing.T) {
	calls := []string{}
	app := newRuntimeApp()
	app.Service(lifecycleRecorder{name: "service", calls: &calls})
	app.Module(lifecycleRecorder{name: "module", calls: &calls})

	if err := app.Freeze(context.Background()); err != nil {
		t.Fatalf("freeze: %v", err)
	}
	if err := app.Shutdown(context.Background()); err != nil {
		t.Fatalf("shutdown: %v", err)
	}

	expected := []string{
		"service:init",
		"module:init",
		"service:register",
		"module:register",
		"service:boot",
		"module:boot",
		"module:shutdown",
		"service:shutdown",
	}
	if !reflect.DeepEqual(calls, expected) {
		t.Fatalf("calls = %#v, want %#v", calls, expected)
	}
}

func TestProviderLifecycle(t *testing.T) {
	calls := []string{}
	app := newRuntimeApp()
	app.Install(providerLifecycleRecorder{name: "late", priority: 10, calls: &calls})
	app.Install(providerLifecycleRecorder{name: "early", priority: -10, calls: &calls})
	app.Service(lifecycleRecorder{name: "service", calls: &calls})
	app.Module(lifecycleRecorder{name: "module", calls: &calls})
	ProvideValue(app, &diShutdownRecorder{calls: &calls})
	if _, err := Invoke[*diShutdownRecorder](app); err != nil {
		t.Fatalf("invoke: %v", err)
	}

	if err := app.Freeze(context.Background()); err != nil {
		t.Fatalf("freeze: %v", err)
	}
	if err := app.Shutdown(context.Background()); err != nil {
		t.Fatalf("shutdown: %v", err)
	}

	expected := []string{
		"early:init",
		"late:init",
		"early:register",
		"late:register",
		"service:init",
		"module:init",
		"service:register",
		"module:register",
		"early:boot",
		"late:boot",
		"service:boot",
		"module:boot",
		"module:shutdown",
		"service:shutdown",
		"late:shutdown",
		"early:shutdown",
		"di:shutdown",
	}
	if !reflect.DeepEqual(calls, expected) {
		t.Fatalf("calls = %#v, want %#v", calls, expected)
	}
}

func TestProviderLifecycleRunsEachInstalledProvider(t *testing.T) {
	calls := []string{}
	app := newRuntimeApp()
	app.Install(
		providerLifecycleRecorder{name: "same", calls: &calls},
		providerLifecycleRecorder{name: "same", calls: &calls},
	)

	if err := app.Freeze(context.Background()); err != nil {
		t.Fatalf("freeze: %v", err)
	}
	if err := app.Shutdown(context.Background()); err != nil {
		t.Fatalf("shutdown: %v", err)
	}

	expected := []string{
		"same:init",
		"same:init",
		"same:register",
		"same:register",
		"same:boot",
		"same:boot",
		"same:shutdown",
		"same:shutdown",
	}
	if !reflect.DeepEqual(calls, expected) {
		t.Fatalf("calls = %#v, want %#v", calls, expected)
	}
}

func TestProviderCanRegisterService(t *testing.T) {
	calls := []string{}
	app := newRuntimeApp()
	app.Install(providerFunc{name: "test", fn: func(app *App) error {
		app.Service(lifecycleRecorder{name: "provider-service", calls: &calls})
		return nil
	}})

	if err := app.Freeze(context.Background()); err != nil {
		t.Fatalf("freeze: %v", err)
	}
	if err := app.Shutdown(context.Background()); err != nil {
		t.Fatalf("shutdown: %v", err)
	}

	if len(calls) == 0 {
		t.Fatal("provider service was not executed")
	}
}

func TestShutdownClosesDIAfterModulesAndServices(t *testing.T) {
	calls := []string{}
	app := newRuntimeApp()
	app.Service(lifecycleRecorder{name: "service", calls: &calls})
	app.Module(lifecycleRecorder{name: "module", calls: &calls})
	ProvideValue(app, &diShutdownRecorder{calls: &calls})
	if _, err := Invoke[*diShutdownRecorder](app); err != nil {
		t.Fatalf("invoke: %v", err)
	}

	if err := app.Freeze(context.Background()); err != nil {
		t.Fatalf("freeze: %v", err)
	}
	if err := app.Shutdown(context.Background()); err != nil {
		t.Fatalf("shutdown: %v", err)
	}

	expectedSuffix := []string{
		"module:shutdown",
		"service:shutdown",
		"di:shutdown",
	}
	if len(calls) < len(expectedSuffix) {
		t.Fatalf("calls = %#v", calls)
	}
	suffix := calls[len(calls)-len(expectedSuffix):]
	if !reflect.DeepEqual(suffix, expectedSuffix) {
		t.Fatalf("shutdown suffix = %#v, want %#v; all=%#v", suffix, expectedSuffix, calls)
	}
}

func TestFreezeRollbackOnBootFailure(t *testing.T) {
	calls := []string{}
	app := newRuntimeApp()
	app.Service(lifecycleRecorder{name: "service", calls: &calls})
	app.Module(failingLifecycle{
		lifecycleRecorder: lifecycleRecorder{name: "module", calls: &calls},
		bootErr:           errors.New("boot failed"),
	})
	ProvideValue(app, &diShutdownRecorder{calls: &calls})
	_, _ = Invoke[*diShutdownRecorder](app)

	err := app.Freeze(context.Background())
	if err == nil {
		t.Fatal("expected freeze error")
	}
	expectedSuffix := []string{"module:boot", "service:shutdown", "di:shutdown"}
	if len(calls) < len(expectedSuffix) {
		t.Fatalf("calls = %#v", calls)
	}
	suffix := calls[len(calls)-len(expectedSuffix):]
	if !reflect.DeepEqual(suffix, expectedSuffix) {
		t.Fatalf("rollback suffix = %#v, want %#v; all=%#v", suffix, expectedSuffix, calls)
	}
	for _, call := range calls {
		if call == "module:shutdown" {
			t.Fatalf("failed boot module should not be shut down: %#v", calls)
		}
	}
}

func TestFreezeRollbackSkipsUnbootedHooks(t *testing.T) {
	calls := []string{}
	app := newRuntimeApp()
	app.Service(failingRegisterLifecycle{
		lifecycleRecorder: lifecycleRecorder{name: "service", calls: &calls},
		registerErr:       errors.New("register failed"),
	})
	app.Module(lifecycleRecorder{name: "module", calls: &calls})
	ProvideValue(app, &diShutdownRecorder{calls: &calls})
	_, _ = Invoke[*diShutdownRecorder](app)

	err := app.Freeze(context.Background())
	if err == nil {
		t.Fatal("expected freeze error")
	}
	joined := strings.Join(calls, ",")
	if strings.Contains(joined, "service:shutdown") || strings.Contains(joined, "module:shutdown") {
		t.Fatalf("rollback should not call unbooted shutdown hooks: %#v", calls)
	}
	if !strings.Contains(joined, "di:shutdown") {
		t.Fatalf("rollback should shutdown DI resources: %#v", calls)
	}
}

func TestFreezeFailurePreventsRetry(t *testing.T) {
	app := newRuntimeApp()
	app.Module(failingLifecycle{
		lifecycleRecorder: lifecycleRecorder{name: "module", calls: &[]string{}},
		bootErr:           errors.New("boot failed"),
	})
	if err := app.Freeze(context.Background()); err == nil {
		t.Fatal("expected first freeze error")
	}
	err := app.Freeze(context.Background())
	if err == nil || !strings.Contains(err.Error(), "create a new app") {
		t.Fatalf("retry error = %v", err)
	}
}

func TestShutdownRunsOnce(t *testing.T) {
	calls := []string{}
	app := newRuntimeApp()
	app.Module(lifecycleRecorder{name: "module", calls: &calls})
	if err := app.Freeze(context.Background()); err != nil {
		t.Fatalf("freeze: %v", err)
	}
	if err := app.Shutdown(context.Background()); err != nil {
		t.Fatalf("first shutdown: %v", err)
	}
	if err := app.Shutdown(context.Background()); err != nil {
		t.Fatalf("second shutdown: %v", err)
	}
	count := 0
	for _, call := range calls {
		if call == "module:shutdown" {
			count++
		}
	}
	if count != 1 {
		t.Fatalf("shutdown count = %d, calls=%#v", count, calls)
	}
}

func TestFreezeConcurrentRunsOnce(t *testing.T) {
	var bootCount atomic.Int32
	started := make(chan struct{})
	release := make(chan struct{})
	app := newRuntimeApp()
	app.Service(blockingLifecycle{name: "blocking", started: started, release: release, count: &bootCount})

	errs := make(chan error, 2)
	go func() { errs <- app.Freeze(context.Background()) }()
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("boot did not start")
	}
	go func() { errs <- app.Freeze(context.Background()) }()
	time.Sleep(50 * time.Millisecond)
	if count := bootCount.Load(); count != 1 {
		t.Fatalf("boot count while blocked = %d", count)
	}
	close(release)
	for i := 0; i < 2; i++ {
		if err := <-errs; err != nil {
			t.Fatalf("freeze: %v", err)
		}
	}
	if count := bootCount.Load(); count != 1 {
		t.Fatalf("boot count = %d", count)
	}
}

func TestDI(t *testing.T) {
	app := newRuntimeApp()
	type dependency struct{ Value string }

	Provide(app, func(*App) (*dependency, error) {
		return &dependency{Value: "ok"}, nil
	})

	dep, err := Invoke[*dependency](app)
	if err != nil {
		t.Fatalf("invoke: %v", err)
	}
	if dep.Value != "ok" {
		t.Fatalf("dep.Value = %q", dep.Value)
	}
}

func TestAppContextDefaultAndSet(t *testing.T) {
	app := newRuntimeApp()
	if app.Context() == nil {
		t.Fatal("default context is nil")
	}

	type key string
	ctx := context.WithValue(context.Background(), key("name"), "runa")
	app.SetContext(ctx)
	if app.Context().Value(key("name")) != "runa" {
		t.Fatal("context value was not stored")
	}

	app.SetContext(nil)
	if app.Context() == nil {
		t.Fatal("nil context should fallback to default context")
	}
}

type flagCommand struct {
	seen *string
}

func (flagCommand) Name() string    { return "flag:test" }
func (flagCommand) Summary() string { return "flag test" }
func (flagCommand) Flags(flags *runacommand.FlagSet) {
	flags.String("name", "default", "name value")
}
func (cmd flagCommand) Run(ctx context.Context, command *runacommand.Context) error {
	*cmd.seen = command.Get[string]("name")
	return nil
}

func TestCommandDefaultServe(t *testing.T) {
	app := newRuntimeApp()
	if err := app.Execute(context.Background(), nil); err == nil || !strings.Contains(err.Error(), "no host registered") {
		t.Fatalf("execute = %v", err)
	}
}

type appHostRecorder struct {
	name    string
	addr    string
	status  host.Status
	started chan struct{}
	calls   []string
	mu      sync.Mutex
}

func newAppHostRecorder(name string) *appHostRecorder {
	return &appHostRecorder{
		name:    name,
		status:  host.Created,
		started: make(chan struct{}),
	}
}

func (recorder *appHostRecorder) Name() string { return recorder.name }
func (recorder *appHostRecorder) Start(context.Context) error {
	recorder.mu.Lock()
	recorder.status = host.Running
	recorder.calls = append(recorder.calls, "start")
	close(recorder.started)
	recorder.mu.Unlock()
	return nil
}
func (recorder *appHostRecorder) Stop(context.Context) error {
	recorder.mu.Lock()
	defer recorder.mu.Unlock()
	recorder.status = host.Stopped
	recorder.calls = append(recorder.calls, "stop")
	return nil
}
func (recorder *appHostRecorder) Status() host.Status {
	recorder.mu.Lock()
	defer recorder.mu.Unlock()
	return recorder.status
}
func (recorder *appHostRecorder) Check(context.Context) host.Health {
	return host.Health{Status: recorder.Status(), Details: map[string]any{"addr": recorder.addr}}
}
func (recorder *appHostRecorder) Calls() []string {
	recorder.mu.Lock()
	defer recorder.mu.Unlock()
	return append([]string(nil), recorder.calls...)
}

func TestServeStartsRegisteredHost(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	recorder := newAppHostRecorder("http")
	app := newRuntimeApp()
	app.Host(recorder)

	errCh := make(chan error, 1)
	go func() {
		errCh <- app.Execute(ctx, []string{"serve"})
	}()

	select {
	case <-recorder.started:
	case <-time.After(time.Second):
		t.Fatal("host was not started")
	}
	cancel()

	select {
	case err := <-errCh:
		if err != nil {
			t.Fatalf("execute: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("serve did not stop")
	}

	expected := []string{"start", "stop"}
	if !reflect.DeepEqual(recorder.Calls(), expected) {
		t.Fatalf("calls = %#v, want %#v", recorder.Calls(), expected)
	}
}

func TestServePrintsStartedHosts(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	var out bytes.Buffer
	httpHost := newAppHostRecorder("http")
	httpHost.addr = "127.0.0.1:18080"
	queueHost := newAppHostRecorder("queue:default")
	app := newRuntimeApp(Writer(&out))
	app.Host(httpHost, queueHost)

	errCh := make(chan error, 1)
	go func() {
		errCh <- app.Execute(ctx, []string{"serve"})
	}()

	select {
	case <-queueHost.started:
	case <-time.After(time.Second):
		t.Fatal("queue host was not started")
	}
	cancel()

	select {
	case err := <-errCh:
		if err != nil {
			t.Fatalf("execute: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("serve did not stop")
	}
	body := out.String()
	for _, expected := range []string{"Hosts", "➜ http", "running", "127.0.0.1:18080", "➜ queue:default"} {
		if !strings.Contains(body, expected) {
			t.Fatalf("output missing %q in:\n%s", expected, body)
		}
	}
}

func TestServePrintsSingleStartedHost(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	var out bytes.Buffer
	httpHost := newAppHostRecorder("http")
	httpHost.addr = "[::]:8080"
	app := newRuntimeApp(Writer(&out))
	app.Host(httpHost)

	errCh := make(chan error, 1)
	go func() {
		errCh <- app.Execute(ctx, []string{"serve"})
	}()

	select {
	case <-httpHost.started:
	case <-time.After(time.Second):
		t.Fatal("http host was not started")
	}
	cancel()

	select {
	case err := <-errCh:
		if err != nil {
			t.Fatalf("execute: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("serve did not stop")
	}
	body := out.String()
	for _, expected := range []string{"Hosts", "➜ http", "running", "*:8080"} {
		if !strings.Contains(body, expected) {
			t.Fatalf("output missing %q in:\n%s", expected, body)
		}
	}
}

func TestDisplayHostAddr(t *testing.T) {
	cases := map[string]string{
		"[::]:18080":      "*:18080",
		"0.0.0.0:18080":   "*:18080",
		"127.0.0.1:18080": "127.0.0.1:18080",
		"queue":           "queue",
	}
	for input, expected := range cases {
		if actual := displayHostAddr(input); actual != expected {
			t.Fatalf("displayHostAddr(%q) = %q, want %q", input, actual, expected)
		}
	}
}

func TestRegistrationErrorsReturnOnFreeze(t *testing.T) {
	app := newRuntimeApp()
	app.Host(&appHostRecorder{name: ""})
	err := app.Freeze(context.Background())
	if err == nil {
		t.Fatal("expected freeze error")
	}
	text := err.Error()
	for _, part := range []string{"host name is empty"} {
		if !strings.Contains(text, part) {
			t.Fatalf("freeze error = %v", err)
		}
	}
}

func TestProviderRegistrationErrorsReturnOnFreeze(t *testing.T) {
	app := newRuntimeApp()
	app.Install(providerFunc{name: "bad", fn: func(app *App) error {
		app.Host(&appHostRecorder{name: ""})
		return nil
	}})
	err := app.Freeze(context.Background())
	if err == nil || !strings.Contains(err.Error(), "host name is empty") {
		t.Fatalf("freeze error = %v", err)
	}
}

func TestCommandNotFound(t *testing.T) {
	app := newRuntimeApp()
	if err := app.Execute(context.Background(), []string{"missing"}); err == nil {
		t.Fatal("expected command error")
	}
}

func TestCommandFlags(t *testing.T) {
	seen := ""
	app := newRuntimeApp()
	app.Command(flagCommand{seen: &seen})

	if err := app.Execute(context.Background(), []string{"flag:test", "--name", "runa"}); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if seen != "runa" {
		t.Fatalf("seen = %q", seen)
	}
}
