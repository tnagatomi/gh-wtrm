// Package deleter executes a batch worktree-removal plan against a single
// repository. Per-target failures are accumulated rather than aborting,
// matching the continue-on-error rule.
package deleter

import (
	"fmt"
	"os/exec"
	"slices"

	"github.com/tnagatomi/gh-wtrm/internal/worktree"
)

// Op names the git operation that failed; carried on Failure so callers can
// group failures by kind without parsing the error message.
type Op string

const (
	OpUnlock Op = "unlock"
	OpRemove Op = "remove"
	OpPrune  Op = "prune"
	OpBranch Op = "branch"
)

// Failure records a single failed git invocation. Path is the affected
// worktree (or the repo path for prune); Op is the operation kind; Err
// carries the command error including any captured stderr.
type Failure struct {
	Path string
	Op   Op
	Err  error
}

func (f Failure) Error() string {
	return fmt.Sprintf("%s %s: %v", f.Op, f.Path, f.Err)
}

// Delete runs the deletion plan against targets in repoPath. Each target is
// independent; a failure on one does not stop the rest, and failures surface
// in targets order.
//
// Worktree removal is always forced (gh-poi semantics): the safety model has
// already cleared each safe target, and a manually selected dirty target is
// removed deliberately. Locked worktrees are unlocked first since a single
// --force does not release a lock. Branch deletion (when alsoBranches) always
// uses -D: a PR merge is not necessarily a local fast-forward merge (squash
// and rebase merges leave the local branch "unmerged"), so -d would wrongly
// refuse. The safety model already proved the work is in a merged PR.
//
// Removal runs serially: `git worktree remove` validates and rewrites the
// repo-wide .git/worktrees directory on every call, so concurrent removals
// against the same repository race (one remove enumerates entries while
// another is deleting one). Interactive deletes are a handful of worktrees,
// so serial removal costs nothing. A single `git worktree prune` is appended
// once when any target was no-dir, since prune is repo-wide.
func Delete(repoPath string, targets []worktree.Worktree, alsoBranches bool) []Failure {
	var failures []Failure
	anyNoDir := false
	for _, w := range targets {
		if slices.Contains(w.Badges, worktree.BadgeNoDir) {
			anyNoDir = true
			continue
		}
		removeFailures, removed := removeWorktree(repoPath, w)
		failures = append(failures, removeFailures...)
		if removed && alsoBranches && w.Branch != "" {
			if err := run(repoPath, "branch", "-D", w.Branch); err != nil {
				failures = append(failures, Failure{Path: w.Path, Op: OpBranch, Err: err})
			}
		}
	}

	if anyNoDir {
		if err := run(repoPath, "worktree", "prune"); err != nil {
			failures = append(failures, Failure{Path: repoPath, Op: OpPrune, Err: err})
		}
	}
	return failures
}

// removeWorktree unlocks (if locked) and force-removes a single target,
// returning any failures and whether the remove succeeded.
func removeWorktree(repoPath string, w worktree.Worktree) (failures []Failure, removed bool) {
	if slices.Contains(w.Badges, worktree.BadgeLocked) {
		if err := run(repoPath, "worktree", "unlock", w.Path); err != nil {
			failures = append(failures, Failure{Path: w.Path, Op: OpUnlock, Err: err})
			// Continue to remove anyway — --force may still succeed.
		}
	}
	if err := run(repoPath, "worktree", "remove", "--force", w.Path); err != nil {
		failures = append(failures, Failure{Path: w.Path, Op: OpRemove, Err: err})
		return failures, false
	}
	return failures, true
}

func run(repoPath string, args ...string) error {
	full := append([]string{"-C", repoPath}, args...)
	out, err := exec.Command("git", full...).CombinedOutput()
	if err != nil {
		return fmt.Errorf("%w: %s", err, out)
	}
	return nil
}
