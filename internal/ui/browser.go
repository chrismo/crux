package ui

import (
	"fmt"
	"os/exec"
	"runtime"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

// browserOpenedMsg reports the result of an open-in-browser attempt so a failure
// can surface in the UI instead of being silently swallowed.
type browserOpenedMsg struct{ err error }

// openInBrowser launches url in the OS default browser (macOS `open`, else
// `xdg-open`). It waits for the launcher (which returns as soon as it hands off)
// so a nonzero exit / stderr is captured rather than lost.
func openInBrowser(url string) error {
	name := "xdg-open"
	if runtime.GOOS == "darwin" {
		name = "open"
	}
	out, err := exec.Command(name, url).CombinedOutput()
	if err != nil {
		if msg := strings.TrimSpace(string(out)); msg != "" {
			return fmt.Errorf("%s: %w: %s", name, err, msg)
		}
		return fmt.Errorf("%s: %w", name, err)
	}
	return nil
}

// openURLCmd opens url via the injected opener off the render loop. It captures
// only the opener and url (not the whole model), and no-ops on an empty url so
// callers can pass a maybe-missing link unconditionally.
func openURLCmd(open func(string) error, url string) tea.Cmd {
	if url == "" || open == nil {
		return nil
	}
	return func() tea.Msg {
		return browserOpenedMsg{err: open(url)}
	}
}

// GithubBaseURL returns the https://github.com/{owner}/{repo} base for origin in
// dir, or "" if there's no remote or it isn't a GitHub URL. Computed once at
// startup so ctrl+g can build commit links without rwx providing a repo URL.
func GithubBaseURL(dir string) string {
	out, err := exec.Command("git", "-C", dir, "remote", "get-url", "origin").Output()
	if err != nil {
		return ""
	}
	return parseGithubRemote(strings.TrimSpace(string(out)))
}

// parseGithubRemote turns a git remote URL into a https://github.com/{owner}/
// {repo} base, handling the ssh and https forms. Returns "" for non-GitHub hosts
// (v1 links GitHub only).
func parseGithubRemote(remote string) string {
	remote = strings.TrimSuffix(remote, ".git")
	switch {
	case strings.HasPrefix(remote, "git@github.com:"):
		return "https://github.com/" + strings.TrimPrefix(remote, "git@github.com:")
	case strings.HasPrefix(remote, "ssh://git@github.com/"):
		return "https://github.com/" + strings.TrimPrefix(remote, "ssh://git@github.com/")
	case strings.HasPrefix(remote, "https://github.com/"):
		return remote
	default:
		return ""
	}
}

// commitURL builds the GitHub commit-page URL, or "" if either part is missing
// (no remote, or a run with no recorded commit) so ctrl+g no-ops gracefully.
func commitURL(base, sha string) string {
	if base == "" || sha == "" {
		return ""
	}
	return base + "/commit/" + sha
}
