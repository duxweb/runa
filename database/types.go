package database

import (
	"context"
	"time"

	"github.com/duxweb/runa/core"
)

const DefaultName = "default"

// Driver opens a named database runtime.
type Driver interface {
	Open(ctx context.Context, config Config) (Database, error)
}

// Config is passed to database drivers.
type Config struct {
	Name string
	App  any
}

// SQLLog describes one SQL execution event emitted by database drivers.
type SQLLog struct {
	Time      time.Time
	Database  string
	Dialect   string
	Operation string
	Model     string
	Table     string
	SQL       string
	Rows      int64
	Latency   time.Duration
	Slow      bool
	Error     string
}

// SQLRecorder consumes SQL execution events.
type SQLRecorder interface {
	RecordSQL(SQLLog)
}

// Database is a named data source runtime.
type Database interface {
	Name() string
	Kind() string
	Raw() any
	Ping(ctx context.Context) error
	Close(ctx context.Context) error
	Info() Info
}

// Info stores database runtime health metadata.
type Info struct {
	Name    string
	Kind    string
	Dialect string
	Status  string
	Meta    core.Map
}
