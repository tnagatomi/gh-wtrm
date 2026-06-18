package tui

import (
	"slices"
	"strings"

	"charm.land/bubbles/v2/table"
	"charm.land/lipgloss/v2"
	"charm.land/lipgloss/v2/compat"

	"github.com/tnagatomi/gh-wtrm/internal/worktree"
)

var worktreeRowStyles = map[worktree.Badge]lipgloss.Style{
	worktree.BadgePrimary:     lipgloss.NewStyle().Foreground(adaptive("240", "245")),
	worktree.BadgeCurrent:     lipgloss.NewStyle().Foreground(adaptive("240", "245")),
	worktree.BadgePRMerged:    lipgloss.NewStyle().Foreground(adaptive("28", "82")),
	worktree.BadgePROpen:      lipgloss.NewStyle().Foreground(adaptive("33", "75")),
	worktree.BadgePRClosed:    lipgloss.NewStyle().Foreground(adaptive("130", "220")),
	worktree.BadgeUncommitted: lipgloss.NewStyle().Foreground(adaptive("160", "203")),
	worktree.BadgeUnpushed:    lipgloss.NewStyle().Foreground(adaptive("160", "203")),
	worktree.BadgeLocked:      lipgloss.NewStyle().Foreground(adaptive("93", "141")),
	worktree.BadgeNoDir:       lipgloss.NewStyle().Foreground(adaptive("240", "245")),
}

// adaptive returns a lipgloss color that resolves to light when the terminal
// background is light and dark when it is dark.
func adaptive(light, dark string) compat.AdaptiveColor {
	return compat.AdaptiveColor{Light: lipgloss.Color(light), Dark: lipgloss.Color(dark)}
}

// worktreeRowBadgePriority orders badges by how much each should dominate a
// row's foreground color: action-required states first, then informational
// ones. The first match in this slice wins.
var worktreeRowBadgePriority = []worktree.Badge{
	worktree.BadgeUncommitted,
	worktree.BadgeLocked,
	worktree.BadgePROpen,
	worktree.BadgePRClosed,
	worktree.BadgeNoDir,
	worktree.BadgeCurrent,
	worktree.BadgePrimary,
	worktree.BadgePRMerged,
}

// isSelectable returns false for the primary worktree and the worktree the
// user is standing in, neither of which is ever eligible for deletion — git
// itself refuses to remove the worktree you are in, and the safety model
// marks both NotDeletable.
func isSelectable(w worktree.Worktree) bool {
	return !slices.Contains(w.Badges, worktree.BadgePrimary) &&
		!slices.Contains(w.Badges, worktree.BadgeCurrent)
}

// isSafeToRemove reports whether w is what the `s` bulk-select key should
// pick: the safety model cleared it for removal.
func isSafeToRemove(w worktree.Worktree) bool {
	return w.Deletable
}

// warningBadges are the states whose presence means a forced removal would
// lose local-only work. Used by the confirm screen.
var warningBadges = []worktree.Badge{
	worktree.BadgeUncommitted,
	worktree.BadgeLocked,
}

// badgeCounts tallies how many worktrees carry each of the given badges.
func badgeCounts(wts []worktree.Worktree, badges []worktree.Badge) map[worktree.Badge]int {
	out := map[worktree.Badge]int{}
	for _, w := range wts {
		for _, b := range badges {
			if slices.Contains(w.Badges, b) {
				out[b]++
			}
		}
	}
	return out
}

// checkboxCell renders the selection state of a worktree row. Non-selectable
// rows (the primary checkout) get a blank cell so columns stay aligned.
func checkboxCell(w worktree.Worktree, sel bool) string {
	if !isSelectable(w) {
		return "   "
	}
	if sel {
		return "[x]"
	}
	return "[ ]"
}

// renderBadges produces a plain, space-separated `[name]` list. Keep this
// value ANSI-free: bubbles/table truncates raw cell values before rendering,
// so embedded escape sequences would be counted as content.
func renderBadges(badges []worktree.Badge) string {
	if len(badges) == 0 {
		return ""
	}
	parts := make([]string, len(badges))
	for i, b := range badges {
		parts[i] = "[" + b.String() + "]"
	}
	return strings.Join(parts, " ")
}

func worktreeTableStyles() table.Styles {
	styles := table.DefaultStyles()
	styles.Selected = lipgloss.NewStyle().Bold(true)
	return styles
}

// renderWorktreeTable post-processes t.View() to tint each rendered data line
// by the highest-priority badge of its worktree. The first visible data row
// is approximated from the cursor and table height.
func renderWorktreeTable(t table.Model, wts []worktree.Worktree) string {
	view := t.View()
	lines := strings.Split(view, "\n")
	if len(lines) <= 1 {
		return view
	}
	height := t.Height()
	cursor := t.Cursor()
	yoffset := 0
	if len(wts) > height && cursor >= height {
		yoffset = min(cursor-height+1, len(wts)-height)
	}
	for lineIdx := 1; lineIdx < len(lines); lineIdx++ {
		rowIdx := yoffset + lineIdx - 1
		if rowIdx >= len(wts) {
			break
		}
		lines[lineIdx] = rowStyleForBadges(wts[rowIdx].Badges).Render(lines[lineIdx])
	}
	return strings.Join(lines, "\n")
}

func rowStyleForBadges(badges []worktree.Badge) lipgloss.Style {
	for _, badge := range worktreeRowBadgePriority {
		if slices.Contains(badges, badge) {
			return worktreeRowStyles[badge]
		}
	}
	return lipgloss.NewStyle()
}

// badgesVisibleWidth returns the longest rendered badges width across the
// worktrees, honoring the header label so the column never collapses below
// "Badges".
func badgesVisibleWidth(wts []worktree.Worktree) int {
	w := len("Badges")
	for _, wt := range wts {
		if l := plainBadgesWidth(wt.Badges); l > w {
			w = l
		}
	}
	return w
}

func plainBadgesWidth(badges []worktree.Badge) int {
	if len(badges) == 0 {
		return 0
	}
	total := 0
	for i, b := range badges {
		if i > 0 {
			total++ // single-space separator
		}
		total += 2 + len(b.String()) // "[" + name + "]"
	}
	return total
}
