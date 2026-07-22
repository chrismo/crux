package rwx

import (
	"context"
	"os"
	"testing"
)

func TestClientListRunsParsesFixture(t *testing.T) {
	data, err := os.ReadFile("testdata/runs_list.json")
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	var gotArgs []string
	c := &Client{exec: func(_ context.Context, _ string, args ...string) ([]byte, error) {
		gotArgs = args
		return data, nil
	}}

	rl, err := c.ListRuns(context.Background(), ListFilter{Limit: 8})
	if err != nil {
		t.Fatalf("ListRuns: %v", err)
	}
	if len(rl.Runs) != 8 {
		t.Errorf("Runs = %d, want 8", len(rl.Runs))
	}
	if rl.Runs[0].ID == "" {
		t.Error("first run has empty ID")
	}
	if rl.Runs[0].Status.Result == "" {
		t.Error("first run has empty Status.Result")
	}

	wantArgs := []string{"runs", "list", "--json", "--limit", "8"}
	if !equalArgs(gotArgs, wantArgs) {
		t.Errorf("args = %v, want %v", gotArgs, wantArgs)
	}
}

// The CLI reports the paging cursor under a Pagination object, not at the top
// level, so ListRuns must read it from there (else paging never triggers).
func TestClientListRunsParsesNextCursor(t *testing.T) {
	data := []byte(`{"Pagination":{"Limit":2,"NextCursor":"abc123"},"Runs":[{"ID":"r1"},{"ID":"r2"}]}`)
	c := &Client{exec: func(_ context.Context, _ string, _ ...string) ([]byte, error) {
		return data, nil
	}}
	rl, err := c.ListRuns(context.Background(), ListFilter{Limit: 2})
	if err != nil {
		t.Fatalf("ListRuns: %v", err)
	}
	if rl.Pagination.NextCursor != "abc123" {
		t.Errorf("NextCursor = %q, want abc123", rl.Pagination.NextCursor)
	}
}

func TestListFilterArgs(t *testing.T) {
	got := ListFilter{Limit: 20, Branch: "main", Mine: true, ResultStatus: "failed"}.args()
	want := []string{"runs", "list", "--json", "--limit", "20", "--branch", "main", "--mine", "--result-status", "failed"}
	if !equalArgs(got, want) {
		t.Errorf("args = %v, want %v", got, want)
	}
}

// Repository is a server-side scope, not a client-side substring: with several
// repos sharing one org the most recent N runs can be all-other-repo, so the
// narrowing has to happen before the page is cut. The flag is repeatable, since
// one typed substring can resolve to more than one repository.
func TestListFilterArgsRepository(t *testing.T) {
	got := ListFilter{Limit: 20, Repositories: []string{"crux", "crux-docs"}}.args()
	want := []string{"runs", "list", "--json", "--limit", "20",
		"--repository", "crux", "--repository", "crux-docs"}
	if !equalArgs(got, want) {
		t.Errorf("args = %v, want %v", got, want)
	}
}

// The CLI's --repository is exact (case-insensitive) — verified against the
// live API, where `--repository cru` returns nothing. Every other crux filter
// takes a substring, so the term is resolved to real names first.
func TestResolveRepositories(t *testing.T) {
	data := []byte(`{"Runs":[
		{"ID":"1","RepositoryName":"crux"},
		{"ID":"2","RepositoryName":"questor"},
		{"ID":"3","RepositoryName":"crux"},
		{"ID":"4","RepositoryName":"crux-docs"},
		{"ID":"5","RepositoryName":""}
	]}`)
	var gotArgs []string
	c := &Client{exec: func(_ context.Context, _ string, args ...string) ([]byte, error) {
		gotArgs = args
		return data, nil
	}}

	// A substring can legitimately hit several repos; both come back, deduped
	// and ordered so the fetch is deterministic.
	got, err := c.ResolveRepositories(context.Background(), "cru")
	if err != nil {
		t.Fatalf("ResolveRepositories: %v", err)
	}
	if !equalArgs(got, []string{"crux", "crux-docs"}) {
		t.Errorf("resolve(cru) = %v, want [crux crux-docs]", got)
	}
	// Discovery must not be narrowed by the term itself — that would only ever
	// find repositories whose exact name was already typed.
	for _, a := range gotArgs {
		if a == "--repository" {
			t.Errorf("discovery fetch passed --repository: %v", gotArgs)
		}
	}

	if got, _ := c.ResolveRepositories(context.Background(), "QUES"); !equalArgs(got, []string{"questor"}) {
		t.Errorf("resolve(QUES) = %v, want [questor] (case-insensitive)", got)
	}
	// A name absent from the discovery window still has to work: pass it
	// through untouched rather than resolving to nothing and showing an
	// empty list.
	if got, _ := c.ResolveRepositories(context.Background(), "archived-thing"); !equalArgs(got, []string{"archived-thing"}) {
		t.Errorf("resolve(miss) = %v, want the term passed through", got)
	}
	if got, _ := c.ResolveRepositories(context.Background(), ""); got != nil {
		t.Errorf("resolve(empty) = %v, want nil", got)
	}
}
