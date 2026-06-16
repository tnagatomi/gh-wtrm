package tui

import (
	"slices"

	"charm.land/bubbles/v2/table"
	tea "charm.land/bubbletea/v2"
)

// setClipboard is a seam over tea.SetClipboard (OSC52) so tests can capture
// the copied string.
var setClipboard = tea.SetClipboard

// emacsTableKeyMap returns the default bubbles/table keymap extended with
// emacs-style movement aliases.
func emacsTableKeyMap() table.KeyMap {
	km := table.DefaultKeyMap()
	km.LineDown.SetKeys(append(km.LineDown.Keys(), "ctrl+n")...)
	km.LineUp.SetKeys(append(km.LineUp.Keys(), "ctrl+p")...)
	km.PageDown.SetKeys(append(km.PageDown.Keys(), "ctrl+v")...)
	km.PageUp.SetKeys(append(km.PageUp.Keys(), "alt+v")...)
	return km
}

// worktreeTableKeyMap is the emacs keymap with two defaults stripped so they
// don't collide with the worktree-screen actions:
//   - "space" is removed from PageDown so it can toggle selection.
//   - "d" is removed from HalfPageDown so it can open the delete confirm.
func worktreeTableKeyMap() table.KeyMap {
	km := emacsTableKeyMap()
	km.PageDown.SetKeys(without(km.PageDown.Keys(), "space")...)
	km.HalfPageDown.SetKeys(without(km.HalfPageDown.Keys(), "d")...)
	return km
}

func without(keys []string, drop string) []string {
	return slices.DeleteFunc(slices.Clone(keys), func(k string) bool { return k == drop })
}
