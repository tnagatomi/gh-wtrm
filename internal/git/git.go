// Package git wraps the local git invocations gh-wtrm needs and the pure
// parsers for their output. Each exec wrapper runs `git -C dir ...` so it
// works identically from the primary checkout or a linked worktree.
package git

import (
	"os/exec"
	"strings"
	"time"
)

// Change is one entry of `git status --porcelain` output. X/Y are the
// staged/worktree status columns; Path is the affected file.
type Change struct {
	X    string
	Y    string
	Path string
}

// IsUntracked reports whether the change is an untracked file (porcelain
// "??", so the worktree column is "?").
func (c Change) IsUntracked() bool {
	return c.Y == "?"
}

// ParseStatus parses `git status --porcelain` output into changes. Each
// non-empty line is "XY path", where X and Y are single status characters.
func ParseStatus(out string) []Change {
	results := []Change{}
	for _, line := range splitLines(out) {
		if len(line) < 4 {
			continue
		}
		results = append(results, Change{
			X:    string(line[0]),
			Y:    string(line[1]),
			Path: string(line[3:]),
		})
	}
	return results
}

// ParseCommitTimes parses lines of "<oid> <RFC3339 time>" into a map keyed
// by commit OID. Lines that lack a parseable timestamp are skipped so a
// single bad entry never drops the whole batch.
func ParseCommitTimes(out string) map[string]time.Time {
	times := map[string]time.Time{}
	for _, line := range splitLines(out) {
		oid, iso, ok := strings.Cut(line, " ")
		if !ok {
			continue
		}
		if t, err := time.Parse(time.RFC3339, iso); err == nil {
			times[oid] = t
		}
	}
	return times
}

// WorktreeList returns the porcelain output of `git worktree list` for the
// repository containing dir.
func WorktreeList(dir string) (string, error) {
	return run(dir, "worktree", "list", "--porcelain")
}

// Toplevel returns the absolute path of the worktree containing dir, i.e.
// the worktree the user is standing in.
func Toplevel(dir string) (string, error) {
	out, err := run(dir, "rev-parse", "--show-toplevel")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(out), nil
}

// Status returns the uncommitted changes in the worktree rooted at dir.
func Status(dir string) ([]Change, error) {
	out, err := run(dir, "status", "--porcelain")
	if err != nil {
		return nil, err
	}
	return ParseStatus(out), nil
}

// CommitTimes returns the commit time of each HEAD oid in a single
// `git log --no-walk` invocation. OIDs git could not resolve are absent
// from the result so callers render a placeholder.
func CommitTimes(dir string, heads []string) (map[string]time.Time, error) {
	if len(heads) == 0 {
		return map[string]time.Time{}, nil
	}
	args := append([]string{"log", "--no-walk", "--pretty=%H %cI"}, heads...)
	out, err := run(dir, args...)
	if err != nil {
		return nil, err
	}
	return ParseCommitTimes(out), nil
}

func run(dir string, args ...string) (string, error) {
	full := append([]string{"-C", dir}, args...)
	out, err := exec.Command("git", full...).Output()
	if err != nil {
		return "", err
	}
	return string(out), nil
}

func splitLines(text string) []string {
	return strings.FieldsFunc(strings.ReplaceAll(text, "\r\n", "\n"),
		func(c rune) bool { return c == '\n' })
}
