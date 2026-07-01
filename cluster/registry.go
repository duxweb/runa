package cluster

import (
	"context"
	"fmt"
	"os"
	"sync"

	"github.com/duxweb/runa/core"
	runaprovider "github.com/duxweb/runa/provider"
)

// Registry stores cluster state for one app instance.
type Registry struct {
	options  options
	instance Instance
	cancel   context.CancelFunc
	done     chan struct{}
	mu       sync.RWMutex
}

// NewRegistry creates a cluster runtime.
func NewRegistry(ctx runaprovider.Context, opts options) (*Registry, error) {
	if opts.driver == nil {
		return nil, fmt.Errorf("cluster driver is required")
	}
	hostname, _ := os.Hostname()
	instanceID := opts.id
	if instanceID == "" {
		instanceID = defaultInstanceID(hostname)
	}
	service := opts.service
	if service == "" {
		service = DefaultService
	}
	instance := Instance{
		ID:        instanceID,
		Service:   service,
		Env:       pick(opts.env, appEnv(ctx)),
		Version:   opts.version,
		Hostname:  hostname,
		PID:       os.Getpid(),
		Addr:      opts.addr,
		Status:    StatusStarting,
		StartedAt: core.Now(),
		TTL:       opts.ttl,
		Meta:      core.CloneMap(opts.meta),
	}
	return &Registry{options: opts, instance: instance}, nil
}
