// Package deleter executes a batch worktree-removal plan against a single
// repository. Per-target failures are accumulated rather than aborting,
// matching the continue-on-error rule.
package deleter

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"slices"
	"sync"

	"github.com/tnagatomi/gh-wtrm/internal/worktree"
)

// errNotLinkedWorktree marks a target whose directory we declined to remove
// ourselves because the guard could not confirm it is a linked worktree.
var errNotLinkedWorktree = errors.New("path is not a linked worktree; refusing to remove")

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

// result accumulates the outcome of processing a single target so failures
// can be reassembled in targets order after the phased work completes.
type result struct {
	failures []Failure
	removed  bool // a present directory was cleared: branch-eligible and prunable
	noDir    bool // the directory was already gone: prunable but not branch-eligible
}

// Delete runs the deletion plan against targets in repoPath. Each target is
// independent; a failure on one does not stop the rest, and failures surface
// in targets order.
//
// Worktree removal is always forced (gh-poi semantics): the safety model has
// already cleared each safe target, and a manually selected dirty target is
// removed deliberately. Locked worktrees are unlocked first since a lock keeps
// `git worktree prune` from reclaiming the metadata. Branch deletion (when
// alsoBranches) always uses -D: a PR merge is not necessarily a local
// fast-forward merge (squash and rebase merges leave the local branch
// "unmerged"), so -d would wrongly refuse. The safety model already proved the
// work is in a merged PR.
//
// The expensive part of removing a worktree is the recursive deletion of its
// working directory (node_modules and the like), not git's metadata rewrite.
// So instead of `git worktree remove` — which couples both and rewrites the
// repo-wide .git/worktrees directory on every call — we delete the directory
// ourselves with os.RemoveAll and then issue a single repo-wide
// `git worktree prune` to drop the metadata for everything we cleared. The
// guard (isLinkedWorktree) stands in for the validation git would have done.
//
// All git metadata mutations (unlock, prune, branch) run serially, since they
// race on the repo-wide .git/worktrees directory. Only the directory removal
// is parallelized — it touches independent file trees and no git state — so a
// batch of large worktrees is cleared concurrently rather than one at a time.
func Delete(repoPath string, targets []worktree.Worktree, alsoBranches bool) []Failure {
	results := make([]result, len(targets))

	// git's authoritative set of this repo's linked worktrees, read once and
	// shared read-only with the parallel removal workers.
	registered := registeredWorktrees(repoPath)

	// Phase A: release locks first (serial git metadata) — a still-locked
	// worktree keeps the later prune from reclaiming its metadata.
	for i, w := range targets {
		if slices.Contains(w.Badges, worktree.BadgeLocked) {
			if err := run(repoPath, "worktree", "unlock", w.Path); err != nil {
				results[i].failures = append(results[i].failures, Failure{Path: w.Path, Op: OpUnlock, Err: err})
				// Continue anyway — the directory removal does not need the lock.
			}
		}
	}

	// Phase B: clear working directories in parallel.
	clearDirs(registered, repoPath, targets, results)

	// One repo-wide prune drops the metadata for every directory we cleared
	// (or that was already missing). Prune is serial and runs at most once.
	needPrune := false
	for i := range results {
		if results[i].removed || results[i].noDir {
			needPrune = true
		}
	}
	var pruneFailure *Failure
	if needPrune {
		if err := run(repoPath, "worktree", "prune"); err != nil {
			pruneFailure = &Failure{Path: repoPath, Op: OpPrune, Err: err}
		}
	}

	// Branch deletion applies only to present-dir worktrees we actually
	// removed, matching prior behavior where missing-dir targets never had
	// their branches deleted.
	if alsoBranches {
		for i, w := range targets {
			if results[i].removed && w.Branch != "" {
				if err := run(repoPath, "branch", "-D", w.Branch); err != nil {
					results[i].failures = append(results[i].failures, Failure{Path: w.Path, Op: OpBranch, Err: err})
				}
			}
		}
	}

	// Reassemble in targets order; the repo-wide prune failure trails last.
	var failures []Failure
	for i := range results {
		failures = append(failures, results[i].failures...)
	}
	if pruneFailure != nil {
		failures = append(failures, *pruneFailure)
	}
	return failures
}

// clearDirs clears every target's working directory concurrently, bounded by
// a worker pool of GOMAXPROCS (capped at the target count). Each worker owns a
// distinct index, so writes to results never overlap; failures are reassembled
// in targets order by the caller. The work is purely filesystem-side — no git
// state is touched here — so concurrency is safe.
func clearDirs(registered map[string]bool, repoPath string, targets []worktree.Worktree, results []result) {
	workers := min(runtime.GOMAXPROCS(0), len(targets))
	if workers < 1 {
		return
	}

	indices := make(chan int)
	var wg sync.WaitGroup
	for range workers {
		wg.Go(func() {
			for i := range indices {
				r := clearWorktreeDir(registered, repoPath, targets[i])
				results[i].failures = append(results[i].failures, r.failures...)
				results[i].removed = r.removed
				results[i].noDir = r.noDir
			}
		})
	}
	for i := range targets {
		indices <- i
	}
	close(indices)
	wg.Wait()
}

// clearWorktreeDir clears a single target's working directory: a no-dir target
// needs only pruning; a guarded directory is removed with os.RemoveAll;
// anything the guard rejects is recorded as a remove failure. It performs no
// git operations, so it is safe to call concurrently across targets.
func clearWorktreeDir(registered map[string]bool, repoPath string, w worktree.Worktree) result {
	var r result
	if slices.Contains(w.Badges, worktree.BadgeNoDir) {
		r.noDir = true
		return r
	}
	if !isLinkedWorktree(registered, repoPath, w.Path) {
		r.failures = append(r.failures, Failure{Path: w.Path, Op: OpRemove, Err: errNotLinkedWorktree})
		return r
	}
	if err := os.RemoveAll(w.Path); err != nil {
		r.failures = append(r.failures, Failure{Path: w.Path, Op: OpRemove, Err: err})
		return r
	}
	r.removed = true
	return r
}

func run(repoPath string, args ...string) error {
	full := append([]string{"-C", repoPath}, args...)
	out, err := exec.Command("git", full...).CombinedOutput()
	if err != nil {
		return fmt.Errorf("%w: %s", err, out)
	}
	return nil
}
