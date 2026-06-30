package console

import runaprovider "github.com/duxweb/runa/provider"

// Default returns the default console registry.
func Default() *Registry { return runaprovider.MustInvokeDefault[*Registry]() }
