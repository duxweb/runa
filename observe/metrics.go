package observe

import (
	"fmt"
	"strings"

	"github.com/duxweb/runa/route"
)

// TextMetrics creates a simple text metrics exporter.
func TextMetrics(lines ...string) Exporter {
	return ExporterFunc(func(ctx *route.Context) error {
		return ctx.Type("text/plain; version=0.0.4; charset=utf-8").Text(strings.Join(lines, "\n"))
	})
}

// DefaultMetrics returns lightweight built-in metrics.
func DefaultMetrics() Exporter {
	return ExporterFunc(func(ctx *route.Context) error {
		body := fmt.Sprintf("runa_info{service=%q} 1\n", "runa")
		return ctx.Type("text/plain; version=0.0.4; charset=utf-8").Text(body)
	})
}
