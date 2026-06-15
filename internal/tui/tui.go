// Package tui implements the bubbletea user interface for gh-wtrm. It
// operates on a single repository's worktrees: a list/selection screen and a
// delete-confirmation screen.
package tui

import (
	"fmt"
	"slices"
	"time"

	"charm.land/bubbles/v2/table"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/tnagatomi/gh-wtrm/internal/deleter"
	"github.com/tnagatomi/gh-wtrm/internal/worktree"
)

const (
	timeLayout = "2006-01-02 15:04:05"
	emptyTime  = "-"
	ellipsis   = "…"
	chromeRows = 4 // title + table header + help + trailing newline
)

// faintStyle is the dim-text style used for help lines and status summaries.
var faintStyle = lipgloss.NewStyle().Faint(true)

type screenID int

const (
	screenWorktrees screenID = iota
	screenConfirmDelete
)

// DeleteFunc performs the deletion batch; injectable so tests avoid real git.
type DeleteFunc func(repoPath string, targets []worktree.Worktree, alsoBranches bool) []deleter.Failure

// ReloadResult is the outcome of refreshing the worktree list.
type ReloadResult struct {
	Worktrees []worktree.Worktree
	// PRError reports a degraded PR fetch; the list is still shown but
	// conservative.
	PRError error
	// FatalErr means the list could not be refreshed at all; the previous
	// list is kept.
	FatalErr error
}

// ReloadFunc re-queries the repository (git + PRs). Injected by the CLI;
// nil disables the `r` reload and post-delete refresh.
type ReloadFunc func() ReloadResult

type Model struct {
	repoPath string
	screen   screenID

	deleteFn DeleteFunc
	reload   ReloadFunc

	worktreeTable     table.Model
	worktreeSorted    []worktree.Worktree
	worktreeVisible   []worktree.Worktree
	worktreeMaxPath   int
	worktreeMaxBranch int
	worktreeMaxBadges int
	selected          map[string]bool

	filterEditing bool
	filterQuery   string

	// copyNotice is a transient status line shown after a copy attempt on
	// the worktree screen. Cleared by the next keypress.
	copyNotice string

	// prError, when set, reports that the PR fetch failed and the list is
	// conservative (everything not deletable except prune targets).
	prError error

	deleteTargets        []worktree.Worktree
	deleteBranchesToggle bool
	deleteFailures       []deleter.Failure
	deleting             bool

	reloading   bool
	reloadError error

	helpVisible bool

	termWidth  int
	termHeight int
}

// NewModel builds the model for one repository's worktrees, opening directly
// on the list screen. Deletion defaults to the real deleter; reload is
// disabled until WithReload is called.
func NewModel(repoPath string, wts []worktree.Worktree, prError error) Model {
	m := Model{repoPath: repoPath, prError: prError, deleteFn: deleter.Delete}
	return m.buildWorktreeScreen(wts)
}

// WithReload sets the reload callback used by the `r` key and the post-delete
// refresh.
func (m Model) WithReload(fn ReloadFunc) Model {
	m.reload = fn
	return m
}

// WithDeleteFunc overrides the deletion function (used in tests).
func (m Model) WithDeleteFunc(fn DeleteFunc) Model {
	m.deleteFn = fn
	return m
}

func (m Model) Init() tea.Cmd {
	return nil
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.termWidth = msg.Width
		m.termHeight = msg.Height
		m.refreshLayout()
		return m, nil
	case tea.KeyPressMsg:
		return m.handleKey(msg)
	case deleteCompleteMsg:
		return m.applyDeleteResult(msg)
	case reloadCompleteMsg:
		return m.applyReloadResult(msg)
	}
	return m.delegateToTable(msg)
}

func (m Model) handleKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	if msg.String() == "ctrl+c" {
		return m, tea.Quit
	}
	if m.screen == screenWorktrees && m.filterEditing {
		return m.handleFilterEditKey(msg), nil
	}
	if msg.String() == "q" {
		return m, tea.Quit
	}
	if msg.String() == "?" {
		m.helpVisible = !m.helpVisible
		return m, nil
	}
	if m.helpVisible {
		if msg.String() == "esc" {
			m.helpVisible = false
		}
		return m, nil
	}
	switch m.screen {
	case screenWorktrees:
		// Any key clears a lingering copy notice; the "y" case re-sets it.
		m.copyNotice = ""
		switch msg.String() {
		case "esc":
			if m.filterQuery != "" {
				return m.clearFilter(), nil
			}
			return m, tea.Quit
		case "/":
			m.filterEditing = true
			return m, nil
		case "space":
			return m.toggleSelection(), nil
		case "s":
			return m.toggleSafeSelection(), nil
		case "y":
			return m.copyFocusedBranch()
		case "d":
			if len(m.selected) > 0 {
				return m.enterConfirmDelete(), nil
			}
			return m, nil
		case "r":
			if m.reload == nil || m.reloading {
				return m, nil
			}
			m.reloading = true
			m.reloadError = nil
			return m, m.reloadCmd()
		}
	case screenConfirmDelete:
		return m.handleConfirmKey(msg)
	}
	return m.delegateToTable(msg)
}

