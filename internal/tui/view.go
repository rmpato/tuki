package tui

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/rmpato/tuki/internal/task"
	"github.com/rmpato/tuki/internal/ui"
)

// Marks for the two states a task can be in. Small and quiet on purpose.
const (
	markPending = "○"
	markDone    = "✓"
	cursorMark  = "▸"
)

// refresh rebuilds everything the view needs. It runs after each message,
// which keeps View a pure assembly step with no hidden state changes.
func (m *Model) refresh() {
	if !m.ready {
		return
	}
	m.rebuildRows()
	m.syncProgress()
	m.rerollMoodLines()

	m.header = m.renderHeader()
	m.footer = m.renderFooter()

	lines, cursorLine := m.renderBody()

	height := m.height - lipgloss.Height(m.header) - lipgloss.Height(m.footer)
	if height < 1 {
		height = 1
	}
	m.vp.SetWidth(m.width)
	m.vp.SetHeight(height)
	m.vp.SetContentLines(lines)
	if cursorLine >= 0 {
		m.vp.EnsureVisible(cursorLine, 0, 0)
	}
}

// rerollMoodLines picks a fresh line only when the mood actually changes, so
// tuki doesn't appear to change its mind on every keystroke.
func (m *Model) rerollMoodLines() {
	done, total := task.Stats(m.data.Tasks)

	isEmpty := total == 0
	if isEmpty && !m.wasEmpty {
		m.emptyLine = ui.Empty()
	}
	m.wasEmpty = isEmpty

	allDone := total > 0 && done == total
	if allDone && !m.wasAllDone {
		m.allDoneLine = ui.AllDone()
	}
	m.wasAllDone = allDone
}

// View assembles the screen. Everything it renders was computed in refresh.
func (m *Model) View() tea.View {
	v := tea.NewView("")
	v.AltScreen = true
	v.WindowTitle = "tuki"
	if !m.ready {
		return v
	}
	frame := strings.Join([]string{m.header, m.vp.View(), m.footer}, "\n")
	v.SetContent(m.clampFrame(frame))
	return v
}

// clampFrame guarantees the frame fits the terminal. The renderers above all
// size themselves properly, but a terminal can always be smaller than any
// layout is willing to be, and a frame that overflows corrupts the display —
// so this is the backstop that makes the invariant unconditional.
func (m *Model) clampFrame(frame string) string {
	lines := strings.Split(frame, "\n")
	if len(lines) > m.height {
		lines = lines[:m.height]
	}
	for i, l := range lines {
		if ansi.StringWidth(l) > m.width {
			lines[i] = ansi.Truncate(l, m.width, "")
		}
	}
	return strings.Join(lines, "\n")
}

// fit trims a styled string to tuki's column, keeping its escape sequences
// intact. Anything free-text goes through here.
func (m *Model) fit(s string) string {
	return ansi.Truncate(s, m.contentWidth(), "…")
}

// gutter is the left margin that centres tuki's narrow column.
func (m *Model) gutter() string {
	return strings.Repeat(" ", m.gutterWidth())
}

// row lays out a left and a right chunk with the gap between them filled.
func (m *Model) row(left, right string) string {
	w := m.contentWidth()
	gap := w - lipgloss.Width(left) - lipgloss.Width(right)
	if gap < 1 {
		gap = 1
	}
	return m.gutter() + left + strings.Repeat(" ", gap) + right
}

// renderHeader is always exactly four lines tall, so nothing below it ever
// shifts around.
func (m *Model) renderHeader() string {
	done, total := task.Stats(m.data.Tasks)

	left := m.theme.Face.Render(m.face()) + "  " + m.theme.Logo.Render("tuki")
	right := m.progressChip(done, total, m.contentWidth()-lipgloss.Width(left)-2)

	// tuki's aside gives way to the filter indicator, which is the one the
	// user actually needs in order to understand what they're looking at.
	scope := m.fit(m.scopeLine())
	mood := ansi.Truncate(m.moodLine(), max(0, m.contentWidth()-lipgloss.Width(scope)-2), "…")

	return strings.Join([]string{
		"",
		m.row(left, right),
		m.row(mood, scope),
		"",
	}, "\n")
}

