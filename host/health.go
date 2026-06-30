package host

// Health describes the health state of a host unit.
type Health struct {
	Status  Status
	Message string
	Details map[string]any
}
