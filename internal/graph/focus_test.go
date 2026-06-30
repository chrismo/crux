package graph

import "testing"

func TestFocusAncestorsAndDescendants(t *testing.T) {
	g := loadFixtureGraph(t)

	// deps sits in the middle: ancestors code+go, descendants vet+test+build.
	got := Focus(g, "deps")
	want := []string{"deps", "code", "go", "vet", "test", "build"}
	for _, k := range want {
		if !got[k] {
			t.Errorf("Focus(deps) missing %q", k)
		}
	}
	// The base-layer component is unrelated and must be excluded.
	for _, k := range []string{"~base-image", "~base-config"} {
		if got[k] {
			t.Errorf("Focus(deps) should not include %q", k)
		}
	}
	if len(got) != len(want) {
		t.Errorf("Focus(deps) size = %d, want %d (%v)", len(got), len(want), got)
	}
}

func TestFocusLeafHasNoDescendants(t *testing.T) {
	g := loadFixtureGraph(t)
	got := Focus(g, "vet") // leaf: only ancestors
	want := map[string]bool{"vet": true, "deps": true, "code": true, "go": true}
	if len(got) != len(want) {
		t.Fatalf("Focus(vet) = %v, want keys %v", got, want)
	}
	for k := range want {
		if !got[k] {
			t.Errorf("Focus(vet) missing %q", k)
		}
	}
}

func TestFocusUnknownNode(t *testing.T) {
	g := loadFixtureGraph(t)
	if got := Focus(g, "nope"); got != nil {
		t.Errorf("Focus(unknown) = %v, want nil", got)
	}
}
