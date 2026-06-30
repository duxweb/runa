package asset

import runaprovider "github.com/duxweb/runa/provider"

// Default returns the default app asset registry.
func Default() *Registry { return runaprovider.MustInvokeDefault[*Registry]() }
