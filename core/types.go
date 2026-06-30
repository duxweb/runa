package core

import (
	"encoding/json"
	"io"
	"mime/multipart"
)

// Map is Runa's common map type used for metadata, JSON payloads, and options.
type Map map[string]any

// JSONRaw stores raw JSON bytes.
type JSONRaw []byte

// Bytes returns raw JSON bytes.
func (raw JSONRaw) Bytes() []byte { return []byte(raw) }

// String returns raw JSON as string.
func (raw JSONRaw) String() string { return string(raw) }

// Map decodes raw JSON into Map.
func (raw JSONRaw) Map() Map {
	var value Map
	_ = json.Unmarshal(raw, &value)
	return value
}

// UnmarshalJSON keeps the original JSON subtree bytes.
func (raw *JSONRaw) UnmarshalJSON(data []byte) error {
	if raw == nil {
		return nil
	}
	*raw = append((*raw)[:0], data...)
	return nil
}

// MarshalJSON returns raw JSON bytes.
func (raw JSONRaw) MarshalJSON() ([]byte, error) {
	if len(raw) == 0 {
		return []byte("null"), nil
	}
	return raw, nil
}

// Stream describes a raw request body stream.
type Stream struct {
	Reader      io.Reader
	Size        int64
	ContentType string
}

// UploadFile describes an uploaded multipart file.
type UploadFile struct {
	Filename    string
	Size        int64
	ContentType string
	Header      *multipart.FileHeader
}

// Open opens the uploaded file.
func (file UploadFile) Open() (multipart.File, error) {
	return file.Header.Open()
}

// Empty represents an empty request or response body.
type Empty struct{}

// ViewBody represents a template response body.
type ViewBody struct {
	Name string
	Data any
	View string
}

// RenderView creates a template response body.
func RenderView(name string, data any, views ...string) ViewBody {
	body := ViewBody{Name: name, Data: data}
	if len(views) > 0 {
		body.View = views[0]
	}
	return body
}
