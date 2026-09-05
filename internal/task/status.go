package task

type Status string

const (
	StatusPending   Status = "pending"
	StatusQueued    Status = "queued"
	StatusRunning   Status = "running"
	StatusSucceeded Status = "succeeded"
	StatusFailed    Status = "failed"
)

func CanTransition(from, to Status) bool {
	switch from {
	case StatusPending:
		return to == StatusQueued || to == StatusFailed
	case StatusQueued:
		return to == StatusRunning || to == StatusFailed
	case StatusRunning:
		return to == StatusSucceeded || to == StatusFailed
	default:
		return false
	}
}
func (status Status) IsTerminal() bool {
	return status == StatusSucceeded || status == StatusFailed
}
