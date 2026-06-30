package console

import (
	"sort"
	"strings"

	"github.com/duxweb/runa/route"
)

// Panel is a console extension panel.
type Panel interface {
	PanelConfig() PanelConfig
	Routes(group *route.Group)
}

// PanelConfig describes one console panel.
type PanelConfig struct {
	Name  string
	Title string
	Icon  string
	Order int
}

// PanelFunc adapts a function to Panel.
type PanelFunc struct {
	Name  string
	Title string
	Icon  string
	Order int
	Mount func(group *route.Group)
}

// PanelConfig returns panel metadata.
func (panel PanelFunc) PanelConfig() PanelConfig {
	return normalizePanelConfig(PanelConfig{
		Name:  panel.Name,
		Title: panel.Title,
		Icon:  panel.Icon,
		Order: panel.Order,
	})
}

// Routes mounts panel routes.
func (panel PanelFunc) Routes(group *route.Group) {
	if panel.Mount != nil {
		panel.Mount(group)
	}
}

// PanelInfo describes a mounted console panel.
type PanelInfo struct {
	Name  string `json:"name"`
	Title string `json:"title"`
	Icon  string `json:"icon,omitempty"`
	Path  string `json:"path"`
	Order int    `json:"order,omitempty"`
}

func panelInfo(config Config) []PanelInfo {
	items := make([]PanelInfo, 0, len(config.Panels))
	base := strings.TrimRight(config.Mount, "/")
	panels := append([]Panel(nil), config.Panels...)
	sort.SliceStable(panels, func(i, j int) bool { return panelOrder(panels[i]) < panelOrder(panels[j]) })
	for _, panel := range panels {
		meta := panelConfig(panel)
		if meta.Name == "" {
			continue
		}
		path := meta.Name
		if config.Mount != "" {
			path = "/" + meta.Name
			if base != "" {
				path = base + path
			}
		}
		items = append(items, PanelInfo{
			Name:  meta.Name,
			Title: meta.Title,
			Icon:  meta.Icon,
			Path:  path,
			Order: meta.Order,
		})
	}
	return items
}

func panelConfig(panel Panel) PanelConfig {
	if panel == nil {
		return PanelConfig{}
	}
	return normalizePanelConfig(panel.PanelConfig())
}

func normalizePanelConfig(config PanelConfig) PanelConfig {
	config.Name = strings.Trim(strings.TrimSpace(config.Name), "/")
	if config.Title == "" {
		config.Title = titleize(config.Name)
	}
	if config.Icon == "" {
		config.Icon = "□"
	}
	return config
}

func panelOrder(panel Panel) int {
	return panelConfig(panel).Order
}

func titleize(value string) string {
	value = strings.TrimSpace(strings.ReplaceAll(value, "-", " "))
	if value == "" {
		return ""
	}
	parts := strings.Fields(value)
	for index, part := range parts {
		if len(part) == 0 {
			continue
		}
		parts[index] = strings.ToUpper(part[:1]) + part[1:]
	}
	return strings.Join(parts, " ")
}
