package rwx

import "testing"

func TestTaskDisplayState(t *testing.T) {
	tests := []struct {
		name string
		task Task
		want DisplayState
	}{
		{
			name: "if-condition false renders as skipped",
			task: Task{Status: TaskStatus{Execution: "skipped"}},
			want: StateSkipped,
		},
		{
			name: "failed result wins over finished execution",
			task: Task{Status: TaskStatus{Execution: "finished", Result: "failed"}},
			want: StateFailed,
		},
		{
			name: "cache hit via FinishedSubStatus",
			task: Task{Status: TaskStatus{Execution: "finished", Result: "succeeded", FinishedSubStatus: "cache_hit"}},
			want: StateCacheHit,
		},
		{
			name: "cache hit via CacheHitFromTaskID",
			task: Task{CacheHitFromTaskID: "task-123", Status: TaskStatus{Execution: "finished", Result: "succeeded"}},
			want: StateCacheHit,
		},
		{
			name: "executed and succeeded renders as ran",
			task: Task{Status: TaskStatus{Execution: "finished", Result: "succeeded", FinishedSubStatus: "executed"}},
			want: StateRan,
		},
		{
			name: "running",
			task: Task{Status: TaskStatus{Execution: "running"}},
			want: StateRunning,
		},
		{
			name: "waiting",
			task: Task{Status: TaskStatus{Execution: "waiting"}},
			want: StateWaiting,
		},
		{
			name: "ready counts as waiting",
			task: Task{Status: TaskStatus{Execution: "ready"}},
			want: StateWaiting,
		},
		{
			name: "not_generated is pending",
			task: Task{Status: TaskStatus{Execution: "not_generated"}},
			want: StatePending,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.task.DisplayState(); got != tt.want {
				t.Errorf("DisplayState() = %q, want %q", got, tt.want)
			}
		})
	}
}
