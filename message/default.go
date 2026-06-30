package message

import runaprovider "github.com/duxweb/runa/provider"

// Default returns the default app message registry.
func Default() *Registry { return runaprovider.MustInvokeDefault[*Registry]() }
