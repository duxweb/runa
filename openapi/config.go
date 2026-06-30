package openapi

// Option configures an OpenAPI document domain.
type Option func(*Config)

// Config stores OpenAPI document domain config.
type Config struct {
	Name        string
	Title       string
	Version     string
	Description string
	Servers     []ServerInfo
	JSONPath    string
	UIPath      string
	Viewer      Viewer
}

// ServerInfo describes an OpenAPI server.
type ServerInfo struct {
	URL         string `json:"url"`
	Description string `json:"description,omitempty"`
}

// Title sets document title.
func Title(value string) Option {
	return func(config *Config) { config.Title = value }
}

// Version sets document version.
func Version(value string) Option {
	return func(config *Config) { config.Version = value }
}

// Description sets document description.
func Description(value string) Option {
	return func(config *Config) { config.Description = value }
}

// ServerURL appends a document server.
func ServerURL(url string, description ...string) Option {
	return func(config *Config) {
		item := ServerInfo{URL: url}
		if len(description) > 0 {
			item.Description = description[0]
		}
		config.Servers = append(config.Servers, item)
	}
}

// Server appends a document server.
func Server(url string, description ...string) Option {
	return ServerURL(url, description...)
}

// JSON sets JSON output path.
func JSON(path string) Option {
	return func(config *Config) { config.JSONPath = path }
}

// UI sets UI output path.
func UI(path string, viewers ...Viewer) Option {
	return func(config *Config) {
		config.UIPath = path
		if len(viewers) > 0 && viewers[0] != nil {
			config.Viewer = viewers[0]
		}
	}
}
