package jsonrpc

import (
	"context"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
	"github.com/duxweb/runa/route"
)

// WS returns a Runa handler that serves JSON-RPC 2.0 over WebSocket.
func (server *Server) WS(options ...WSOption) route.Handler {
	config := defaultWSConfig()
	for _, option := range options {
		if option != nil {
			option(&config)
		}
	}
	return func(ctx *route.Context) error {
		conn, err := websocket.Accept(ctx.Response(), ctx.Request(), &websocket.AcceptOptions{OriginPatterns: config.Origin})
		if err != nil {
			return err
		}
		defer conn.CloseNow()
		conn.SetReadLimit(config.MaxMessageSize)
		for {
			var raw any
			readCtx := context.Background()
			cancel := func() {}
			if config.ReadTimeout > 0 {
				readCtx, cancel = context.WithTimeout(readCtx, config.ReadTimeout)
			}
			err := wsjson.Read(readCtx, conn, &raw)
			cancel()
			if err != nil {
				return nil
			}
			body, err := marshalWithoutNewline(raw)
			if err != nil {
				if writeErr := writeWS(conn, config, Response{Version: version, Error: parseError(), ID: nullID}); writeErr != nil {
					return nil
				}
				continue
			}
			payload, ok := server.handleBody(ctx.Context(), body, "ws")
			if !ok {
				continue
			}
			var response any
			if err := unmarshal(payload, &response); err != nil {
				response = Response{Version: version, Error: internalError(err), ID: nullID}
			}
			if err := writeWS(conn, config, response); err != nil {
				return nil
			}
		}
	}
}

func writeWS(conn *websocket.Conn, config WSConfig, value any) error {
	writeCtx := context.Background()
	cancel := func() {}
	if config.WriteTimeout > 0 {
		writeCtx, cancel = context.WithTimeout(writeCtx, config.WriteTimeout)
	}
	err := wsjson.Write(writeCtx, conn, value)
	cancel()
	return err
}
