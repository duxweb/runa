package queue

import runaprovider "github.com/duxweb/runa/provider"

// Default returns the default app queue registry.
func Default() *Registry { return runaprovider.MustInvokeDefault[*Registry]() }
