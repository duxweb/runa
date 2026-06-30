package database

import runaprovider "github.com/duxweb/runa/provider"

// Default returns the default app database registry.
func Default() *Registry { return runaprovider.MustInvokeDefault[*Registry]() }
