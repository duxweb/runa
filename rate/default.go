package rate

import runaprovider "github.com/duxweb/runa/provider"

// Default returns the default app rate registry.
func Default() *Registry { return runaprovider.MustInvokeDefault[*Registry]() }
