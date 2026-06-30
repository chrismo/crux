package rwx

import (
	"encoding/json"
	"os"
	"testing"
)

// TestParseRealRun parses a real `rwx results <id> --json` payload (a green run
// of this repo's own .rwx/ci.yml) to keep the model honest against the live CLI.
func TestParseRealRun(t *testing.T) {
	data, err := os.ReadFile("testdata/run_succeeded.json")
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}

	var run Run
	if err := json.Unmarshal(data, &run); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if run.RunID == "" {
		t.Error("RunID is empty")
	}
	if run.ResultStatus != "succeeded" {
		t.Errorf("ResultStatus = %q, want succeeded", run.ResultStatus)
	}
	if !run.Completed {
		t.Error("Completed = false, want true")
	}
	if run.DefinitionPath != ".rwx/ci.yml" {
		t.Errorf("DefinitionPath = %q, want .rwx/ci.yml", run.DefinitionPath)
	}

	got := map[string]DisplayState{}
	for _, task := range run.Tasks {
		got[task.Key] = task.DisplayState()
	}

	// Derived states for this run: fresh command tasks executed ("ran"); the
	// Go install and base layers were cache hits. Note `go` reports cache_hit
	// via FinishedSubStatus with CacheHitFromTaskID null — the OR in
	// DisplayState is what catches it.
	want := map[string]DisplayState{
		"code":        StateRan,
		"go":          StateCacheHit,
		"deps":        StateRan,
		"vet":         StateRan,
		"test":        StateRan,
		"build":       StateRan,
		"~base-image": StateCacheHit,
	}
	for key, wantState := range want {
		if got[key] != wantState {
			t.Errorf("task %q DisplayState = %q, want %q", key, got[key], wantState)
		}
	}
}
