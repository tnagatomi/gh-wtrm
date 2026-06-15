// Package deleter executes a batch worktree-removal plan against a single
// repository. Per-target failures are accumulated rather than aborting,
// matching the continue-on-error rule.
package deleter

import (
	"fmt"
	"os/exec"
	"runtime"
	"slices"
	"sync"

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

// removal holds the per-target outcome of the parallel remove phase, indexed
// back to its position in targets so failures reassemble in caller order.
type removal struct {
	idx      int
	failures []Failure
	removed  bool // remove succeeded → branch deletion is eligible
}

// Delete runs the deletion plan against targets in repoPath. Each target is
// independent; a failure on one does not stop the rest.
//
// Worktree removal is always forced (gh-poi semantics): the safety model has
// already cleared each safe target, and a manually selected dirty target is
// removed deliberately. Locked worktrees are unlocked first since a single
// --force does not release a lock. Branch deletion (when alsoBranches) always
// uses -D: a PR merge is not necessarily a local fast-forward merge (squash
// and rebase merges leave the local branch "unmerged"), so -d would wrongly
// refuse. The safety model already proved the work is in a merged PR.
//
// Removal is fanned out across a bounded worker pool since each target
// touches its own directory and .git/worktrees entry. Branch deletion runs
// serially afterwards because it mutates packed-refs, which concurrent
// `git branch -D` calls would contend on. A single `git worktree prune` is
// appended once when any target was no-dir, since prune is repo-wide.
func Delete(repoPath string, targets []worktree.Worktree, alsoBranches bool) []Failure {
	anyNoDir := false
	var procIdx []int
	for i, w := range targets {
		if slices.Contains(w.Badges, worktree.BadgeNoDir) {
			anyNoDir = true
			continue
		}
		procIdx = append(procIdx, i)
	}

	results := make([]removal, len(procIdx))
	if len(procIdx) > 0 {
		workers := min(len(procIdx), max(runtime.GOMAXPROCS(0), 4))
		sem := make(chan struct{}, workers)
		var wg sync.WaitGroup
		for j, i := range procIdx {
			wg.Add(1)
			go func() {
				defer wg.Done()
				sem <- struct{}{}
				defer func() { <-sem }()
				results[j] = removeWorktree(repoPath, i, targets[i])
			}()
		}
		wg.Wait()
	}

	var failures []Failure
	for _, r := range results {
		failures = append(failures, r.failures...)
		if r.removed && alsoBranches {
			w := targets[r.idx]
			if w.Branch != "" {
				if err := run(repoPath, "branch", "-D", w.Branch); err != nil {
					failures = append(failures, Failure{Path: w.Path, Op: OpBranch, Err: err})
				}
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

// removeWorktree unlocks (if locked) and force-removes a single target. It
// never touches refs, so it is safe to run alongside other removeWorktree
// calls against the same repository.
func removeWorktree(repoPath string, idx int, w worktree.Worktree) removal {
	r := removal{idx: idx}
	if slices.Contains(w.Badges, worktree.BadgeLocked) {
		if err := run(repoPath, "worktree", "unlock", w.Path); err != nil {
			r.failures = append(r.failures, Failure{Path: w.Path, Op: OpUnlock, Err: err})
			// Continue to remove anyway — --force may still succeed.
		}
	}
	if err := run(repoPath, "worktree", "remove", "--force", w.Path); err != nil {
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
