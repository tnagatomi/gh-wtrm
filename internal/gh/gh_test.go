package gh

import (
	"strings"
	"testing"
)

func TestToPullRequestState(t *testing.T) {
	cases := []struct {
		in   string
		want PullRequestState
		err  bool
	}{
		{"OPEN", Open, false},
		{"MERGED", Merged, false},
		{"CLOSED", Closed, false},
		{"WAT", 0, true},
	}
	for _, c := range cases {
		got, err := ToPullRequestState(c.in)
		if c.err {
			if err == nil {
				t.Errorf("%q: expected error", c.in)
			}
			continue
		}
		if err != nil {
			t.Errorf("%q: unexpected error %v", c.in, err)
		}
		if got != c.want {
			t.Errorf("%q: got %v, want %v", c.in, got, c.want)
		}
	}
}

func TestToPullRequests(t *testing.T) {
	var resp searchResponse
	resp.Search.Edges = make([]searchEdge, 2)
	resp.Search.Edges[0].Node.Number = 7
	resp.Search.Edges[0].Node.HeadRefName = "feat-x"
	resp.Search.Edges[0].Node.URL = "https://github.com/o/r/pull/7"
	resp.Search.Edges[0].Node.State = "MERGED"
	resp.Search.Edges[0].Node.IsDraft = false
	resp.Search.Edges[0].Node.Author.Login = "alice"
	resp.Search.Edges[0].Node.Commits.Nodes = []commitNode{{}, {}}
	resp.Search.Edges[0].Node.Commits.Nodes[0].Commit.Oid = "aaaa"
	resp.Search.Edges[0].Node.Commits.Nodes[1].Commit.Oid = "bbbb"
	resp.Search.Edges[1].Node.Number = 9
	resp.Search.Edges[1].Node.HeadRefName = "feat-y"
	resp.Search.Edges[1].Node.State = "OPEN"
	resp.Search.Edges[1].Node.IsDraft = true

	prs, err := toPullRequests(resp)
	if err != nil {
		t.Fatalf("toPullRequests: %v", err)
	}
	if len(prs) != 2 {
		t.Fatalf("got %d PRs, want 2", len(prs))
	}
	if prs[0].Number != 7 || prs[0].Name != "feat-x" || prs[0].State != Merged || prs[0].Author != "alice" {
		t.Errorf("pr[0] = %+v", prs[0])
	}
	if len(prs[0].Commits) != 2 || prs[0].Commits[0] != "aaaa" || prs[0].Commits[1] != "bbbb" {
		t.Errorf("pr[0].Commits = %v", prs[0].Commits)
	}
	if prs[1].State != Open || !prs[1].IsDraft {
		t.Errorf("pr[1] = %+v", prs[1])
	}
}

func TestToPullRequestsRejectsUnknownState(t *testing.T) {
	var resp searchResponse
	resp.Search.Edges = make([]searchEdge, 1)
	resp.Search.Edges[0].Node.State = "BOGUS"
	if _, err := toPullRequests(resp); err == nil {
		t.Error("expected error for unknown PR state")
	}
}

func TestGetQueryRepos(t *testing.T) {
	got := GetQueryRepos([]string{"o/r", "o/fork"})
	if got != "repo:o/r repo:o/fork" {
		t.Errorf("got %q", got)
	}
}

func TestGetQueryHashesEmpty(t *testing.T) {
	if got := GetQueryHashes(nil); len(got) != 0 {
		t.Errorf("got %v, want empty", got)
	}
}

func TestGetQueryHashesSingleBatch(t *testing.T) {
	got := GetQueryHashes([]string{"aaaa", "bbbb"})
	if len(got) != 1 {
		t.Fatalf("got %d batches, want 1", len(got))
	}
	if !strings.Contains(got[0], "hash:aaaa") || !strings.Contains(got[0], "hash:bbbb") {
		t.Errorf("batch = %q", got[0])
	}
}

func TestGetQueryHashesSplitsOverLimit(t *testing.T) {
	oids := make([]string, 20)
	for i := range oids {
		oids[i] = strings.Repeat("a", 40) // realistic SHA length
	}
	got := GetQueryHashes(oids)
	if len(got) < 2 {
		t.Fatalf("expected multiple batches for 20 SHAs, got %d", len(got))
	}
	for _, b := range got {
		if len(b) > 256 {
			t.Errorf("batch exceeds 256 chars: %d", len(b))
		}
	}
}
