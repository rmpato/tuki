// Package tui is tuki's full-screen interface: the thing you get when you
// type `tuki` with nothing after it.
package tui

import (
	"fmt"
	"strings"
	"time"

	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/progress"
	"charm.land/bubbles/v2/textinput"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"

	"github.com/rmpato/tuki/internal/store"
	"github.com/rmpato/tuki/internal/task"
	"github.com/rmpato/tuki/internal/ui"
)

type mode int

const (
	modeNormal mode = iota
	modeAdd
	modeEdit
	modeDue
	modeSearch
	modeConfirmClear
)

// How long tuki's little celebration lasts, and how smooth it is.
const (
	flashSteps    = 12
	flashInterval = 55 * time.Millisecond
	statusLife    = 3 * time.Second
)

// Layout constants. tuki keeps a narrow column even on wide terminals — the
// whitespace is the point.
const (
	maxContentWidth = 76
	minContentWidth = 24
)

type flashMsg struct{ seq int }
type statusExpiredMsg struct{ seq int }

// row is one rendered line of the task area: either a group heading or a task.
type row struct {
	header string
	tag    string
	done   int
	total  int

	task   task.Task
	isTask bool
}

type undoEntry struct {
	label string
	tasks []task.Task
}

// Model is the Bubble Tea model for tuki.
type Model struct {
	store *store.Store
	data  *store.Data
	theme ui.Theme
	keys  KeyMap

	help  help.Model
	vp    viewport.Model
	input textinput.Model
	prog  progress.Model

	rows     []row
	taskRows []int // indices into rows that are tasks
	cursor   int   // index into taskRows
	selected int   // ID of the selected task, so it survives rebuilds

	mode      mode
	filterTag string
	query     string

	flash     *flashState
	flashSeq  int
	status    string
	statusSeq int

	undo []undoEntry

	// pending collects commands produced deep inside helpers that don't
	// otherwise have a way to hand one back. Update drains it every tick.
	pending []tea.Cmd

	// Rendered chrome, rebuilt by refresh so View stays a pure assembly step.
	header, footer string

	// Mood lines are held steady until the mood actually changes, so tuki
	// doesn't reword itself on every keystroke.
	emptyLine, allDoneLine string
	wasEmpty, wasAllDone   bool

	width, height int
	ready         bool
	saveErr       error
	now           time.Time
	lastRatio     float64
}

type flashState struct {
	id     int
	phrase string
	step   int
}

// New builds the model around an already-loaded task list.
func New(s *store.Store, d *store.Data) *Model {
	in := textinput.New()
	in.Prompt = ""
	in.CharLimit = 240
	in.SetVirtualCursor(true)

	m := &Model{
		store: s,
		data:  d,
		keys:  DefaultKeys(),
		help:  help.New(),
		vp:    viewport.New(),
		input: in,
		now:   time.Now(),
	}
	m.applyTheme(ui.PrefersDark())
	return m
}

// applyTheme rebuilds every style-carrying component for a light or dark
// terminal. It runs once at startup with a guess, and again for real once the
// terminal tells us its background color.
func (m *Model) applyTheme(isDark bool) {
	m.theme = ui.New(isDark)

	m.help.Styles = help.DefaultStyles(isDark)
	m.help.Styles.ShortKey = m.help.Styles.ShortKey.Foreground(m.theme.Muted)
	m.help.Styles.ShortDesc = m.help.Styles.ShortDesc.Foreground(m.theme.Faint)
	m.help.Styles.ShortSeparator = m.help.Styles.ShortSeparator.Foreground(m.theme.Faint)
	m.help.Styles.FullKey = m.help.Styles.FullKey.Foreground(m.theme.Muted)
	m.help.Styles.FullDesc = m.help.Styles.FullDesc.Foreground(m.theme.Faint)
	m.help.Styles.FullSeparator = m.help.Styles.FullSeparator.Foreground(m.theme.Faint)

	styles := textinput.DefaultStyles(isDark)
	styles.Focused.Text = styles.Focused.Text.Foreground(m.theme.Text)
	styles.Focused.Placeholder = styles.Focused.Placeholder.Foreground(m.theme.Faint)
	styles.Cursor.Color = m.theme.Brand
	m.input.SetStyles(styles)

	pct := m.prog.Percent()
	m.prog = progress.New(
		progress.WithWidth(12),
		progress.WithoutPercentage(),
		progress.WithColors(m.theme.Brand, m.theme.Gold),
		progress.WithFillCharacters('━', '━'),
		progress.WithSpringOptions(28, 1.2),
	)
	m.prog.EmptyColor = m.theme.Faint
	_ = m.prog.SetPercent(pct)
}

