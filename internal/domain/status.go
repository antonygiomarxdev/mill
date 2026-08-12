package domain

// TaskStatus represents the lifecycle state of a mill task.
type TaskStatus string

const (
	TaskPending TaskStatus = "pending"
	TaskRunning TaskStatus = "running"
	TaskDone    TaskStatus = "done"
	TaskError   TaskStatus = "error"
	TaskAborted TaskStatus = "aborted"
)

// SessionStatus represents the lifecycle state of an agent session.
type SessionStatus string

const (
	SessionPending SessionStatus = "pending"
	SessionRunning SessionStatus = "running"
	SessionDone    SessionStatus = "done"
	SessionError   SessionStatus = "error"
)

// TaskPhase represents a phase in a task's lifecycle, used for
// failure classification and valid-phase transition enforcement.
type TaskPhase string

const (
	TaskPhaseDispatch   TaskPhase = "dispatch"
	TaskPhaseProduce    TaskPhase = "produce"
	TaskPhaseReview     TaskPhase = "review"
	TaskPhaseRework     TaskPhase = "rework"
	TaskPhaseRejected   TaskPhase = "rejected"
	TaskPhaseGateFailed TaskPhase = "gate_failed"
	TaskPhaseAborted    TaskPhase = "aborted"
)

// allowedTransitions maps each phase to the phases it may transition to.
var allowedTransitions = map[TaskPhase][]TaskPhase{
	TaskPhaseDispatch:   {TaskPhaseProduce, TaskPhaseAborted},
	TaskPhaseProduce:    {TaskPhaseReview, TaskPhaseRejected, TaskPhaseGateFailed, TaskPhaseAborted},
	TaskPhaseReview:     {TaskPhaseRework, TaskPhaseAborted},
	TaskPhaseRework:     {TaskPhaseReview, TaskPhaseAborted},
	TaskPhaseRejected:   {TaskPhaseProduce, TaskPhaseAborted},
	TaskPhaseGateFailed: {TaskPhaseRework, TaskPhaseAborted},
	TaskPhaseAborted:    {},
}

// CanTransitionTo reports whether a task may move from phase p to next.
func (p TaskPhase) CanTransitionTo(next TaskPhase) bool {
	for _, allowed := range allowedTransitions[p] {
		if allowed == next {
			return true
		}
	}
	return false
}
