package tui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/tnagatomi/gh-wtrm/internal/deleter"
	"github.com/tnagatomi/gh-wtrm/internal/worktree"
)

// selectFirstLinked moves to the first linked worktree and selects it.
func selectFirstLinked(m tea.Model) tea.Model {
	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeySpace})
	return m
}

func TestEnterConfirmShowsTargets(t *testing.T) {
	m := screenModel(t, sampleWorktrees())
	m = selectFirstLinked(m)
	m, _ = m.Update(tea.KeyPressMsg{Text: "d", Code: 'd'})
	got := m.(Model)
	if got.screen != screenConfirmDelete {
		t.Fatalf("screen: got %v, want confirm", got.screen)
	}
	view := got.View().Content
	if !strings.Contains(view, "Deleting 1 worktrees") || !strings.Contains(view, "/repo/wt/a") {
		t.Errorf("confirm view missing target:\n%s", view)
	}
}

func TestConfirmBranchToggleDefaultsOnForMergedTargets(t *testing.T) {
	m := screenModel(t, sampleWorktrees())
	m = selectFirstLinked(m) // /repo/wt/a has pr-merged
	m, _ = m.Update(tea.KeyPressMsg{Text: "d", Code: 'd'})
	if !m.(Model).deleteBranchesToggle {
		t.Error("branch toggle should default ON when all targets are pr-merged")
	}
	// space flips it off
	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeySpace})
	if m.(Model).deleteBranchesToggle {
		t.Error("space should toggle branch deletion off")
	}
}

func TestConfirmCancelReturnsToList(t *testing.T) {
	m := screenModel(t, sampleWorktrees())
	m = selectFirstLinked(m)
	m, _ = m.Update(tea.KeyPressMsg{Text: "d", Code: 'd'})
	m, _ = m.Update(tea.KeyPressMsg{Text: "n", Code: 'n'})
	if m.(Model).screen != screenWorktrees {
		t.Errorf("n should cancel back to the list, got screen %v", m.(Model).screen)
	}
}

func TestConfirmYRunsDeleteFuncAndReloads(t *testing.T) {
	var gotRepo string
	var gotAlso bool
	fakeDelete := func(repoPath string, targets []worktree.Worktree, alsoBranches bool) []deleter.Failure {
		gotRepo = repoPath
		gotAlso = alsoBranches
		return nil
	}
	reloadCalled := false
	reload := func() ReloadResult {
		reloadCalled = true
		return ReloadResult{Worktrees: []worktree.Worktree{
			{Path: "/repo", Branch: "main", Badges: []worktree.Badge{worktree.BadgePrimary}},
		}}
	}
	m := tea.Model(NewModel("/repo", sampleWorktrees(), nil).WithDeleteFunc(fakeDelete).WithReload(reload))
	m, _ = m.Update(tea.WindowSizeMsg{Width: 120, Height: 20})
	m = selectFirstLinked(m)
	m, _ = m.Update(tea.KeyPressMsg{Text: "d", Code: 'd'})
	m, cmd := m.Update(tea.KeyPressMsg{Text: "y", Code: 'y'})
	if !m.(Model).deleting {
		t.Fatal("y should set deleting=true")
	}
	if cmd == nil {
		t.Fatal("y should return a delete command")
	}
	// Run the delete command and feed its message back.
	msg := cmd()
	if gotRepo != "/repo" {
		t.Errorf("delete func got repo %q", gotRepo)
	}
	if !gotAlso {
		t.Error("branch toggle should have been passed through as true")
	}
	m, cmd2 := m.Update(msg)
	// The delete result triggers a reload command.
	if cmd2 == nil {
		t.Fatal("delete result should dispatch a reload command")
	}
	m, _ = m.Update(cmd2())
	if !reloadCalled {
		t.Error("reload should have been invoked after delete")
	}
	got := m.(Model)
	if got.deleting {
		t.Error("deleting should be cleared after result")
	}
	if got.screen != screenWorktrees {
		t.Error("should be back on the worktree list after delete")
	}
	if len(got.worktreeSorted) != 1 {
		t.Errorf("list should reflect reloaded worktrees, got %d", len(got.worktreeSorted))
	}
}

func TestApplyDeleteResultWithoutReloadDropsRemoved(t *testing.T) {
	m := screenModel(t, sampleWorktrees()).(Model)
	// Target /repo/wt/a for deletion; no reload injected.
	m.deleteTargets = []worktree.Worktree{{Path: "/repo/wt/a", Branch: "feat-a"}}
	got, _ := m.applyDeleteResult(deleteCompleteMsg{failures: nil})
	for _, w := range got.worktreeSorted {
		if w.Path == "/repo/wt/a" {
			t.Error("successfully-removed target should be dropped from the list")
		}
	}
}

func TestReloadFatalKeepsList(t *testing.T) {
	m := screenModel(t, sampleWorktrees()).(Model)
	before := len(m.worktreeSorted)
	got, _ := m.applyReloadResult(reloadCompleteMsg{result: ReloadResult{FatalErr: errStub{}}})
	if len(got.worktreeSorted) != before {
		t.Error("fatal reload should keep the previous list")
	}
	if got.reloadError == nil {
		t.Error("fatal reload should surface an error")
	}
}
