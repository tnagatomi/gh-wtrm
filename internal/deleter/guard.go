package deleter

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/tnagatomi/gh-wtrm/internal/git"
	"github.com/tnagatomi/gh-wtrm/internal/worktree"
)

// worktreeGuard holds the repository facts needed to decide, per target,
// whether a directory may be recursively removed. It is built once per Delete
// and shared read-only with the parallel removal workers.
type worktreeGuard struct {
	repoPath string
	// worktreesDir is the resolved <commonDir>/worktrees directory that every
	// legitimate linked-worktree admin dir of this repo lives under. Empty when
	// git could not report the common dir, which makes every removal fail safe.
	worktreesDir string
	// registered is the set of symlink-resolved paths git currently lists as
	// this repo's linked worktrees.
	registered map[string]bool
}

// newWorktreeGuard captures git's authoritative view of repoPath's linked
// worktrees, re-read at deletion time so the guard reflects current state
// rather than the loader's earlier snapshot.
func newWorktreeGuard(repoPath string) worktreeGuard {
	g := worktreeGuard{repoPath: repoPath, registered: registeredWorktrees(repoPath)}
	if commonDir, err := git.CommonDir(repoPath); err == nil {
		g.worktreesDir = resolvePath(filepath.Join(commonDir, "worktrees"))
	}
	return g
}

// registeredWorktrees returns the set of symlink-resolved paths of repoPath's
// linked worktrees. The primary worktree (porcelain index 0) is excluded: it
// is the repository itself and must never be removed. Prunable entries are
// excluded too — git considers their working tree gone or broken, so they are
// pruned, not recursively removed. A nil set (e.g. git could not list) means
// nothing qualifies for self-directed removal, which fails safe.
func registeredWorktrees(repoPath string) map[string]bool {
	out, err := git.WorktreeList(repoPath)
	if err != nil {
		return nil
	}
	wts := worktree.Parse(out)
	set := make(map[string]bool, len(wts))
	for i, w := range wts {
		if i == 0 || w.Path == "" || w.Prunable {
			continue // skip the primary and any prunable worktree
		}
		set[resolvePath(w.Path)] = true
	}
	return set
}

// allowsRemoval reports whether path is safe to recursively delete as a linked
// worktree of this repo. It is the guard in front of os.RemoveAll; a false
// result means the directory is not removed.
//
// The path-art checks reject the repository itself, any ancestor of it, the
// filesystem root, and the empty path independently of any git state, so the
// critical "never delete the repo" invariant does not rely on parsing.
// Membership in the registered set is git's own snapshot of ownership. The
// final ownsLiveWorktree check is the current on-disk truth: it defeats the
// window between listing and removal where the path could be replaced by a
// plain directory, a worktree of another repository, or any directory bearing
// a "gitdir:" file pointing elsewhere.
func (g worktreeGuard) allowsRemoval(path string) bool {
	if path == "" {
		return false
	}
	cleaned := filepath.Clean(path)
	repo := filepath.Clean(g.repoPath)
	if cleaned == string(filepath.Separator) || cleaned == "." || cleaned == repo {
		return false
	}
	// Refuse when the repo sits at or below cleaned, i.e. cleaned is an
	// ancestor: filepath.Rel then yields a path that does not climb upward.
	if rel, err := filepath.Rel(cleaned, repo); err == nil {
		upward := rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator))
		if !upward {
			return false
		}
	}
	if !g.registered[resolvePath(path)] {
		return false
	}
	return g.ownsLiveWorktree(cleaned)
}

// ownsLiveWorktree verifies that dir currently hosts a linked worktree of this
// repo by reproducing git's reciprocal validation: dir/.git must point to an
// admin directory that lives directly under this repo's worktrees dir, and
// that admin's gitdir back-pointer must reference dir. A path reoccupied by a
// foreign or stale gitdir-backed directory fails one of these and is rejected.
func (g worktreeGuard) ownsLiveWorktree(dir string) bool {
	admin := worktreeAdminDir(dir)
	if admin == "" || g.worktreesDir == "" {
		return false
	}
	if resolvePath(filepath.Dir(admin)) != g.worktreesDir {
		return false
	}
	back, err := os.ReadFile(filepath.Join(admin, "gitdir"))
	if err != nil {
		return false
	}
	// The back-pointer is relative to admin when the worktree uses relative
	// links (git worktree add --relative-paths / worktree.useRelativePaths);
	// anchor it there before comparing.
	backPath := strings.TrimSpace(string(back))
	if !filepath.IsAbs(backPath) {
		backPath = filepath.Join(admin, backPath)
	}
	return resolvePath(filepath.Dir(backPath)) == resolvePath(dir)
}

// worktreeAdminDir resolves dir/.git and returns the linked-worktree admin
// directory it points to (e.g. <repo>/.git/worktrees/<id>), or "" if dir/.git
// is not currently a "gitdir:" pointer file. A regular .git pointer file is
// the hallmark of a live linked worktree, so a non-empty result also confirms
// existence.
func worktreeAdminDir(dir string) string {
	dotGit := filepath.Join(dir, ".git")
	info, err := os.Lstat(dotGit)
	if err != nil || !info.Mode().IsRegular() {
		return ""
	}
	data, err := os.ReadFile(dotGit)
	if err != nil {
		return ""
	}
	pointer, ok := strings.CutPrefix(strings.TrimSpace(string(data)), "gitdir:")
	if !ok {
		return ""
	}
	admin := strings.TrimSpace(pointer)
	if !filepath.IsAbs(admin) {
		admin = filepath.Join(dir, admin)
	}
	return filepath.Clean(admin)
}

// isLockedWorktree reports whether the linked worktree at path currently
// carries git's lock marker. git refuses to remove a locked worktree without a
// double --force, so self-directed removal must honor the same lock: this
// recheck runs immediately before os.RemoveAll, after Phase A has had its
// chance to release locks, so it catches both a failed unlock and a lock added
// after the worktree list was built.
func isLockedWorktree(path string) bool {
	admin := worktreeAdminDir(path)
	if admin == "" {
		return false
	}
	_, err := os.Lstat(filepath.Join(admin, "locked"))
	return err == nil
}

// resolvePath canonicalizes p through symlinks so paths from `git worktree
// list` (which git stores resolved) compare equal to caller-supplied paths on
// systems that symlink temp or home directories. It falls back to a lexical
// clean when the path cannot be resolved (e.g. it no longer exists).
func resolvePath(p string) string {
	if resolved, err := filepath.EvalSymlinks(p); err == nil {
		return resolved
	}
	return filepath.Clean(p)
}
