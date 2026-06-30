package rhtml

import (
	"bytes"
	"context"
	"fmt"
	"html/template"
	"io"
	"io/fs"
	"path/filepath"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/duxweb/runa/view"
)

// New creates an enhanced HTML renderer that can also be used directly.
func New(sources ...view.Source) *Renderer {
	return &Renderer{set: view.Set{Sources: append([]view.Source(nil), sources...)}}
}

// Renderer compiles rhtml tags to html/template and renders templates.
type Renderer struct {
	set     view.Set
	funcs   template.FuncMap
	tags    map[string]TagHandler
	tpl     *template.Template
	files   map[string]view.File
	aliases map[string]string
	loaded  bool
	mu      sync.RWMutex
}

// TagHandler resolves custom r: tag data.
type TagHandler func(context.Context, Props) (any, error)

// Props contains explicit custom tag attributes.
type Props struct {
	values map[string]any
}

// Get returns one prop value.
func (props Props) Get(name string) any { return props.values[name] }

// String returns one prop as string.
func (props Props) String(name string) string {
	value := props.values[name]
	switch typed := value.(type) {
	case string:
		return typed
	case fmt.Stringer:
		return typed.String()
	case nil:
		return ""
	default:
		return fmt.Sprint(typed)
	}
}

// Int returns one prop as int.
func (props Props) Int(name string) int {
	value := props.values[name]
	switch typed := value.(type) {
	case int:
		return typed
	case int8:
		return int(typed)
	case int16:
		return int(typed)
	case int32:
		return int(typed)
	case int64:
		return int(typed)
	case uint:
		return int(typed)
	case uint8:
		return int(typed)
	case uint16:
		return int(typed)
	case uint32:
		return int(typed)
	case uint64:
		return int(typed)
	case float32:
		return int(typed)
	case float64:
		return int(typed)
	case string:
		number, _ := strconv.Atoi(typed)
		return number
	default:
		return 0
	}
}

// Bool returns one prop as bool.
func (props Props) Bool(name string) bool {
	value := props.values[name]
	switch typed := value.(type) {
	case bool:
		return typed
	case string:
		ok, _ := strconv.ParseBool(typed)
		return ok
	default:
		return false
	}
}

// Root returns the original render data.
func (props Props) Root() any { return props.values["root"] }

// Map returns a cloned prop map.
func (props Props) Map() map[string]any {
	output := make(map[string]any, len(props.values))
	for key, value := range props.values {
		if strings.HasPrefix(key, "__rhtml") {
			continue
		}
		output[key] = value
	}
	return output
}

// Func registers a template function.
func (renderer *Renderer) Func(name string, fn any) *Renderer {
	if renderer.funcs == nil {
		renderer.funcs = make(template.FuncMap)
	}
	renderer.funcs[name] = fn
	return renderer
}

// Funcs registers template functions.
func (renderer *Renderer) Funcs(funcs template.FuncMap) *Renderer {
	if renderer.funcs == nil {
		renderer.funcs = make(template.FuncMap)
	}
	for name, fn := range funcs {
		renderer.funcs[name] = fn
	}
	return renderer
}

// Tag registers a custom r: tag.
func (renderer *Renderer) Tag(name string, handler TagHandler) *Renderer {
	if !validIdentifier(name) {
		panic(fmt.Sprintf("rhtml tag %q is not a valid identifier", name))
	}
	if reservedTag(name) {
		panic(fmt.Sprintf("rhtml tag %q is reserved", name))
	}
	if handler == nil {
		panic(fmt.Sprintf("rhtml tag %q handler is nil", name))
	}
	renderer.mu.Lock()
	if renderer.tags == nil {
		renderer.tags = make(map[string]TagHandler)
	}
	renderer.tags[name] = handler
	renderer.loaded = false
	renderer.mu.Unlock()
	return renderer
}

// ViewSet exposes renderer sources to the view registry.
func (renderer *Renderer) ViewSet() view.Set { return renderer.set }

