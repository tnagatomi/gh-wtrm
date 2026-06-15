// Package safety decides whether a worktree may be safely removed, porting
// gh-poi's getDeleteStatus / isFullyMerged logic to a worktree-centric model.
//
// The data-loss bug this replaces (wtclean's upstream-gone heuristic) is
// closed by the final check in IsFullyMerged: a worktree is only deletable
// when its local HEAD commit is contained in a merged pull request. A commit
// added locally after the PR merged is absent from the PR's commit set, so
// the worktree stays NotDeletable and the unmerged work is protected.
package safety

import (
	"slices"

	"github.com/tnagatomi/gh-wtrm/internal/gh"
)

// Status is the deletability verdict for a single worktree.
type Status int

const (
	NotDeletable Status = iota
	Deletable
)

// Input is the per-worktree state the deletability decision consumes. The
// loader builds it from the worktree porcelain entry, the working-tree
// status, and the matched pull requests.
type Input struct {
	// IsMain marks the primary worktree, which can never be removed.
	IsMain bool
	// IsCurrent marks the worktree containing the working directory; git
	// cannot remove the worktree you are standing in.
	IsCurrent bool
	// Locked marks a deliberately locked worktree; gh-wtrm treats a lock as
	// a refusal to delete (conservative, unlike wtclean's force-unlock).
	Locked bool
	// HasUntrackedFiles and HasTrackedChanges flag uncommitted work that
	// removal would destroy.
	HasUntrackedFiles bool
	HasTrackedChanges bool
	// Commits holds the worktree branch's local commit OIDs, HEAD first.
	Commits []string
	// PullRequests are the PRs whose commit set was matched to this branch.
	PullRequests []gh.PullRequest
}

// DeleteStatus reports whether the worktree may be removed. state selects
// the PR states that count as "fully merged": gh.Merged (the default) or
// gh.Closed (opt-in, which also accepts merged). Ported from gh-poi's
// getDeleteStatus, mapped from branches to worktrees.
func DeleteStatus(in Input, state gh.PullRequestState) Status {
	// The primary worktree and the worktree the user is standing in cannot
	// be removed by git; a lock is an explicit refusal.
	if in.IsMain || in.IsCurrent || in.Locked {
		return NotDeletable
	}

	// Any uncommitted work — tracked or untracked — would be lost.
	if in.HasUntrackedFiles || in.HasTrackedChanges {
		return NotDeletable
	}

	// No PR means no proof of merge; gh-wtrm never falls back to a local
	// `git branch --merged` heuristic (that is the bug it exists to avoid).
	if len(in.PullRequests) == 0 {
		return NotDeletable
	}

	fullyMergedCnt := 0
	for _, pr := range in.PullRequests {
		// An open PR is active work; refuse outright.
		if pr.State == gh.Open {
			return NotDeletable
		}
		if IsFullyMerged(in, pr, state) {
			fullyMergedCnt++
		}
	}
	if fullyMergedCnt == 0 {
		return NotDeletable
	}

	return Deletable
}

// IsFullyMerged reports whether pr proves the worktree's local HEAD is
// merged: the PR's state must match the scan mode, and the local HEAD OID
// must appear in the PR's commit set. Ported from gh-poi's isFullyMerged.
func IsFullyMerged(in Input, pr gh.PullRequest, state gh.PullRequestState) bool {
	if len(in.Commits) == 0 {
		return false
	}
	if (state == gh.Merged && pr.State != gh.Merged) ||
		// In the GitHub interface, closed status includes merged status, so
		// we make it behave the same way.
		// https://github.com/cli/cli/issues/8102
		(state == gh.Closed && pr.State != gh.Closed && pr.State != gh.Merged) {
		return false
	}

	localHeadOid := in.Commits[0]
	return slices.Contains(pr.Commits, localHeadOid)
}
