package console

import (
	"context"

	"github.com/duxweb/runa/route"
)

// ComponentResolver resolves component data for a console panel.
type ComponentResolver func(context.Context, AppContext) (any, error)

// ComponentType declares how a console component is rendered.
type ComponentType string

const (
	// ComponentMetric renders a compact stat value.
	ComponentMetric ComponentType = "metric"
	// ComponentStatus renders a status badge/card.
	ComponentStatus ComponentType = "status"
	// ComponentTable renders list-like data as a table.
	ComponentTable ComponentType = "table"
	// ComponentLine renders series data as a line chart.
	ComponentLine ComponentType = "line"
	// ComponentBar renders series data as a bar chart.
	ComponentBar ComponentType = "bar"
	// ComponentJSON renders data as formatted JSON.
	ComponentJSON ComponentType = "json"
)

// Component renders one data block in a console panel.
type Component struct {
	Name    string            `json:"name,omitempty"`
	Label   string            `json:"label,omitempty"`
	Type    ComponentType     `json:"type,omitempty"`
	Resolve ComponentResolver `json:"-"`
}

func (component Component) id() string { return component.Name }
func (component Component) title() string {
	if component.Label != "" {
		return component.Label
	}
	return titleize(component.Name)
}
func (component Component) kind() string {
	if component.Type != "" {
		return string(component.Type)
	}
	return string(ComponentJSON)
}
func (component Component) data(ctx context.Context, app AppContext) (any, error) {
	if component.Resolve == nil {
		return nil, nil
	}
	return component.Resolve(ctx, app)
}

// ComponentPanel is an auto-rendered console panel configured by Go structs.
type ComponentPanel struct {
	Name       string
	Title      string
	Icon       string
	Order      int
	Components []Component
}

// PanelConfig returns panel metadata.
func (panel ComponentPanel) PanelConfig() PanelConfig {
	config := normalizePanelConfig(PanelConfig{
		Name:  panel.Name,
		Title: panel.Title,
		Icon:  panel.Icon,
		Order: panel.Order,
	})
	if panel.Icon == "" {
		config.Icon = "◫"
	}
	return config
}

func (panel ComponentPanel) Routes(group *route.Group) {
	group.Get("/", func(ctx *route.Context) error {
		body, err := renderComponentPanel(ctx.Context(), consoleConfig(ctx), panel)
		if err != nil {
			return err
		}
		return ctx.HTML(body)
	}).SkipDoc()
	group.Get("/api/components", func(ctx *route.Context) error {
		app := consoleApp(ctx)
		items := make([]ComponentInfo, 0, len(panel.Components))
		for _, component := range panel.Components {
			if component.id() == "" {
				continue
			}
			data, err := component.data(ctx.Context(), app)
			item := ComponentInfo{ID: component.id(), Title: component.title(), Kind: component.kind(), Data: data}
			if err != nil {
				item.Error = err.Error()
			}
			items = append(items, item)
		}
		return ctx.JSON(items)
	}).SkipDoc()
}

// ComponentInfo is the serializable component payload.
type ComponentInfo struct {
	ID    string `json:"id"`
	Title string `json:"title"`
	Kind  string `json:"kind"`
	Data  any    `json:"data,omitempty"`
	Error string `json:"error,omitempty"`
}
