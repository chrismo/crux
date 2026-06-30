package rwx

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
)

// Client wraps the `rwx` CLI. The exec function is injectable for testing.
type Client struct {
	exec func(ctx context.Context, name string, args ...string) ([]byte, error)
}

// NewClient returns a Client that shells out to the real `rwx` binary.
func NewClient() *Client {
	return &Client{exec: runRwx}
}

// Results fetches a run and its full task tree via `rwx results <id> --json`.
func (c *Client) Results(ctx context.Context, runID string) (Run, error) {
	out, err := c.exec(ctx, "rwx", "results", runID, "--json")
	if err != nil {
		return Run{}, fmt.Errorf("rwx results %s: %w", runID, err)
	}
	var run Run
	if err := json.Unmarshal(out, &run); err != nil {
		return Run{}, fmt.Errorf("parse rwx results %s: %w", runID, err)
	}
	return run, nil
}

// runRwx executes the rwx binary and returns stdout. Stderr (e.g. the
// "new release available" notice) is ignored; only stdout carries the JSON.
func runRwx(ctx context.Context, name string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	return cmd.Output()
}
