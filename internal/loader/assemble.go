package loader

import (
	"github.com/tnagatomi/gh-wtrm/internal/gh"
	"github.com/tnagatomi/gh-wtrm/internal/safety"
	"github.com/tnagatomi/gh-wtrm/internal/worktree"
)

// Info carries the per-worktree state the IO layer gathers outside the
// porcelain output: working-tree status and whether the directory is gone.
type Info struct {
	HasTrackedChanges bool
	HasUntrackedFiles bool
	NoDir             bool
}

// assemble computes badges, PR association, and deletability for each
// worktree from already-fetched data. It is the pure core of the loader,
// kept free of IO so the deletability rules can be tested directly.
//
// wts must be in git's emitted order (the primary worktree first) with
// Commits already populated. currentPath is the absolute, symlink-resolved
// path of the worktree the user is standing in. info is keyed by worktree
// path; a missing entry means "clean and present". prs are all the pull
// requests fetched for the repository.
func assemble(wts []worktree.Worktree, currentPath string, info map[string]Info, prs []gh.PullRequest, state gh.PullRequestState) []worktree.Worktree {
	out := make([]worktree.Worktree, len(wts))
	for i, w := range wts {
		isMain := i == 0
		isCurrent := w.Path == currentPath
		nfo := info[w.Path]
		matched := matchPullRequests(w.Branch, prs)

		in := safety.Input{
			IsMain:            isMain,
			IsCurrent:         isCurrent,
			Locked:            w.Locked,
			NoDir:             nfo.NoDir,
			HasUntrackedFiles: nfo.HasUntrackedFiles,
			HasTrackedChanges: nfo.HasTrackedChanges,
			Commits:           w.Commits,
			PullRequests:      matched,
		}
		w.Deletable = safety.DeleteStatus(in, state) == safety.Deletable
		w.Badges = badgesFor(w, isMain, isCurrent, nfo, matched)
		out[i] = w
	}
	return out
}

// matchPullRequests returns the PRs whose head branch matches branch. A
// blank branch (detached/bare worktree) matches nothing.
func matchPullRequests(branch string, prs []gh.PullRequest) []gh.PullRequest {
	if branch == "" {
		return nil
	}
	var matched []gh.PullRequest
	for _, pr := range prs {
		if pr.Name == branch {
			matched = append(matched, pr)
		}
	}
	return matched
}

// badgesFor derives the display badges for a worktree. The PR-state badges
// reflect the matched pull requests; the working-tree badges reflect info.
func badgesFor(w worktree.Worktree, isMain, isCurrent bool, nfo Info, prs []gh.PullRequest) []worktree.Badge {
	var badges []worktree.Badge
	if isMain {
		badges = append(badges, worktree.BadgePrimary)
	}
	if isCurrent && !isMain {
		badges = append(badges, worktree.BadgeCurrent)
	}

	var hasOpen, hasMerged, hasClosed bool
	for _, pr := range prs {
		switch pr.State {
		case gh.Open:
			hasOpen = true
		case gh.Merged:
			hasMerged = true
		case gh.Closed:
			hasClosed = true
		}
	}
	if hasMerged {
		badges = append(badges, worktree.BadgePRMerged)
	}
	if hasOpen {
		badges = append(badges, worktree.BadgePROpen)
	}
	if hasClosed {
		badges = append(badges, worktree.BadgePRClosed)
	}

	if nfo.HasTrackedChanges {
		badges = append(badges, worktree.BadgeUncommitted)
	}
	if w.Locked {
		badges = append(badges, worktree.BadgeLocked)
	}
	if nfo.NoDir {
		badges = append(badges, worktree.BadgeNoDir)
	}
	return badges
}
