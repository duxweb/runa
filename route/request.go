package route

import "net/http"

// SetRequest replaces the request used by this context.
func (ctx *Context) SetRequest(request *http.Request) {
	ctx.request = request
	if ctx.scope != nil && request != nil {
		ctx.scope.SetContext(request.Context())
	}
}
