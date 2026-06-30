package core

// ListMeta is the common list metadata contract.
type ListMeta interface {
	ListMeta() Map
}

// PageRequest stores offset pagination input.
type PageRequest struct {
	Page     int
	PageSize int
	Offset   int
	Limit    int
}

// PageMeta stores offset pagination output metadata.
type PageMeta struct {
	Page     int `json:"page"`
	PageSize int `json:"page_size"`
	Total    int `json:"total"`
}

// ListMeta returns page metadata as a map.
func (meta PageMeta) ListMeta() Map {
	return Map{"page": meta.Page, "page_size": meta.PageSize, "total": meta.Total}
}

// ScrollRequest stores cursor pagination input.
type ScrollRequest struct {
	Cursor string
	Limit  int
}

// ScrollMeta stores cursor pagination output metadata.
type ScrollMeta struct {
	Cursor string `json:"cursor,omitempty"`
	Limit  int    `json:"limit"`
	Next   string `json:"next,omitempty"`
}

// ListMeta returns scroll metadata as a map.
func (meta ScrollMeta) ListMeta() Map {
	return Map{"cursor": meta.Cursor, "limit": meta.Limit, "next": meta.Next}
}

// Page stores a paginated response body.
type Page[T any] struct {
	Items []T `json:"items"`
	Meta  Map `json:"meta,omitempty"`
}

// Scroll stores a cursor response body.
type Scroll[T any] struct {
	Items []T `json:"items"`
	Meta  Map `json:"meta,omitempty"`
}
