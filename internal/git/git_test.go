package git

import (
	"testing"
	"time"
)

func TestParseStatus(t *testing.T) {
	out := " M tracked-modified.go\n?? untracked.txt\nA  staged-new.go\n"
	got := ParseStatus(out)
	if len(got) != 3 {
		t.Fatalf("got %d changes, want 3", len(got))
	}
	if got[0].X != " " || got[0].Y != "M" || got[0].Path != "tracked-modified.go" {
		t.Errorf("change[0] = %+v", got[0])
	}
	if !got[1].IsUntracked() {
		t.Errorf("change[1] should be untracked: %+v", got[1])
	}
	if got[0].IsUntracked() {
		t.Errorf("change[0] should be tracked: %+v", got[0])
	}
	if got[2].X != "A" || got[2].Path != "staged-new.go" {
		t.Errorf("change[2] = %+v", got[2])
	}
}

func TestParseStatusEmpty(t *testing.T) {
	if got := ParseStatus(""); len(got) != 0 {
		t.Errorf("empty status: got %d changes, want 0", len(got))
	}
}

func TestParseCommitTimes(t *testing.T) {
	out := "aaaa 2026-05-17T10:30:00+09:00\nbbbb 2025-01-01T00:00:00Z\n"
	got := ParseCommitTimes(out)
	if len(got) != 2 {
		t.Fatalf("got %d entries, want 2", len(got))
	}
	want := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	if !got["bbbb"].Equal(want) {
		t.Errorf("bbbb = %v, want %v", got["bbbb"], want)
	}
	if got["aaaa"].IsZero() {
		t.Errorf("aaaa should be parsed, got zero")
	}
}

func TestParseCommitTimesSkipsMalformed(t *testing.T) {
	out := "no-time-here\ncccc not-a-timestamp\ndddd 2026-01-02T03:04:05Z\n"
	got := ParseCommitTimes(out)
	if len(got) != 1 {
		t.Fatalf("got %d entries, want 1 (only the valid line)", len(got))
	}
	if got["dddd"].IsZero() {
		t.Errorf("dddd should be parsed")
	}
}
