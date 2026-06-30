package cluster

import (
	"context"
	"time"

	"github.com/duxweb/runa/core"
)

// Status describes instance state.
type Status string

const (
	StatusStarting Status = "starting"
	StatusRunning  Status = "running"
	StatusDraining Status = "draining"
	StatusStopped  Status = "stopped"
)

// Instance describes one running app process.
type Instance struct {
	ID          string        `json:"id"`
	Service     string        `json:"service"`
	Env         string        `json:"env"`
	Version     string        `json:"version"`
	Hostname    string        `json:"hostname"`
	PID         int           `json:"pid"`
	Addr        string        `json:"addr,omitempty"`
	Status      Status        `json:"status"`
	StartedAt   time.Time     `json:"started_at"`
	HeartbeatAt time.Time     `json:"heartbeat_at"`
	TTL         time.Duration `json:"ttl"`
	Meta        core.Map      `json:"meta,omitempty"`
}

// Driver stores cluster instance heartbeats.
type Driver interface {
	Name() string
	Register(ctx context.Context, instance Instance) error
	Heartbeat(ctx context.Context, id string, status Status) error
	Unregister(ctx context.Context, id string) error
	Instances(ctx context.Context, service string) ([]Instance, error)
	Close(ctx context.Context) error
}
