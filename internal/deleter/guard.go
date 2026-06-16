package deleter

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
)

// isLinkedWorktree reports whether path is safe to recursively delete as a
// linked worktree of repoPath. It is the guard in front of os.RemoveAll: a
// false result means "do not delete this directory ourselves" (the caller
// falls back to git, which does its own validation).
//
// A path qualifies only when it is non-empty, is neither the repository nor an
// ancestor of it (deleting an ancestor would take the repo with it), and its
// <path>/.git is a regular file beginning with "gitdir:". That gitdir pointer
// file is the hallmark of a linked worktree; the primary repository instead
// has .git as a directory, so it can never be mistaken for a removable target.
func isLinkedWorktree(repoPath, path string) bool {
	if path == "" {
		return false
	}
	cleaned := filepath.Clean(path)
	repo := filepath.Clean(repoPath)
	if cleaned == "/" || cleaned == "." || cleaned == repo {
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
	dotGit := filepath.Join(cleaned, ".git")
	info, err := os.Lstat(dotGit)
	if err != nil || !info.Mode().IsRegular() {
		return false
	}
	data, err := os.ReadFile(dotGit)
	if err != nil {
		return false
	}
	return bytes.HasPrefix(data, []byte("gitdir:"))
}
