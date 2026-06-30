package host

// Status is the runtime state of a host unit.
type Status string

const (
	Created   Status = "created"
	Starting  Status = "starting"
	Running   Status = "running"
	Draining  Status = "draining"
	Stopping  Status = "stopping"
	Stopped   Status = "stopped"
	Failed    Status = "failed"
	Unhealthy Status = "unhealthy"
)