// Load scans sources, compiles rhtml tags, and parses templates.
func (renderer *Renderer) Load(ctx context.Context, set *view.Set) error {
	_ = ctx
	if set != nil {
		renderer.set = *set
	}
	files, raw, aliases, err := readFiles(renderer.set.Sources)
	if err != nil {
		return err
	}
	customTags := renderer.customTagSet()
	tpl := template.New(renderer.set.Name).Option("missingkey=error").Funcs(template.FuncMap{
		"__rhtmlData":     includeData,
		"__rhtmlRoot":     rootData,
		"__rhtmlScope":    scopeData,
		"__rhtmlTagRows":  renderer.renderTagRows,
		"__rhtmlTagValue": renderer.renderTagValue,
	})
	if len(renderer.set.Funcs) > 0 {
		tpl = tpl.Funcs(template.FuncMap(renderer.set.Funcs))
	}
	if len(renderer.set.ContextFuncs) > 0 {
		tpl = tpl.Funcs(buildPlaceholderFuncMap(renderer.set.ContextFuncs))
	}
	if len(renderer.funcs) > 0 {
		tpl = tpl.Funcs(renderer.funcs)
	}
	compiler := compiler{raw: raw, aliases: aliases, tags: customTags}
	for _, file := range files {
		name := filepath.ToSlash(file.Name)
		compiled, err := compiler.compileTemplate(name)
		if err != nil {
			return fmt.Errorf("compile rhtml template %s: %w", name, err)
		}
		if existing := tpl.Lookup(name); existing != nil {
			_, _ = tpl.New(name).Parse("")
		}
		if _, err := tpl.New(name).Parse(compiled); err != nil {
			return fmt.Errorf("parse rhtml template %s: %w", name, err)
		}
	}
	renderer.mu.Lock()
	renderer.tpl = tpl
	renderer.files = fileMap(files)
	renderer.aliases = aliases
	renderer.loaded = true
	renderer.mu.Unlock()
	return nil
}

func (renderer *Renderer) customTagSet() map[string]struct{} {
	renderer.mu.RLock()
	defer renderer.mu.RUnlock()
	items := make(map[string]struct{}, len(renderer.tags))
	for name := range renderer.tags {
		items[name] = struct{}{}
	}
	return items
}

func (renderer *Renderer) tagHandler(name string) TagHandler {
	renderer.mu.RLock()
	handler := renderer.tags[name]
	renderer.mu.RUnlock()
	return handler
}

func (renderer *Renderer) renderTagRows(parent any, name string, as string, pairs ...any) ([]map[string]any, error) {
	result, props, err := renderer.callTag(parent, name, pairs...)
	if err != nil {
		return nil, err
	}
	if as == "" {
		as = "item"
	}
	base := props.Map()
	return tagRows(base, as, result), nil
}

