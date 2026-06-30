package cluster

import runaprovider "github.com/duxweb/runa/provider"

// Default returns the optional default cluster runtime.
func Default() *Registry {
	runtime, err := runaprovider.InvokeDefault[*Registry]()
	if err != nil {
		return nil
	}
	return runtime
}
