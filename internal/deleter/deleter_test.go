package deleter

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tnagatomi/gh-wtrm/internal/worktree"
)

func TestDeleteRemovesPlainWorktree(t *testing.T) {
	requireGit(t)
	repo, wtPath := setupWorktree(t, "feat")

	failures := Delete(repo, []worktree.Worktree{{Path: wtPath, Branch: "feat"}}, false)
	if len(failures) != 0 {
		t.Fatalf("unexpected failures: %v", failures)
	}
	if _, err := os.Stat(wtPath); !os.IsNotExist(err) {
		t.Errorf("worktree directory should be gone: stat err=%v", err)
	}
	if listWorktrees(t, repo) != 1 {
		t.Errorf("porcelain should list only the primary after remove")
	}
}

func TestDeleteForcesWhenDirty(t *testing.T) {
	requireGit(t)
	repo, wtPath := setupWorktree(t, "wip")
	mustRun(t, "sh", "-c", "echo dirty > "+filepath.Join(wtPath, "f.txt"))

	failures := Delete(repo, []worktree.Worktree{
		{Path: wtPath, Branch: "wip", Badges: []worktree.Badge{worktree.BadgeUncommitted}},
	}, false)
	if len(failures) != 0 {
		t.Fatalf("dirty force-remove should succeed: %v", failures)
	}
	if _, err := os.Stat(wtPath); !os.IsNotExist(err) {
		t.Errorf("worktree directory should be gone: stat err=%v", err)
	}
}

func TestDeleteUnlocksThenForceRemovesLocked(t *testing.T) {
	requireGit(t)
	repo, wtPath := setupWorktree(t, "locked")
	mustRun(t, "git", "-C", repo, "worktree", "lock", wtPath)

	failures := Delete(repo, []worktree.Worktree{
		{Path: wtPath, Branch: "locked", Badges: []worktree.Badge{worktree.BadgeLocked}},
	}, false)
	if len(failures) != 0 {
		t.Fatalf("locked unlock+force-remove should succeed: %v", failures)
	}
	if _, err := os.Stat(wtPath); !os.IsNotExist(err) {
		t.Errorf("worktree directory should be gone: stat err=%v", err)
	}
}

func TestDeletePrunesMissingWorktree(t *testing.T) {
	requireGit(t)
	repo, wtPath := setupWorktree(t, "ghost")
	if err := os.RemoveAll(wtPath); err != nil {
		t.Fatal(err)
	}

	failures := Delete(repo, []worktree.Worktree{
		{Path: wtPath, Branch: "ghost", Badges: []worktree.Badge{worktree.BadgeNoDir}},
	}, false)
	if len(failures) != 0 {
		t.Fatalf("prune should succeed for missing: %v", failures)
	}
	if listWorktrees(t, repo) != 1 {
		t.Errorf("porcelain should list only the primary after prune")
	}
}

func TestDeleteBranchForceDeletesEvenUnmerged(t *testing.T) {
	requireGit(t)
	repo, wtPath := setupWorktree(t, "wip")
	// A commit only on the branch makes it unmerged; the safety model has
	// already proven mergedness via the PR, so the deleter force-deletes.
	mustRun(t, "git", "-C", wtPath, "commit", "--allow-empty", "-q", "-m", "wip work")

	failures := Delete(repo, []worktree.Worktree{{Path: wtPath, Branch: "wip"}}, true)
	if len(failures) != 0 {
		t.Fatalf("branch delete should succeed with -D: %v", failures)
	}
	if branchExists(t, repo, "wip") {
		t.Errorf("wip branch should be gone")
	}
}

func TestDeleteSkipsBranchWhenToggleOff(t *testing.T) {
	requireGit(t)
	repo, wtPath := setupWorktree(t, "keep")

	failures := Delete(repo, []worktree.Worktree{{Path: wtPath, Branch: "keep"}}, false)
	if len(failures) != 0 {
		t.Fatalf("unexpected failures: %v", failures)
	}
	if !branchExists(t, repo, "keep") {
		t.Errorf("branch should survive when alsoBranches is false")
	}
}

