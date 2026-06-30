package queue

import (
	"context"
	"time"

	"github.com/duxweb/runa/core"
)

const (
	// InternalTaskJob is the reserved job name used by Runa task dispatch.
	InternalTaskJob = "runa.task"
	// InternalEventJob is the reserved job name used by Runa event dispatch.
	InternalEventJob = "runa.event"
	// All targets every queue worker where supported.
	All = "*"
)

// JobState describes a stored job state.
type JobState string

const (
	StatePending  JobState = "pending"
	StateDelayed  JobState = "delayed"
	StateReserved JobState = "reserved"
	StateFailed   JobState = "failed"
)

// Job stores one typed job execution.
type Job[T any] struct {
	ID         string
	Queue      string
	Name       string
	Payload    T
	Meta       core.Map
	Attempt    int
	MaxAttempt int
	CreatedAt  time.Time
	RunAt      time.Time
	Timeout    time.Duration
}

// Handler executes a typed job.
type Handler[T any] func(ctx context.Context, job *Job[T]) error

// HandlerFunc executes one driver-level job message.
type HandlerFunc func(ctx context.Context, message *JobMessage) error

// Middleware wraps queue job execution.
type Middleware func(HandlerFunc) HandlerFunc

// PushHandlerFunc pushes one serialized job into a queue.
type PushHandlerFunc func(ctx context.Context, queue string, job *JobMessage) (string, error)

// PushMiddleware wraps queue job publishing.
type PushMiddleware func(PushHandlerFunc) PushHandlerFunc

// JobMessage is the driver-level serialized queue payload.
type JobMessage struct {
	ID            string        `json:"id"`
	Queue         string        `json:"queue"`
	Name          string        `json:"name"`
	Payload       []byte        `json:"payload"`
	Meta          core.Map      `json:"meta"`
	Attempt       int           `json:"attempt"`
	MaxAttempt    int           `json:"max_attempt"`
	CreatedAt     time.Time     `json:"created_at"`
	RunAt         time.Time     `json:"run_at"`
	ReservedUntil time.Time     `json:"reserved_until"`
	Timeout       time.Duration `json:"timeout"`
	RetryDelay    time.Duration `json:"retry_delay"`
	Unique        string        `json:"unique"`
	LastError     string        `json:"last_error"`
	UpdatedAt     time.Time     `json:"updated_at"`
}

// JobQuery filters driver job listing.
type JobQuery struct {
	State JobState
	Page  int
	Limit int
}

// Driver is the minimal queue storage and reliable-consumption contract.
type Driver interface {
	Name() string
	Push(ctx context.Context, queue string, job *JobMessage) (string, error)
	Reserve(ctx context.Context, queue string, limit int, lease time.Duration) ([]*JobMessage, error)
	Ack(ctx context.Context, queue string, id string) error
	Release(ctx context.Context, queue string, id string, delay time.Duration, reason string) error
	Fail(ctx context.Context, queue string, id string, reason string) error
	Renew(ctx context.Context, queue string, id string, lease time.Duration) error
	Delete(ctx context.Context, queue string, id string) error
	Count(ctx context.Context, queue string, state JobState) (int64, error)
	List(ctx context.Context, queue string, query JobQuery) ([]*JobMessage, error)
	Close(ctx context.Context) error
}

// WorkerState stores running worker instance heartbeats.
type WorkerState interface {
	Register(ctx context.Context, worker WorkerInstance) error
	Heartbeat(ctx context.Context, id string) error
	Unregister(ctx context.Context, id string) error
	Instances(ctx context.Context, worker string) ([]WorkerInstance, error)
}

// QueueInfo describes one configured queue and its current driver state.
type QueueInfo struct {
	Name     string
	Driver   string
	Workers  []string
	Pending  int64
	Reserved int64
	Delayed  int64
	Failed   int64
	Meta     core.Map
}

// WorkerInfo describes one configured worker group.
type WorkerInfo struct {
	Name        string
	Queues      []string
	Concurrency int
	Instances   int
	Status      string
	Processed   int64
	Succeeded   int64
	Failed      int64
	Retried     int64
	Meta        core.Map
}

// JobInfo describes one registered job handler.
type JobInfo struct {
	Name    string
	Payload string
	Source  string
	Meta    core.Map
}

// WorkerInstance describes one running worker process instance.
type WorkerInstance struct {
	ID          string
	Worker      string
	Hostname    string
	PID         int
	Queues      []string
	Concurrency int
	Busy        int
	Status      string
	StartedAt   time.Time
	HeartbeatAt time.Time
}

func mergeMap(values ...core.Map) core.Map {
	var output core.Map
	for _, item := range values {
		for key, value := range item {
			if output == nil {
				output = make(core.Map)
			}
			output[key] = value
		}
	}
	return output
}
