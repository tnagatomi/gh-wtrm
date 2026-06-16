package tui

import (
	"fmt"
	"slices"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/tnagatomi/gh-wtrm/internal/deleter"
	"github.com/tnagatomi/gh-wtrm/internal/worktree"
)

// deleteCompleteMsg carries the deletion batch result back into the event
// loop so the slow git work runs off the Update goroutine.
type deleteCompleteMsg struct {
	failures []deleter.Failure
}

// warningMessages maps each warning badge to the wording shown in the
// confirmation summary.
var warningMessages = map[worktree.Badge]string{
	worktree.BadgeUncommitted: "uncommitted changes will be lost",
	worktree.BadgeLocked:      "the lock will be released",
}

// enterConfirmDelete captures the selected worktrees as deletion targets,
// derives the default "Also delete branches" toggle, and switches to the
// confirmation screen. The toggle defaults ON only when every target has a
// merged PR; any other target is worth a deliberate opt-in.
func (m Model) enterConfirmDelete() Model {
	targets := make([]worktree.Worktree, 0, len(m.selected))
	for _, w := range m.worktreeSorted {
		if m.selected[w.Path] {
			targets = append(targets, w)
		}
	}
	m.deleteTargets = targets
	m.deleteBranchesToggle = allPRMerged(targets)
	m.screen = screenConfirmDelete
	return m
}

// allPRMerged reports whether every target carries the pr-merged badge.
func allPRMerged(wts []worktree.Worktree) bool {
	if len(wts) == 0 {
		return false
	}
	for _, w := range wts {
		if !slices.Contains(w.Badges, worktree.BadgePRMerged) {
			return false
		}
	}
	return true
}

func (m Model) handleConfirmKey(msg tea.KeyPressMsg) (Model, tea.Cmd) {
	// Once a deletion is in flight the confirm screen is read-only.
	if m.deleting {
		return m, nil
	}
	switch msg.String() {
	case "esc", "n":
		m.screen = screenWorktrees
		return m, nil
	case "space":
		m.deleteBranchesToggle = !m.deleteBranchesToggle
		return m, nil
	case "y":
		m.deleting = true
		return m, m.deleteCmd()
	}
	return m, nil
}

// deleteCmd runs the deletion off the Update goroutine and posts a
// deleteCompleteMsg back.
func (m Model) deleteCmd() tea.Cmd {
	repoPath := m.repoPath
	targets := m.deleteTargets
	alsoBranches := m.deleteBranchesToggle
	fn := m.deleteFn
	return func() tea.Msg {
		return deleteCompleteMsg{failures: fn(repoPath, targets, alsoBranches)}
	}
}

// applyDeleteResult records the failures, then refreshes the list: via the
// injected reload (authoritative) when present, otherwise by dropping the
// successfully-removed targets from the in-memory list.
func (m Model) applyDeleteResult(msg deleteCompleteMsg) (Model, tea.Cmd) {
	m.deleting = false
	m.deleteFailures = msg.failures
	if m.reload != nil {
		m.reloading = true
		m.screen = screenWorktrees
		return m, m.reloadCmd()
	}
	m = m.buildWorktreeScreen(remainingAfterDelete(m.worktreeSorted, m.deleteTargets, msg.failures))
	m.deleteFailures = msg.failures
	return m, nil
}

// remainingAfterDelete returns the worktrees minus those targets that were
// removed without a remove-phase failure.
func remainingAfterDelete(all, targets []worktree.Worktree, failures []deleter.Failure) []worktree.Worktree {
	failedRemove := map[string]bool{}
	for _, f := range failures {
		if f.Op == deleter.OpRemove {
			failedRemove[f.Path] = true
		}
	}
	removed := map[string]bool{}
	for _, t := range targets {
		if !failedRemove[t.Path] {
			removed[t.Path] = true
		}
	}
	out := make([]worktree.Worktree, 0, len(all))
	for _, w := range all {
		if !removed[w.Path] {
			out = append(out, w)
		}
	}
	return out
}

func (m Model) confirmDeleteView() string {
	var b strings.Builder
	title := lipgloss.NewStyle().Bold(true).Render(fmt.Sprintf("Deleting %d worktrees:", len(m.deleteTargets)))
	fmt.Fprintf(&b, "%s\n\n", title)
	for _, w := range m.deleteTargets {
		prefix := "    "
		if w.HasAnyBadge(warningBadges) {
			prefix = "  ⚠ "
		}
		fmt.Fprintf(&b, "%s%s    %s\n", prefix, w.Path, renderBadges(w.Badges))
	}
	checkbox := "[ ]"
	if m.deleteBranchesToggle {
		checkbox = "[x]"
	}
	fmt.Fprintf(&b, "\nOptions:\n  %s Also delete branches (git branch -D)\n", checkbox)
	if counts := badgeCounts(m.deleteTargets, warningBadges); len(counts) > 0 {
		b.WriteString("\n⚠ Warnings (deletion will be forced):\n")
		labelWidth := warningLabelWidth()
		for _, badge := range warningBadges {
			if n := counts[badge]; n > 0 {
				fmt.Fprintf(&b, "  - %-*s %s (%d)\n", labelWidth, badge.String()+":", warningMessages[badge], n)
			}
		}
	}
	if m.deleting {
		fmt.Fprintf(&b, "\n%s\n", faintStyle.Render("⏳ Deleting..."))
		return b.String()
	}
	help := faintStyle.Render("[y] Confirm    [n] Cancel    [space] toggle branches    [?] help")
	fmt.Fprintf(&b, "\n%s\n", help)
	return b.String()
}

// warningLabelWidth returns the column width needed to align the longest
// "<badge>:" label across the warnings block.
func warningLabelWidth() int {
	w := 0
	for _, b := range warningBadges {
		if l := len(b.String()) + 1; l > w {
			w = l
		}
	}
	return w
}
