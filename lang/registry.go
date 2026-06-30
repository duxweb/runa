package lang

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/BurntSushi/toml"
	"github.com/duxweb/runa/core"
	runaprovider "github.com/duxweb/runa/provider"
	"github.com/nicksnyder/go-i18n/v2/i18n"
	"github.com/samber/do/v2"
	"golang.org/x/text/language"
)

type contextKey struct{}

// Registry stores translations and creates request-scoped translators.
type Registry struct {
	bundle   *i18n.Bundle
	defaults []string
	mu       sync.RWMutex
}

// New creates an i18n registry.
func New(options ...Option) *Registry {
	config := optionConfig{defaultLocale: "en"}
	for _, option := range options {
		if option != nil {
			option(&config)
		}
	}
	registry := &Registry{}
	registry.reset(config.defaultLocale)
	return registry
}

func (registry *Registry) reset(defaultLocale string) {
	defaultLocale = clean(defaultLocale)
	if defaultLocale == "" {
		defaultLocale = "en"
	}
	tag, err := language.Parse(defaultLocale)
	if err != nil {
		tag = language.English
	}
	bundle := i18n.NewBundle(tag)
	bundle.RegisterUnmarshalFunc("toml", toml.Unmarshal)
	bundle.RegisterUnmarshalFunc("json", json.Unmarshal)
	registry.bundle = bundle
	registry.defaults = []string{defaultLocale}
}

// LoadFile loads a TOML or JSON message file.
func (registry *Registry) LoadFile(path string) error {
	if registry == nil || path == "" {
		return nil
	}
	registry.mu.Lock()
	defer registry.mu.Unlock()
	_, err := registry.bundle.LoadMessageFile(path)
	return err
}

// LoadDir loads message files from a directory.
func (registry *Registry) LoadDir(dir string) error {
	if registry == nil || dir == "" {
		return nil
	}
	files, err := filepath.Glob(filepath.Join(dir, "*.*"))
	if err != nil {
		return err
	}
	sort.Strings(files)
	for _, file := range files {
		ext := strings.ToLower(filepath.Ext(file))
		if ext != ".toml" && ext != ".json" {
			continue
		}
		if err := registry.LoadFile(file); err != nil {
			return err
		}
	}
	return nil
}

// Match returns the best matching locale for language preferences.
func (registry *Registry) Match(values ...string) string {
	if registry == nil {
		return ""
	}
	registry.mu.RLock()
	bundle := registry.bundle
	defaults := append([]string(nil), registry.defaults...)
	registry.mu.RUnlock()
	return matchLocale(bundle, append(cleanList(values), defaults...), defaults)
}

func matchLocale(bundle *i18n.Bundle, preferences []string, defaults []string) string {
	if bundle == nil {
		if len(defaults) > 0 {
			return defaults[0]
		}
		return ""
	}
	available := bundle.LanguageTags()
	if len(available) == 0 {
		if len(defaults) > 0 {
			return defaults[0]
		}
		return ""
	}
	preferred := parsePreferences(preferences)
	matcher := language.NewMatcher(available)
	_, index, confidence := matcher.Match(preferred...)
	if confidence == language.No {
		if len(defaults) > 0 {
			return defaults[0]
		}
		return ""
	}
	return available[index].String()
}

// Translator creates a translator for locale preferences.
func (registry *Registry) Translator(values ...string) *Translator {
	if registry == nil {
		return NewTranslator(values...)
	}
	registry.mu.RLock()
	bundle := registry.bundle
	defaults := append([]string(nil), registry.defaults...)
	registry.mu.RUnlock()
	preferences := append(cleanList(values), defaults...)
	return &Translator{
		localizer: i18n.NewLocalizer(bundle, preferences...),
		locale:    matchLocale(bundle, preferences, defaults),
	}
}

// T translates a key using the registry default locale.
func (registry *Registry) T(key string, params ...any) string {
	return registry.Translator().T(key, params...)
}

