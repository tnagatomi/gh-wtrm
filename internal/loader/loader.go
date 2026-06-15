// Package loader assembles the per-worktree state gh-wtrm renders: it lists
// the repository's worktrees, gathers their working-tree status, fetches the
// matching GitHub pull requests, and computes deletability via the safety
// model.
package loader

import (
	"context"
	"os"
	"path/filepath"

	"github.com/cli/go-gh/v2/pkg/repository"

	"github.com/tnagatomi/gh-wtrm/internal/gh"
	"github.com/tnagatomi/gh-wtrm/internal/git"
	"github.com/tnagatomi/gh-wtrm/internal/worktree"
)

// Repo is a single repository's worktrees with deletability resolved.
type Repo struct {
	// Path is the primary worktree path.
	Path string
	// Worktrees is the assembled list, primary first.
	Worktrees []worktree.Worktree
	// PRError is non-nil when the PR fetch failed. The worktree list is
	// still returned, but with no PRs matched, so every non-prunable
	// worktree falls to the safe (not-deletable) side per HANDOFF §7.
	PRError error
}

// Load lists the worktrees of the repository containing dir, fetches their
// pull requests, and resolves deletability for scan state. A fatal error is
// returned only when the local git state cannot be read; a PR-fetch failure
// is reported via Repo.PRError so the conservative list is still usable.
func Load(ctx context.Context, dir string, state gh.PullRequestState) (Repo, error) {
	out, err := git.WorktreeList(dir)
	if err != nil {
		return Repo{}, err
	}
	wts := worktree.Parse(out)
	if len(wts) == 0 {
		return Repo{Path: dir}, nil
	}

	currentPath := resolveCurrent(dir, wts)
	info := gatherInfo(wts)
	populateCommits(wts)
	populateCommitTimes(dir, wts)

	prs, prErr := fetchPullRequests(ctx, wts, state)

	assembled := assemble(wts, currentPath, info, prs, state)
	return Repo{Path: wts[0].Path, Worktrees: assembled, PRError: prErr}, nil
}

// resolveCurrent returns the porcelain path of the worktree the user is
// standing in, matching it through symlink resolution so the comparison in
// assemble succeeds on systems (e.g. macOS) that symlink temp/home paths.
func resolveCurrent(dir string, wts []worktree.Worktree) string {
	top, err := git.Toplevel(dir)
	if err != nil {
		return ""
	}
	topResolved := evalSymlinks(top)
	for _, w := range wts {
		if evalSymlinks(w.Path) == topResolved {
			return w.Path
		}
	}
	return top
}

// gatherInfo collects each worktree's no-dir state and working-tree status.
// The primary worktree (index 0) and bare worktrees never report no-dir from
// a missing directory; a prunable porcelain flag always does.
func gatherInfo(wts []worktree.Worktree) map[string]Info {
	info := make(map[string]Info, len(wts))
	for _, w := range wts {
		var nfo Info
		nfo.NoDir = w.Prunable || !dirExists(w.Path)
		if !nfo.NoDir && !w.Bare {
			if changes, err := git.Status(w.Path); err == nil {
				for _, c := range changes {
					if c.IsUntracked() {
						nfo.HasUntrackedFiles = true
					} else {
						nfo.HasTrackedChanges = true
					}
				}
			}
		}
		info[w.Path] = nfo
	}
	return info
}

// populateCommits sets each branch-bearing worktree's local HEAD OID as its
// single commit, mirroring gh-poi's quick scan mode (Commits = [HEAD]).
func populateCommits(wts []worktree.Worktree) {
	for i := range wts {
		if wts[i].HEAD != "" && !wts[i].Bare {
			wts[i].Commits = []string{wts[i].HEAD}
		}
	}
}

// populateCommitTimes fills LastCommit for display/sorting from one
// `git log --no-walk` over every worktree HEAD.
func populateCommitTimes(dir string, wts []worktree.Worktree) {
	var heads []string
	for _, w := range wts {
		if w.HEAD != "" {
			heads = append(heads, w.HEAD)
		}
	}
	times, err := git.CommitTimes(dir, heads)
	if err != nil {
		return
	}
	for i := range wts {
		if t, ok := times[wts[i].HEAD]; ok {
			wts[i].LastCommit = t
		}
	}
}

// fetchPullRequests resolves the current repository and queries PRs whose
// commit set contains any worktree's local HEAD. A nil PR slice with a
// non-nil error means the list should fall to the conservative side.
func fetchPullRequests(ctx context.Context, wts []worktree.Worktree, state gh.PullRequestState) ([]gh.PullRequest, error) {
	var oids []string
	for _, w := range wts {
		if w.Branch != "" && len(w.Commits) > 0 {
			oids = append(oids, w.Commits[0])
		}
	}
	if len(oids) == 0 {
		return nil, nil
	}

	repo, err := repository.Current()
	if err != nil {
		return nil, err
	}
	client, err := gh.NewClient(repo.Host)
	if err != nil {
		return nil, err
	}

	repos := gh.GetQueryRepos([]string{repo.Owner + "/" + repo.Name})
	seen := make(map[string]bool)
	var all []gh.PullRequest
	for _, batch := range gh.GetQueryHashes(oids) {
		prs, err := client.SearchPullRequests(ctx, repos, batch)
		if err != nil {
			return nil, err
		}
		for _, pr := range prs {
			key := pr.URL
			if seen[key] {
				continue
			}
			seen[key] = true
			all = append(all, pr)
		}
	}
	return all, nil
}

func dirExists(p string) bool {
	info, err := os.Stat(p)
	return err == nil && info.IsDir()
}

func evalSymlinks(p string) string {
	if resolved, err := filepath.EvalSymlinks(p); err == nil {
		return resolved
	}
	return p
}