// progressChip is the bar and count on the right of the header. It sheds
// parts of itself until it fits the space available, so a narrow terminal
// still gets a readable header instead of an overflowing one.
func (m *Model) progressChip(done, total, avail int) string {
	if total == 0 || avail < 3 {
		return ""
	}

	bar := m.prog.View()
	long := m.theme.Count.Render(fmt.Sprintf("%d/%d done", done, total))
	short := m.theme.Count.Render(fmt.Sprintf("%d/%d", done, total))

	for _, candidate := range []string{
		bar + "  " + long,
		bar + "  " + short,
		long,
		short,
	} {
		if lipgloss.Width(candidate) <= avail {
			return candidate
		}
	}
	return ""
}

// face picks tuki's expression from whatever is going on.
func (m *Model) face() string {
	done, total := task.Stats(m.data.Tasks)

	switch {
	case m.flash != nil:
		return ui.FaceHappy
	case m.mode != modeNormal:
		return ui.FaceCurious
	case total == 0:
		return ui.FaceSleepy
	case done == total:
		return ui.FaceCheer
	case m.data.Settings.Judge && task.CountOverdue(m.data.Tasks, m.now) >= 6:
		return ui.FaceSide
	default:
		return ui.FaceIdle
	}
}

// moodLine is the one thing tuki gets to say about your list.
func (m *Model) moodLine() string {
	if m.flash != nil {
		return m.theme.Cheer.Render(m.flash.phrase)
	}
	done, total := task.Stats(m.data.Tasks)
	if total > 0 && done == total {
		return m.theme.Aside.Render(m.allDoneLine)
	}
	if m.data.Settings.Judge {
		if line := ui.Judge(task.CountOverdue(m.data.Tasks, m.now)); line != "" {
			return m.theme.Aside.Render(line)
		}
	}
	return ""
}

// scopeLine shows which filters are narrowing the list, if any.
func (m *Model) scopeLine() string {
	var parts []string
	if m.filterTag != "" {
		parts = append(parts, m.theme.TagStyle(m.filterTag).Render(m.filterTag))
	}
	if q := strings.TrimSpace(m.query); q != "" {
		parts = append(parts, m.theme.Hint.Render("/"+q))
	}
	if len(parts) == 0 {
		return ""
	}
	return strings.Join(parts, m.theme.Hint.Render(" · "))
}

// renderBody produces the scrollable task area and the line the cursor is on.
func (m *Model) renderBody() (lines []string, cursorLine int) {
	cursorLine = -1
	if len(m.taskRows) == 0 {
		return m.renderEmpty(), -1
	}

	selectedRow := -1
	if m.cursor >= 0 && m.cursor < len(m.taskRows) {
		selectedRow = m.taskRows[m.cursor]
	}

	for i, r := range m.rows {
		if !r.isTask {
			if len(lines) > 0 {
				lines = append(lines, "")
			}
			lines = append(lines, m.renderSection(r))
			lines = append(lines, "")
			continue
		}
		if i == selectedRow {
			cursorLine = len(lines)
		}
		lines = append(lines, m.renderTask(r.task, i == selectedRow))
	}
	lines = append(lines, "")
	return lines, cursorLine
}

// renderSection is a group heading: WORK ──────────── 1/3
func (m *Model) renderSection(r row) string {
	label := m.theme.TagStyle(r.tag).Render(r.header)
	count := m.theme.Hint.Render(fmt.Sprintf("%d/%d", r.done, r.total))

	fill := m.contentWidth() - lipgloss.Width(label) - lipgloss.Width(count) - 2
	if fill < 1 {
		fill = 1
	}
	rule := m.theme.Rule.Render(strings.Repeat("─", fill))
	return m.gutter() + label + " " + rule + " " + count
}

// renderTask draws one task line: cursor, mark, text, and due date.
func (m *Model) renderTask(t task.Task, selected bool) string {
	flashing := m.flash != nil && m.flash.id == t.ID

	cursor := "  "
	if selected {
		cursor = m.theme.Cursor.Render(cursorMark) + " "
	}

	textStyle := m.theme.Pending
	markStyle := m.theme.Mark
	mark := markPending

	switch {
	case flashing:
		// Fade the freshly-completed task from gold back to its resting colour.
		c := m.theme.Fade(m.theme.Faint, m.flash.step, flashSteps)
		textStyle = lipgloss.NewStyle().Foreground(c).Strikethrough(true).StrikethroughSpaces(true)
		markStyle = lipgloss.NewStyle().Foreground(c)
		mark = markDone
	case t.Done:
		textStyle = m.theme.Completed
		markStyle = m.theme.MarkDone
		mark = markDone
		if selected {
			textStyle = textStyle.Foreground(m.theme.Brand)
		}
	case selected:
		textStyle = m.theme.Selected
	}

	due := m.dueChip(t)
	dueW := lipgloss.Width(due)

	textW := m.contentWidth() - 4 - dueW
	if dueW > 0 {
		textW -= 2
	}
	if textW < 8 {
		textW = 8
	}

	text := textStyle.Render(ansi.Truncate(t.Text, textW, "…"))
	return m.row(cursor+markStyle.Render(mark)+" "+text, due)
}