func (renderer *Renderer) renderTagValue(parent any, name string, pairs ...any) (any, error) {
	result, _, err := renderer.callTag(parent, name, pairs...)
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (renderer *Renderer) callTag(parent any, name string, pairs ...any) (any, Props, error) {
	handler := renderer.tagHandler(name)
	if handler == nil {
		return nil, Props{}, fmt.Errorf("rhtml tag %s is not registered", name)
	}
	props := Props{values: includeData(parent, pairs...)}
	result, err := handler(contextData(parent), props)
	if err != nil {
		return nil, props, err
	}
	return result, props, nil
}

// Render renders one compiled template by name.
func (renderer *Renderer) Render(ctx view.Context, writer io.Writer, name string, data any) error {
	if err := renderer.ensureLoaded(ctx.Context()); err != nil {
		return err
	}
	if err := renderer.reloadIfChanged(ctx.Context()); err != nil {
		return err
	}
	renderer.mu.RLock()
	tpl := renderer.tpl
	contextFuncs := cloneContextFuncs(renderer.set.ContextFuncs)
	templateName := renderer.aliases[normalizeName(name)]
	renderer.mu.RUnlock()
	if templateName == "" {
		return fmt.Errorf("template %s is not found", name)
	}
	if len(contextFuncs) > 0 {
		clone, err := tpl.Clone()
		if err != nil {
			return err
		}
		tpl = clone.Funcs(buildContextFuncMap(ctx.Context(), contextFuncs))
	}
	var buffer bytes.Buffer
	if err := tpl.ExecuteTemplate(&buffer, templateName, renderData(ctx.Context(), data)); err != nil {
		return err
	}
	_, err := writer.Write(buffer.Bytes())
	return err
}

func (renderer *Renderer) ensureLoaded(ctx context.Context) error {
	renderer.mu.RLock()
	loaded := renderer.loaded
	renderer.mu.RUnlock()
	if loaded {
		return nil
	}
	return renderer.Load(ctx, &renderer.set)
}

func (renderer *Renderer) reloadIfChanged(ctx context.Context) error {
	renderer.mu.RLock()
	files := make(map[string]view.File, len(renderer.files))
	for name, file := range renderer.files {
		files[name] = file
	}
	sources := append([]view.Source(nil), renderer.set.Sources...)
	renderer.mu.RUnlock()
	if len(files) == 0 {
		return nil
	}
	for _, source := range sources {
		if !source.ReloadEnabled() {
			continue
		}
		items, err := view.Files(source)
		if err != nil {
			return err
		}
		if len(items) != countSourceFiles(files, source) {
			return renderer.Load(ctx, &renderer.set)
		}
		for _, item := range items {
			old := files[item.Name]
			if old.Size != item.Size || !sameModTime(old.ModTime, item.ModTime) {
				return renderer.Load(ctx, &renderer.set)
			}
		}
	}
	return nil
}

func readFiles(sources []view.Source) ([]view.File, map[string]string, map[string]string, error) {
	files := make([]view.File, 0)
	raw := make(map[string]string)
	aliases := make(map[string]string)
	for _, source := range sources {
		items, err := view.Files(source)
		if err != nil {
			return nil, nil, nil, err
		}
		for _, file := range items {
			body, err := fs.ReadFile(file.Source.FS, file.Path)
			if err != nil {
				return nil, nil, nil, err
			}
			name := filepath.ToSlash(file.Name)
			files = append(files, file)
			raw[name] = string(body)
			aliases[name] = name
			aliases[strings.TrimSuffix(name, filepath.Ext(name))] = name
		}
	}
	return files, raw, aliases, nil
}

func fileMap(files []view.File) map[string]view.File {
	items := make(map[string]view.File, len(files))
	for _, file := range files {
		items[file.Name] = file
	}
	return items
}

func countSourceFiles(files map[string]view.File, source view.Source) int {
	count := 0
	for _, file := range files {
		if file.Source.Root == source.Root {
			count++
		}
	}
	return count
}

func sameModTime(a time.Time, b time.Time) bool {
	return a.Equal(b) || a.Truncate(time.Second).Equal(b.Truncate(time.Second))
}

func cloneContextFuncs(funcs map[string]func(context.Context) any) map[string]func(context.Context) any {
	if len(funcs) == 0 {
		return nil
	}
	output := make(map[string]func(context.Context) any, len(funcs))
	for name, fn := range funcs {
		output[name] = fn
	}
	return output
}

func buildContextFuncMap(ctx context.Context, funcs map[string]func(context.Context) any) template.FuncMap {
	output := make(template.FuncMap, len(funcs))
	for name, build := range funcs {
		if build == nil {
			continue
		}
		output[name] = build(ctx)
	}
	return output
}

func buildPlaceholderFuncMap(funcs map[string]func(context.Context) any) template.FuncMap {
	output := make(template.FuncMap, len(funcs))
	for name := range funcs {
		output[name] = func(...any) any { return "" }
	}
	return output
}

func normalizeName(name string) string {
	name = filepath.ToSlash(strings.TrimPrefix(name, "./"))
	return strings.TrimPrefix(name, "/")
}

type compiler struct {
	raw     map[string]string
	aliases map[string]string
	stack   []string
	tags    map[string]struct{}
}

type section struct {
	body string
	args []string
}

func (compiler compiler) compileTemplate(name string) (string, error) {
	body, ok := compiler.raw[name]
	if !ok {
		return "", fmt.Errorf("template %s is not found", name)
	}
	compiler.stack = append([]string(nil), name)
	return compiler.compilePage(name, body, nil)
}

func (compiler compiler) compilePage(name string, body string, sections map[string]section) (string, error) {
	layoutName, layoutAttrs, layoutBody, ok, err := extractLayout(body)
	if err != nil {
		return "", err
	}
	if !ok {
		return compiler.compileContent(name, body, sections)
	}
	layoutFile := compiler.resolveLayout(layoutName)
	if layoutFile == "" {
		return "", fmt.Errorf("template %s layout %s is not found", name, layoutName)
	}
	if contains(compiler.stack, layoutFile) {
		return "", fmt.Errorf("template %s layout cycle: %s", name, layoutFile)
	}
	collected, err := compiler.collectSections(name, layoutBody, sections)
	if err != nil {
		return "", err
	}
	layoutRaw := compiler.raw[layoutFile]
	layoutCompiler := compiler
	layoutCompiler.stack = append(append([]string(nil), compiler.stack...), layoutFile)
	compiled, err := layoutCompiler.compilePage(layoutFile, layoutRaw, collected)
	if err != nil {
		return "", err
	}
	return wrapData(compiled, dataArgsRoot(layoutAttrs)), nil
}

func (compiler compiler) collectSections(name string, body string, overrides map[string]section) (map[string]section, error) {
	sections := make(map[string]section)
	rest, err := replacePairedTag(body, "section", func(attr string, inner string) (string, error) {
		attrs := parseAttrList(attr)
		sectionName := attrValue(attrs, "name")
		if sectionName == "" {
			return "", fmt.Errorf("template %s section name is required", name)
		}
		if _, ok := sections[sectionName]; ok {
			return "", fmt.Errorf("template %s section %s is duplicated", name, sectionName)
		}
		compiled, err := compiler.compileContent(name, inner, overrides)
		if err != nil {
			return "", err
		}
		sections[sectionName] = section{body: compiled, args: dataArgsRoot(attrs)}
		return "", nil
	})
	if err == nil && strings.TrimSpace(rest) != "" {
		err = fmt.Errorf("template %s layout can only contain sections", name)
	}
	return sections, err
}

func (compiler compiler) compileContent(name string, body string, sections map[string]section) (string, error) {
	var err error
	body, err = replaceIncludes(body, func(attrs []attr) (string, error) {
		includeName := attrValue(attrs, "name")
		resolved := compiler.resolve(includeName)
		if resolved == "" {
			return "", fmt.Errorf("template %s include %s is not found", name, includeName)
		}
		args := dataArgsRoot(attrs)
		return fmt.Sprintf(`{{ template %q (__rhtmlData . %s) }}`, resolved, strings.Join(args, " ")), nil
	})
	if err != nil {
		return "", err
	}
	body, err = replacePairedTag(body, "if", func(attr string, inner string) (string, error) {
		attrs := parseAttrs(attr)
		cond := attrs["cond"]
		if cond == "" {
			return "", fmt.Errorf("template %s if cond is required", name)
		}
		compiled, err := compiler.compileContent(name, inner, sections)
		if err != nil {
			return "", err
		}
		return "{{ if " + cond + " }}" + compiled + "{{ end }}", nil
	})
	if err != nil {
		return "", err
	}
	body, err = replacePairedTag(body, "for", func(attr string, inner string) (string, error) {
		attrs := parseAttrs(attr)
		value := attrs["value"]
		as := attrs["as"]
		if value == "" {
			return "", fmt.Errorf("template %s for value is required", name)
		}
		if as == "" {
			as = "item"
		}
		if !validIdentifier(as) {
			return "", fmt.Errorf("template %s for as must be a valid identifier", name)
		}
		compiled, err := compiler.compileContent(name, inner, sections)
		if err != nil {
			return "", err
		}
		compiled = rewriteActionIdent(compiled, as, "$"+as)
		return `{{ $__rhtmlScope := . }}{{ range $` + as + " := " + value + ` }}{{ with __rhtmlScope $__rhtmlScope ` + fmt.Sprintf("%q", as) + " $" + as + " }}" + compiled + "{{ end }}{{ end }}", nil
	})
	if err != nil {
		return "", err
	}
	body, err = replacePairedTag(body, "section", func(attr string, inner string) (string, error) {
		return "", fmt.Errorf("template %s section must be inside layout", name)
	})
	if err != nil {
		return "", err
	}
	body, err = replacePairedTag(body, "block", func(attr string, inner string) (string, error) {
		attrs := parseAttrList(attr)
		blockName := attrValue(attrs, "name")
		if blockName == "" {
			return "", fmt.Errorf("template %s block name is required", name)
		}
		if section, ok := sections[blockName]; ok {
			return wrapData(section.body, mergeArgs(dataArgs(attrs), section.args)), nil
		}
		compiled, err := compiler.compileContent(name, inner, sections)
		if err != nil {
			return "", err
		}
		return wrapData(compiled, dataArgsRoot(attrs)), nil
	})
	if err != nil {
		return "", err
	}
	body, err = compiler.compileCustomTags(name, body, sections)
	if err != nil {
		return "", err
	}
	body = elseRegexp.ReplaceAllString(body, "{{ else }}")
	if unknown := unknownRTag(body); unknown != "" {
		return "", fmt.Errorf("template %s tag %s is not registered", name, unknown)
	}
	return body, nil
}

func (compiler compiler) compileCustomTags(name string, body string, sections map[string]section) (string, error) {
	var out strings.Builder
	for {
		start, tag := compiler.findCustomOpenTag(body, 0)
		if start < 0 {
			out.WriteString(body)
			return out.String(), nil
		}
		out.WriteString(body[:start])
		openEnd := strings.IndexByte(body[start:], '>')
		if openEnd < 0 {
			return "", fmt.Errorf("r:%s opening tag is not closed", tag)
		}
		openEnd += start
		attrText := body[start+len("<r:"+tag) : openEnd]
		attrs := parseAttrList(attrText)
		if strings.HasSuffix(strings.TrimSpace(attrText), "/") {
			compiled, scoped, err := compileCustomTagValue(name, tag, attrs)
			if err != nil {
				return "", err
			}
			out.WriteString(compiled)
			if scoped {
				deferEnd := "{{ end }}"
				body = body[openEnd+1:] + deferEnd
			} else {
				body = body[openEnd+1:]
			}
			continue
		}
		closeStart, closeEnd, err := findCloseTag(body, tag, openEnd+1)
		if err != nil {
			return "", err
		}
		inner := body[openEnd+1 : closeStart]
		compiled, err := compiler.compileContent(name, inner, sections)
		if err != nil {
			return "", err
		}
		as := attrValue(attrs, "as")
		if as == "" {
			as = attrValue(attrs, "item")
		}
		if as == "" {
			as = "item"
		}
		if !validIdentifier(as) {
			return "", fmt.Errorf("template %s tag %s as must be a valid identifier", name, tag)
		}
		out.WriteString(compileCustomTagBlock(tag, as, attrs, compiled))
		body = body[closeEnd:]
	}
}

func (compiler compiler) resolve(name string) string {
	name = normalizeName(name)
	if value := compiler.aliases[name]; value != "" {
		return value
	}
	return ""
}

func (compiler compiler) resolveLayout(name string) string {
	if value := compiler.resolve(name); value != "" {
		return value
	}
	return compiler.resolve("layouts/" + name)
}

func (compiler compiler) findCustomOpenTag(body string, offset int) (int, string) {
	for offset < len(body) {
		index := strings.Index(body[offset:], "<r:")
		if index < 0 {
			return -1, ""
		}
		index += offset
		nameStart := index + len("<r:")
		nameEnd := nameStart
		for nameEnd < len(body) && isIdentifierByte(body[nameEnd]) {
			nameEnd++
		}
		if nameEnd == nameStart {
			offset = nameStart
			continue
		}
		tag := body[nameStart:nameEnd]
		if _, ok := compiler.tags[tag]; !ok {
			offset = nameEnd
			continue
		}
		if nameEnd >= len(body) || body[nameEnd] == '>' || body[nameEnd] == '/' || body[nameEnd] == ' ' || body[nameEnd] == '\t' || body[nameEnd] == '\n' || body[nameEnd] == '\r' {
			return index, tag
		}
		offset = nameEnd
	}
	return -1, ""
}

func extractLayout(body string) (string, []attr, string, bool, error) {
	var layoutName string
	var layoutAttrs []attr
	var layoutBody string
	found := false
	rest, err := replacePairedTag(body, "layout", func(attr string, inner string) (string, error) {
		if found {
			return "", fmt.Errorf("only one r:layout is allowed")
		}
		attrs := parseAttrList(attr)
		layoutName = attrValue(attrs, "name")
		if layoutName == "" {
			return "", fmt.Errorf("layout name is required")
		}
		layoutAttrs = attrs
		layoutBody = inner
		found = true
		return "", nil
	})
	if err == nil && found && strings.TrimSpace(rest) != "" {
		err = fmt.Errorf("layout must wrap the whole template")
	}
	return layoutName, layoutAttrs, layoutBody, found, err
}

type attr struct {
	name  string
	value string
}

var attrRegexp = regexp.MustCompile(`([A-Za-z_][A-Za-z0-9_:-]*)\s*=\s*("([^"]*)"|'([^']*)')`)
var elseRegexp = regexp.MustCompile(`(?s)<r:else\s*/?>`)

func parseAttrs(raw string) map[string]string {
	attrs := make(map[string]string)
	for _, item := range parseAttrList(raw) {
		attrs[item.name] = item.value
	}
	return attrs
}

func parseAttrList(raw string) []attr {
	items := make([]attr, 0)
	for _, match := range attrRegexp.FindAllStringSubmatch(raw, -1) {
		value := match[3]
		if value == "" {
			value = match[4]
		}
		items = append(items, attr{name: match[1], value: value})
	}
	return items
}

func attrValue(attrs []attr, name string) string {
	for _, attr := range attrs {
		if attr.name == name {
			return attr.value
		}
	}
	return ""
}

func replaceIncludes(body string, fn func([]attr) (string, error)) (string, error) {
	var out strings.Builder
	for {
		start := findOpenTag(body, "include", 0)
		if start < 0 {
			out.WriteString(body)
			return out.String(), nil
		}
		out.WriteString(body[:start])
		openEnd := strings.IndexByte(body[start:], '>')
		if openEnd < 0 {
			return "", fmt.Errorf("r:include opening tag is not closed")
		}
		openEnd += start
		attr := body[start+len("<r:include") : openEnd]
		if !strings.HasSuffix(strings.TrimSpace(attr), "/") {
			return "", fmt.Errorf("r:include must be self closing")
		}
		attrs := parseAttrList(attr)
		if name := attrValue(attrs, "name"); name == "" {
			return "", fmt.Errorf("include name is required")
		}
		replacement, err := fn(attrs)
		if err != nil {
			return "", err
		}
		out.WriteString(replacement)
		body = body[openEnd+1:]
	}
}

func replacePairedTag(body string, tag string, fn func(attr string, inner string) (string, error)) (string, error) {
	var out strings.Builder
	for {
		start := findOpenTag(body, tag, 0)
		if start < 0 {
			out.WriteString(body)
			return out.String(), nil
		}
		out.WriteString(body[:start])
		openEnd := strings.IndexByte(body[start:], '>')
		if openEnd < 0 {
			return "", fmt.Errorf("r:%s opening tag is not closed", tag)
		}
		openEnd += start
		attr := body[start+len("<r:"+tag) : openEnd]
		closeStart, closeEnd, err := findCloseTag(body, tag, openEnd+1)
		if err != nil {
			return "", err
		}
		inner := body[openEnd+1 : closeStart]
		replacement, err := fn(attr, inner)
		if err != nil {
			return "", err
		}
		out.WriteString(replacement)
		body = body[closeEnd:]
	}
}

func findOpenTag(body string, tag string, offset int) int {
	needle := "<r:" + tag
	for {
		index := strings.Index(body[offset:], needle)
		if index < 0 {
			return -1
		}
		index += offset
		next := index + len(needle)
		if next >= len(body) || body[next] == '>' || body[next] == '/' || body[next] == ' ' || body[next] == '\t' || body[next] == '\n' || body[next] == '\r' {
			return index
		}
		offset = next
	}
}

func findCloseTag(body string, tag string, offset int) (int, int, error) {
	closeTag := "</r:" + tag + ">"
	depth := 1
	for offset < len(body) {
		nextOpen := findOpenTag(body, tag, offset)
		nextClose := strings.Index(body[offset:], closeTag)
		if nextClose >= 0 {
			nextClose += offset
		}
		if nextClose < 0 {
			return 0, 0, fmt.Errorf("r:%s closing tag is missing", tag)
		}
		if nextOpen >= 0 && nextOpen < nextClose {
			openEnd := strings.IndexByte(body[nextOpen:], '>')
			if openEnd < 0 {
				return 0, 0, fmt.Errorf("r:%s opening tag is not closed", tag)
			}
			if !strings.HasSuffix(strings.TrimSpace(body[nextOpen:nextOpen+openEnd]), "/") {
				depth++
			}
			offset = nextOpen + openEnd + 1
			continue
		}
		depth--
		if depth == 0 {
			return nextClose, nextClose + len(closeTag), nil
		}
		offset = nextClose + len(closeTag)
	}
	return 0, 0, fmt.Errorf("r:%s closing tag is missing", tag)
}

func rewriteActionIdent(body string, ident string, replacement string) string {
	actionRegexp := regexp.MustCompile(`(?s){{(.*?)}}`)
	return actionRegexp.ReplaceAllStringFunc(body, func(action string) string {
		if strings.HasPrefix(strings.TrimSpace(strings.TrimPrefix(strings.TrimSuffix(action, "}}"), "{{")), "/*") {
			return action
		}
		return rewriteIdentOutsideStrings(action, ident, replacement)
	})
}
func rewriteIdentOutsideStrings(input string, ident string, replacement string) string {
	var out strings.Builder
	for i := 0; i < len(input); {
		switch input[i] {
		case '"', '\'', '`':
			end := scanQuoted(input, i)
			out.WriteString(input[i:end])
			i = end
		default:
			if identifierAt(input, i, ident) && !qualifiedIdentifier(input, i) {
				out.WriteString(replacement)
				i += len(ident)
				continue
			}
			out.WriteByte(input[i])
			i++
		}
	}
	return out.String()
}

func scanQuoted(input string, start int) int {
	quote := input[start]
	for i := start + 1; i < len(input); i++ {
		if quote != '`' && input[i] == '\\' {
			i++
			continue
		}
		if input[i] == quote {
			return i + 1
		}
	}
	return len(input)
}

func identifierAt(input string, offset int, ident string) bool {
	if offset+len(ident) > len(input) || input[offset:offset+len(ident)] != ident {
		return false
	}
	before := byte(0)
	after := byte(0)
	if offset > 0 {
		before = input[offset-1]
	}
	if offset+len(ident) < len(input) {
		after = input[offset+len(ident)]
	}
	return !isIdentifierByte(before) && !isIdentifierByte(after)
}

func qualifiedIdentifier(input string, offset int) bool {
	if offset == 0 {
		return false
	}
	switch input[offset-1] {
	case '.', '$':
		return true
	default:
		return false
	}
}

func isIdentifierByte(value byte) bool {
	return value == '_' || value >= 'a' && value <= 'z' || value >= 'A' && value <= 'Z' || value >= '0' && value <= '9'
}

func validIdentifier(value string) bool {
	if value == "" {
		return false
	}
	for index := 0; index < len(value); index++ {
		char := value[index]
		if index == 0 {
			if !(char == '_' || char >= 'a' && char <= 'z' || char >= 'A' && char <= 'Z') {
				return false
			}
			continue
		}
		if !isIdentifierByte(char) {
			return false
		}
	}
	return true
}

func dataArgs(attrs []attr) []string {
	args := make([]string, 0, len(attrs)*2)
	for _, attr := range attrs {
		if attr.name == "name" || attr.name == "as" {
			continue
		}
		args = append(args, fmt.Sprintf("%q", attr.name), attrExpression(attr.value))
	}
	return args
}

func dataArgsRoot(attrs []attr) []string {
	args := dataArgs(attrs)
	for index := 0; index < len(args); index += 2 {
		if args[index] == `"root"` || args[index] == `"Root"` {
			return args
		}
	}
	return append([]string{`"root"`, "(__rhtmlRoot .)"}, args...)
}

func mergeArgs(base []string, overrides []string) []string {
	if len(base) == 0 {
		return append([]string(nil), overrides...)
	}
	if len(overrides) == 0 {
		return append([]string(nil), base...)
	}
	output := make([]string, 0, len(base)+len(overrides))
	seen := make(map[string]struct{}, len(overrides)/2)
	for index := 0; index+1 < len(overrides); index += 2 {
		seen[overrides[index]] = struct{}{}
	}
	for index := 0; index+1 < len(base); index += 2 {
		if _, ok := seen[base[index]]; ok {
			continue
		}
		output = append(output, base[index], base[index+1])
	}
	output = append(output, overrides...)
	return output
}

func wrapData(body string, args []string) string {
	if len(args) == 0 {
		return body
	}
	return "{{ with __rhtmlData . " + strings.Join(args, " ") + " }}" + body + "{{ end }}"
}

func compileCustomTagValue(templateName string, name string, attrs []attr) (string, bool, error) {
	args := dataArgsRoot(attrs)
	as := attrValue(attrs, "as")
	if as == "" {
		return fmt.Sprintf(`{{ __rhtmlTagValue . %q %s }}`, name, strings.Join(args, " ")), false, nil
	}
	if !validIdentifier(as) {
		return "", false, fmt.Errorf("template %s tag %s as must be a valid identifier", templateName, name)
	}
	return fmt.Sprintf(`{{ with __rhtmlScope . %q (__rhtmlTagValue . %q %s) }}`, as, name, strings.Join(args, " ")), true, nil
}

func compileCustomTagBlock(name string, as string, attrs []attr, body string) string {
	args := dataArgsRoot(attrs)
	return fmt.Sprintf(`{{ range $__rhtmlRow := __rhtmlTagRows . %q %q %s }}{{ with $__rhtmlRow }}%s{{ end }}{{ end }}`, name, as, strings.Join(args, " "), body)
}

func attrExpression(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return `""`
	}
	if isTemplateExpression(value) {
		return value
	}
	return fmt.Sprintf("%q", value)
}

