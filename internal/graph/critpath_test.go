package graph

import (
	"testing"
	"time"

	"github.com/chrismo/crux/internal/rwx"
)

// A running task's reported runtime is not yet meaningful: observed live, it
// carries ExecutionRuntimeSeconds=null and CompletedRuntimeSeconds=**0** — the
// zero is present, not absent, so trusting it weighs the task 0 and the
// critical-path total sits frozen until the task finishes. That was the stale
// header in issue #1; elapsed-since-StartedAt makes the total climb.
func TestCriticalPathCountsRunningTasks(t *testing.T) {
	start := time.Date(2026, 7, 21, 12, 0, 0, 0, time.UTC)
	done, zero := 10, 0
	run := rwx.Run{Tasks: []rwx.Task{
		{
			Key: "build", Status: rwx.TaskStatus{Execution: "finished"},
			StartedAt: start.Format(time.RFC3339), CompletedRuntimeSeconds: &done,
		},
		{
			Key: "test", Status: rwx.TaskStatus{Execution: "running"},
			StartedAt: start.Add(10 * time.Second).Format("2006-01-02T15:04:05.000Z"),
			// Mirrors the live payload: a bare 0, which must not win over elapsed.
			CompletedRuntimeSeconds: &zero,
			RawDefinition:           "use: build\n",
		},
	}}

	// 30s into the running task: 10s (build) + 20s elapsed (test).
	at30 := BuildAt(run, start.Add(30*time.Second))
	cp30 := CriticalPath(at30)
	if !equalStrings(cp30.Keys, []string{"build", "test"}) {
		t.Fatalf("Keys = %v, want [build test]", cp30.Keys)
	}
	if cp30.Total != 30 {
		t.Errorf("Total at +30s = %d, want 30 (10 done + 20 elapsed)", cp30.Total)
	}

	// The whole point: a later poll of the same unchanged payload reads higher.
	cp60 := CriticalPath(BuildAt(run, start.Add(60*time.Second)))
	if cp60.Total != 60 {
		t.Errorf("Total at +60s = %d, want 60 (10 done + 50 elapsed)", cp60.Total)
	}
	if cp60.Total <= cp30.Total {
		t.Errorf("total did not advance with the run: %d then %d", cp30.Total, cp60.Total)
	}

	// A `parallel:` task whose first subtask has already failed reports
	// Result=failed while Execution is still "running" — observed live. It is
	// still burning wall-clock, so it still has to be timed, even though it
	// *renders* as failed.
	run.Tasks[1].Status.Result = "failed"
	if got := CriticalPath(BuildAt(run, start.Add(60*time.Second))).Total; got != 60 {
		t.Errorf("running-but-failed task Total = %d, want 60 (still executing)", got)
	}
}

// A task that hasn't started yet has no elapsed time to count, and a running
// task with an unparseable StartedAt must not poison the total.
func TestCriticalPathIgnoresUnstartedTasks(t *testing.T) {
	now := time.Date(2026, 7, 21, 12, 0, 0, 0, time.UTC)
	run := rwx.Run{Tasks: []rwx.Task{
		{Key: "waiting", Status: rwx.TaskStatus{Execution: "waiting"}},
		{Key: "bogus", Status: rwx.TaskStatus{Execution: "running"}, StartedAt: "not-a-time"},
	}}
	for _, n := range BuildAt(run, now).Nodes {
		if n.DurationSeconds != 0 || n.HasTiming {
			t.Errorf("node %s = %ds (timing %v), want 0s and no timing", n.Key, n.DurationSeconds, n.HasTiming)
		}
	}
}

func TestCriticalPathByDuration(t *testing.T) {
	g := loadFixtureGraph(t)
	cp := CriticalPath(g)

	wantKeys := []string{"code", "deps", "test"} // 5 + 2 + 13 = 20
	if !equalStrings(cp.Keys, wantKeys) {
		t.Errorf("Keys = %v, want %v", cp.Keys, wantKeys)
	}
	if cp.Total != 20 {
		t.Errorf("Total = %d, want 20", cp.Total)
	}
	if !cp.Contains("test") {
		t.Error("Contains(test) = false, want true")
	}
	if cp.Contains("vet") {
		t.Error("Contains(vet) = true; vet is not on the critical path")
	}
}

// With no timing data anywhere, the critical path falls back to the longest
// chain by depth (each node weighted 1).
func TestCriticalPathDepthFallback(t *testing.T) {
	g := &Graph{index: map[string]*Node{}}
	for _, k := range []string{"a", "b", "c", "d"} {
		n := &Node{Key: k, HasTiming: false}
		g.Nodes = append(g.Nodes, n)
		g.index[k] = n
	}
	g.Edges = []Edge{
		{From: "a", To: "b"},
		{From: "b", To: "c"},
		{From: "a", To: "d"},
	}
	cp := CriticalPath(g)
	want := []string{"a", "b", "c"}
	if !equalStrings(cp.Keys, want) {
		t.Errorf("Keys = %v, want %v", cp.Keys, want)
	}
	if cp.Total != 3 {
		t.Errorf("Total = %d, want 3 (depth)", cp.Total)
	}
}
