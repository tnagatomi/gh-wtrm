// Package worktree parses the porcelain output of `git worktree list` and
// models the per-worktree state badges shown in the TUI.
package worktree

import (
	"bufio"
	"slices"
	"strings"
	"time"
)

// Worktree describes a single entry from `git worktree list --porcelain`.
// Branch is stripped of its refs/heads/ prefix so callers can render it
// directly.
type Worktree struct {
	Path           string
	HEAD           string
	Branch         string
	Bare           bool
	Detached       bool
	Locked         bool
	LockReason     string
	Prunable       bool
	PrunableReason string

	// LastCommit is the commit time of HEAD. Parse leaves it zero; the
	// loader populates it after parsing since porcelain output does not
	// carry commit metadata.
	LastCommit time.Time

	// Commits holds the worktree branch's local commit OIDs (HEAD first),
	// used to match the local HEAD against a PR's commit set. The loader
	// fills this from `git log`.
	Commits []string

	// Deletable reports whether the safety model cleared this worktree for
	// removal (gh-poi's Deletable status). The loader computes it from PR
	// merge state and the working-tree condition; the TUI uses it to drive
	// the `s` safe-select key.
	Deletable bool

	// Badges summarize derived state (pr-merged, uncommitted, etc.) for the
	// UI. Parse leaves this nil; the loader fills it after consulting the
	// working tree and the matched pull requests.
	Badges []Badge
}

// Badge identifies a state callout shown next to a worktree in the TUI.
type Badge int

const (
	BadgePrimary Badge = iota
	BadgeUncommitted
	BadgeUnpushed
	BadgeLocked
	BadgeNoDir
	BadgePRMerged
	BadgePROpen
	BadgePRClosed
)

// HasAnyBadge returns true when w carries at least one of badges. Used by
// the deleter to decide on --force and by the TUI to flag warning rows.
func (w Worktree) HasAnyBadge(badges []Badge) bool {
	for _, b := range badges {
		if slices.Contains(w.Badges, b) {
			return true
		}
	}
	return false
}

func (b Badge) String() string {
	switch b {
	case BadgePrimary:
		return "primary"
	case BadgeUncommitted:
		return "uncommitted"
	case BadgeUnpushed:
		return "unpushed"
	case BadgeLocked:
		return "locked"
	case BadgeNoDir:
		return "no-dir"
	case BadgePRMerged:
		return "pr-merged"
	case BadgePROpen:
		return "pr-open"
	case BadgePRClosed:
		return "pr-closed"
	}
	return ""
}

// Parse reads the porcelain output of `git worktree list --porcelain` and
// returns the worktree entries in the order git emitted them. Unknown
// attribute lines are ignored so future git versions adding fields do not
// break parsing.
func Parse(s string) []Worktree {
	var (
		results []Worktree
		current *Worktree
	)
	scanner := bufio.NewScanner(strings.NewReader(s))
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			if current != nil {
				results = append(results, *current)
				current = nil
			}
			continue
		}
		if current == nil {
			current = &Worktree{}
		}
		key, value, _ := strings.Cut(line, " ")
		switch key {
		case "worktree":
			current.Path = value
		case "HEAD":
			current.HEAD = value
		case "branch":
			current.Branch = strings.TrimPrefix(value, "refs/heads/")
		case "bare":
			current.Bare = true
		case "detached":
			current.Detached = true
		case "locked":
			current.Locked = true
			current.LockReason = value
		case "prunable":
			current.Prunable = true
			current.PrunableReason = value
		}
	}
	if current != nil {
		results = append(results, *current)
	}
	return results
}
