package cli

import (
	"testing"

	"github.com/tnagatomi/gh-wtrm/internal/gh"
)

func TestParseState(t *testing.T) {
	cases := []struct {
		in   string
		want gh.PullRequestState
		err  bool
	}{
		{"merged", gh.Merged, false},
		{"closed", gh.Closed, false},
		{"MERGED", gh.Merged, false},
		{"open", 0, true},
		{"", 0, true},
	}
	for _, c := range cases {
		got, err := parseState(c.in)
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

func TestStateFlagDefaultsToMerged(t *testing.T) {
	cmd := NewRootCmd()
	v, err := cmd.Flags().GetString("state")
	if err != nil {
		t.Fatalf("state flag missing: %v", err)
	}
	if v != "merged" {
		t.Errorf("default state: got %q, want merged", v)
	}
}
