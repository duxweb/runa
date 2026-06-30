package view

import (
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

const DefaultPattern = "**/*.{html,tmpl,tpl}"

// Source describes one template or asset source.
type Source struct {
	FS       fs.FS
	Root     string
	Patterns []string
	reload   bool
	dev      *Source
}

// Dir creates a local directory source.
func Dir(root string, patterns ...string) Source {
	return Source{FS: os.DirFS(root), Root: ".", Patterns: normalizePatterns(patterns)}
}

// Embed creates an embedded filesystem source.
func Embed(source fs.FS, root string, patterns ...string) Source {
	if source == nil {
		source = os.DirFS(".")
	}
	return Source{FS: source, Root: cleanRoot(root), Patterns: normalizePatterns(patterns)}
}

// Reload sets whether this source reloads before render.
func (source Source) Reload(value bool) Source {
	source.reload = value
	return source
}

// Dev sets a development override source.
func (source Source) Dev(dev Source) Source {
	source.dev = &dev
	return source
}

// UseDev returns the development source when configured.
func (source Source) UseDev() Source {
	if source.dev != nil {
		dev := *source.dev
		if source.reload {
			dev.reload = true
		}
		return dev
	}
	return source
}

// ReloadEnabled reports whether reload is enabled.
func (source Source) ReloadEnabled() bool {
	return source.reload
}

// Files scans matching files from a source.
func Files(source Source) ([]File, error) {
	root := cleanRoot(source.Root)
	patterns := normalizePatterns(source.Patterns)
	var items []File
	err := fs.WalkDir(source.FS, root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		relative := strings.TrimPrefix(strings.TrimPrefix(path, root), "/")
		if root == "." {
			relative = path
		}
		if !matchAny(filepath.ToSlash(relative), patterns) {
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		items = append(items, File{
			Name:    filepath.ToSlash(relative),
			Path:    path,
			Size:    info.Size(),
			ModTime: info.ModTime(),
			Source:  source,
		})
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.SliceStable(items, func(i, j int) bool { return items[i].Name < items[j].Name })
	return items, nil
}

// File describes one matched source file.
type File struct {
	Name    string
	Path    string
	Size    int64
	ModTime time.Time
	Source  Source
}

func normalizePatterns(patterns []string) []string {
	if len(patterns) == 0 {
		return []string{DefaultPattern}
	}
	output := make([]string, 0, len(patterns))
	for _, pattern := range patterns {
		if pattern != "" {
			output = append(output, filepath.ToSlash(pattern))
		}
	}
	if len(output) == 0 {
		return []string{DefaultPattern}
	}
	return output
}

func cleanRoot(root string) string {
	root = filepath.ToSlash(strings.TrimSpace(root))
	if root == "" || root == "/" {
		return "."
	}
	return strings.Trim(strings.TrimPrefix(root, "./"), "/")
}

func matchAny(name string, patterns []string) bool {
	name = filepath.ToSlash(strings.TrimPrefix(name, "./"))
	for _, pattern := range patterns {
		if matchPattern(name, pattern) {
			return true
		}
	}
	return false
}

func matchPattern(name string, pattern string) bool {
	pattern = filepath.ToSlash(strings.TrimPrefix(pattern, "./"))
	if strings.HasPrefix(pattern, "**/") {
		if matchPattern(name, strings.TrimPrefix(pattern, "**/")) {
			return true
		}
	}
	regex := patternRegexp(pattern)
	return regex.MatchString(name)
}

func patternRegexp(pattern string) *regexp.Regexp {
	var builder strings.Builder
	builder.WriteString("^")
	for i := 0; i < len(pattern); {
		switch pattern[i] {
		case '*':
			if i+1 < len(pattern) && pattern[i+1] == '*' {
				builder.WriteString(".*")
				i += 2
			} else {
				builder.WriteString("[^/]*")
				i++
			}
		case '?':
			builder.WriteString("[^/]")
			i++
		case '{':
			end := strings.IndexByte(pattern[i:], '}')
			if end <= 0 {
				builder.WriteString(regexp.QuoteMeta(pattern[i : i+1]))
				i++
				continue
			}
			group := pattern[i+1 : i+end]
			parts := strings.Split(group, ",")
			builder.WriteString("(")
			for index, part := range parts {
				if index > 0 {
					builder.WriteString("|")
				}
				builder.WriteString(regexp.QuoteMeta(part))
			}
			builder.WriteString(")")
			i += end + 1
		default:
			builder.WriteString(regexp.QuoteMeta(pattern[i : i+1]))
			i++
		}
	}
	builder.WriteString("$")
	return regexp.MustCompile(builder.String())
}
