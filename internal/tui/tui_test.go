package tui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/tnagatomi/gh-wtrm/internal/worktree"
)

// screenModel returns a Model sized so the table viewport renders rows.
func screenModel(t *testing.T, wts []worktree.Worktree) tea.Model {
	t.Helper()
	m := tea.Model(NewModel("/repo", wts, nil))
	m, _ = m.Update(tea.WindowSizeMsg{Width: 120, Height: 20})
	return m
}

func sampleWorktrees() []worktree.Worktree {
	return []worktree.Worktree{
		{Path: "/repo", Branch: "main", Badges: []worktree.Badge{worktree.BadgePrimary}},
		{Path: "/repo/wt/a", Branch: "feat-a", Deletable: true, Badges: []worktree.Badge{worktree.BadgePRMerged}},
		{Path: "/repo/wt/b", Branch: "feat-b", Badges: []worktree.Badge{worktree.BadgePROpen}},
	}
}

func TestViewListsWorktrees(t *testing.T) {
	m := screenModel(t, sampleWorktrees())
	view := m.View().Content
	for _, want := range []string{"/repo/wt/a", "feat-a", "feat-b", "[pr-merged]", "[pr-open]"} {
		if !strings.Contains(view, want) {
			t.Errorf("view missing %q:\n%s", want, view)
		}
	}
}

func TestSpaceTogglesSelectableRow(t *testing.T) {
	m := screenModel(t, sampleWorktrees())
	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyDown}) // to /repo/wt/a
	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeySpace})
	if !m.(Model).selected["/repo/wt/a"] {
		t.Fatalf("space did not select /repo/wt/a: %v", m.(Model).selected)
	}
	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeySpace})
	if m.(Model).selected["/repo/wt/a"] {
		t.Fatalf("second space did not deselect: %v", m.(Model).selected)
	}
}

func TestSpaceNoOpOnPrimary(t *testing.T) {
	m := screenModel(t, sampleWorktrees())
	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeySpace}) // cursor at primary
	if len(m.(Model).selected) != 0 {
		t.Fatalf("space on primary selected something: %v", m.(Model).selected)
	}
}

func TestSafeSelectPicksOnlyDeletable(t *testing.T) {
	m := screenModel(t, sampleWorktrees())
	m, _ = m.Update(tea.KeyPressMsg{Text: "s", Code: 's'})
	sel := m.(Model).selected
	if !sel["/repo/wt/a"] {
		t.Errorf("safe-select should pick deletable /repo/wt/a: %v", sel)
	}
	if sel["/repo/wt/b"] {
		t.Errorf("safe-select must not pick non-deletable /repo/wt/b: %v", sel)
	}
	if sel["/repo"] {
		t.Errorf("safe-select must not pick the primary: %v", sel)
	}
}

func TestFilterNarrowsVisible(t *testing.T) {
	m := screenModel(t, sampleWorktrees())
	m, _ = m.Update(tea.KeyPressMsg{Text: "/", Code: '/'})
	for _, r := range "feat-a" {
		m, _ = m.Update(tea.KeyPressMsg{Text: string(r), Code: r})
	}
	view := m.View().Content
	if !strings.Contains(view, "feat-a") {
		t.Errorf("filtered view should keep feat-a:\n%s", view)
	}
	if strings.Contains(view, "feat-b") {
		t.Errorf("filtered view should drop feat-b:\n%s", view)
	}
}

func TestCopyBranchSetsNotice(t *testing.T) {
	m := screenModel(t, sampleWorktrees())
	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyDown}) // /repo/wt/a
	m, _ = m.Update(tea.KeyPressMsg{Text: "y", Code: 'y'})
	if !strings.Contains(m.(Model).copyNotice, "feat-a") {
		t.Errorf("copy notice missing branch: %q", m.(Model).copyNotice)
	}
}

func TestPRErrorSurfacedInView(t *testing.T) {
	m := tea.Model(NewModel("/repo", sampleWorktrees(), errStub{}))
	m, _ = m.Update(tea.WindowSizeMsg{Width: 120, Height: 20})
	if !strings.Contains(m.View().Content, "PR lookup failed") {
		t.Errorf("view should surface PR fetch error:\n%s", m.View().Content)
	}
}

func TestHelpToggle(t *testing.T) {
	m := screenModel(t, sampleWorktrees())
	m, _ = m.Update(tea.KeyPressMsg{Text: "?", Code: '?'})
	if !strings.Contains(m.View().Content, "keyboard reference") {
		t.Errorf("help overlay not shown:\n%s", m.View().Content)
	}
}

type errStub struct{}

func (errStub) Error() string { return "boom" }
