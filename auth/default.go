package auth

import runaprovider "github.com/duxweb/runa/provider"

// Default returns the default app auth registry.
func Default() *Registry { return runaprovider.MustInvokeDefault[*Registry]() }
