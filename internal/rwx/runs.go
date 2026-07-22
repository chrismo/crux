package rwx

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
)

// RunSummary is one entry from `rwx runs list --json`.
type RunSummary struct {
	ID                      string    `json:"ID"`
	Status                  RunStatus `json:"Status"`
	DefinitionPath          string    `json:"DefinitionPath"`
	Title                   string    `json:"Title"`
	Trigger                 string    `json:"Trigger"`
	Branch                  string    `json:"Branch"`
	CommitSha               string    `json:"CommitSha"`
	RepositoryName          string    `json:"RepositoryName"`
	RunUrl                  string    `json:"RunUrl"`
	CreatedAt               string    `json:"CreatedAt"`
	CompletedAt             string    `json:"CompletedAt"`
	CompletedRuntimeSeconds *int      `json:"CompletedRuntimeSeconds"`
}

// RunList is the `rwx runs list --json` payload: a page of runs plus a
// Pagination object whose NextCursor drives paging (empty = no more pages).
type RunList struct {
	Runs       []RunSummary `json:"Runs"`
	Pagination Pagination   `json:"Pagination"`
}

// Pagination carries the cursor for fetching the next page.
type Pagination struct {
	NextCursor string `json:"NextCursor"`
}

// ListFilter narrows `rwx runs list`. Zero-value fields are omitted.
type ListFilter struct {
	Limit  int
	Branch string
	// Repositories are exact repository names (case-insensitive), as the CLI
	// requires. A user-typed substring is turned into these by
	// ResolveRepositories, and may expand to more than one.
	Repositories []string
	Mine         bool
	ResultStatus string // succeeded|failed|debugged|sandboxed|no_result
	Cursor       string
}

func (f ListFilter) args() []string {
	args := []string{"runs", "list", "--json"}
	if f.Limit > 0 {
		args = append(args, "--limit", strconv.Itoa(f.Limit))
	}
	if f.Branch != "" {
		args = append(args, "--branch", f.Branch)
	}
	for _, r := range f.Repositories {
		args = append(args, "--repository", r)
	}
	if f.Mine {
		args = append(args, "--mine")
	}
	if f.ResultStatus != "" {
		args = append(args, "--result-status", f.ResultStatus)
	}
	if f.Cursor != "" {
		args = append(args, "--cursor", f.Cursor)
	}
	return args
}

// repoDiscoveryLimit is the page size used to learn which repositories exist.
// The API has no "list repositories" endpoint, so recent runs are the only
// source of names; 100 is the CLI's maximum page.
const repoDiscoveryLimit = 100

// ResolveRepositories turns a user-typed substring into the exact repository
// names the CLI's --repository flag requires (it matches whole names only —
// `--repository cru` returns nothing). Every other crux filter takes a
// substring, so this keeps --repository consistent with them.
//
// A term that matches nothing in the discovery window is passed through
// unchanged: a repository with no runs in the last 100 is invisible here, and
// silently resolving it to "no repositories" would show an empty list instead
// of just fetching what was asked for.
func (c *Client) ResolveRepositories(ctx context.Context, term string) ([]string, error) {
	if strings.TrimSpace(term) == "" {
		return nil, nil
	}
	rl, err := c.ListRuns(ctx, ListFilter{Limit: repoDiscoveryLimit})
	if err != nil {
		return nil, err
	}
	needle := strings.ToLower(term)
	seen := map[string]bool{}
	var names []string
	for _, r := range rl.Runs {
		n := r.RepositoryName
		if n == "" || seen[n] || !strings.Contains(strings.ToLower(n), needle) {
			continue
		}
		seen[n] = true
		names = append(names, n)
	}
	if len(names) == 0 {
		return []string{term}, nil
	}
	sort.Strings(names) // deterministic fetch args
	return names, nil
}

// ListRuns fetches a page of recent runs, most recent first.
func (c *Client) ListRuns(ctx context.Context, f ListFilter) (RunList, error) {
	out, err := c.exec(ctx, "rwx", f.args()...)
	if err != nil {
		return RunList{}, fmt.Errorf("rwx runs list: %w", err)
	}
	var rl RunList
	if err := json.Unmarshal(out, &rl); err != nil {
		return RunList{}, fmt.Errorf("parse rwx runs list: %w", err)
	}
	return rl, nil
}
