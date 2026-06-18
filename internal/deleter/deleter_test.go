package deleter

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
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

func TestDeleteParallelPreservesOrderAndRemovesAll(t *testing.T) {
	requireGit(t)
	// Force the worker pool to drain through a single worker so the queueing
	// path (more targets than workers) is exercised deterministically.
	defer runtime.GOMAXPROCS(runtime.GOMAXPROCS(1))

	repo, first := setupWorktree(t, "first")
	targets := []worktree.Worktree{
		{Path: "/nonexistent/a", Branch: "a"},
		{Path: first, Branch: "first"},
	}
	for i := range 5 {
		branch := "p" + string(rune('a'+i))
		wtPath := filepath.Join(filepath.Dir(repo), "wt-"+branch)
		mustRun(t, "git", "-C", repo, "worktree", "add", "-q", "-b", branch, wtPath)
		targets = append(targets, worktree.Worktree{Path: wtPath, Branch: branch})
	}
	targets = append(targets, worktree.Worktree{Path: "/nonexistent/b", Branch: "b"})

	failures := Delete(repo, targets, false)

	if len(failures) != 2 {
		t.Fatalf("expected two failures for the nonexistent targets, got %d: %v", len(failures), failures)
	}
	if failures[0].Path != "/nonexistent/a" || failures[1].Path != "/nonexistent/b" {
		t.Errorf("failures must stay in targets order despite parallel removal: %v", failures)
	}
	if listWorktrees(t, repo) != 1 {
		t.Errorf("all real worktrees should be removed, leaving only the primary")
	}
}

