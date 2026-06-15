package safety

import (
	"testing"

	"github.com/tnagatomi/gh-wtrm/internal/gh"
)

// mergedInput is a baseline deletable worktree: a linked, clean worktree
// whose HEAD commit is contained in a merged PR. Individual tests mutate
// one field to assert it flips the decision to NotDeletable.
func mergedInput() Input {
	return Input{
		Commits: []string{"head-oid"},
		PullRequests: []gh.PullRequest{
			{Number: 1, State: gh.Merged, Commits: []string{"old-oid", "head-oid"}},
		},
	}
}

func TestDeletableBaseline(t *testing.T) {
	if got := DeleteStatus(mergedInput(), gh.Merged); got != Deletable {
		t.Fatalf("baseline should be Deletable, got %v", got)
	}
}

func TestNotDeletableConditions(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(*Input)
	}{
		{"main worktree", func(in *Input) { in.IsMain = true }},
		{"current worktree", func(in *Input) { in.IsCurrent = true }},
		{"locked worktree", func(in *Input) { in.Locked = true }},
		{"untracked files", func(in *Input) { in.HasUntrackedFiles = true }},
		{"tracked changes", func(in *Input) { in.HasTrackedChanges = true }},
		{"no pull requests", func(in *Input) { in.PullRequests = nil }},
		{"empty commits", func(in *Input) { in.Commits = nil }},
		{"an open PR present", func(in *Input) {
			in.PullRequests = append(in.PullRequests, gh.PullRequest{Number: 2, State: gh.Open, Commits: []string{"head-oid"}})
		}},
		{"merged PR but HEAD not in its commits", func(in *Input) {
			in.PullRequests = []gh.PullRequest{{Number: 1, State: gh.Merged, Commits: []string{"old-oid"}}}
		}},
		{"only a closed PR under default merged state", func(in *Input) {
			in.PullRequests = []gh.PullRequest{{Number: 1, State: gh.Closed, Commits: []string{"head-oid"}}}
		}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			in := mergedInput()
			c.mutate(&in)
			if got := DeleteStatus(in, gh.Merged); got != NotDeletable {
				t.Errorf("%s: got %v, want NotDeletable", c.name, got)
			}
		})
	}
}

func TestClosedStateOptsInClosedAndMerged(t *testing.T) {
	closed := mergedInput()
	closed.PullRequests = []gh.PullRequest{{Number: 1, State: gh.Closed, Commits: []string{"head-oid"}}}
	if got := DeleteStatus(closed, gh.Closed); got != Deletable {
		t.Errorf("closed PR under --state closed: got %v, want Deletable", got)
	}

	// In the GitHub UI a merged PR is also "closed", so --state closed must
	// still treat a merged PR as deletable.
	merged := mergedInput()
	if got := DeleteStatus(merged, gh.Closed); got != Deletable {
		t.Errorf("merged PR under --state closed: got %v, want Deletable", got)
	}
}

func TestNoDirIsDeletableWithoutPR(t *testing.T) {
	// A worktree whose directory is gone has no local work to lose; pruning
	// it only removes git's stale admin entry, so it is deletable even with
	// no PR proof.
	in := Input{NoDir: true}
	if got := DeleteStatus(in, gh.Merged); got != Deletable {
		t.Errorf("no-dir worktree: got %v, want Deletable", got)
	}
}

func TestNoDirStillBlockedByLockOrMain(t *testing.T) {
	for _, c := range []struct {
		name   string
		mutate func(*Input)
	}{
		{"locked", func(in *Input) { in.Locked = true }},
		{"main", func(in *Input) { in.IsMain = true }},
	} {
		t.Run(c.name, func(t *testing.T) {
			in := Input{NoDir: true}
			c.mutate(&in)
			if got := DeleteStatus(in, gh.Merged); got != NotDeletable {
				t.Errorf("no-dir + %s: got %v, want NotDeletable", c.name, got)
			}
		})
	}
}

func TestIsFullyMergedStateMatrix(t *testing.T) {
	in := Input{Commits: []string{"h"}}
	cases := []struct {
		name    string
		prState gh.PullRequestState
		scan    gh.PullRequestState
		want    bool
	}{
		{"merged PR, merged scan", gh.Merged, gh.Merged, true},
		{"closed PR, merged scan", gh.Closed, gh.Merged, false},
		{"open PR, merged scan", gh.Open, gh.Merged, false},
		{"merged PR, closed scan", gh.Merged, gh.Closed, true},
		{"closed PR, closed scan", gh.Closed, gh.Closed, true},
		{"open PR, closed scan", gh.Open, gh.Closed, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			pr := gh.PullRequest{State: c.prState, Commits: []string{"h"}}
			if got := IsFullyMerged(in, pr, c.scan); got != c.want {
				t.Errorf("got %v, want %v", got, c.want)
			}
		})
	}
}

func TestIsFullyMergedRequiresHeadInCommits(t *testing.T) {
	in := Input{Commits: []string{"head"}}
	pr := gh.PullRequest{State: gh.Merged, Commits: []string{"other"}}
	if IsFullyMerged(in, pr, gh.Merged) {
		t.Error("HEAD not in PR commits must not be fully merged")
	}
}