func (m Model) delegateToTable(msg tea.Msg) (tea.Model, tea.Cmd) {
	if m.screen != screenWorktrees {
		return m, nil
	}
	var cmd tea.Cmd
	m.worktreeTable, cmd = m.worktreeTable.Update(msg)
	return m, cmd
}

func (m Model) View() tea.View {
	var content string
	switch {
	case m.helpVisible:
		content = helpView()
	case m.screen == screenConfirmDelete:
		content = m.confirmDeleteView()
	default:
		content = m.worktreeView()
	}
	v := tea.NewView(content)
	v.AltScreen = true
	return v
}

func (m Model) worktreeView() string {
	titleText := fmt.Sprintf("gh-wtrm — worktrees in %s", m.repoPath)
	if m.filterEditing || m.filterQuery != "" {
		cursor := ""
		if m.filterEditing {
			cursor = "_"
		}
		titleText += "    /" + m.filterQuery + cursor
	}
	title := lipgloss.NewStyle().Bold(true).Render(titleText)
	help := faintStyle.Render("[↑/k] up  [↓/j] down  [space] select  [s] select safe  [/] filter  [d] delete  [y] copy branch  [r] reload  [?] help  [esc/q] quit")
	body := renderWorktreeTable(m.worktreeTable, m.worktreeVisible)
	if m.reloading {
		body += "\n" + faintStyle.Render("⏳ Reloading...")
	}
	if m.reloadError != nil {
		body += "\n" + faintStyle.Render(fmt.Sprintf("⚠ reload failed: %v", m.reloadError))
	}
	if m.prError != nil {
		body += "\n" + faintStyle.Render(fmt.Sprintf("⚠ PR lookup failed (%v) — nothing is shown as safe to remove", m.prError))
	}
	if n := len(m.deleteFailures); n > 0 {
		body += "\n" + faintStyle.Render(fmt.Sprintf("⚠ %d operation(s) failed during last delete", n))
	}
	if m.copyNotice != "" {
		body += "\n" + faintStyle.Render(m.copyNotice)
	}
	return fmt.Sprintf("%s\n%s\n%s\n", title, body, help)
}

// buildWorktreeScreen sorts the worktrees and rebuilds the table from
// scratch so column widths reflect the current data. Selection and filter
// are reset.
func (m Model) buildWorktreeScreen(wts []worktree.Worktree) Model {
	sorted := sortWorktrees(wts)
	maxPath, maxBranch := maxWorktreeWidths(sorted)
	maxBadges := badgesVisibleWidth(sorted)
	m.selected = map[string]bool{}
	m.filterEditing = false
	m.filterQuery = ""
	cols, rs := worktreeLayout(sorted, m.selected, maxPath, maxBranch, maxBadges, m.termWidth)
	t := table.New(
		table.WithColumns(cols),
		table.WithRows(rs),
		table.WithFocused(true),
		table.WithKeyMap(worktreeTableKeyMap()),
		table.WithStyles(worktreeTableStyles()),
		table.WithWidth(tableWidth(cols)),
	)
	if m.termHeight > 0 {
		t.SetHeight(max(1, m.termHeight-chromeRows))
	}
	m.worktreeTable = t
	m.worktreeSorted = sorted
	m.worktreeVisible = sorted
	m.worktreeMaxPath = maxPath
	m.worktreeMaxBranch = maxBranch
	m.worktreeMaxBadges = maxBadges
	m.screen = screenWorktrees
	return m
}

func (m *Model) refreshLayout() {
	if m.screen != screenWorktrees {
		return
	}
	cols, rs := worktreeLayout(m.worktreeVisible, m.selected, m.worktreeMaxPath, m.worktreeMaxBranch, m.worktreeMaxBadges, m.termWidth)
	m.worktreeTable.SetColumns(cols)
	m.worktreeTable.SetRows(rs)
	m.worktreeTable.SetWidth(tableWidth(cols))
	m.worktreeTable.SetHeight(max(1, m.termHeight-chromeRows))
}

// sortWorktrees orders by HEAD commit time, oldest first, so the most stale
// entries (typical cleanup targets) appear at the top.
func sortWorktrees(in []worktree.Worktree) []worktree.Worktree {
	out := slices.Clone(in)
	slices.SortFunc(out, func(a, b worktree.Worktree) int {
		return a.LastCommit.Compare(b.LastCommit)
	})
	return out
}