func isTemplateExpression(value string) bool {
	if value == "true" || value == "false" || value == "nil" {
		return true
	}
	first := value[0]
	return first == '.' || first == '$' || first == '(' || first == '-' || first >= '0' && first <= '9'
}

func includeData(parent any, pairs ...any) map[string]any {
	data := make(map[string]any, len(pairs)/2+1)
	data["root"] = rootData(parent)
	data["Root"] = data["root"]
	if ctx := rhtmlContext(parent); ctx != nil {
		data["__rhtmlContext"] = ctx
	}
	for index := 0; index+1 < len(pairs); index += 2 {
		key, ok := pairs[index].(string)
		if !ok || key == "" {
			continue
		}
		data[key] = pairs[index+1]
	}
	return data
}

func scopeData(parent any, pairs ...any) map[string]any {
	data := cloneData(parent)
	root := rootData(parent)
	if _, ok := data["root"]; !ok {
		data["root"] = root
	}
	if _, ok := data["Root"]; !ok {
		data["Root"] = root
	}
	if ctx := rhtmlContext(parent); ctx != nil {
		data["__rhtmlContext"] = ctx
	}
	for index := 0; index+1 < len(pairs); index += 2 {
		key, ok := pairs[index].(string)
		if !ok || key == "" {
			continue
		}
		data[key] = pairs[index+1]
	}
	return data
}

