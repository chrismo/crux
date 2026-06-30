// Package rwx is the data layer: it wraps the `rwx` CLI and models its JSON.
//
// Field names mirror the authoritative results reference (`rwx docs pull
// /results`). Only the subset the TUI needs is modeled; unknown JSON fields are
// ignored by encoding/json.
package rwx

// Run is the payload from `rwx results <id> --json` (alias `rwx runs show`):
// run-level fields plus the full recursive task tree.
type Run struct {
	RunID          string    `json:"RunID"`
	Completed      bool      `json:"Completed"`
	ResultStatus   string    `json:"ResultStatus"`
	ResultPrompt   string    `json:"ResultPrompt"`
	Status         RunStatus `json:"Status"`
	Branch         string    `json:"Branch"`
	CommitSha      string    `json:"CommitSha"`
	CommitMessage  string    `json:"CommitMessage"`
	DefinitionPath string    `json:"DefinitionPath"`
	Title          string    `json:"Title"`
	Tasks          []Task    `json:"Tasks"`
}

// RunStatus is a run's Status object.
type RunStatus struct {
	Result    string `json:"Result"`    // succeeded|debugged|sandboxed|failed|no_result
	Execution string `json:"Execution"` // waiting|in_progress|finished|aborted
}

// Task is an entry in a run's Tasks tree (and recursively in Subtasks).
type Task struct {
	ID                      string     `json:"ID"`
	Key                     string     `json:"Key"`
	TaskType                string     `json:"TaskType"` // command|parallel|package|embedded-run|app-config
	CacheKey                string     `json:"CacheKey"`
	CacheHitFromTaskID      string     `json:"CacheHitFromTaskID"`
	Status                  TaskStatus `json:"Status"`
	CompletedRuntimeSeconds *int       `json:"CompletedRuntimeSeconds"`
	ExecutionRuntimeSeconds *int       `json:"ExecutionRuntimeSeconds"`
	RawDefinition           string     `json:"RawDefinition"` // task YAML as it ran; carries `use:`
	Subtasks                []Task     `json:"Subtasks"`
}

// TaskStatus is a task's Status object.
type TaskStatus struct {
	Result            string `json:"Result"`            // succeeded|failed|no_result
	Execution         string `json:"Execution"`         // not_generated|waiting|ready|running|finished|aborted|skipped|user_error
	FinishedSubStatus string `json:"FinishedSubStatus"` // cache_hit|executed|sandbox_closed|...
}

// DisplayState is the rendered category for a task node — the "cache clarity"
// win. It is derived purely from the results JSON; no `if:`/`filter:`
// re-evaluation is needed.
type DisplayState string

const (
	StateCacheHit DisplayState = "cache-hit"
	StateRan      DisplayState = "ran"
	StateSkipped  DisplayState = "skipped"
	StateRunning  DisplayState = "running"
	StateWaiting  DisplayState = "waiting"
	StateFailed   DisplayState = "failed"
	StatePending  DisplayState = "pending"
)

// DisplayState maps a task's status into a single render category. Order
// matters: a failed result must win over an otherwise-finished execution, and a
// skipped if-condition is reported regardless of result.
func (t Task) DisplayState() DisplayState {
	switch {
	case t.Status.Execution == "skipped":
		return StateSkipped
	case t.Status.Result == "failed" || t.Status.Execution == "user_error":
		return StateFailed
	case t.Status.Execution == "running":
		return StateRunning
	case t.Status.Execution == "waiting" || t.Status.Execution == "ready":
		return StateWaiting
	case t.Status.Execution == "not_generated":
		return StatePending
	case t.Status.Execution == "finished":
		if t.CacheHitFromTaskID != "" || t.Status.FinishedSubStatus == "cache_hit" {
			return StateCacheHit
		}
		return StateRan
	default:
		return StatePending
	}
}
