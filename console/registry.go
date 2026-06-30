package console

import (
	"context"
	"sort"
	"sync"

	runaprovider "github.com/duxweb/runa/provider"
)

// Registry stores console panels registered by framework and app providers.
type Registry struct {
	mu        sync.RWMutex
	panels    map[string]Panel
	order     []string
	summaries []SummaryResolver
	builtins  bool
}

// New creates a console panel registry.
func New() *Registry { return &Registry{panels: make(map[string]Panel)} }

// Register adds or replaces console panels.
func (registry *Registry) Register(panels ...Panel) {
	if registry == nil {
		return
	}
	registry.mu.Lock()
	defer registry.mu.Unlock()
	if registry.panels == nil {
		registry.panels = make(map[string]Panel)
	}
	for _, panel := range panels {
		config := panelConfig(panel)
		if config.Name == "" {
			continue
		}
		if _, exists := registry.panels[config.Name]; !exists {
			registry.order = append(registry.order, config.Name)
		}
		registry.panels[config.Name] = panel
	}
}

// Panels returns registered panels ordered by menu order then registration order.
func (registry *Registry) Panels() []Panel {
	if registry == nil {
		return nil
	}
	registry.mu.RLock()
	defer registry.mu.RUnlock()
	items := make([]Panel, 0, len(registry.order))
	index := make(map[string]int, len(registry.order))
	for i, name := range registry.order {
		index[name] = i
		if panel := registry.panels[name]; panel != nil {
			items = append(items, panel)
		}
	}
	sort.SliceStable(items, func(i, j int) bool {
		left := panelOrder(items[i])
		right := panelOrder(items[j])
		if left == right {
			return index[panelConfig(items[i]).Name] < index[panelConfig(items[j]).Name]
		}
		return left < right
	})
	return items
}

// RegisterSummary adds console summary resolvers.
func (registry *Registry) RegisterSummary(resolvers ...SummaryResolver) {
	if registry == nil {
		return
	}
	registry.mu.Lock()
	defer registry.mu.Unlock()
	for _, resolver := range resolvers {
		if resolver != nil {
			registry.summaries = append(registry.summaries, resolver)
		}
	}
}

// RegisterBuiltinSummaries adds built-in summary resolvers once.
func (registry *Registry) RegisterBuiltinSummaries() {
	if registry == nil {
		return
	}
	registry.mu.Lock()
	if registry.builtins {
		registry.mu.Unlock()
		return
	}
	registry.builtins = true
	registry.mu.Unlock()
	registry.RegisterSummary(BuiltinSummaries()...)
}

// Summaries resolves all registered summary rows.
func (registry *Registry) Summaries(ctx context.Context, app AppContext) []Summary {
	if registry == nil {
		return nil
	}
	registry.mu.RLock()
	resolvers := append([]SummaryResolver(nil), registry.summaries...)
	registry.mu.RUnlock()
	items := make([]Summary, 0)
	for _, resolver := range resolvers {
		items = append(items, resolver(ctx, app)...)
	}
	return items
}

// Register adds panels to the app-scoped console registry.
func Register(app AppContext, panels ...Panel) {
	registry := runaprovider.MustInvoke[*Registry](app)
	registry.Register(panels...)
}

// RegisterSummary adds summary resolvers to the app-scoped console registry.
func RegisterSummary(app AppContext, resolvers ...SummaryResolver) {
	registry := runaprovider.MustInvoke[*Registry](app)
	registry.RegisterSummary(resolvers...)
}

// RegisterBuiltinSummaries adds built-in summary resolvers to the app-scoped registry once.
func RegisterBuiltinSummaries(app AppContext) {
	registry := runaprovider.MustInvoke[*Registry](app)
	registry.RegisterBuiltinSummaries()
}
