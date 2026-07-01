package log

const (
	DefaultName = "default"
	HTTP        = "http"
	Error       = "error"
	Audit       = "audit"
	Queue       = "queue"
	Schedule    = "schedule"
	Task        = "task"
	ORM         = "orm"
	Redis       = "redis"
)

// ServiceName returns the DI service name for a log channel.
func ServiceName(name string) string {
	if name == "" {
		name = DefaultName
	}
	return "runa.log." + name
}
