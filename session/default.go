package session

import runaprovider "github.com/duxweb/runa/provider"

// Default returns the default app session registry.
func Default() *Registry { return runaprovider.MustInvokeDefault[*Registry]() }
