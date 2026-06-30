package jsonrpc

import (
	"net/http"

	"github.com/duxweb/runa/core"
	"github.com/duxweb/runa/route"
)

// HTTP returns a Runa handler that serves JSON-RPC 2.0 over HTTP POST.
func (server *Server) HTTP() route.Handler {
	return func(ctx *route.Context) error {
		if ctx.Request().Method != http.MethodPost {
			return ctx.Status(http.StatusMethodNotAllowed).Blob(core.MIMEApplicationJSON, errorBody(invalidRequest("only POST is allowed"), nullID))
		}
		body, err := ctx.Body()
		if err != nil {
			return ctx.Status(http.StatusBadRequest).Blob(core.MIMEApplicationJSON, errorBody(parseError(), nullID))
		}
		payload, ok := server.handleBody(ctx.Context(), body, "http")
		if !ok {
			return ctx.SendStatus(http.StatusNoContent)
		}
		return ctx.Blob(core.MIMEApplicationJSON, payload)
	}
}

func errorBody(rpcErr *Error, id ID) []byte {
	body, _ := marshal(Response{Version: version, Error: rpcErr, ID: id})
	return body
}
