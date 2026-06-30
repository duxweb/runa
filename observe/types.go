package observe

import (
	"context"
	"time"

	"github.com/duxweb/runa/core"
)

// Status describes a health check result state.
type Status string

const (
	Pass Status = "pass"
	Warn Status = "warn"
	Fail Status = "fail"
)

// Result describes one checker result.
type Result struct {
	Name      string        `json:"name"`
	Status    Status        `json:"status"`
	Message   string        `json:"message,omitempty"`
	Meta      core.Map      `json:"meta,omitempty"`
	Error     string        `json:"error,omitempty"`
	Duration  time.Duration `json:"duration,omitempty"`
	CheckedAt time.Time     `json:"checked_at"`
}

// Checker checks one runtime dependency or subsystem.
type Checker interface {
	Name() string
	Check(ctx context.Context) Result
}

// CheckerFunc adapts a function to Checker.
type CheckerFunc func(ctx context.Context) Result

// Name returns the default checker name.
func (fn CheckerFunc) Name() string { return "custom" }

// Check runs the checker function.
func (fn CheckerFunc) Check(ctx context.Context) Result { return fn(ctx) }

// Report describes aggregated checker results.
type Report struct {
	Status    Status   `json:"status"`
	Duration  string   `json:"duration"`
	CheckedAt string   `json:"checked_at"`
	Results   []Result `json:"results"`
}
