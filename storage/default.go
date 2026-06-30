package storage

import runaprovider "github.com/duxweb/runa/provider"

// Default returns the default app storage registry.
func Default() *Registry { return runaprovider.MustInvokeDefault[*Registry]() }
