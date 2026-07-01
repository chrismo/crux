package rwx

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

// MinVersion is the lowest rwx CLI version crux is known to work with.
// 3.16.0 is known-broken; 3.19.0 is verified working. Bump this as newer CLI
// features are relied upon.
const MinVersion = "3.19.0"

// Version runs `rwx --version` and returns the parsed version (without the
// leading "v"), e.g. "3.19.0".
func (c *Client) Version(ctx context.Context) (string, error) {
	out, err := c.exec(ctx, "rwx", "--version")
	if err != nil {
		if errors.Is(err, exec.ErrNotFound) {
			return "", errors.New("rwx CLI not found on PATH; install it: brew install rwx-cloud/tap/rwx")
		}
		return "", fmt.Errorf("rwx --version: %w", err)
	}
	return parseVersion(string(out))
}

// CheckVersion verifies the installed rwx CLI is at least MinVersion, returning
// a user-facing error otherwise. crux calls this at startup so an outdated (or
// missing) CLI produces a clear message instead of an opaque non-zero exit.
func (c *Client) CheckVersion(ctx context.Context) error {
	got, err := c.Version(ctx)
	if err != nil {
		return err
	}
	ok, err := atLeast(got, MinVersion)
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("rwx CLI %s is too old; crux needs %s or newer (upgrade: brew upgrade rwx-cloud/tap/rwx)", got, MinVersion)
	}
	return nil
}

// parseVersion extracts the version token from `rwx version v3.19.0` style
// output (also accepting a bare "v3.19.0" or "3.19.0"). It validates the
// numeric core but preserves any pre-release suffix in the returned string.
func parseVersion(s string) (string, error) {
	fields := strings.Fields(s)
	if len(fields) == 0 {
		return "", errors.New("empty rwx version output")
	}
	v := strings.TrimPrefix(fields[len(fields)-1], "v")
	if _, err := parseSemver(v); err != nil {
		return "", fmt.Errorf("parse rwx version %q: %w", s, err)
	}
	return v, nil
}

// atLeast reports whether version got is >= version min (major.minor.patch,
// ignoring any pre-release suffix).
func atLeast(got, min string) (bool, error) {
	g, err := parseSemver(got)
	if err != nil {
		return false, err
	}
	m, err := parseSemver(min)
	if err != nil {
		return false, err
	}
	for i := range g {
		if g[i] != m[i] {
			return g[i] > m[i], nil
		}
	}
	return true, nil
}

// parseSemver parses "major.minor.patch" into its three numeric components,
// discarding any pre-release ("-rc1") or build ("+meta") suffix.
func parseSemver(v string) ([3]int, error) {
	v = strings.SplitN(v, "-", 2)[0]
	v = strings.SplitN(v, "+", 2)[0]
	parts := strings.SplitN(v, ".", 3)
	if len(parts) != 3 {
		return [3]int{}, fmt.Errorf("not semver: %q", v)
	}
	var out [3]int
	for i, p := range parts {
		n, err := strconv.Atoi(p)
		if err != nil {
			return [3]int{}, fmt.Errorf("not semver: %q", v)
		}
		out[i] = n
	}
	return out, nil
}
