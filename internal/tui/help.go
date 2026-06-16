package tui

import (
	"strings"

	"charm.land/lipgloss/v2"
)

// helpView renders the modal help overlay: a single static reference grouped
// by screen, shown by the global `?` toggle.
func helpView() string {
	var b strings.Builder
	b.WriteString(lipgloss.NewStyle().Bold(true).Render("gh-wtrm — keyboard reference"))
	b.WriteString("\n\n")
	for _, g := range helpGroups {
		b.WriteString(lipgloss.NewStyle().Bold(true).Render(g.title))
		b.WriteString("\n")
		for _, e := range g.entries {
			b.WriteString("  ")
			b.WriteString(e.keys)
			b.WriteString(strings.Repeat(" ", helpKeyColumn-len(e.keys)))
			b.WriteString(e.desc)
			b.WriteString("\n")
		}
		b.WriteString("\n")
	}
	b.WriteString(faintStyle.Render("[?] close help    [q] quit"))
	b.WriteString("\n")
	return b.String()
}

const helpKeyColumn = 28

type helpEntry struct {
	keys string
	desc string
}

type helpGroup struct {
	title   string
	entries []helpEntry
}

var helpGroups = []helpGroup{
	{
		title: "Global",
		entries: []helpEntry{
			{"?", "toggle this help"},
			{"q  /  ctrl+c", "quit"},
		},
	},
	{
		title: "Worktree list",
		entries: []helpEntry{
			{"↑ / k  /  ctrl+p", "previous worktree"},
			{"↓ / j  /  ctrl+n", "next worktree"},
			{"ctrl+v / alt+v", "page down / up"},
			{"g / G", "jump to top / bottom"},
			{"space", "toggle selection on focused row"},
			{"s", "select all safe-to-remove worktrees"},
			{"y", "copy focused branch name to clipboard"},
			{"/", "start incremental filter"},
			{"d", "open delete confirmation"},
			{"r", "reload worktrees and PR state"},
			{"esc", "clear filter, or quit"},
		},
	},
	{
		title: "Delete confirmation",
		entries: []helpEntry{
			{"y", "confirm deletion"},
			{"n  /  esc", "cancel back to the list"},
			{"space", "toggle [Also delete branches]"},
		},
	},
	{
		title: "Filter (while editing)",
		entries: []helpEntry{
			{"<printable>", "append to query"},
			{"backspace", "remove last character"},
			{"enter", "apply filter and exit edit"},
			{"esc", "clear filter and exit edit"},
		},
	},
	{
		title: "Badges — safe to remove",
		entries: []helpEntry{
			{"[pr-merged]", "branch has a merged PR containing the local HEAD"},
			{"[no-dir]", "the working directory is already gone (prune)"},
		},
	},
	{
		title: "Badges — removal needs care",
		entries: []helpEntry{
			{"[pr-open]", "an open PR still references this branch"},
			{"[pr-closed]", "PR closed without merging"},
			{"[uncommitted]", "has tracked changes not yet committed"},
			{"[locked]", "deliberately protected with a worktree lock"},
			{"[primary]", "the main checkout; never selectable"},
		},
	},
}
