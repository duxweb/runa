package console

import (
	"bytes"
	"context"
	"embed"
	"strings"
	"sync"
	"time"

	"github.com/duxweb/runa/view"
	"github.com/duxweb/runa/view/rhtml"
)

//go:embed templates/*.html
var templateFS embed.FS

var (
	rendererOnce sync.Once
	renderer     = rhtml.New(view.Embed(templateFS, "templates", "*.html"))
	rendererErr  error
)

func renderComponentPanel(ctx context.Context, config Config, panel ComponentPanel) (string, error) {
	rendererOnce.Do(func() { rendererErr = renderer.Load(ctx, nil) })
	if rendererErr != nil {
		return "", rendererErr
	}
	if config.Interval <= 0 {
		config.Interval = 5 * time.Second
	}
	panelConfig := panel.PanelConfig()
	var buffer bytes.Buffer
	if err := renderer.Render(viewContext{ctx: ctx}, &buffer, "panel", map[string]any{
		"Title":       config.Title,
		"PanelName":   panelConfig.Name,
		"PanelTitle":  panelConfig.Title,
		"ConsolePath": strings.TrimRight(config.Mount, "/"),
		"RefreshMS":   int(config.Interval / time.Millisecond),
	}); err != nil {
		return "", err
	}
	return buffer.String(), nil
}

type viewContext struct{ ctx context.Context }

func (ctx viewContext) Context() context.Context { return ctx.ctx }
