package rate

import (
	"context"
	"fmt"
	"strings"

	"github.com/duxweb/runa/core"
)

type limiter struct {
	rule   Rule
	driver Driver
}

func newLimiter(rule Rule, driver Driver) Limiter {
	return &limiter{rule: normalizeRule(rule), driver: driver}
}

func (limiter *limiter) Allow(ctx context.Context, keys ...string) (Result, error) {
	ctx = core.NormalizeContext(ctx)
	key := limiter.key(keys...)
	return limiter.driver.Allow(ctx, limiter.rule, key)
}

func (limiter *limiter) Reset(ctx context.Context, keys ...string) error {
	ctx = core.NormalizeContext(ctx)
	return limiter.driver.Reset(ctx, limiter.rule, limiter.key(keys...))
}

func (limiter *limiter) key(keys ...string) string {
	parts := []string{limiter.rule.Name}
	for _, key := range keys {
		key = strings.TrimSpace(key)
		if key != "" {
			parts = append(parts, key)
		}
	}
	return strings.Join(parts, ":")
}

func normalizeRule(rule Rule) Rule {
	if rule.Driver == "" {
		rule.Driver = DefaultDriver
	}
	if rule.Algorithm == "" {
		rule.Algorithm = AlgorithmTokenBucket
	}
	if rule.Limit <= 0 {
		rule.Limit = 60
	}
	if rule.Window <= 0 {
		return rule
	}
	if rule.Burst <= 0 {
		rule.Burst = rule.Limit
	}
	return rule
}

func validateRule(rule Rule) error {
	if rule.Name == "" {
		return fmt.Errorf("rate name is required")
	}
	if rule.Driver == "" {
		return fmt.Errorf("rate %s driver is required", rule.Name)
	}
	if rule.Limit <= 0 {
		return fmt.Errorf("rate %s limit must be greater than zero", rule.Name)
	}
	if rule.Window <= 0 {
		return fmt.Errorf("rate %s window must be greater than zero", rule.Name)
	}
	return nil
}
