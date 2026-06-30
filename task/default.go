package task

import runaprovider "github.com/duxweb/runa/provider"

// Default returns the default app task registry.
func Default() *Registry { return runaprovider.MustInvokeDefault[*Registry]() }
