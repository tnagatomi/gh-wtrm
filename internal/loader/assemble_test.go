package loader

import (
	"slices"
	"testing"

	"github.com/tnagatomi/gh-wtrm/internal/gh"
	"github.com/tnagatomi/gh-wtrm/internal/worktree"
)

func hasBadge(w worktree.Worktree, b worktree.Badge) bool {
	return slices.Contains(w.Badges, b)
}

func TestAssemblePrimaryNeverDeletable(t *testing.T) {
	wts := []worktree.Worktree{
		{Path: "/repo", Branch: "main", Commits: []string{"m"}},
	}
	got := assemble(wts, "/elsewhere", nil, nil, gh.Merged)
	if got[0].Deletable {
		t.Error("primary worktree must not be deletable")
	}
	if !hasBadge(got[0], worktree.BadgePrimary) {
		t.Errorf("primary worktree missing primary badge: %v", got[0].Badges)
	}
}

func TestAssembleMergedLinkedIsDeletable(t *testing.T) {
	wts := []worktree.Worktree{
		{Path: "/repo", Branch: "main", Commits: []string{"m"}},
		{Path: "/repo/wt/feat", Branch: "feat-x", Commits: []string{"head"}},
	}
	prs := []gh.PullRequest{
		{Number: 1, Name: "feat-x", State: gh.Merged, Commits: []string{"old", "head"}},
	}
	got := assemble(wts, "/repo", nil, prs, gh.Merged)
	feat := got[1]
	if !feat.Deletable {
		t.Error("merged linked worktree should be deletable")
	}
	if !hasBadge(feat, worktree.BadgePRMerged) {
		t.Errorf("missing pr-merged badge: %v", feat.Badges)
	}
}

func TestAssembleOpenPRNotDeletable(t *testing.T) {
	wts := []worktree.Worktree{
		{Path: "/repo", Branch: "main", Commits: []string{"m"}},
		{Path: "/repo/wt/feat", Branch: "feat-x", Commits: []string{"head"}},
	}
	prs := []gh.PullRequest{
		{Number: 1, Name: "feat-x", State: gh.Open, Commits: []string{"head"}},
	}
	got := assemble(wts, "/repo", nil, prs, gh.Merged)
	if got[1].Deletable {
		t.Error("worktree with an open PR must not be deletable")
	}
	if !hasBadge(got[1], worktree.BadgePROpen) {
		t.Errorf("missing pr-open badge: %v", got[1].Badges)
	}
}

func TestAssembleUncommittedNotDeletable(t *testing.T) {
	wts := []worktree.Worktree{
		{Path: "/repo", Branch: "main", Commits: []string{"m"}},
		{Path: "/repo/wt/feat", Branch: "feat-x", Commits: []string{"head"}},
	}
	prs := []gh.PullRequest{
		{Number: 1, Name: "feat-x", State: gh.Merged, Commits: []string{"head"}},
	}
	info := map[string]Info{"/repo/wt/feat": {HasTrackedChanges: true}}
	got := assemble(wts, "/repo", info, prs, gh.Merged)
	if got[1].Deletable {
		t.Error("worktree with tracked changes must not be deletable")
	}
	if !hasBadge(got[1], worktree.BadgeUncommitted) {
		t.Errorf("missing uncommitted badge: %v", got[1].Badges)
	}
}

func TestAssembleNoDirIsDeletable(t *testing.T) {
	wts := []worktree.Worktree{
		{Path: "/repo", Branch: "main", Commits: []string{"m"}},
		{Path: "/repo/wt/gone", Branch: "gone", Prunable: true},
	}
	info := map[string]Info{"/repo/wt/gone": {NoDir: true}}
	got := assemble(wts, "/repo", info, nil, gh.Merged)
	if !got[1].Deletable {
		t.Error("no-dir worktree should be deletable (prune target)")
	}
	if !hasBadge(got[1], worktree.BadgeNoDir) {
		t.Errorf("missing no-dir badge: %v", got[1].Badges)
	}
}

func TestAssembleCurrentWorktreeNotDeletable(t *testing.T) {
	wts := []worktree.Worktree{
		{Path: "/repo", Branch: "main", Commits: []string{"m"}},
		{Path: "/repo/wt/feat", Branch: "feat-x", Commits: []string{"head"}},
	}
	prs := []gh.PullRequest{
		{Number: 1, Name: "feat-x", State: gh.Merged, Commits: []string{"head"}},
	}
	// Standing in the linked worktree: it must not be deletable even though
	// its PR is merged.
	got := assemble(wts, "/repo/wt/feat", nil, prs, gh.Merged)
	if got[1].Deletable {
		t.Error("the current worktree must not be deletable")
	}
}

func TestAssembleLockedHasBadgeAndNotDeletable(t *testing.T) {
	wts := []worktree.Worktree{
		{Path: "/repo", Branch: "main", Commits: []string{"m"}},
		{Path: "/repo/wt/feat", Branch: "feat-x", Commits: []string{"head"}, Locked: true},
	}
	prs := []gh.PullRequest{
		{Number: 1, Name: "feat-x", State: gh.Merged, Commits: []string{"head"}},
	}
	got := assemble(wts, "/repo", nil, prs, gh.Merged)
	if got[1].Deletable {
		t.Error("locked worktree must not be deletable")
	}
	if !hasBadge(got[1], worktree.BadgeLocked) {
		t.Errorf("missing locked badge: %v", got[1].Badges)
	}
}
