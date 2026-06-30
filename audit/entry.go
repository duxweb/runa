package audit

import (
	"time"

	"github.com/duxweb/runa/core"
)

// Entry describes one business operation audit record.
type Entry struct {
	ID       string        `json:"id,omitempty"`
	Time     time.Time     `json:"time"`
	Method   string        `json:"method"`
	Path     string        `json:"path"`
	Route    string        `json:"route,omitempty"`
	Action   string        `json:"action,omitempty"`
	Status   int           `json:"status"`
	Success  bool          `json:"success"`
	Error    string        `json:"error,omitempty"`
	Duration time.Duration `json:"duration"`

	ActorID   string `json:"actor_id,omitempty"`
	ActorName string `json:"actor_name,omitempty"`
	Guard     string `json:"guard,omitempty"`

	IP        string `json:"ip,omitempty"`
	UserAgent string `json:"user_agent,omitempty"`
	RequestID string `json:"request_id,omitempty"`
	TraceID   string `json:"trace_id,omitempty"`

	Input core.Map `json:"input,omitempty"`
	Meta  core.Map `json:"meta,omitempty"`
}

// Data is used by manual audit recording helpers.
type Data struct {
	Input core.Map
	Meta  core.Map
}
