package asset

import (
	"bytes"
	"context"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/fs"
	"net/http"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/duxweb/runa/view"
)

// Set stores one asset domain.
type Set struct {
	Sources  []view.Source
	prefix   string
	manifest string
	files    map[string]File
	mu       sync.RWMutex
}

// Assets creates an asset set.
func Assets(sources ...view.Source) *Set {
	return &Set{Sources: append([]view.Source(nil), sources...), prefix: "/assets"}
}

// Prefix sets URL prefix.
func (set *Set) Prefix(prefix string) *Set {
	if prefix != "" {
		set.prefix = "/" + strings.Trim(prefix, "/")
	}
	return set
}

// Manifest sets a manifest JSON file path inside sources.
func (set *Set) Manifest(path string) *Set {
	set.manifest = strings.TrimPrefix(filepath.ToSlash(path), "/")
	return set
}

// Load scans asset files.
func (set *Set) Load(context.Context) error {
	files := make(map[string]File)
	manifest := map[string]string{}
	for _, source := range set.Sources {
		active := source.UseDev()
		items, err := view.Files(active)
		if err != nil {
			return err
		}
		for _, item := range items {
			if item.Name == set.manifest {
				body, err := fs.ReadFile(item.Source.FS, item.Path)
				if err == nil {
					_ = json.Unmarshal(body, &manifest)
				}
				continue
			}
			body, err := fs.ReadFile(item.Source.FS, item.Path)
			if err != nil {
				return err
			}
			hash := shortHash(body)
			files[item.Name] = File{
				Name:    item.Name,
				Path:    item.Path,
				Source:  item.Source,
				Size:    item.Size,
				ModTime: item.ModTime,
				Hash:    hash,
			}
		}
	}
	for name, mapped := range manifest {
		file := files[name]
		file.Manifest = mapped
		files[name] = file
	}
	set.mu.Lock()
	set.files = files
	set.mu.Unlock()
	return nil
}

// URL returns the public URL for an asset path.
func (set *Set) URL(path string) string {
	path = strings.TrimPrefix(filepath.ToSlash(path), "/")
	set.mu.RLock()
	file := set.files[path]
	set.mu.RUnlock()
	if file.Manifest != "" {
		return set.prefix + "/" + strings.TrimPrefix(file.Manifest, "/")
	}
	if file.Hash != "" {
		return set.prefix + "/" + path + "?v=" + file.Hash
	}
	return set.prefix + "/" + path
}

// Handler returns an HTTP handler serving this asset set.
func (set *Set) Handler() http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		name := strings.TrimPrefix(request.URL.Path, set.prefix)
		name = strings.TrimPrefix(filepath.ToSlash(name), "/")
		set.mu.RLock()
		file := set.files[name]
		set.mu.RUnlock()
		if file.Name == "" {
			http.NotFound(writer, request)
			return
		}
		body, err := fs.ReadFile(file.Source.FS, file.Path)
		if err != nil {
			http.NotFound(writer, request)
			return
		}
		etag := `"` + file.Hash + `"`
		writer.Header().Set("ETag", etag)
		writer.Header().Set("Last-Modified", file.ModTime.UTC().Format(http.TimeFormat))
		if strings.Contains(file.Name, "."+file.Hash+".") {
			writer.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		} else {
			writer.Header().Set("Cache-Control", "public, max-age=300")
		}
		if request.Header.Get("If-None-Match") == etag {
			writer.WriteHeader(http.StatusNotModified)
			return
		}
		http.ServeContent(writer, request, file.Name, file.ModTime, bytes.NewReader(body))
	})
}

// Info returns file snapshots.
func (set *Set) Info() []File {
	set.mu.RLock()
	defer set.mu.RUnlock()
	items := make([]File, 0, len(set.files))
	for _, file := range set.files {
		items = append(items, file)
	}
	sort.SliceStable(items, func(i, j int) bool { return items[i].Name < items[j].Name })
	return items
}

// File describes one asset file.
type File struct {
	Name     string
	Path     string
	Source   view.Source
	Size     int64
	ModTime  time.Time
	Hash     string
	Manifest string
}

func shortHash(body []byte) string {
	sum := sha1.Sum(body)
	return hex.EncodeToString(sum[:])[:8]
}

// Register adds or replaces an asset domain.
func (registry *Registry) Register(ctx context.Context, name string, set *Set) error {
	if name == "" {
		return fmt.Errorf("asset name is required")
	}
	if set == nil {
		return fmt.Errorf("asset %s set is required", name)
	}
	if err := set.Load(ctx); err != nil {
		return err
	}
	registry.entries.Register(name, set)
	return nil
}

// URL returns asset URL by domain.
func (registry *Registry) URL(domain string, path string) string {
	set, ok := registry.entries.Entry(domain)
	if !ok || set == nil {
		return path
	}
	return set.URL(path)
}

// Handler returns one asset domain handler.
func (registry *Registry) Handler(domain string) http.Handler {
	set, ok := registry.entries.Entry(domain)
	if !ok || set == nil {
		return http.NotFoundHandler()
	}
	return set.Handler()
}

// Info returns domain names.
func (registry *Registry) Info() []Info {
	entries := registry.entries.All()
	fallback := registry.entries.Fallback()
	items := make([]Info, 0, len(entries))
	for name, set := range entries {
		items = append(items, Info{Name: name, Default: name == fallback, Files: len(set.files)})
	}
	sort.SliceStable(items, func(i, j int) bool { return items[i].Name < items[j].Name })
	return items
}

// Info describes one asset domain.
type Info struct {
	Name    string
	Default bool
	Files   int
}