// worktreeLayout sizes Path and Branch to their longest content, capped by
// the terminal width. Path absorbs reductions first; Badges are never clamped.
func worktreeLayout(wts []worktree.Worktree, selected map[string]bool, contentPathWidth, contentBranchWidth, contentBadgesWidth, termWidth int) ([]table.Column, []table.Row) {
	const (
		timeWidth     = len(timeLayout)
		checkboxWidth = 3
		padding       = 12
		minPath       = 20
		minBranch     = 6
	)
	pathWidth := contentPathWidth
	branchWidth := contentBranchWidth
	if termWidth > 0 {
		available := termWidth - timeWidth - checkboxWidth - contentBadgesWidth - padding
		if pathWidth+branchWidth > available {
			pathWidth = available - branchWidth
			if pathWidth < minPath {
				pathWidth = minPath
				branchWidth = max(minBranch, available-pathWidth)
			}
		}
	}
	pathWidth = max(pathWidth, minPath)
	branchWidth = max(branchWidth, minBranch)
	cols := []table.Column{
		{Title: "", Width: checkboxWidth},
		{Title: "Path", Width: pathWidth},
		{Title: "Branch", Width: branchWidth},
		{Title: "Last commit", Width: timeWidth},
		{Title: "Badges", Width: contentBadgesWidth},
	}
	rs := make([]table.Row, len(wts))
	for i, w := range wts {
		rs[i] = table.Row{
			checkboxCell(w, selected[w.Path]),
			truncateHead(w.Path, pathWidth),
			truncateHead(w.Branch, branchWidth),
			formatTime(w.LastCommit),
			renderBadges(w.Badges),
		}
	}
	return cols, rs
}

// toggleSelection flips the selection state on the focused worktree.
func (m Model) toggleSelection() Model {
	cursor := m.worktreeTable.Cursor()
	if cursor < 0 || cursor >= len(m.worktreeVisible) {
		return m
	}
	w := m.worktreeVisible[cursor]
	if !isSelectable(w) {
		return m
	}
	if m.selected[w.Path] {
		delete(m.selected, w.Path)
	} else {
		m.selected[w.Path] = true
	}
	_, rs := worktreeLayout(m.worktreeVisible, m.selected, m.worktreeMaxPath, m.worktreeMaxBranch, m.worktreeMaxBadges, m.termWidth)
	m.worktreeTable.SetRows(rs)
	return m
}

// toggleSafeSelection toggles selection across the safe-to-remove worktrees
// currently visible. If every visible safe worktree is already selected they
// are all deselected; otherwise they are all selected. Non-safe and
// filtered-out rows are never touched. An empty visible safe set is a no-op.
func (m Model) toggleSafeSelection() Model {
	var safe []string
	for _, w := range m.worktreeVisible {
		if isSafeToRemove(w) {
			safe = append(safe, w.Path)
		}
	}
	if len(safe) == 0 {
		return m
	}
	allSelected := true
	for _, p := range safe {
		if !m.selected[p] {
			allSelected = false
			break
		}
	}
	for _, p := range safe {
		if allSelected {
			delete(m.selected, p)
		} else {
			m.selected[p] = true
		}
	}
	_, rs := worktreeLayout(m.worktreeVisible, m.selected, m.worktreeMaxPath, m.worktreeMaxBranch, m.worktreeMaxBadges, m.termWidth)
	m.worktreeTable.SetRows(rs)
	return m
}

// copyFocusedBranch copies the full branch name of the focused worktree to
// the clipboard via OSC52. Branchless rows are a no-op.
func (m Model) copyFocusedBranch() (Model, tea.Cmd) {
	cursor := m.worktreeTable.Cursor()
	if cursor < 0 || cursor >= len(m.worktreeVisible) {
		return m, nil
	}
	branch := m.worktreeVisible[cursor].Branch
	if branch == "" {
		m.copyNotice = "– No branch on this row"
		return m, nil
	}
	m.copyNotice = "✓ Copied branch: " + branch
	return m, setClipboard(branch)
}

// tableWidth returns the natural viewport width needed to render every column
// in full, including bubbles/table's default per-cell padding.
func tableWidth(cols []table.Column) int {
	const cellPadding = 2
	w := 0
	for _, c := range cols {
		w += c.Width + cellPadding
	}
	return w
}

func maxWorktreeWidths(wts []worktree.Worktree) (path, branch int) {
	path = len("Path")
	branch = len("Branch")
	for _, w := range wts {
		if l := len([]rune(w.Path)); l > path {
			path = l
		}
		if l := len([]rune(w.Branch)); l > branch {
			branch = l
		}
	}
	return
}

func formatTime(t time.Time) string {
	if t.IsZero() {
		return emptyTime
	}
	return t.Format(timeLayout)
}

// truncateHead returns s clipped to width runes, replacing the leading
// portion with an ellipsis when truncation is needed, preserving the
// informative tail (the repo/branch name).
func truncateHead(s string, width int) string {
	if width <= 0 {
		return ""
	}
	runes := []rune(s)
	if len(runes) <= width {
		return s
	}
	if width == 1 {
		return ellipsis
	}
	return ellipsis + string(runes[len(runes)-width+1:])
}
