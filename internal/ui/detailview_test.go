package ui

import (
	"strings"
	"testing"
)

func TestRenderDetailFailedTask(t *testing.T) {
	run := loadRun(t, "run_failed.json")
	task := run.FindTask("code")
	if task == nil {
		t.Fatal("code task not found in fixture")
	}
	out := RenderDetail(task)

	for _, want := range []string{"code", "status:", "failed", "cache:", "timing:"} {
		if !strings.Contains(out, want) {
			t.Errorf("detail missing %q:\n%s", want, out)
		}
	}
}

func TestRenderDetailCacheHit(t *testing.T) {
	run := loadRun(t, "run_succeeded.json")
	task := run.FindTask("go") // cache_hit via FinishedSubStatus
	if task == nil {
		t.Fatal("go task not found")
	}
	out := RenderDetail(task)
	if !strings.Contains(out, "cache:     hit") {
		t.Errorf("expected cache hit in detail:\n%s", out)
	}
}

func TestRenderDetailNil(t *testing.T) {
	if !strings.Contains(RenderDetail(nil), "no task") {
		t.Error("nil task should render a placeholder")
	}
}
