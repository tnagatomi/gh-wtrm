package tui

import (
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/tnagatomi/gh-wtrm/internal/worktree"
)

func (m Model) handleFilterEditKey(msg tea.KeyPressMsg) Model {
	switch msg.String() {
	case "enter":
		m.filterEditing = false
		return m
	case "esc":
		return m.clearFilter()
	case "backspace":
		if r := []rune(m.filterQuery); len(r) > 0 {
			m.filterQuery = string(r[:len(r)-1])
			m.refreshFilteredRows()
		}
		return m
	}
	if msg.Text != "" {
		m.filterQuery += msg.Text
		m.refreshFilteredRows()
	}
	return m
}

func (m Model) clearFilter() Model {
	m.filterEditing = false
	m.filterQuery = ""
	m.refreshFilteredRows()
	return m
}

// refreshFilteredRows recomputes the visible worktrees from the current
// filter query and pushes the new rows into the table, clamping the cursor.
func (m *Model) refreshFilteredRows() {
	m.worktreeVisible = filterWorktrees(m.worktreeSorted, m.filterQuery)
	_, rs := worktreeLayout(m.worktreeVisible, m.selected, m.worktreeMaxPath, m.worktreeMaxBranch, m.worktreeMaxBadges, m.termWidth)
	m.worktreeTable.SetRows(rs)
	if m.worktreeTable.Cursor() >= len(rs) {
		m.worktreeTable.SetCursor(max(0, len(rs)-1))
	}
}

// filterWorktrees returns the worktrees matching a case-insensitive substring
// search on Path or Branch. An empty query returns the slice unchanged.
func filterWorktrees(wts []worktree.Worktree, query string) []worktree.Worktree {
	if query == "" {
		return wts
	}
	q := strings.ToLower(query)
	out := make([]worktree.Worktree, 0, len(wts))
	for _, w := range wts {
		if strings.Contains(strings.ToLower(w.Path), q) || strings.Contains(strings.ToLower(w.Branch), q) {
			out = append(out, w)
		}
	}
	return out
}