func rootData(parent any) any {
	if data, ok := parent.(map[string]any); ok {
		if root, ok := data["root"]; ok {
			return root
		}
		if root, ok := data["Root"]; ok {
			return root
		}
	}
	return parent
}

func renderData(ctx context.Context, data any) map[string]any {
	output := cloneData(data)
	output["root"] = data
	output["Root"] = data
	output["__rhtmlContext"] = ctx
	return output
}

func contextData(data any) context.Context {
	if ctx := rhtmlContext(data); ctx != nil {
		return ctx
	}
	return context.Background()
}

func rhtmlContext(data any) context.Context {
	if values, ok := data.(map[string]any); ok {
		if ctx, ok := values["__rhtmlContext"].(context.Context); ok && ctx != nil {
			return ctx
		}
	}
	return nil
}

func tagRows(base map[string]any, as string, result any) []map[string]any {
	if result == nil {
		return nil
	}
	value := reflect.ValueOf(result)
	if !value.IsValid() {
		return nil
	}
	if value.Kind() == reflect.Pointer {
		if value.IsNil() {
			return nil
		}
		value = value.Elem()
	}
	switch value.Kind() {
	case reflect.Slice, reflect.Array:
		rows := make([]map[string]any, 0, value.Len())
		for index := 0; index < value.Len(); index++ {
			rows = append(rows, tagRow(base, as, value.Index(index).Interface()))
		}
		return rows
	default:
		return []map[string]any{tagRow(base, as, result)}
	}
}

