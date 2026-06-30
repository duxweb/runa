package route

// run executes route middleware and handler.
func (route *Route) run(ctx *Context) error {
	handler := route.Handler
	if handler == nil {
		return nil
	}
	for i := len(route.Middlewares) - 1; i >= 0; i-- {
		middleware := route.Middlewares[i]
		if middleware != nil {
			handler = middleware(handler)
		}
	}
	return handler(ctx)
}
