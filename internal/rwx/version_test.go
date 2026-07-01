package rwx

import (
	"context"
	"strings"
	"testing"
)

func TestParseVersion(t *testing.T) {
	tests := []struct {
		name    string
		in      string
		want    string
		wantErr bool
	}{
		{"cli output", "rwx version v3.19.0\n", "3.19.0", false},
		{"bare v-prefixed", "v3.19.0", "3.19.0", false},
		{"bare plain", "3.19.0", "3.19.0", false},
		{"prerelease patch", "rwx version v3.20.0-rc1", "3.20.0-rc1", false},
		{"empty", "", "", true},
		{"not semver", "rwx version banana", "", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseVersion(tt.in)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("parseVersion(%q) = %q, want error", tt.in, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("parseVersion(%q): %v", tt.in, err)
			}
			if got != tt.want {
				t.Errorf("parseVersion(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestAtLeast(t *testing.T) {
	tests := []struct {
		got, min string
		want     bool
	}{
		{"3.19.0", "3.19.0", true},
		{"3.16.0", "3.19.0", false},
		{"3.20.0", "3.19.0", true},
		{"3.19.1", "3.19.0", true},
		{"4.0.0", "3.19.0", true},
		{"2.99.99", "3.19.0", false},
		{"3.19.0-rc1", "3.19.0", true},
	}
	for _, tt := range tests {
		ok, err := atLeast(tt.got, tt.min)
		if err != nil {
			t.Fatalf("atLeast(%q, %q): %v", tt.got, tt.min, err)
		}
		if ok != tt.want {
			t.Errorf("atLeast(%q, %q) = %v, want %v", tt.got, tt.min, ok, tt.want)
		}
	}
}

func TestCheckVersion(t *testing.T) {
	newClient := func(out string, execErr error) *Client {
		return &Client{exec: func(_ context.Context, _ string, _ ...string) ([]byte, error) {
			return []byte(out), execErr
		}}
	}

	t.Run("new enough", func(t *testing.T) {
		if err := newClient("rwx version v3.19.0\n", nil).CheckVersion(context.Background()); err != nil {
			t.Fatalf("CheckVersion: %v", err)
		}
	})

	t.Run("too old mentions both versions", func(t *testing.T) {
		err := newClient("rwx version v3.16.0\n", nil).CheckVersion(context.Background())
		if err == nil {
			t.Fatal("expected error for old version")
		}
		if !strings.Contains(err.Error(), "3.16.0") || !strings.Contains(err.Error(), MinVersion) {
			t.Errorf("error %q should mention installed and required versions", err)
		}
	})
}
