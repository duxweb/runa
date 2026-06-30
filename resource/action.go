package resource

import "github.com/duxweb/runa/route"

// List registers the resource list action.
func (res *Resource) List[Input any, Output any](handler route.TypedHandler[Input, Output]) *route.Route {
	return res.Get[Input, Output]("list", "", handler)
}

// Show registers the resource show action.
func (res *Resource) Show[Input any, Output any](handler route.TypedHandler[Input, Output]) *route.Route {
	return res.Get[Input, Output]("show", "/{id}", handler)
}

// Create registers the resource create action.
func (res *Resource) Create[Input any, Output any](handler route.TypedHandler[Input, Output]) *route.Route {
	return res.Post[Input, Output]("create", "", handler)
}

// Edit registers the resource edit action.
func (res *Resource) Edit[Input any, Output any](handler route.TypedHandler[Input, Output]) *route.Route {
	return res.Put[Input, Output]("edit", "/{id}", handler)
}

// Store registers the resource store action.
func (res *Resource) Store[Input any, Output any](handler route.TypedHandler[Input, Output]) *route.Route {
	return res.Patch[Input, Output]("store", "/{id}", handler)
}

// Delete registers the resource delete action.
func (res *Resource) Delete[Input any, Output any](handler route.TypedHandler[Input, Output]) *route.Route {
	return res.DeleteAction[Input, Output]("delete", "/{id}", handler)
}

// Get registers a typed GET resource action.
func (res *Resource) Get[Input any, Output any](name string, path string, handler route.TypedHandler[Input, Output]) *route.Route {
	return res.typed("GET", name, path, handler)
}

// Post registers a typed POST resource action.
func (res *Resource) Post[Input any, Output any](name string, path string, handler route.TypedHandler[Input, Output]) *route.Route {
	return res.typed("POST", name, path, handler)
}

// Put registers a typed PUT resource action.
func (res *Resource) Put[Input any, Output any](name string, path string, handler route.TypedHandler[Input, Output]) *route.Route {
	return res.typed("PUT", name, path, handler)
}

// Patch registers a typed PATCH resource action.
func (res *Resource) Patch[Input any, Output any](name string, path string, handler route.TypedHandler[Input, Output]) *route.Route {
	return res.typed("PATCH", name, path, handler)
}

// DeleteAction registers a typed DELETE resource action.
func (res *Resource) DeleteAction[Input any, Output any](name string, path string, handler route.TypedHandler[Input, Output]) *route.Route {
	return res.typed("DELETE", name, path, handler)
}

func (res *Resource) typed[Input any, Output any](method string, name string, path string, handler route.TypedHandler[Input, Output]) *route.Route {
	item := res.route(method, name, path, func(ctx *route.Context) error {
		input, err := route.Input[Input](ctx)
		if err != nil {
			return err
		}
		output, err := handler(ctx, input)
		if err != nil {
			return err
		}
		return ctx.RenderOutput(output)
	})
	return item.Schema(route.TypedSchema[Input, Output]())
}
