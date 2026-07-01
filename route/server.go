package route

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"runtime/debug"
	"strconv"
	"strings"
	"time"

	"github.com/duxweb/runa/core"
	"github.com/duxweb/runa/host"
)

// ServerConfig configures a route HTTP server.
type ServerConfig struct {
	Name            string
	Addr            string
	ReadTimeout     time.Duration
	WriteTimeout    time.Duration
	IdleTimeout     time.Duration
	ShutdownTimeout time.Duration
}

// BannerConfig configures route startup output.
type BannerConfig struct {
	Enabled *bool
	Writer  io.Writer
	Env     string
}

// Server creates an HTTP host unit backed by this route registry.
func (registry *Registry) Server(config ServerConfig) *host.HTTPServer {
	return host.NewHTTP(host.HTTPConfig{
		Name:            config.Name,
		Addr:            config.Addr,
		Handler:         lazyHandler(func() http.Handler { return registry.Handler() }),
		ReadTimeout:     config.ReadTimeout,
		WriteTimeout:    config.WriteTimeout,
		IdleTimeout:     config.IdleTimeout,
		ShutdownTimeout: config.ShutdownTimeout,
	})
}

func lazyHandler(resolve func() http.Handler) http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		handler := http.NotFoundHandler()
		if resolve != nil {
			if resolved := resolve(); resolved != nil {
				handler = resolved
			}
		}
		handler.ServeHTTP(writer, request)
	})
}

func withStartupBanner(server *host.HTTPServer, registry *Registry, config BannerConfig) host.Unit {
	if config.Enabled != nil && !*config.Enabled {
		return server
	}
	return &startupBannerHost{HTTPServer: server, registry: registry, config: config}
}

type startupBannerHost struct {
	*host.HTTPServer
	registry *Registry
	config   BannerConfig
}

func (item *startupBannerHost) Start(ctx context.Context) error {
	if err := item.HTTPServer.Start(ctx); err != nil {
		return err
	}
	item.print()
	return nil
}

func (item *startupBannerHost) print() {
	writer := item.config.Writer
	if writer == nil {
		writer = io.Discard
	}
	addr := item.Addr()
	baseURL := displayHTTPURL(addr)
	env := strings.TrimSpace(item.config.Env)
	if env == "" {
		env = "local"
	}
	palette := bannerPalette{enabled: core.ColorEnabled(writer)}
	tools := startupTools(item.registry, baseURL)
	entries := []startupEntry{
		{Label: "URL", Value: baseURL, ValueColor: palette.link()},
		{Label: "Bind", Value: displayBind(addr)},
		{Label: "Env", Value: env},
		{Label: "Unit", Value: item.Name()},
		{Label: "PID", Value: strconv.Itoa(os.Getpid())},
		{Label: "Routes", Value: strconv.Itoa(routeCount(item.registry))},
	}
	fmt.Fprintln(writer)
	fmt.Fprintf(writer, "%s%s%s  %s%s%s\n\n", palette.brand(), runaLogo, palette.reset(), palette.ready(), startupReadyText(), palette.reset())
	startupInline(writer, palette, "Runa HTTP", entries)
	if len(tools) > 0 {
		fmt.Fprintln(writer)
		toolEntries := make([]startupEntry, 0, len(tools))
		for _, item := range tools {
			toolEntries = append(toolEntries, startupEntry{Label: item.Name, Value: item.URL, ValueColor: palette.link()})
		}
		startupInline(writer, palette, "Tools", toolEntries)
	}
	fmt.Fprintln(writer)
}

func displayHTTPURL(addr string) string {
	hostName, port, err := net.SplitHostPort(addr)
	if err != nil {
		if strings.HasPrefix(addr, ":") {
			return "http://localhost" + addr
		}
		return "http://" + addr
	}
	if hostName == "" || hostName == "::" || hostName == "[::]" || hostName == "0.0.0.0" {
		hostName = "localhost"
	}
	return "http://" + net.JoinHostPort(hostName, port)
}

func displayBind(addr string) string {
	hostName, port, err := net.SplitHostPort(addr)
	if err != nil {
		return addr
	}
	if hostName == "" || hostName == "::" || hostName == "[::]" || hostName == "0.0.0.0" {
		hostName = "*"
	}
	return net.JoinHostPort(hostName, port)
}

type startupTool struct {
	Name string
	URL  string
}

