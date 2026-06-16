package deleter

import (
	"os"
	"path/filepath"
	"testing"
)

func TestIsLinkedWorktreeAcceptsRealWorktree(t *testing.T) {
	requireGit(t)
	repo, wtPath := setupWorktree(t, "feat")

	if !isLinkedWorktree(repo, wtPath) {
		t.Errorf("a real linked worktree should be accepted")
	}
}

func TestIsLinkedWorktreeRejectsPrimaryRepo(t *testing.T) {
	requireGit(t)
	repo, _ := setupWorktree(t, "feat")

	// The primary repo's .git is a directory, not a gitdir pointer file, and
	// the path equals repoPath; deleting it would nuke the repository.
	if isLinkedWorktree(repo, repo) {
		t.Errorf("the primary repo must never be accepted for deletion")
	}
}

func TestIsLinkedWorktreeRejectsAncestorOfRepo(t *testing.T) {
	requireGit(t)
	repo, _ := setupWorktree(t, "feat")

	// repo lives at <dir>/repo; <dir> is its ancestor and removing it would
	// take the repository with it.
	if isLinkedWorktree(repo, filepath.Dir(repo)) {
		t.Errorf("an ancestor of the repo must never be accepted")
	}
}

func TestIsLinkedWorktreeRejectsNonWorktreeDir(t *testing.T) {
	repo := filepath.Join(t.TempDir(), "repo")
	plain := filepath.Join(t.TempDir(), "plain")
	if err := os.MkdirAll(plain, 0o755); err != nil {
		t.Fatal(err)
	}

	// A directory with no gitdir pointer file is not a linked worktree.
	if isLinkedWorktree(repo, plain) {
		t.Errorf("a directory without a .git pointer file must be rejected")
	}
}

func TestIsLinkedWorktreeRejectsEmptyPath(t *testing.T) {
	if isLinkedWorktree("/some/repo", "") {
		t.Errorf("an empty path must be rejected")
	}
}
