package cache

import (
	"bytes"
	"encoding/gob"
	"encoding/json"
	"fmt"
)

type jsonCodec struct{}

// JSONCodec creates a JSON codec.
func JSONCodec() Serializer { return jsonCodec{} }

func (jsonCodec) Marshal(value any) ([]byte, error)      { return json.Marshal(value) }
func (jsonCodec) Unmarshal(data []byte, value any) error { return json.Unmarshal(data, value) }

type gobCodec struct{}

// GobCodec creates a gob codec.
func GobCodec() Serializer { return gobCodec{} }

func (gobCodec) Marshal(value any) ([]byte, error) {
	var buffer bytes.Buffer
	if err := gob.NewEncoder(&buffer).Encode(value); err != nil {
		return nil, err
	}
	return buffer.Bytes(), nil
}

func (gobCodec) Unmarshal(data []byte, value any) error {
	return gob.NewDecoder(bytes.NewReader(data)).Decode(value)
}

type stringCodec struct{}

// StringCodec creates a string codec.
func StringCodec() Serializer { return stringCodec{} }

func (stringCodec) Marshal(value any) ([]byte, error) {
	switch v := value.(type) {
	case string:
		return []byte(v), nil
	case *string:
		if v == nil {
			return nil, nil
		}
		return []byte(*v), nil
	case []byte:
		return v, nil
	default:
		return []byte(fmt.Sprint(value)), nil
	}
}

func (stringCodec) Unmarshal(data []byte, value any) error {
	switch v := value.(type) {
	case *string:
		*v = string(data)
		return nil
	case *[]byte:
		*v = append((*v)[:0], data...)
		return nil
	default:
		return json.Unmarshal(data, value)
	}
}

type bytesCodec struct{}

// BytesCodec creates a byte-slice codec.
func BytesCodec() Serializer { return bytesCodec{} }

func (bytesCodec) Marshal(value any) ([]byte, error) {
	switch v := value.(type) {
	case []byte:
		return append([]byte(nil), v...), nil
	case *[]byte:
		if v == nil {
			return nil, nil
		}
		return append([]byte(nil), (*v)...), nil
	case string:
		return []byte(v), nil
	default:
		return nil, fmt.Errorf("cache bytes codec expects []byte or string, got %T", value)
	}
}

func (bytesCodec) Unmarshal(data []byte, value any) error {
	switch v := value.(type) {
	case *[]byte:
		*v = append((*v)[:0], data...)
		return nil
	case *string:
		*v = string(data)
		return nil
	default:
		return fmt.Errorf("cache bytes codec expects *[]byte or *string, got %T", value)
	}
}
