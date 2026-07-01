package graph

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/chrismo/crux/internal/rwx"
)

func loadFailedGraph(t *testing.T) *Graph {
	t.Helper()
	data, err := os.ReadFile("../rwx/testdata/run_failed.json")
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	var run rwx.Run
	if err := json.Unmarshal(data, &run); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	return Build(run)
}

func TestAnalyzeFailures(t *testing.T) {
	fi := AnalyzeFailures(loadFailedGraph(t))

	if !equalStrings(fi.Failed, []string{"code"}) {
		t.Errorf("Failed = %v, want [code]", fi.Failed)
	}
	if fi.First() != "code" {
		t.Errorf("First() = %q, want code", fi.First())
	}

	// code's failure blasts everything downstream (deps -> vet/test/build).
	for _, k := range []string{"deps", "vet", "test", "build"} {
		if !fi.InBlast(k) {
			t.Errorf("expected %q in blast radius", k)
		}
	}
	// Unaffected branches and the failed node itself are not blast radius.
	for _, k := range []string{"code", "go", "~base-image", "~base-config"} {
		if fi.InBlast(k) {
			t.Errorf("did not expect %q in blast radius", k)
		}
	}
}

func TestAnalyzeFailuresNoneOnSuccess(t *testing.T) {
	fi := AnalyzeFailures(loadFixtureGraph(t)) // the succeeded fixture
	if len(fi.Failed) != 0 {
		t.Errorf("Failed = %v, want none", fi.Failed)
	}
	if fi.First() != "" {
		t.Errorf("First() = %q, want empty", fi.First())
	}
}
