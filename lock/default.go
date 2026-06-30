package lock

import runaprovider "github.com/duxweb/runa/provider"

// Default returns the default app lock registry.
func Default() *Registry { return runaprovider.MustInvokeDefault[*Registry]() }
