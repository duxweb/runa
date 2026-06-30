package route

// Get registers a typed GET route.
func Get[In any, Out any](target Target, path string, handler TypedHandler[In, Out]) *Route {
	return typed(target, "GET", path, handler)
}

// Post registers a typed POST route.
func Post[In any, Out any](target Target, path string, handler TypedHandler[In, Out]) *Route {
	return typed(target, "POST", path, handler)
}

// Put registers a typed PUT route.
func Put[In any, Out any](target Target, path string, handler TypedHandler[In, Out]) *Route {
	return typed(target, "PUT", path, handler)
}

// Patch registers a typed PATCH route.
func Patch[In any, Out any](target Target, path string, handler TypedHandler[In, Out]) *Route {
	return typed(target, "PATCH", path, handler)
}

// Delete registers a typed DELETE route.
func Delete[In any, Out any](target Target, path string, handler TypedHandler[In, Out]) *Route {
	return typed(target, "DELETE", path, handler)
}

// Options registers a typed OPTIONS route.
func Options[In any, Out any](target Target, path string, handler TypedHandler[In, Out]) *Route {
	return typed(target, "OPTIONS", path, handler)
}

// Head registers a typed HEAD route.
func Head[In any, Out any](target Target, path string, handler TypedHandler[In, Out]) *Route {
	return typed(target, "HEAD", path, handler)
}

// Any registers a typed route for common HTTP methods.
func Any[In any, Out any](target Target, path string, handler TypedHandler[In, Out]) *Route {
	return typed(target, "ANY", path, handler)
}

func typed[In any, Out any](target Target, method string, path string, handler TypedHandler[In, Out]) *Route {
	route := target.RouteGroup().Handle(method, path, func(ctx *Context) error {
		input, err := Input[In](ctx)
		if err != nil {
			return err
		}
		output, err := handler(ctx, input)
		if err != nil {
			return err
		}
		return ctx.RenderOutput(output)
	})
	return route.Schema(TypedSchema[In, Out]())
}
