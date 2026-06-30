package schedule

import runaprovider "github.com/duxweb/runa/provider"

// Default returns the default app schedule registry.
func Default() *Registry { return runaprovider.MustInvokeDefault[*Registry]() }
