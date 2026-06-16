package deleter

import (
	"path/filepath"
	"strings"

	"github.com/tnagatomi/gh-wtrm/internal/git"
	"github.com/tnagatomi/gh-wtrm/internal/worktree"
)

// registeredWorktrees returns the set of symlink-resolved paths of repoPath's
// linked worktrees — git's own authoritative view, re-read at deletion time so
// the guard reflects current state rather than the loader's earlier snapshot.
// The primary worktree (porcelain index 0) is excluded: it is the repository
// itself and must never be removed. A nil set (e.g. git could not list) means
// nothing qualifies for self-directed removal, which fails safe.
func registeredWorktrees(repoPath string) map[string]bool {
	out, err := git.WorktreeList(repoPath)
	if err != nil {
		return nil
	}
	wts := worktree.Parse(out)
	set := make(map[string]bool, len(wts))
	for i, w := range wts {
		if i == 0 || w.Path == "" {
			continue // skip the primary worktree
		}
		set[resolvePath(w.Path)] = true
	}
	return set
}

// isLinkedWorktree reports whether path is safe to recursively delete as a
// linked worktree of repoPath. It is the guard in front of os.RemoveAll: a
// false result means the directory is not removed.
//
// Membership in registered is what proves ownership. Checking only that
// <path>/.git looks like a gitdir pointer is not enough — a worktree of
// another repository, or a plain directory carrying a forged "gitdir: ..."
// file, would pass that check yet is not registered to this repo. Such paths
// are absent from `git worktree list` for repoPath and are therefore rejected,
// exactly as `git worktree remove` would have rejected them. The leading
// path-art checks are belt-and-suspenders: they reject the repository itself,
// any ancestor of it, the filesystem root, and the empty path independently of
// the git listing, so the critical "never delete the repo" invariant does not
// rely on parsing.
func isLinkedWorktree(registered map[string]bool, repoPath, path string) bool {
	if path == "" {
		return false
	}
	cleaned := filepath.Clean(path)
	repo := filepath.Clean(repoPath)
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
	return registered[resolvePath(path)]
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
