package event

import runaprovider "github.com/duxweb/runa/provider"

// Default returns the default app event registry.
func Default() *Registry { return runaprovider.MustInvokeDefault[*Registry]() }