func startupTools(registry *Registry, baseURL string) []startupTool {
	if registry == nil {
		return nil
	}
	routes := registry.Routes()
	paths := make(map[string]string)
	add := func(name string, path string) {
		if path == "" || paths[name] != "" {
			return
		}
		paths[name] = path
	}
	for _, item := range routes {
		if item == nil || item.Method != "GET" {
			continue
		}
		path := routePath(item)
		switch {
		case path == "/__runa" || strings.HasPrefix(path, "/__runa/"):
			add("Console", "/__runa")
		case strings.HasSuffix(path, "/openapi.json") || strings.HasSuffix(path, "/docs/openapi.json"):
			add("OpenAPI", path)
		case path == "/docs" || strings.HasSuffix(path, "/docs"):
			add("Docs", path)
		case item.MetaAs[bool]("observe") || path == "/debug/health" || path == "/debug/ready" || strings.HasPrefix(path, "/debug/"):
			add("Observe", observeMountPath(path))
		}
	}
	var tools []startupTool
	for _, name := range []string{"Console", "OpenAPI", "Docs", "Observe"} {
		if path := paths[name]; path != "" {
			tools = append(tools, startupTool{Name: name, URL: joinURL(baseURL, path)})
		}
	}
	return tools
}

func observeMountPath(path string) string {
	for _, suffix := range []string{"/health", "/ready", "/metrics", "/debug/monitor", "/debug/pprof", "/debug/vars"} {
		if strings.HasSuffix(path, suffix) {
			value := strings.TrimSuffix(path, suffix)
			if value == "" {
				return "/"
			}
			return value
		}
	}
	if strings.HasPrefix(path, "/debug/") {
		return "/debug"
	}
	return path
}

func routePath(item *Route) string {
	if item.FullPath != "" {
		return item.FullPath
	}
	return item.Path
}

func joinURL(baseURL string, path string) string {
	if path == "" {
		return baseURL
	}
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	return strings.TrimRight(baseURL, "/") + path
}

type startupEntry struct {
	Label      string
	Value      string
	ValueColor string
}

func routeCount(registry *Registry) int {
	if registry == nil {
		return 0
	}
	return len(registry.Routes())
}

func startupInline(writer io.Writer, palette bannerPalette, title string, entries []startupEntry) {
	if title != "" {
		fmt.Fprintf(writer, "%s%s%s\n", palette.title(), title, palette.reset())
	}
	labelWidth := startupLabelWidth(entries)
	for _, entry := range entries {
		valueColor := entry.ValueColor
		if valueColor == "" {
			valueColor = palette.value()
		}
		fmt.Fprintf(writer, "%s➜%s %s%-*s%s  %s%s%s\n",
			palette.arrow(), palette.reset(),
			palette.label(), labelWidth, entry.Label, palette.reset(),
			valueColor, entry.Value, palette.reset(),
		)
	}
}

func startupLabelWidth(entries []startupEntry) int {
	width := 0
	for _, entry := range entries {
		width = max(width, len(entry.Label))
	}
	return width
}

type bannerPalette struct{ enabled bool }

func (palette bannerPalette) reset() string {
	if !palette.enabled {
		return ""
	}
	return "\x1b[0m"
}

func (palette bannerPalette) brand() string {
	if !palette.enabled {
		return ""
	}
	return "\x1b[36;1m"
}

func (palette bannerPalette) arrow() string {
	if !palette.enabled {
		return ""
	}
	return "\x1b[36;1m"
}

func (palette bannerPalette) title() string {
	if !palette.enabled {
		return ""
	}
	return "\x1b[36m"
}

func (palette bannerPalette) label() string {
	if !palette.enabled {
		return ""
	}
	return "\x1b[32;1m"
}

func (palette bannerPalette) link() string {
	if !palette.enabled {
		return ""
	}
	return "\x1b[34;1m"
}

func (palette bannerPalette) value() string {
	if !palette.enabled {
		return ""
	}
	return "\x1b[37m"
}

func (palette bannerPalette) ready() string {
	if !palette.enabled {
		return ""
	}
	return "\x1b[90m"
}

func startupReadyText() string {
	version := "dev"
	if info, ok := debug.ReadBuildInfo(); ok {
		if info.Main.Version != "" && info.Main.Version != "(devel)" {
			version = info.Main.Version
		}
	}
	if !strings.HasPrefix(version, "v") && version != "dev" {
		version = "v" + version
	}
	return version + " ready"
}

const runaLogo = `   ____                  
  / __ \__  ______  ____ _
 / /_/ / / / / __ \/ __ ` + "`" + `/
/ _, _/ /_/ / / / / /_/ / 
/_/ |_|\__,_/_/ /_/\__,_/`
