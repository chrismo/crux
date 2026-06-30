package rwx

import (
	"context"
	"errors"
	"os"
	"testing"
)

func TestClientResultsParsesFixture(t *testing.T) {
	data, err := os.ReadFile("testdata/run_succeeded.json")
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}

	var gotArgs []string
	c := &Client{exec: func(_ context.Context, _ string, args ...string) ([]byte, error) {
		gotArgs = args
		return data, nil
	}}

	run, err := c.Results(context.Background(), "cc0c3a70744d4346ba7fd57954701ad3")
	if err != nil {
		t.Fatalf("Results: %v", err)
	}
	if run.RunID != "cc0c3a70744d4346ba7fd57954701ad3" {
		t.Errorf("RunID = %q", run.RunID)
	}
	if len(run.Tasks) != 8 {
		t.Errorf("Tasks = %d, want 8", len(run.Tasks))
	}

	wantArgs := []string{"results", "cc0c3a70744d4346ba7fd57954701ad3", "--json"}
	if !equalArgs(gotArgs, wantArgs) {
		t.Errorf("exec args = %v, want %v", gotArgs, wantArgs)
	}
}

func TestClientResultsPropagatesError(t *testing.T) {
	c := &Client{exec: func(_ context.Context, _ string, _ ...string) ([]byte, error) {
		return nil, errors.New("boom")
	}}
	if _, err := c.Results(context.Background(), "x"); err == nil {
		t.Error("expected error, got nil")
	}
}

func equalArgs(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
