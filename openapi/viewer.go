package openapi

import (
	"encoding/json"
	"html"
)

// Viewer renders an OpenAPI web UI.
type Viewer interface {
	HTML(config Config, specURL string) string
}

// ViewerFunc adapts a function to Viewer.
type ViewerFunc func(config Config, specURL string) string

// HTML renders an OpenAPI web UI.
func (fn ViewerFunc) HTML(config Config, specURL string) string {
	if fn == nil {
		return ""
	}
	return fn(config, specURL)
}

// ScalarViewer renders Scalar API Reference from CDN.
func ScalarViewer() Viewer {
	return ViewerFunc(func(config Config, specURL string) string {
		title := config.Title
		if title == "" {
			title = config.Name
		}
		if title == "" {
			title = "API Reference"
		}
		return `<!doctype html><html><head><meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1"><title>` +
			html.EscapeString(title) +
			`</title></head><body><div id="app"></div><script src="https://cdn.jsdelivr.net/npm/@scalar/api-reference"></script><script>Scalar.createApiReference('#app',{url:` +
			jsString(specURL) +
			`})</script></body></html>`
	})
}

func renderViewer(config Config, specURL string) string {
	viewer := config.Viewer
	if viewer == nil {
		viewer = ScalarViewer()
	}
	return viewer.HTML(config, specURL)
}

func jsString(value string) string {
	body, err := json.Marshal(value)
	if err != nil {
		return `""`
	}
	return string(body)
}
