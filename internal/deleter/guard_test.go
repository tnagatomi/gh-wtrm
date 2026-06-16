package deleter

import (
	"os"
	"path/filepath"
	"testing"
)

func TestIsLinkedWorktreeAcceptsRealWorktree(t *testing.T) {
	requireGit(t)
	repo, wtPath := setupWorktree(t, "feat")

	if !isLinkedWorktree(registeredWorktrees(repo), repo, wtPath) {
		t.Errorf("a real linked worktree should be accepted")
	}
}

func TestIsLinkedWorktreeRejectsPrimaryRepo(t *testing.T) {
	requireGit(t)
	repo, _ := setupWorktree(t, "feat")

	// The primary worktree is excluded from the registered set and the path
	// equals repoPath; deleting it would nuke the repository.
	if isLinkedWorktree(registeredWorktrees(repo), repo, repo) {
		t.Errorf("the primary repo must never be accepted for deletion")
	}
}

func TestIsLinkedWorktreeRejectsAncestorOfRepo(t *testing.T) {
	requireGit(t)
	repo, _ := setupWorktree(t, "feat")

	// repo lives at <dir>/repo; <dir> is its ancestor and removing it would
	// take the repository with it.
	if isLinkedWorktree(registeredWorktrees(repo), repo, filepath.Dir(repo)) {
		t.Errorf("an ancestor of the repo must never be accepted")
	}
}

func TestIsLinkedWorktreeRejectsUnregisteredDir(t *testing.T) {
	repo := filepath.Join(t.TempDir(), "repo")
	plain := filepath.Join(t.TempDir(), "plain")
	if err := os.MkdirAll(plain, 0o755); err != nil {
		t.Fatal(err)
	}

	// A directory git does not list as a worktree of repo is not removable,
	// even with an empty registered set.
	if isLinkedWorktree(nil, repo, plain) {
		t.Errorf("a directory not registered to the repo must be rejected")
	}
}

func TestIsLinkedWorktreeRejectsEmptyPath(t *testing.T) {
	if isLinkedWorktree(nil, "/some/repo", "") {
		t.Errorf("an empty path must be rejected")
	}
}
