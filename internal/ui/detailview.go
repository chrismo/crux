package ui

import (
	"fmt"
	"strings"

	"github.com/chrismo/crux/internal/rwx"
)

// RenderDetail renders a task's detail pane: status, cache, timing, tests, and
// any messages. Pure — also drives any headless detail output.
func RenderDetail(t *rwx.Task) string {
	if t == nil {
		return theme.Faint.Render("no task selected")
	}
	var b strings.Builder
	b.WriteString(theme.Header.Render(fmt.Sprintf("%s  (%s)", t.Key, t.TaskType)))
	b.WriteString("\n\n")

	status := fmt.Sprintf("%s / %s", t.Status.Execution, t.Status.Result)
	if fss := t.Status.FinishedSubStatus; fss != "" && fss != "not_applicable" {
		status += " (" + fss + ")"
	}
	b.WriteString("status:    " + status + "\n")

	cache := "miss"
	if t.CacheHitFromTaskID != "" || t.Status.FinishedSubStatus == "cache_hit" {
		cache = "hit"
	}
	b.WriteString("cache:     " + cache + "\n")
	b.WriteString("timing:    " + timingLine(t) + "\n")

	if t.TestCount != nil {
		failed := 0
		if t.FailedTestCount != nil {
			failed = *t.FailedTestCount
		}
		b.WriteString(fmt.Sprintf("tests:     %d failed / %d\n", failed, *t.TestCount))
	}
	if t.ArtifactCount > 0 {
		b.WriteString(fmt.Sprintf("artifacts: %d\n", t.ArtifactCount))
	}

	if len(t.Messages) > 0 {
		b.WriteString("\nmessages:\n")
		for _, m := range t.Messages {
			line := "  • " + m.Type + ": " + m.Message
			if m.FileName != "" {
				loc := m.FileName
				if m.Line != nil {
					loc += fmt.Sprintf(":%d", *m.Line)
				}
				line += " (" + loc + ")"
			}
			b.WriteString(strings.TrimRight(line, " ") + "\n")
		}
	}

	b.WriteString("\n" + theme.Faint.Render("L: logs · esc: back"))
	return b.String()
}

func timingLine(t *rwx.Task) string {
	var parts []string
	if t.ExecutionRuntimeSeconds != nil {
		parts = append(parts, fmt.Sprintf("exec %ds", *t.ExecutionRuntimeSeconds))
	}
	if t.CompletedRuntimeSeconds != nil {
		parts = append(parts, fmt.Sprintf("total %ds", *t.CompletedRuntimeSeconds))
	}
	if len(parts) == 0 {
		return "—"
	}
	return strings.Join(parts, " · ")
}
