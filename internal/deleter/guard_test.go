package deleter

import (
	"os"
	"path/filepath"
	"testing"
)

func TestGuardAllowsRealWorktree(t *testing.T) {
	requireGit(t)
	repo, wtPath := setupWorktree(t, "feat")

	if !newWorktreeGuard(repo).allowsRemoval(wtPath) {
		t.Errorf("a real linked worktree should be allowed")
	}
}

func TestGuardRejectsPrimaryRepo(t *testing.T) {
	requireGit(t)
	repo, _ := setupWorktree(t, "feat")

	// The primary worktree is excluded from the registered set and the path
	// equals repoPath; deleting it would nuke the repository.
	if newWorktreeGuard(repo).allowsRemoval(repo) {
		t.Errorf("the primary repo must never be allowed for deletion")
	}
}

func TestGuardRejectsAncestorOfRepo(t *testing.T) {
	requireGit(t)
	repo, _ := setupWorktree(t, "feat")

	// repo lives at <dir>/repo; <dir> is its ancestor and removing it would
	// take the repository with it.
	if newWorktreeGuard(repo).allowsRemoval(filepath.Dir(repo)) {
		t.Errorf("an ancestor of the repo must never be allowed")
	}
}

func TestGuardRejectsUnregisteredDir(t *testing.T) {
	requireGit(t)
	repo, _ := setupWorktree(t, "feat")
	plain := filepath.Join(t.TempDir(), "plain")
	if err := os.MkdirAll(plain, 0o755); err != nil {
		t.Fatal(err)
	}

	// A directory git does not list as a worktree of repo is not removable.
	if newWorktreeGuard(repo).allowsRemoval(plain) {
		t.Errorf("a directory not registered to the repo must be rejected")
	}
}

func TestGuardRejectsEmptyPath(t *testing.T) {
	if (worktreeGuard{repoPath: "/some/repo"}).allowsRemoval("") {
		t.Errorf("an empty path must be rejected")
	}
}
