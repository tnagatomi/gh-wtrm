package worktree

import (
	"reflect"
	"testing"
)

func TestParse(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  []Worktree
	}{
		{
			name:  "empty input",
			input: "",
			want:  nil,
		},
		{
			name: "primary plus linked worktree",
			input: `worktree /home/user/repo
HEAD aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa
branch refs/heads/main

worktree /home/user/repo/wt/feat
HEAD bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb
branch refs/heads/feat-x

`,
			want: []Worktree{
				{Path: "/home/user/repo", HEAD: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", Branch: "main"},
				{Path: "/home/user/repo/wt/feat", HEAD: "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb", Branch: "feat-x"},
			},
		},
		{
			name: "detached HEAD has no branch",
			input: `worktree /home/user/repo/wt/det
HEAD cccccccccccccccccccccccccccccccccccccccc
detached

`,
			want: []Worktree{
				{Path: "/home/user/repo/wt/det", HEAD: "cccccccccccccccccccccccccccccccccccccccc", Detached: true},
			},
		},
		{
			name: "bare repo emits only worktree and bare lines",
			input: `worktree /home/user/repo.git
bare

`,
			want: []Worktree{
				{Path: "/home/user/repo.git", Bare: true},
			},
		},
		{
			name: "locked with reason",
			input: `worktree /home/user/repo/wt/locked
HEAD eeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee
branch refs/heads/exp
locked manual lock for safety

`,
			want: []Worktree{
				{Path: "/home/user/repo/wt/locked", HEAD: "eeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee", Branch: "exp", Locked: true, LockReason: "manual lock for safety"},
			},
		},
		{
			name: "prunable carries reason",
			input: `worktree /home/user/repo/wt/gone
HEAD ffffffffffffffffffffffffffffffffffffffff
branch refs/heads/gone
prunable gitdir file points to non-existent location

`,
			want: []Worktree{
				{Path: "/home/user/repo/wt/gone", HEAD: "ffffffffffffffffffffffffffffffffffffffff", Branch: "gone", Prunable: true, PrunableReason: "gitdir file points to non-existent location"},
			},
		},
		{
			name: "missing trailing blank line still emits last record",
			input: `worktree /home/user/repo
HEAD 1111111111111111111111111111111111111111
branch refs/heads/main`,
			want: []Worktree{
				{Path: "/home/user/repo", HEAD: "1111111111111111111111111111111111111111", Branch: "main"},
			},
		},
		{
			name: "unknown attribute lines are ignored",
			input: `worktree /home/user/repo
HEAD 2222222222222222222222222222222222222222
branch refs/heads/main
futurefield some-value

`,
			want: []Worktree{
				{Path: "/home/user/repo", HEAD: "2222222222222222222222222222222222222222", Branch: "main"},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := Parse(tc.input)
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("Parse() mismatch\n got: %+v\nwant: %+v", got, tc.want)
			}
		})
	}
}

func TestBadgeString(t *testing.T) {
	cases := []struct {
		b    Badge
		want string
	}{
		{BadgePrimary, "primary"},
		{BadgeCurrent, "current"},
		{BadgeUncommitted, "uncommitted"},
		{BadgeUnpushed, "unpushed"},
		{BadgeLocked, "locked"},
		{BadgeNoDir, "no-dir"},
		{BadgePRMerged, "pr-merged"},
		{BadgePROpen, "pr-open"},
		{BadgePRClosed, "pr-closed"},
		{Badge(-1), ""},
	}
	for _, c := range cases {
		if got := c.b.String(); got != c.want {
			t.Errorf("Badge(%d).String() = %q, want %q", c.b, got, c.want)
		}
	}
}

func TestHasAnyBadge(t *testing.T) {
	w := Worktree{Badges: []Badge{BadgePRMerged, BadgeUnpushed}}
	if !w.HasAnyBadge([]Badge{BadgeUnpushed}) {
		t.Error("expected HasAnyBadge to find unpushed")
	}
	if w.HasAnyBadge([]Badge{BadgeLocked}) {
		t.Error("did not expect HasAnyBadge to find locked")
	}
}