// WithTranslator stores a translator in a context.
func WithTranslator(ctx context.Context, translator *Translator) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	if translator == nil {
		return ctx
	}
	return context.WithValue(ctx, contextKey{}, translator)
}

// From reads a translator from context or returns the default translator.
func From(ctx context.Context) *Translator {
	if ctx != nil {
		if translator, ok := ctx.Value(contextKey{}).(*Translator); ok && translator != nil {
			return translator
		}
	}
	if injector := runaprovider.DefaultInjector(); injector != nil {
		if registry, err := do.Invoke[*Registry](injector); err == nil && registry != nil {
			return registry.Translator()
		}
	}
	return NewTranslator()
}

// Translator translates message keys.
type Translator struct {
	localizer *i18n.Localizer
	locale    string
}

// NewTranslator creates a fallback translator without a registry.
func NewTranslator(locales ...string) *Translator {
	return &Translator{locale: first(cleanList(locales))}
}

// Locale returns the translator locale hint.
func (translator *Translator) Locale() string {
	if translator == nil {
		return ""
	}
	return translator.locale
}

// T translates a message key.
func (translator *Translator) T(key string, params ...any) string {
	if key == "" {
		return ""
	}
	data, plural := normalizeParams(params...)
	if translator == nil || translator.localizer == nil {
		return replace(key, data)
	}
	config := &i18n.LocalizeConfig{
		MessageID:    key,
		TemplateData: data,
	}
	if plural != nil {
		config.PluralCount = plural
	}
	message, err := translator.localizer.Localize(config)
	if err != nil {
		return replace(key, data)
	}
	return message
}

func normalizeParams(values ...any) (map[string]any, any) {
	output := make(map[string]any)
	var plural any
	if len(values) == 1 {
		switch typed := values[0].(type) {
		case nil:
			return output, nil
		case core.Map:
			for key, value := range typed {
				output[key] = value
				if strings.EqualFold(key, "count") || strings.EqualFold(key, "pluralcount") {
					plural = value
				}
			}
			return output, plural
		case map[string]any:
			for key, value := range typed {
				output[key] = value
				if strings.EqualFold(key, "count") || strings.EqualFold(key, "pluralcount") {
					plural = value
				}
			}
			return output, plural
		}
	}
	for i := 0; i+1 < len(values); i += 2 {
		key := fmt.Sprint(values[i])
		if key == "" {
			continue
		}
		value := values[i+1]
		output[key] = value
		if strings.EqualFold(key, "count") || strings.EqualFold(key, "pluralcount") {
			plural = value
		}
	}
	return output, plural
}

func replace(message string, params map[string]any) string {
	if message == "" || len(params) == 0 {
		return message
	}
	pairs := make([]string, 0, len(params)*2)
	for key, value := range params {
		pairs = append(pairs, "{"+key+"}", fmt.Sprint(value))
		pairs = append(pairs, "{{."+key+"}}", fmt.Sprint(value))
	}
	return strings.NewReplacer(pairs...).Replace(message)
}

func cleanList(values []string) []string {
	output := make([]string, 0, len(values))
	for _, value := range values {
		value = clean(value)
		if value != "" {
			output = append(output, value)
		}
	}
	return output
}

func parsePreferences(values []string) []language.Tag {
	items := make([]language.Tag, 0, len(values))
	for _, value := range values {
		tags, _, err := language.ParseAcceptLanguage(value)
		if err == nil && len(tags) > 0 {
			items = append(items, tags...)
			continue
		}
		tag, err := language.Parse(value)
		if err == nil {
			items = append(items, tag)
		}
	}
	if len(items) == 0 {
		items = append(items, language.English)
	}
	return items
}

func first(values []string) string {
	for _, value := range values {
		if value = clean(value); value != "" {
			return value
		}
	}
	return ""
}

func clean(value string) string {
	return strings.TrimSpace(value)
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func dirExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}
