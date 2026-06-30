package route

// Handler handles a Runa HTTP request.
type Handler func(*Context) error

// Middleware wraps a route handler.
type Middleware func(Handler) Handler

// TypedHandler handles a typed request.
type TypedHandler[Input any, Output any] func(*Context, *Input) (*Output, error)

// Target is implemented by route registration targets.
type Target interface {
	RouteGroup() *Group
}
