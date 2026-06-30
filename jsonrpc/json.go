package jsonrpc

import "encoding/json"

func marshalWithoutNewline(value any) ([]byte, error) {
	return json.Marshal(value)
}

func unmarshal(body []byte, output any) error {
	return json.Unmarshal(body, output)
}
