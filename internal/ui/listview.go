package ui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"

	"github.com/chrismo/rwx-tui/internal/rwx"
)

// runGlyph returns a glyph and color for a run-level status. In-progress runs
// are distinguished from their (not yet meaningful) result.
func runGlyph(s rwx.RunStatus) (string, lipgloss.Color) {
	switch s.Execution {
	case "in_progress":
		return "●", lipgloss.Color("3") // yellow
	case "waiting":
		return "○", lipgloss.Color("8") // gray
	case "aborted":
		return "⊘", lipgloss.Color("8")
	}
	switch s.Result {
	case "succeeded":
		return "✓", lipgloss.Color("2") // green
	case "failed":
		return "✗", lipgloss.Color("1") // red
	case "debugged", "sandboxed":
		return "◆", lipgloss.Color("5") // magenta
	default:
		return "○", lipgloss.Color("8") // no_result
	}
}

// RenderRunList renders the run-list rows, most recent first, with the selected
// row marked. now is injected so the relative ages are testable.
func RenderRunList(runs []rwx.RunSummary, selected int, now time.Time) string {
	if len(runs) == 0 {
		return lipgloss.NewStyle().Faint(true).Render("  no runs found") + "\n"
	}
	var b strings.Builder
	for i, r := range runs {
		cursor := "  "
		if i == selected {
			cursor = "› "
		}
		glyph, color := runGlyph(r.Status)
		result := r.Status.Result
		if r.Status.Execution == "in_progress" {
			result = "running"
		}
		left := lipgloss.NewStyle().Foreground(color).Render(fmt.Sprintf("%s %-9s", glyph, result))

		row := fmt.Sprintf("%s%s  %-13s  %-26s  %8s  %5s",
			cursor, left,
			r.DefinitionPath,
			truncate(r.Title, 26),
			humanizeAge(r.CreatedAt, now),
			runtimeStr(r.CompletedRuntimeSeconds),
		)
		if i == selected {
			row = lipgloss.NewStyle().Bold(true).Render(row)
		}
		b.WriteString(row)
		b.WriteString("\n")
	}
	return b.String()
}

func runtimeStr(secs *int) string {
	if secs == nil {
		return "—"
	}
	return fmt.Sprintf("%ds", *secs)
}

// humanizeAge renders an ISO-8601 timestamp as a coarse "N<unit> ago" relative
// to now. Returns "?" if the timestamp can't be parsed.
func humanizeAge(iso string, now time.Time) string {
	t, err := time.Parse(time.RFC3339, iso)
	if err != nil {
		return "?"
	}
	d := now.Sub(t)
	switch {
	case d < time.Minute:
		return fmt.Sprintf("%ds ago", int(d.Seconds()))
	case d < time.Hour:
		return fmt.Sprintf("%dm ago", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh ago", int(d.Hours()))
	default:
		return fmt.Sprintf("%dd ago", int(d.Hours())/24)
	}
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	if max <= 1 {
		return s[:max]
	}
	return s[:max-1] + "…"
}
