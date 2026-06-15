package tui

import (
	tea "charm.land/bubbletea/v2"
)

// reloadCompleteMsg carries a refreshed worktree list back into the event
// loop after the `r` key or a post-delete refresh.
type reloadCompleteMsg struct {
	result ReloadResult
}

// reloadCmd runs the injected reload off the Update goroutine.
func (m Model) reloadCmd() tea.Cmd {
	fn := m.reload
	return func() tea.Msg {
		return reloadCompleteMsg{result: fn()}
	}
}

// applyReloadResult rebuilds the worktree screen from a refreshed list. A
// fatal refresh failure keeps the previous list and surfaces the error; a
// successful refresh adopts the new list and the (possibly degraded) PR
// status.
func (m Model) applyReloadResult(msg reloadCompleteMsg) (Model, tea.Cmd) {
	m.reloading = false
	if msg.result.FatalErr != nil {
		m.reloadError = msg.result.FatalErr
		return m, nil
	}
	failures := m.deleteFailures
	m = m.buildWorktreeScreen(msg.result.Worktrees)
	m.prError = msg.result.PRError
	m.reloadError = nil
	m.deleteFailures = failures
	return m, nil
}
