package observe

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/duxweb/runa/core"
)

// Registry stores named checkers.
type Registry struct {
	mu       sync.RWMutex
	checkers map[string]Checker
	order    []string
}

// New creates a registry.
func New() *Registry {
	return &Registry{checkers: make(map[string]Checker)}
}

// Add registers or replaces a checker.
func (registry *Registry) Add(name string, checker Checker) {
	name = strings.TrimSpace(name)
	if name == "" && checker != nil {
		name = checker.Name()
	}
	if name == "" || checker == nil {
		return
	}
	registry.mu.Lock()
	defer registry.mu.Unlock()
	if _, exists := registry.checkers[name]; !exists {
		registry.order = append(registry.order, name)
	}
	registry.checkers[name] = namedChecker{name: name, checker: checker}
}

// Run executes all checkers and returns an aggregate report.
func (registry *Registry) Run(ctx context.Context, timeout time.Duration) Report {
	started := time.Now()
	items := registry.items()
	results := make([]Result, 0, len(items))
	overall := Pass
	for _, item := range items {
		result := runCheck(ctx, item, timeout)
		if result.Status == "" {
			result.Status = Pass
		}
		if result.Status == Fail {
			overall = Fail
		} else if result.Status == Warn && overall == Pass {
			overall = Warn
		}
		results = append(results, result)
	}
	checkedAt := core.Now()
	return Report{
		Status:    overall,
		Duration:  time.Since(started).String(),
		CheckedAt: checkedAt.Format(time.RFC3339Nano),
		Results:   results,
	}
}

func (registry *Registry) items() []namedChecker {
	registry.mu.RLock()
	defer registry.mu.RUnlock()
	items := make([]namedChecker, 0, len(registry.order))
	for _, name := range registry.order {
		if checker, ok := registry.checkers[name].(namedChecker); ok {
			items = append(items, checker)
		}
	}
	return items
}

func runCheck(parent context.Context, checker namedChecker, timeout time.Duration) Result {
	started := time.Now()
	ctx := parent
	cancel := func() {}
	if timeout > 0 {
		ctx, cancel = context.WithTimeout(parent, timeout)
	}
	defer cancel()

	done := make(chan Result, 1)
	go func() {
		defer func() {
			if value := recover(); value != nil {
				done <- failed(checker.Name(), fmt.Errorf("panic: %v", value), nil)
			}
		}()
		done <- checker.Check(ctx)
	}()

	var result Result
	select {
	case result = <-done:
	case <-ctx.Done():
		result = failed(checker.Name(), ctx.Err(), nil)
		if result.Message == "" {
			result.Message = "timeout"
		}
	}
	result.Name = checker.Name()
	result.CheckedAt = core.Now()
	result.Duration = time.Since(started)
	if ctx.Err() != nil {
		result.Status = Fail
		result.Error = ctx.Err().Error()
		if result.Message == "" {
			result.Message = "timeout"
		}
	}
	return result
}

type namedChecker struct {
	name    string
	checker Checker
}

func (checker namedChecker) Name() string { return checker.name }
func (checker namedChecker) Check(ctx context.Context) Result {
	result := checker.checker.Check(ctx)
	if result.Name == "" {
		result.Name = checker.name
	}
	return result
}

func ok(name string, message string, meta core.Map) Result {
	return Result{Name: name, Status: Pass, Message: message, Meta: meta}
}

func failed(name string, err error, meta core.Map) Result {
	message := "fail"
	errorText := ""
	if err != nil {
		message = err.Error()
		errorText = err.Error()
	}
	return Result{Name: name, Status: Fail, Message: message, Error: errorText, Meta: meta}
}
