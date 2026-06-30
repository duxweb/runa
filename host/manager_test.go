package host

import (
	"context"
	"errors"
	"reflect"
	"testing"
)

type unitRecorder struct {
	name     string
	status   Status
	calls    *[]string
	startErr error
	stopErr  error
	drainErr error
}

func (unit *unitRecorder) Name() string { return unit.name }
func (unit *unitRecorder) Start(context.Context) error {
	*unit.calls = append(*unit.calls, unit.name+":start")
	if unit.startErr != nil {
		unit.status = Failed
		return unit.startErr
	}
	unit.status = Running
	return nil
}
func (unit *unitRecorder) Stop(context.Context) error {
	*unit.calls = append(*unit.calls, unit.name+":stop")
	if unit.stopErr != nil {
		unit.status = Failed
		return unit.stopErr
	}
	unit.status = Stopped
	return nil
}
func (unit *unitRecorder) Drain(context.Context) error {
	*unit.calls = append(*unit.calls, unit.name+":drain")
	if unit.drainErr != nil {
		unit.status = Failed
		return unit.drainErr
	}
	unit.status = Draining
	return nil
}
func (unit *unitRecorder) Status() Status { return unit.status }

func TestManagerStartStopOrder(t *testing.T) {
	calls := []string{}
	manager := NewManager()
	if err := manager.Register(
		&unitRecorder{name: "http", status: Created, calls: &calls},
		&unitRecorder{name: "queue", status: Created, calls: &calls},
	); err != nil {
		t.Fatalf("register: %v", err)
	}

	if err := manager.Start(context.Background()); err != nil {
		t.Fatalf("start: %v", err)
	}
	if err := manager.Stop(context.Background()); err != nil {
		t.Fatalf("stop: %v", err)
	}

	expected := []string{
		"http:start",
		"queue:start",
		"queue:drain",
		"queue:stop",
		"http:drain",
		"http:stop",
	}
	if !reflect.DeepEqual(calls, expected) {
		t.Fatalf("calls = %#v, want %#v", calls, expected)
	}
}

func TestManagerStartSelected(t *testing.T) {
	calls := []string{}
	manager := NewManager()
	if err := manager.Register(
		&unitRecorder{name: "http", status: Created, calls: &calls},
		&unitRecorder{name: "queue", status: Created, calls: &calls},
	); err != nil {
		t.Fatalf("register: %v", err)
	}

	if err := manager.Start(context.Background(), "queue"); err != nil {
		t.Fatalf("start selected: %v", err)
	}
	if err := manager.Stop(context.Background()); err != nil {
		t.Fatalf("stop: %v", err)
	}

	expected := []string{"queue:start", "queue:drain", "queue:stop"}
	if !reflect.DeepEqual(calls, expected) {
		t.Fatalf("calls = %#v, want %#v", calls, expected)
	}
}

func TestManagerInfoAndStatus(t *testing.T) {
	calls := []string{}
	manager := NewManager()
	if err := manager.Register(&unitRecorder{name: "http", status: Created, calls: &calls}); err != nil {
		t.Fatalf("register: %v", err)
	}

	if status := manager.Status("http"); status != Created {
		t.Fatalf("status = %s, want %s", status, Created)
	}
	if status := manager.Status("missing"); status != Stopped {
		t.Fatalf("missing status = %s, want %s", status, Stopped)
	}

	info := manager.Info()
	if len(info) != 1 || info[0].Name != "http" || info[0].Status != Created {
		t.Fatalf("info = %#v", info)
	}
}

func TestManagerStartFailureStopsStartedUnits(t *testing.T) {
	calls := []string{}
	manager := NewManager()
	boom := errors.New("boom")
	if err := manager.Register(
		&unitRecorder{name: "http", status: Created, calls: &calls},
		&unitRecorder{name: "queue", status: Created, calls: &calls, startErr: boom},
	); err != nil {
		t.Fatalf("register: %v", err)
	}

	if err := manager.Start(context.Background()); err == nil {
		t.Fatal("expected start error")
	}

	expected := []string{"http:start", "queue:start", "http:drain", "http:stop"}
	if !reflect.DeepEqual(calls, expected) {
		t.Fatalf("calls = %#v, want %#v", calls, expected)
	}
}

func TestManagerRegisterInvalidUnit(t *testing.T) {
	manager := NewManager()
	calls := []string{}
	if err := manager.Register(&unitRecorder{name: "", calls: &calls}); err == nil {
		t.Fatal("expected empty name error")
	}
}
