package observe

import runaprovider "github.com/duxweb/runa/provider"

// Default returns the default observe health registry.
func Default() *Registry { return runaprovider.MustInvokeDefault[*Registry]() }
