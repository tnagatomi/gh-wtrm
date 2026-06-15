package git

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// initRepo creates a throwaway git repo with one commit and returns its path.
func initRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	for _, args := range [][]string{
		{"init", "-q", "-b", "main"},
		{"config", "user.email", "test@example.com"},
		{"config", "user.name", "test"},
	} {
		cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	if err := os.WriteFile(filepath.Join(dir, "f.txt"), []byte("hello\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{
		{"add", "."},
		{"commit", "-q", "-m", "init"},
	} {
		cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	return dir
}

func TestWorktreeListAndToplevel(t *testing.T) {
	dir := initRepo(t)

	out, err := WorktreeList(dir)
	if err != nil {
		t.Fatalf("WorktreeList: %v", err)
	}
	if !strings.Contains(out, "worktree ") || !strings.Contains(out, "branch refs/heads/main") {
		t.Errorf("unexpected porcelain output:\n%s", out)
	}

	top, err := Toplevel(dir)
	if err != nil {
		t.Fatalf("Toplevel: %v", err)
	}
	// macOS temp dirs are symlinked (/var -> /private/var); compare basenames.
	if filepath.Base(top) != filepath.Base(dir) {
		t.Errorf("Toplevel = %q, want basename %q", top, filepath.Base(dir))
	}
}

func TestStatusDetectsUntracked(t *testing.T) {
	dir := initRepo(t)
	if err := os.WriteFile(filepath.Join(dir, "new.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	changes, err := Status(dir)
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	if len(changes) != 1 || !changes[0].IsUntracked() || changes[0].Path != "new.txt" {
		t.Errorf("changes = %+v", changes)
	}
}

func TestCommitTimesResolvesHead(t *testing.T) {
	dir := initRepo(t)
	headOut, err := run(dir, "rev-parse", "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	head := strings.TrimSpace(headOut)
	times, err := CommitTimes(dir, []string{head})
	if err != nil {
		t.Fatalf("CommitTimes: %v", err)
	}
	if times[head].IsZero() {
		t.Errorf("expected a commit time for HEAD %s, got zero", head)
	}
}