// Init asks the terminal what color it is, then gets out of the way.
func (m *Model) Init() tea.Cmd {
	return tea.RequestBackgroundColor
}

// Update handles one message. It ends by rebuilding the layout, which is cheap
// enough for a task list that there's no reason to be clever about it.
func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd
	m.now = time.Now()

	switch msg := msg.(type) {
	case tea.BackgroundColorMsg:
		m.applyTheme(msg.IsDark())

	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.ready = true

	case flashMsg:
		if m.flash != nil && msg.seq == m.flashSeq {
			m.flash.step++
			if m.flash.step >= flashSteps {
				m.flash = nil
			} else {
				cmds = append(cmds, m.tickFlash())
			}
		}

	case statusExpiredMsg:
		if msg.seq == m.statusSeq {
			m.status = ""
		}

	case progress.FrameMsg:
		p, cmd := m.prog.Update(msg)
		m.prog = p
		cmds = append(cmds, cmd)

	case tea.KeyPressMsg:
		cmd, quit := m.handleKey(msg)
		if quit {
			return m, tea.Quit
		}
		cmds = append(cmds, cmd)

	case tea.MouseWheelMsg:
		if m.mode == modeNormal {
			p, cmd := m.vp.Update(msg)
			m.vp = p
			cmds = append(cmds, cmd)
		}
	}

	cmds = append(cmds, m.pending...)
	m.pending = nil

	m.refresh()
	return m, tea.Batch(cmds...)
}

// handleKey routes a keypress to whichever mode is active. It returns a
// command and whether tuki should stop.
func (m *Model) handleKey(msg tea.KeyPressMsg) (tea.Cmd, bool) {
	// Ctrl+C always works, everywhere, no matter what's focused.
	if msg.String() == "ctrl+c" {
		return nil, true
	}

	switch m.mode {
	case modeAdd, modeEdit, modeDue, modeSearch:
		return m.handleInputKey(msg), false
	case modeConfirmClear:
		return m.handleConfirmKey(msg), false
	}

	switch {
	case key.Matches(msg, m.keys.Quit):
		return nil, true

	case key.Matches(msg, m.keys.Help):
		m.help.ShowAll = !m.help.ShowAll

	case key.Matches(msg, m.keys.Up):
		m.moveCursor(-1)
	case key.Matches(msg, m.keys.Down):
		m.moveCursor(1)
	case key.Matches(msg, m.keys.Top):
		m.setCursor(0)
	case key.Matches(msg, m.keys.Bottom):
		m.setCursor(len(m.taskRows) - 1)

	case key.Matches(msg, m.keys.Toggle):
		return m.toggleSelected(), false

	case key.Matches(msg, m.keys.Add):
		m.beginInput(modeAdd, "", "what needs doing?   #tag  @when")

	case key.Matches(msg, m.keys.Edit):
		if t, ok := m.selectedTask(); ok {
			m.beginInput(modeEdit, task.FormatInline(t, m.now), "")
		}

	case key.Matches(msg, m.keys.Due):
		if t, ok := m.selectedTask(); ok {
			cur := ""
			if t.Due != nil {
				cur = t.Due.Format("2006-01-02")
			}
			m.beginInput(modeDue, cur, "today · fri · +3d · 2026-08-20 · none")
		}

	case key.Matches(msg, m.keys.Delete):
		m.deleteSelected()

	case key.Matches(msg, m.keys.Undo):
		m.popUndo()

	case key.Matches(msg, m.keys.Tag):
		m.cycleTag()

	case key.Matches(msg, m.keys.Search):
		m.beginInput(modeSearch, m.query, "search")

	case key.Matches(msg, m.keys.Clear):
		if n := m.doneCount(); n > 0 {
			m.mode = modeConfirmClear
		} else {
			m.setStatus("nothing to clear")
		}

	case key.Matches(msg, m.keys.Filter):
		m.cycleFilter(msg.String() == "left" || msg.String() == "h")
	}

	return nil, false
}

// handleInputKey drives the single-line editor used for adding, editing,
// setting a due date, and searching.
func (m *Model) handleInputKey(msg tea.KeyPressMsg) tea.Cmd {
	switch {
	case key.Matches(msg, m.keys.Cancel):
		if m.mode == modeSearch {
			m.query = ""
		}
		m.endInput()
		return nil

	case key.Matches(msg, m.keys.Submit):
		return m.submitInput()
	}

	in, cmd := m.input.Update(msg)
	m.input = in
	if m.mode == modeSearch {
		m.query = m.input.Value()
	}
	return cmd
}

func (m *Model) handleConfirmKey(msg tea.KeyPressMsg) tea.Cmd {
	switch msg.String() {
	case "y", "Y", "enter":
		m.pushUndo("clear")
		n := m.data.ClearDone()
		m.save()
		m.setStatus(fmt.Sprintf("cleared %s · u to undo", plural(n, "task", "tasks")))
	}
	m.mode = modeNormal
	return nil
}

