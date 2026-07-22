package rwx

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
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
	Limit        int
	Branch       string
	Repository   string // repository name, case-insensitive (server-side scope)
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
	if f.Repository != "" {
		args = append(args, "--repository", f.Repository)
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
