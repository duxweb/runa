package message

import (
	"encoding/json"
	"fmt"
)

type jsonCodec struct{}

func JSONCodec() Codec { return jsonCodec{} }

func (jsonCodec) Name() string { return ContentTypeJSON }

func (jsonCodec) Marshal(value any) ([]byte, error) {
	return json.Marshal(value)
}

func (jsonCodec) Unmarshal(data []byte, output any) error {
	return json.Unmarshal(data, output)
}

type rawCodec struct{}

func RawCodec() Codec { return rawCodec{} }

func (rawCodec) Name() string { return ContentTypeRaw }

func (rawCodec) Marshal(value any) ([]byte, error) {
	switch typed := value.(type) {
	case nil:
		return nil, nil
	case []byte:
		return append([]byte(nil), typed...), nil
	case string:
		return []byte(typed), nil
	default:
		return nil, fmt.Errorf("message raw codec expects []byte or string")
	}
}

func (rawCodec) Unmarshal(data []byte, output any) error {
	switch typed := output.(type) {
	case *[]byte:
		*typed = append((*typed)[:0], data...)
		return nil
	case *string:
		*typed = string(data)
		return nil
	default:
		return json.Unmarshal(data, output)
	}
}
