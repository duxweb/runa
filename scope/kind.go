package scope

// Kind identifies the runtime scope type.
type Kind string

const (
	HTTP     Kind = "http"
	Queue    Kind = "queue"
	Schedule Kind = "schedule"
	Task     Kind = "task"
	Command  Kind = "command"
	WS       Kind = "ws"
)