func tagRow(base map[string]any, as string, value any) map[string]any {
	row := make(map[string]any, len(base)+1)
	for key, item := range base {
		row[key] = item
	}
	row[as] = value
	return row
}

func cloneData(data any) map[string]any {
	output := make(map[string]any)
	switch typed := data.(type) {
	case map[string]any:
		for key, value := range typed {
			output[key] = value
		}
	case map[string]string:
		for key, value := range typed {
			output[key] = value
		}
	default:
		if data != nil {
			output["value"] = data
		}
	}
	return output
}

func contains(items []string, value string) bool {
	for _, item := range items {
		if item == value {
			return true
		}
	}
	return false
}

func reservedTag(name string) bool {
	switch name {
	case "layout", "section", "block", "include", "if", "else", "for":
		return true
	default:
		return false
	}
}

func unknownRTag(body string) string {
	for offset := 0; offset < len(body); {
		index := strings.Index(body[offset:], "<r:")
		if index < 0 {
			return ""
		}
		index += offset
		nameStart := index + len("<r:")
		nameEnd := nameStart
		for nameEnd < len(body) && isIdentifierByte(body[nameEnd]) {
			nameEnd++
		}
		if nameEnd > nameStart {
			return body[nameStart:nameEnd]
		}
		offset = nameStart
	}
	return ""
}