// dueChip renders the small date on the right of a task.
func (m *Model) dueChip(t task.Task) string {
	if t.Due == nil {
		return ""
	}
	label := task.HumanDue(*t.Due, m.now)
	switch {
	case t.Done:
		return m.theme.Hint.Render(label)
	case t.Overdue(m.now):
		return m.theme.DueLate.Render(label)
	case t.DueToday(m.now):
		return m.theme.DueSoon.Render(label)
	default:
		return m.theme.Due.Render(label)
	}
}

// renderEmpty is the screen you get with nothing to show: either tuki napping,
// or a note that your filter matched nothing.
func (m *Model) renderEmpty() []string {
	var out []string
	out = append(out, "", "")

	if len(m.data.Tasks) == 0 {
		for _, line := range strings.Split(ui.Blob(ui.FaceSleepy), "\n") {
			out = append(out, m.gutter()+m.theme.Face.Render(line))
		}
		out = append(out, "")
		out = append(out, m.gutter()+m.fit(m.theme.Aside.Render(m.emptyLine)))
		out = append(out, "")
		out = append(out, m.gutter()+m.fit(m.theme.Hint.Render("press a to add the first one")))
		return out
	}

	out = append(out, m.gutter()+m.fit(m.theme.Aside.Render("nothing matches.")))
	out = append(out, "")
	out = append(out, m.gutter()+m.fit(m.theme.Hint.Render("tab changes the group · esc clears the search")))
	return out
}

// renderFooter is the status line plus either the editor, a confirmation, or
// the key hints.
func (m *Model) renderFooter() string {
	status := ""
	if m.saveErr != nil {
		status = m.fit(m.theme.Warn.Render("couldn't save: " + m.saveErr.Error()))
	} else if m.status != "" {
		status = m.fit(m.theme.Hint.Render(m.status))
	}

	var bottom string
	switch m.mode {
	case modeAdd:
		bottom = m.editor("+", "enter adds · esc closes")
	case modeEdit:
		bottom = m.editor("~", "enter saves · esc cancels")
	case modeDue:
		bottom = m.editor("@", "enter saves · esc cancels")
	case modeSearch:
		bottom = m.editor("/", "enter keeps · esc clears")
	case modeConfirmClear:
		q := fmt.Sprintf("clear %s? ", plural(m.doneCount(), "completed task", "completed tasks"))
		bottom = m.gutter() + m.fit(m.theme.Prompt.Render(q)+m.theme.Hint.Render("y / n"))
	default:
		m.help.SetWidth(m.contentWidth())
		// The full key list needs room to be worth showing; on a short
		// terminal the one-line version is more use than a truncated table.
		full := m.help.ShowAll
		m.help.ShowAll = full && m.height >= 16 && m.contentWidth() >= 56
		bottom = indentLines(m.help.View(m.keys), m.gutter())
		m.help.ShowAll = full
	}

	return strings.Join([]string{m.gutter() + status, bottom}, "\n")
}

// editor renders the single-line input with its sigil and a quiet hint.
func (m *Model) editor(sigil, hint string) string {
	// Two cells for the sigil, one so the cursor at the end of a full line has
	// somewhere to sit.
	m.input.SetWidth(max(1, m.contentWidth()-3))
	line := m.gutter() + m.theme.Prompt.Render(sigil+" ") + m.input.View()
	return line + "\n" + m.gutter() + m.fit(m.theme.Hint.Render(hint))
}

func indentLines(s, pad string) string {
	lines := strings.Split(s, "\n")
	for i, l := range lines {
		lines[i] = pad + l
	}
	return strings.Join(lines, "\n")
}
