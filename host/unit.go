package host

import "context"

// Unit is a long-running application runtime unit.
type Unit interface {
	Name() string
	Start(ctx context.Context) error
	Stop(ctx context.Context) error
	Status() Status
}

// Checker can report host health.
type Checker interface {
	Check(ctx context.Context) Health
}

// Drainer can stop accepting new work before shutdown.
type Drainer interface {
	Drain(ctx context.Context) error
}
