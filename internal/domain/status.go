package domain

// TaskStatus represents the lifecycle state of a mill task.
type TaskStatus string

const (
	TaskPending TaskStatus = "pending"
	TaskRunning TaskStatus = "running"
	TaskDone    TaskStatus = "done"
	TaskError   TaskStatus = "error"
)

// SessionStatus represents the lifecycle state of an agent session.
type SessionStatus string

const (
	SessionPending SessionStatus = "pending"
	SessionRunning SessionStatus = "running"
	SessionDone    SessionStatus = "done"
	SessionError   SessionStatus = "error"
)
