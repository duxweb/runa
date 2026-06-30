package provider

import "context"

// Service is a framework or infrastructure capability with lifecycle hooks.
type Service interface {
	Name() string
	Init(ctx context.Context, app Context) error
	Register(ctx context.Context, app Context) error
	Boot(ctx context.Context, app Context) error
	Shutdown(ctx context.Context, app Context) error
}

// ServiceBase provides no-op lifecycle methods for services.
type ServiceBase struct{}

func (ServiceBase) Init(context.Context, Context) error     { return nil }
func (ServiceBase) Register(context.Context, Context) error { return nil }
func (ServiceBase) Boot(context.Context, Context) error     { return nil }
func (ServiceBase) Shutdown(context.Context, Context) error { return nil }

// Module is a business module entry.
type Module interface {
	Name() string
	Init(ctx context.Context, app Context) error
	Register(ctx context.Context, app Context) error
	Boot(ctx context.Context, app Context) error
	Shutdown(ctx context.Context, app Context) error
}

// ModuleBase provides no-op lifecycle methods for modules.
type ModuleBase struct{}

func (ModuleBase) Init(context.Context, Context) error     { return nil }
func (ModuleBase) Register(context.Context, Context) error { return nil }
func (ModuleBase) Boot(context.Context, Context) error     { return nil }
func (ModuleBase) Shutdown(context.Context, Context) error { return nil }

// ModuleDepends lets a module declare hard dependencies by module name.
type ModuleDepends interface {
	Depends() []string
}
