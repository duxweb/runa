package jsonrpc

import (
	"context"
	"encoding/json"
)

func (server *Server) handleBody(ctx context.Context, body []byte, transport ...string) ([]byte, bool) {
	requests, batch, rpcErr := decodeRequest(body)
	if rpcErr != nil {
		payload, _ := marshal(Response{Version: version, Error: rpcErr, ID: nullID})
		return payload, true
	}
	responses := make([]*Response, 0, len(requests))
	for _, request := range requests {
		if isNotification(request) && validateRequest(request) == nil {
			_ = server.handle(ctx, request, transport...)
			continue
		}
		responses = append(responses, server.handle(ctx, request, transport...))
	}
	if len(responses) == 0 {
		return nil, false
	}
	if batch {
		payload, _ := marshal(responses)
		return payload, true
	}
	payload, _ := marshal(responses[0])
	return payload, true
}

func marshal(value any) ([]byte, error) {
	body, err := json.Marshal(value)
	if err != nil {
		body, err = json.Marshal(Response{Version: version, Error: internalError(err), ID: nullID})
	}
	if err != nil {
		return []byte(`{"jsonrpc":"2.0","error":{"code":-32603,"message":"Internal error"},"id":null}`), nil
	}
	body = append(body, '\n')
	return body, nil
}
