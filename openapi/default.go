package openapi

import runaprovider "github.com/duxweb/runa/provider"

// Default returns the default OpenAPI registry.
func Default() *Registry { return runaprovider.MustInvokeDefault[*Registry]() }