func (m *Model) beginInput(md mode, value, placeholder string) {
	m.mode = md
	m.input.SetValue(value)
	m.input.Placeholder = placeholder
	m.input.CursorEnd()
	m.pending = append(m.pending, m.input.Focus())
}

func (m *Model) endInput() {
	m.mode = modeNormal
	m.input.Blur()
	m.input.Reset()
}

// submitInput commits whatever the editor is being used for. Adding stays open
// afterwards so you can rattle off several tasks in a row.
func (m *Model) submitInput() tea.Cmd {
	value := strings.TrimSpace(m.input.Value())

	switch m.mode {
	case modeAdd:
		if value == "" {
			m.endInput()
			return nil
		}
		in := task.ParseInline(value, m.now)
		if in.Text == "" {
			m.endInput()
			return nil
		}
		t := m.data.Add(in.Text, in.Tag, in.Due)
		m.selected = t.ID
		m.save()
		m.input.Reset()
		m.setStatus("added")
		return nil

	case modeEdit:
		t, ok := m.selectedTask()
		if !ok || value == "" {
			m.endInput()
			return nil
		}
		in := task.ParseInline(value, m.now)
		if cur, ok := m.data.Find(t.ID); ok && in.Text != "" {
			cur.Text = in.Text
			if in.Tag != "" {
				cur.Tag = in.Tag
			}
			if in.Due != nil {
				cur.Due = in.Due
			}
			m.save()
		}
		m.endInput()
		return nil

	case modeDue:
		t, ok := m.selectedTask()
		if !ok {
			m.endInput()
			return nil
		}
		due, err := task.ParseDue(value, m.now)
		if err != nil {
			m.setStatus(err.Error())
			return nil
		}
		if cur, ok := m.data.Find(t.ID); ok {
			cur.Due = due
			m.save()
		}
		m.endInput()
		return nil

	case modeSearch:
		m.query = value
		m.mode = modeNormal
		m.input.Blur()
		return nil
	}

	m.endInput()
	return nil
}

// toggleSelected flips the selected task and, if it just got done, starts the
// celebration.
func (m *Model) toggleSelected() tea.Cmd {
	t, ok := m.selectedTask()
	if !ok {
		return nil
	}
	updated, ok := m.data.Toggle(t.ID)
	if !ok {
		return nil
	}
	m.save()

	if updated.Done && m.data.Settings.Celebrate {
		m.flashSeq++
		m.flash = &flashState{id: updated.ID, phrase: ui.Cheer()}
		return m.tickFlash()
	}
	m.flash = nil
	return nil
}

func (m *Model) deleteSelected() {
	t, ok := m.selectedTask()
	if !ok {
		return
	}
	// Move the cursor first so it lands somewhere sensible afterwards.
	next := m.cursor
	m.pushUndo("delete")
	if _, _, ok := m.data.Remove(t.ID); !ok {
		return
	}
	m.save()
	m.selected = 0
	m.cursor = next
	m.setStatus("deleted · u to undo")
}

func (m *Model) cycleTag() {
	t, ok := m.selectedTask()
	if !ok {
		return
	}
	cur, ok := m.data.Find(t.ID)
	if !ok {
		return
	}
	order := task.Known
	idx := len(order) - 1
	for i, name := range order {
		if name == task.NormalizeTag(cur.Tag) {
			idx = i
			break
		}
	}
	cur.Tag = order[(idx+1)%len(order)]
	m.selected = cur.ID
	m.save()
}

// cycleFilter walks through "all" and each group that currently has tasks.
func (m *Model) cycleFilter(backwards bool) {
	tags, _ := task.Group(m.data.Tasks)
	options := append([]string{""}, tags...)

	idx := 0
	for i, o := range options {
		if o == m.filterTag {
			idx = i
			break
		}
	}
	if backwards {
		idx = (idx - 1 + len(options)) % len(options)
	} else {
		idx = (idx + 1) % len(options)
	}
	m.filterTag = options[idx]
}

func (m *Model) pushUndo(label string) {
	snapshot := make([]task.Task, len(m.data.Tasks))
	copy(snapshot, m.data.Tasks)
	m.undo = append(m.undo, undoEntry{label: label, tasks: snapshot})
	if len(m.undo) > 25 {
		m.undo = m.undo[len(m.undo)-25:]
	}
}

func (m *Model) popUndo() {
	if len(m.undo) == 0 {
		m.setStatus("nothing to undo")
		return
	}
	last := m.undo[len(m.undo)-1]
	m.undo = m.undo[:len(m.undo)-1]
	m.data.Tasks = last.tasks
	m.save()
	m.setStatus("undone")
}