func TestDeleteContinuesOnError(t *testing.T) {
	requireGit(t)
	repo, wtPath := setupWorktree(t, "ok")

	failures := Delete(repo, []worktree.Worktree{
		{Path: "/nonexistent/path", Branch: "ghost"},
		{Path: wtPath, Branch: "ok"},
	}, false)
	if len(failures) != 1 {
		t.Fatalf("expected exactly one failure, got %d: %v", len(failures), failures)
	}
	if _, err := os.Stat(wtPath); !os.IsNotExist(err) {
		t.Errorf("second target should have been removed despite first failing: stat err=%v", err)
	}
}

func TestDeleteFailureOrderMatchesTargets(t *testing.T) {
	requireGit(t)
	repo, wtPath := setupWorktree(t, "ok")

	failures := Delete(repo, []worktree.Worktree{
		{Path: "/nonexistent/a", Branch: "a"},
		{Path: wtPath, Branch: "ok"},
		{Path: "/nonexistent/b", Branch: "b"},
	}, false)
	if len(failures) != 2 {
		t.Fatalf("expected two failures, got %d: %v", len(failures), failures)
	}
	if failures[0].Path != "/nonexistent/a" || failures[1].Path != "/nonexistent/b" {
		t.Errorf("failures out of order: %v", failures)
	}
}

func TestDeleteRemovesManyWorktrees(t *testing.T) {
	requireGit(t)
	repo, _ := setupWorktree(t, "first")
	targets := []worktree.Worktree{{Path: filepath.Join(filepath.Dir(repo), "wt-first"), Branch: "first"}}
	for i := 0; i < 9; i++ {
		branch := "p" + string(rune('a'+i))
		wtPath := filepath.Join(filepath.Dir(repo), "wt-"+branch)
		mustRun(t, "git", "-C", repo, "worktree", "add", "-q", "-b", branch, wtPath)
		targets = append(targets, worktree.Worktree{Path: wtPath, Branch: branch})
	}

	failures := Delete(repo, targets, false)
	if len(failures) != 0 {
		t.Fatalf("unexpected failures: %v", failures)
	}
	if listWorktrees(t, repo) != 1 {
		t.Errorf("porcelain should list only the primary after removing all")
	}
}

func TestDeleteNeverRemovesRepoItself(t *testing.T) {
	requireGit(t)
	repo, _ := setupWorktree(t, "feat")

	// A repoPath mistakenly handed in as a target must be rejected by the
	// guard, never recursively deleted by the deleter itself.
	failures := Delete(repo, []worktree.Worktree{{Path: repo, Branch: "main"}}, false)
	if len(failures) != 1 || failures[0].Op != OpRemove {
		t.Fatalf("expected one remove failure for the repo target, got %v", failures)
	}
	if _, err := os.Stat(repo); err != nil {
		t.Errorf("repo directory must survive: %v", err)
	}
	if _, err := os.Stat(filepath.Join(repo, ".git")); err != nil {
		t.Errorf("repo .git must survive: %v", err)
	}
}

func setupWorktree(t *testing.T, branch string) (repo, wtPath string) {
	t.Helper()
	repo = filepath.Join(t.TempDir(), "repo")
	mustRun(t, "git", "init", "-q", "-b", "main", repo)
	mustRun(t, "git", "-C", repo, "config", "user.email", "test@example.com")
	mustRun(t, "git", "-C", repo, "config", "user.name", "Test")
	mustRun(t, "git", "-C", repo, "commit", "--allow-empty", "-q", "-m", "init")
	wtPath = filepath.Join(filepath.Dir(repo), "wt-"+branch)
	mustRun(t, "git", "-C", repo, "worktree", "add", "-q", "-b", branch, wtPath)
	return repo, wtPath
}

func listWorktrees(t *testing.T, repo string) int {
	t.Helper()
	out, err := exec.Command("git", "-C", repo, "worktree", "list", "--porcelain").Output()
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	return bytes.Count(out, []byte("worktree "))
}

func branchExists(t *testing.T, repo, branch string) bool {
	t.Helper()
	out, err := exec.Command("git", "-C", repo, "branch", "--list", branch).Output()
	if err != nil {
		t.Fatalf("branch --list: %v", err)
	}
	return strings.TrimSpace(string(out)) != ""
}

func requireGit(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
}

func mustRun(t *testing.T, name string, args ...string) {
	t.Helper()
	if out, err := exec.Command(name, args...).CombinedOutput(); err != nil {
		t.Fatalf("%s %v: %v\n%s", name, args, err, out)
	}
}
