package view

import (
	"context"
	"fmt"

	runalang "github.com/duxweb/runa/lang"
	"github.com/duxweb/runa/provider"
	runaview "github.com/duxweb/runa/view"
)

type providerImpl struct {
	provider.Base
	name string
}

// Provider injects the request-scoped translation function into view templates.
func Provider(options ...Option) provider.Provider {
	config := optionConfig{name: "t"}
	for _, option := range options {
		if option != nil {
			option(&config)
		}
	}
	return providerImpl{name: config.name}
}

func (providerImpl) Name() string { return "lang.view" }

func (item providerImpl) Register(ctx provider.Context) error {
	views, err := provider.Invoke[*runaview.Registry](ctx)
	if err != nil {
		return err
	}
	name := item.name
	if name == "" {
		name = "t"
	}
	views.ContextFunc(name, func(ctx context.Context) any {
		translator := runalang.From(ctx)
		return func(key string, kv ...any) string {
			return translator.T(key, normalizePairs(kv...)...)
		}
	})
	return nil
}

func normalizePairs(values ...any) []any {
	if len(values) == 1 {
		return values
	}
	if len(values)%2 == 0 {
		return values
	}
	items := make([]any, 0, len(values))
	for i := 0; i+1 < len(values); i += 2 {
		items = append(items, fmt.Sprint(values[i]), values[i+1])
	}
	return items
}
