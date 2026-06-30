package runtime

import runaprovider "github.com/duxweb/runa/provider"

// Service is a framework or infrastructure capability with lifecycle hooks.
type Service = runaprovider.Service

// ServiceBase provides no-op lifecycle methods for services.
type ServiceBase = runaprovider.ServiceBase
