package cluster

import (
	"context"

	runaprovider "github.com/duxweb/runa/provider"
)

// RegistryOf returns the optional cluster runtime from provider context.
func RegistryOf(ctx runaprovider.Context) *Registry {
	if ctx == nil {
		return nil
	}
	runtime, err := runaprovider.Invoke[*Registry](ctx)
	if err != nil {
		return nil
	}
	return runtime
}

// Info returns active default cluster instances when cluster is enabled.
func Info(ctx context.Context) []Instance {
	runtime := Default()
	if runtime == nil {
		return nil
	}
	items, err := runtime.Instances(ctx)
	if err != nil {
		return nil
	}
	return items
}
