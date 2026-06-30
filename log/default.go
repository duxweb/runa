package log

import runaprovider "github.com/duxweb/runa/provider"

// Default returns the default app log registry.
func Default() *Registry { return runaprovider.MustInvokeDefault[*Registry]() }
