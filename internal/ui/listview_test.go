package ui

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/mattn/go-runewidth"

	"github.com/chrismo/crux/internal/rwx"
)

func loadRunList(t *testing.T) []rwx.RunSummary {
	t.Helper()
	data, err := os.ReadFile("../rwx/testdata/runs_list.json")
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	var rl rwx.RunList
	if err := json.Unmarshal(data, &rl); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	return rl.Runs
}

func TestRenderRunList(t *testing.T) {
	runs := loadRunList(t)
	now := time.Date(2026, 6, 30, 21, 0, 0, 0, time.UTC)
	out := RenderRunList(runs, 1, now)

	if !strings.Contains(out, ".rwx/ci.yml") {
		t.Error("expected the definition path in output")
	}
	if !strings.Contains(out, "›") {
		t.Error("expected a cursor marker for the selected row")
	}
	// The selected row (index 1) gets the marker; index 0 does not.
	lines := strings.Split(strings.TrimRight(out, "\n"), "\n")
	if len(lines) != len(runs) {
		t.Fatalf("rendered %d lines, want %d", len(lines), len(runs))
	}
	if strings.Contains(lines[0], "›") {
		t.Error("row 0 should not be selected")
	}
	if !strings.Contains(lines[1], "›") {
		t.Error("row 1 should be selected")
	}
}

// FilterByDefinition is the --definition scope: a substring match over the
// DefinitionPath ONLY (not Title/Branch), case-insensitive.
func TestFilterByDefinition(t *testing.T) {
	runs := loadRunList(t) // 7 .rwx/ci.yml + 1 .rwx/prime-cache.yml

	got := FilterByDefinition(runs, "prime")
	if len(got) != 1 || got[0].DefinitionPath != ".rwx/prime-cache.yml" {
		t.Fatalf("FilterByDefinition(prime) = %d runs, want 1 prime-cache", len(got))
	}
	if len(FilterByDefinition(runs, "PRIME")) != 1 {
		t.Error("match should be case-insensitive")
	}
	if len(FilterByDefinition(runs, "")) != len(runs) {
		t.Error("empty term should return all runs")
	}
	// "initiated" appears only in the Titles, never a DefinitionPath — a def
	// filter must not match it (that's FilterRunList's job).
	if n := len(FilterByDefinition(runs, "initiated")); n != 0 {
		t.Errorf("def filter matched a title substring (%d); should match DefinitionPath only", n)
	}
}

// The header status line surfaces the def scope alongside the typed filter.
func TestListStatusShowsDefScope(t *testing.T) {
	got := listStatus("", "app-dscout", "", 3, 40)
	if !strings.Contains(got, "def: app-dscout") || !strings.Contains(got, "3 of 40") {
		t.Errorf("def-only status = %q", got)
	}
	got = listStatus("mine", "app-dscout", "web", 2, 40)
	if !strings.Contains(got, "mine") || !strings.Contains(got, "def: app-dscout") || !strings.Contains(got, "filter: web") {
		t.Errorf("scope+def+filter status = %q", got)
	}
}

func TestParseGithubRemote(t *testing.T) {
	want := "https://github.com/chrismo/crux"
	for _, remote := range []string{
		"git@github.com:chrismo/crux.git",
		"git@github.com:chrismo/crux",
		"https://github.com/chrismo/crux.git",
		"https://github.com/chrismo/crux",
		"ssh://git@github.com/chrismo/crux.git",
	} {
		if got := parseGithubRemote(remote); got != want {
			t.Errorf("parseGithubRemote(%q) = %q, want %q", remote, got, want)
		}
	}
	// Non-GitHub hosts get no link (v1).
	for _, remote := range []string{
		"git@gitlab.com:chrismo/crux.git",
		"https://bitbucket.org/chrismo/crux.git",
		"",
	} {
		if got := parseGithubRemote(remote); got != "" {
			t.Errorf("parseGithubRemote(%q) = %q, want empty", remote, got)
		}
	}
}

func TestCommitURL(t *testing.T) {
	if got := commitURL("https://github.com/chrismo/crux", "abc123"); got != "https://github.com/chrismo/crux/commit/abc123" {
		t.Errorf("commitURL = %q", got)
	}
	if got := commitURL("", "abc123"); got != "" {
		t.Errorf("commitURL with no base should be empty, got %q", got)
	}
	if got := commitURL("https://github.com/chrismo/crux", ""); got != "" {
		t.Errorf("commitURL with no sha should be empty, got %q", got)
	}
}

func TestRenderRunListEmpty(t *testing.T) {
	out := RenderRunList(nil, 0, time.Now())
	if !strings.Contains(out, "no runs") {
		t.Errorf("expected empty-state message, got %q", out)
	}
}

// padRight/padLeft must produce exact display-cell widths regardless of
// multibyte or wide runes, or the run-list columns drift out of alignment.
func TestPadCellWidths(t *testing.T) {
	cases := []struct {
		s string
		w int
	}{
		{"hi", 5},                   // short: pad
		{"a-very-long-path.yml", 8}, // long: truncate + …
		{"—", 5},                    // multibyte em-dash (3 bytes, 1 cell)
		{"日本語テスト", 6},          // wide runes (2 cells each)
		{"", 4},                     // empty
	}
	for _, c := range cases {
		if got := runewidth.StringWidth(padRight(c.s, c.w)); got != c.w {
			t.Errorf("padRight(%q, %d) width = %d, want %d", c.s, c.w, got, c.w)
		}
		if got := runewidth.StringWidth(padLeft(c.s, c.w)); got != c.w {
			t.Errorf("padLeft(%q, %d) width = %d, want %d", c.s, c.w, got, c.w)
		}
	}
}

func TestHumanizeAge(t *testing.T) {
	now := time.Date(2026, 6, 30, 21, 0, 0, 0, time.UTC)
	tests := []struct {
		iso  string
		want string
	}{
		{"2026-06-30T20:59:30Z", "30s ago"},
		{"2026-06-30T20:45:00Z", "15m ago"},
		{"2026-06-30T18:00:00Z", "3h ago"},
		{"2026-06-27T21:00:00Z", "3d ago"},
		{"not-a-time", "?"},
	}
	for _, tt := range tests {
		if got := humanizeAge(tt.iso, now); got != tt.want {
			t.Errorf("humanizeAge(%q) = %q, want %q", tt.iso, got, tt.want)
		}
	}
}