func TestDeleteRejectsForgedGitdirDirectory(t *testing.T) {
	requireGit(t)
	repo, _ := setupWorktree(t, "feat")

	// A directory that is not a registered worktree of repo but carries a
	// .git file with a gitdir: pointer, mimicking a linked worktree.
	fake := filepath.Join(filepath.Dir(repo), "fake")
	if err := os.MkdirAll(fake, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(fake, ".git"), []byte("gitdir: /tmp/nowhere\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(fake, "important.txt"), []byte("keep me"), 0o644); err != nil {
		t.Fatal(err)
	}

	failures := Delete(repo, []worktree.Worktree{{Path: fake, Branch: "x"}}, false)

	if len(failures) != 1 || failures[0].Op != OpRemove {
		t.Fatalf("a forged worktree must be rejected with a remove failure, got %v", failures)
	}
	if _, err := os.Stat(filepath.Join(fake, "important.txt")); err != nil {
		t.Errorf("a directory not registered to the repo must not be deleted: %v", err)
	}
}

func TestDeleteRejectsForeignWorktree(t *testing.T) {
	requireGit(t)
	repo, _ := setupWorktree(t, "feat")
	// A genuine linked worktree, but of a different repository.
	_, foreign := setupWorktree(t, "other")

	failures := Delete(repo, []worktree.Worktree{{Path: foreign, Branch: "other"}}, false)

	if len(failures) != 1 || failures[0].Op != OpRemove {
		t.Fatalf("a worktree of another repo must be rejected, got %v", failures)
	}
	if _, err := os.Stat(foreign); err != nil {
		t.Errorf("a foreign worktree must not be deleted: %v", err)
	}
}

func TestDeleteRejectsWorktreeReplacedByPlainDir(t *testing.T) {
	requireGit(t)
	repo, wtPath := setupWorktree(t, "feat")

	// Simulate the worktree directory being swapped for a plain directory
	// after it was loaded: git still lists the path (now prunable) but its
	// .git pointer is gone, exactly the case git worktree remove --force
	// rejects.
	if err := os.RemoveAll(wtPath); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(wtPath, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(wtPath, "important.txt"), []byte("keep me"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Passed as a healthy target (no no-dir badge), mimicking a stale load.
	failures := Delete(repo, []worktree.Worktree{{Path: wtPath, Branch: "feat"}}, false)

	if len(failures) != 1 || failures[0].Op != OpRemove {
		t.Fatalf("a worktree whose .git pointer is gone must be rejected, got %v", failures)
	}
	if _, err := os.Stat(filepath.Join(wtPath, "important.txt")); err != nil {
		t.Errorf("the replacement directory must not be deleted: %v", err)
	}
}

func TestDeleteRefusesCurrentlyLockedWorktree(t *testing.T) {
	requireGit(t)
	repo, wtPath := setupWorktree(t, "feat")
	mustRun(t, "git", "-C", repo, "worktree", "lock", wtPath)

	// Locked after the UI built badges, so no BadgeLocked is present: Phase A
	// never unlocks it. Removal must refuse rather than bypass git's lock
	// protection, matching git worktree remove --force on a locked worktree.
	failures := Delete(repo, []worktree.Worktree{{Path: wtPath, Branch: "feat"}}, false)

	if len(failures) != 1 || failures[0].Op != OpRemove {
		t.Fatalf("a currently locked worktree must be refused, got %v", failures)
	}
	if _, err := os.Stat(wtPath); err != nil {
		t.Errorf("a locked worktree must not be deleted: %v", err)
	}
}

func TestDeleteRejectsReoccupiedWorktreePath(t *testing.T) {
	requireGit(t)
	repo, wtPath := setupWorktree(t, "feat")
	// A separate repository whose worktree's gitdir pointer we graft onto our
	// target, simulating the path being reoccupied by a foreign gitdir-backed
	// directory after repo's worktree list was captured. repo still lists the
	// path, but its .git now points to the other repo's admin dir.
	_, foreign := setupWorktree(t, "other")
	foreignDotGit, err := os.ReadFile(filepath.Join(foreign, ".git"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(wtPath, ".git"), foreignDotGit, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(wtPath, "important.txt"), []byte("keep me"), 0o644); err != nil {
		t.Fatal(err)
	}

	failures := Delete(repo, []worktree.Worktree{{Path: wtPath, Branch: "feat"}}, false)

	if len(failures) != 1 || failures[0].Op != OpRemove {
		t.Fatalf("a path whose gitdir no longer belongs to repo must be refused, got %v", failures)
	}
	if _, err := os.Stat(filepath.Join(wtPath, "important.txt")); err != nil {
		t.Errorf("a reoccupied directory must not be deleted: %v", err)
	}
}

func TestDeleteRemovesRelativePathsWorktree(t *testing.T) {
	requireGit(t)
	repo := filepath.Join(t.TempDir(), "repo")
	mustRun(t, "git", "init", "-q", "-b", "main", repo)
	mustRun(t, "git", "-C", repo, "config", "user.email", "test@example.com")
	mustRun(t, "git", "-C", repo, "config", "user.name", "Test")
	mustRun(t, "git", "-C", repo, "commit", "--allow-empty", "-q", "-m", "init")
	wtPath := filepath.Join(filepath.Dir(repo), "wt-rel")

	// --relative-paths makes git store relative gitdir links (both the
	// worktree's .git and the admin back-pointer). Skip on git too old for it.
	if out, err := exec.Command("git", "-C", repo, "worktree", "add", "--relative-paths", "-q", "-b", "rel", wtPath).CombinedOutput(); err != nil {
		t.Skipf("git worktree add --relative-paths unsupported: %v\n%s", err, out)
	}

	failures := Delete(repo, []worktree.Worktree{{Path: wtPath, Branch: "rel"}}, false)

	if len(failures) != 0 {
		t.Fatalf("a relative-paths worktree should be removed cleanly, got %v", failures)
	}
	if _, err := os.Stat(wtPath); !os.IsNotExist(err) {
		t.Errorf("relative-paths worktree directory should be gone: stat err=%v", err)
	}
	if listWorktrees(t, repo) != 1 {
		t.Errorf("porcelain should list only the primary after remove")
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

func TestDeleteRefusesCurrentWorktree(t *testing.T) {
	requireGit(t)
	repo, wtPath := setupWorktree(t, "feat")

	// Stand inside the target: the deleter must refuse it rather than remove
	// the process's own CWD, which would break the post-delete reload.
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := os.Chdir(wd); err != nil {
			t.Logf("restore cwd: %v", err)
		}
	}()
	if err := os.Chdir(wtPath); err != nil {
		t.Fatal(err)
	}

	failures := Delete(repo, []worktree.Worktree{{Path: wtPath, Branch: "feat"}}, false)
	if len(failures) != 1 || failures[0].Op != OpRemove {
		t.Fatalf("the current worktree must be refused with a remove failure, got %v", failures)
	}
	if _, err := os.Stat(wtPath); err != nil {
		t.Errorf("the current worktree must not be deleted: %v", err)
	}
}

func TestDeleteRefusesWorktreeWhenCwdIsSubdir(t *testing.T) {
	requireGit(t)
	repo, wtPath := setupWorktree(t, "feat")
	sub := filepath.Join(wtPath, "sub")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}

	// Standing in a subdirectory of the target is the same hazard: the CWD
	// vanishes with the worktree, so the reload's git -C <gone> would 128.
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := os.Chdir(wd); err != nil {
			t.Logf("restore cwd: %v", err)
		}
	}()
	if err := os.Chdir(sub); err != nil {
		t.Fatal(err)
	}

	failures := Delete(repo, []worktree.Worktree{{Path: wtPath, Branch: "feat"}}, false)
	if len(failures) != 1 || failures[0].Op != OpRemove {
		t.Fatalf("a worktree containing the CWD must be refused, got %v", failures)
	}
	if _, err := os.Stat(wtPath); err != nil {
		t.Errorf("the worktree containing the CWD must not be deleted: %v", err)
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
