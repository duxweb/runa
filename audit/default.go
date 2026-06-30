package audit

import runaprovider "github.com/duxweb/runa/provider"

// Default returns the default audit registry.
func Default() *Registry { return runaprovider.MustInvokeDefault[*Registry]() }
