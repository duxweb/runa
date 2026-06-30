package cache

import runaprovider "github.com/duxweb/runa/provider"

// Default returns the default app cache registry.
func Default() *Registry { return runaprovider.MustInvokeDefault[*Registry]() }