func (m *Model) tickFlash() tea.Cmd {
	seq := m.flashSeq
	return tea.Tick(flashInterval, func(time.Time) tea.Msg { return flashMsg{seq: seq} })
}

// setStatus shows a transient line in the footer and schedules its removal.
func (m *Model) setStatus(s string) {
	m.status = s
	m.statusSeq++
	seq := m.statusSeq
	m.pending = append(m.pending, tea.Tick(statusLife, func(time.Time) tea.Msg {
		return statusExpiredMsg{seq: seq}
	}))
}

// syncProgress nudges the progress bar towards the current ratio, but only
// when that ratio actually moved — otherwise the spring would be re-triggered
// on every keystroke and never settle.
func (m *Model) syncProgress() {
	done, total := task.Stats(m.data.Tasks)
	ratio := 0.0
	if total > 0 {
		ratio = float64(done) / float64(total)
	}
	if ratio == m.lastRatio {
		return
	}
	m.lastRatio = ratio
	m.pending = append(m.pending, m.prog.SetPercent(ratio))
}

func (m *Model) save() {
	if err := m.store.Save(m.data); err != nil {
		m.saveErr = err
	}
}

// visible applies the group filter and the search query.
func (m *Model) visible() []task.Task {
	q := strings.ToLower(strings.TrimSpace(m.query))
	out := make([]task.Task, 0, len(m.data.Tasks))
	for _, t := range m.data.Tasks {
		if m.filterTag != "" && task.NormalizeTag(t.Tag) != m.filterTag {
			continue
		}
		if q != "" && !strings.Contains(strings.ToLower(t.Text), q) {
			continue
		}
		out = append(out, t)
	}
	return out
}

// rebuildRows flattens the visible tasks into headings and rows, keeping the
// cursor on the same task where possible.
func (m *Model) rebuildRows() {
	tags, byTag := task.Group(m.visible())

	m.rows = m.rows[:0]
	m.taskRows = m.taskRows[:0]
	for _, tag := range tags {
		list := byTag[tag]
		done, total := task.Stats(list)
		m.rows = append(m.rows, row{header: strings.ToUpper(tag), tag: tag, done: done, total: total})
		for _, t := range list {
			m.taskRows = append(m.taskRows, len(m.rows))
			m.rows = append(m.rows, row{task: t, tag: tag, isTask: true})
		}
	}

	// Prefer to keep the previously selected task under the cursor.
	if m.selected != 0 {
		for i, ri := range m.taskRows {
			if m.rows[ri].task.ID == m.selected {
				m.cursor = i
				m.syncSelected()
				return
			}
		}
	}
	m.clampCursor()
	m.syncSelected()
}

func (m *Model) clampCursor() {
	if m.cursor >= len(m.taskRows) {
		m.cursor = len(m.taskRows) - 1
	}
	if m.cursor < 0 {
		m.cursor = 0
	}
}

func (m *Model) syncSelected() {
	if t, ok := m.selectedTask(); ok {
		m.selected = t.ID
	} else {
		m.selected = 0
	}
}

func (m *Model) selectedTask() (task.Task, bool) {
	if m.cursor < 0 || m.cursor >= len(m.taskRows) {
		return task.Task{}, false
	}
	return m.rows[m.taskRows[m.cursor]].task, true
}

func (m *Model) moveCursor(delta int) {
	if len(m.taskRows) == 0 {
		return
	}
	m.setCursor(m.cursor + delta)
}

func (m *Model) setCursor(i int) {
	if len(m.taskRows) == 0 {
		return
	}
	if i < 0 {
		i = 0
	}
	if i >= len(m.taskRows) {
		i = len(m.taskRows) - 1
	}
	m.cursor = i
	m.syncSelected()
}

func (m *Model) doneCount() int {
	done, _ := task.Stats(m.data.Tasks)
	return done
}

// contentWidth is the width of tuki's column. It prefers a narrow column with
// room either side, but never claims more than the terminal actually has.
func (m *Model) contentWidth() int {
	w := m.width - 4
	if w > maxContentWidth {
		w = maxContentWidth
	}
	if w < minContentWidth {
		w = minContentWidth
	}
	if w > m.width {
		w = m.width
	}
	if w < 1 {
		w = 1
	}
	return w
}

// gutterWidth centres that column in whatever space is left over.
func (m *Model) gutterWidth() int {
	pad := (m.width - m.contentWidth()) / 2
	if pad < 0 {
		pad = 0
	}
	return pad
}

func plural(n int, one, many string) string {
	if n == 1 {
		return fmt.Sprintf("%d %s", n, one)
	}
	return fmt.Sprintf("%d %s", n, many)
}
