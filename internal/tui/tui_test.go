package tui

import (
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/rmpato/tuki/internal/store"
	"github.com/rmpato/tuki/internal/task"
)

// harness drives a Model the way Bubble Tea would, so the tests exercise the
// real Update/View path rather than the internals.
type harness struct {
	t     *testing.T
	m     *Model
	store *store.Store
}

func newHarness(t *testing.T, seed func(*store.Data)) *harness {
	t.Helper()

	s := store.New(filepath.Join(t.TempDir(), "tasks.json"))
	d, err := s.Load()
	if err != nil {
		t.Fatal(err)
	}
	if seed != nil {
		seed(d)
	}
	if err := s.Save(d); err != nil {
		t.Fatal(err)
	}

	h := &harness{t: t, m: New(s, d), store: s}
	h.m.Init()
	h.send(tea.WindowSizeMsg{Width: 90, Height: 24})
	return h
}

// send delivers a message and runs any command it returns, so a command that
// panics still fails the test. Commands are capped at a short timeout because
// several of them are timers that would otherwise sleep for seconds; the
// resulting message is discarded, since every assertion here is about state
// that Update sets synchronously.
func (h *harness) send(msg tea.Msg) {
	h.t.Helper()
	model, cmd := h.m.Update(msg)
	h.m = model.(*Model)
	if cmd == nil {
		return
	}
	done := make(chan struct{})
	go func() {
		defer close(done)
		cmd()
	}()
	select {
	case <-done:
	case <-time.After(20 * time.Millisecond):
	}
}

// press sends a key by name, the same string Bubble Tea would produce.
func (h *harness) press(keys ...string) {
	h.t.Helper()
	for _, k := range keys {
		h.send(keyPress(k))
	}
}

// typeText sends each character as its own key press.
func (h *harness) typeText(s string) {
	h.t.Helper()
	for _, r := range s {
		if r == ' ' {
			h.send(tea.KeyPressMsg{Code: ' ', Text: " "})
			continue
		}
		h.send(tea.KeyPressMsg{Code: r, Text: string(r)})
	}
}

// screen is the rendered frame with all styling stripped, which is what the
// user actually sees laid out.
func (h *harness) screen() string {
	h.t.Helper()
	return ansi.Strip(h.m.View().Content)
}

func (h *harness) reload() *store.Data {
	h.t.Helper()
	d, err := h.store.Load()
	if err != nil {
		h.t.Fatal(err)
	}
	return d
}

// keyPress builds the KeyPressMsg for a named key.
func keyPress(name string) tea.KeyPressMsg {
	switch name {
	case "space":
		return tea.KeyPressMsg{Code: ' ', Text: " "}
	case "enter":
		return tea.KeyPressMsg{Code: tea.KeyEnter}
	case "esc":
		return tea.KeyPressMsg{Code: tea.KeyEscape}
	case "tab":
		return tea.KeyPressMsg{Code: tea.KeyTab}
	case "backspace":
		return tea.KeyPressMsg{Code: tea.KeyBackspace}
	case "up":
		return tea.KeyPressMsg{Code: tea.KeyUp}
	case "down":
		return tea.KeyPressMsg{Code: tea.KeyDown}
	default:
		r := []rune(name)[0]
		return tea.KeyPressMsg{Code: r, Text: name}
	}
}

func seedThree(d *store.Data) {
	due := task.StartOfDay(time.Now()).AddDate(0, 0, -2)
	d.Add("write the docs", "work", nil)
	d.Add("reply to sam", "work", &due)
	d.Add("buy oat milk", "home", nil)
}

func TestTasksAreVisibleImmediately(t *testing.T) {
	h := newHarness(t, seedThree)
	s := h.screen()

	for _, want := range []string{"tuki", "WORK", "HOME", "write the docs", "buy oat milk"} {
		if !strings.Contains(s, want) {
			t.Errorf("screen is missing %q:\n%s", want, s)
		}
	}
	if !strings.Contains(s, "0/3 done") {
		t.Errorf("screen is missing the progress count:\n%s", s)
	}
}

func TestEmptyStateShowsTuki(t *testing.T) {
	h := newHarness(t, nil)
	s := h.screen()

	if !strings.Contains(s, "press a to add the first one") {
		t.Errorf("empty screen should invite you to add something:\n%s", s)
	}
}

func TestSpaceCompletesAndPersists(t *testing.T) {
	h := newHarness(t, seedThree)
	h.press("space")

	d := h.reload()
	if !d.Tasks[0].Done {
		t.Error("space should have completed the first task on disk")
	}
	if d.Tasks[0].DoneAt == nil {
		t.Error("completing a task should stamp DoneAt")
	}
	if !strings.Contains(h.screen(), "1/3 done") {
		t.Errorf("progress should read 1/3:\n%s", h.screen())
	}
}

// Completed tasks stay put and stay visible — they don't vanish.
func TestCompletedTaskStaysInPlace(t *testing.T) {
	h := newHarness(t, seedThree)
	h.press("space")

	s := h.screen()
	if !strings.Contains(s, "write the docs") {
		t.Errorf("a completed task should still be on screen:\n%s", s)
	}
	if strings.Index(s, "write the docs") > strings.Index(s, "reply to sam") {
		t.Error("completing a task should not reorder the list")
	}
}

func TestCursorMovesAndWrapsAtEnds(t *testing.T) {
	h := newHarness(t, seedThree)

	h.press("j", "j")
	if got := h.m.selected; got != 3 {
		t.Errorf("after two downs, selected id = %d, want 3", got)
	}

	// Moving past the last task stays on the last task.
	h.press("j", "j")
	if got := h.m.selected; got != 3 {
		t.Errorf("cursor should stop at the end, got id %d", got)
	}

	h.press("g")
	if got := h.m.selected; got != 1 {
		t.Errorf("g should jump to the top, got id %d", got)
	}

	h.press("G")
	if got := h.m.selected; got != 3 {
		t.Errorf("G should jump to the bottom, got id %d", got)
	}
}

func TestAddFlow(t *testing.T) {
	h := newHarness(t, seedThree)

	h.press("a")
	if h.m.mode != modeAdd {
		t.Fatal("a should open the editor")
	}
	h.typeText("learn a chord #hobby")
	h.press("enter")

	d := h.reload()
	last := d.Tasks[len(d.Tasks)-1]
	if last.Text != "learn a chord" {
		t.Errorf("added text = %q", last.Text)
	}
	if last.Tag != "hobby" {
		t.Errorf("added tag = %q, want hobby", last.Tag)
	}

	// The editor stays open so several tasks can be entered in a row.
	if h.m.mode != modeAdd {
		t.Error("the editor should stay open after adding")
	}
	if h.m.input.Value() != "" {
		t.Errorf("the editor should be cleared, got %q", h.m.input.Value())
	}

	h.press("esc")
	if h.m.mode != modeNormal {
		t.Error("esc should close the editor")
	}
}

// Keys typed into the editor must not also act as commands.
func TestTypingDoesNotTriggerCommands(t *testing.T) {
	h := newHarness(t, seedThree)
	before := len(h.reload().Tasks)

	h.press("a")
	h.typeText("delete quit add")
	h.press("esc")

	if got := len(h.reload().Tasks); got != before {
		t.Errorf("typing changed the task count: %d, want %d", got, before)
	}
}

func TestDeleteAndUndo(t *testing.T) {
	h := newHarness(t, seedThree)
	h.press("d")

	if got := len(h.reload().Tasks); got != 2 {
		t.Fatalf("after delete: %d tasks, want 2", got)
	}

	h.press("u")
	d := h.reload()
	if len(d.Tasks) != 3 {
		t.Fatalf("after undo: %d tasks, want 3", len(d.Tasks))
	}
	if d.Tasks[0].Text != "write the docs" {
		t.Errorf("undo should restore position, got %q first", d.Tasks[0].Text)
	}
}

func TestEditRewritesTask(t *testing.T) {
	h := newHarness(t, seedThree)

	h.press("e")
	if h.m.input.Value() == "" {
		t.Fatal("edit should prefill the current task")
	}
	// Clear the field, then type something new.
	for range 60 {
		h.press("backspace")
	}
	h.typeText("write better docs")
	h.press("enter")

	if got := h.reload().Tasks[0].Text; got != "write better docs" {
		t.Errorf("edited text = %q", got)
	}
}

func TestCycleTagMovesTaskBetweenGroups(t *testing.T) {
	h := newHarness(t, seedThree)
	h.press("t") // work -> home

	if got := h.reload().Tasks[0].Tag; got != "home" {
		t.Errorf("tag after cycling = %q, want home", got)
	}
	// The cursor follows the task into its new group.
	if h.m.selected != 1 {
		t.Errorf("selection should follow the task, got id %d", h.m.selected)
	}
}

func TestSearchFiltersLive(t *testing.T) {
	h := newHarness(t, seedThree)

	h.press("/")
	h.typeText("milk")

	s := h.screen()
	if !strings.Contains(s, "buy oat milk") {
		t.Errorf("search should keep the match:\n%s", s)
	}
	if strings.Contains(s, "write the docs") {
		t.Errorf("search should hide non-matches:\n%s", s)
	}

	h.press("esc")
	if !strings.Contains(h.screen(), "write the docs") {
		t.Error("esc should clear the search")
	}
}

func TestFilterCyclesGroups(t *testing.T) {
	h := newHarness(t, seedThree)
	h.press("tab") // all -> work

	s := h.screen()
	if !strings.Contains(s, "write the docs") {
		t.Errorf("work filter should show work tasks:\n%s", s)
	}
	if strings.Contains(s, "buy oat milk") {
		t.Errorf("work filter should hide home tasks:\n%s", s)
	}
}

func TestClearAsksFirst(t *testing.T) {
	h := newHarness(t, seedThree)
	h.press("space") // complete one
	h.press("c")

	if h.m.mode != modeConfirmClear {
		t.Fatal("c should ask before clearing")
	}
	if !strings.Contains(h.screen(), "clear 1 completed task?") {
		t.Errorf("the confirmation should name the count:\n%s", h.screen())
	}

	h.press("n")
	if got := len(h.reload().Tasks); got != 3 {
		t.Errorf("declining should keep everything, got %d tasks", got)
	}

	h.press("c")
	h.press("y")
	if got := len(h.reload().Tasks); got != 2 {
		t.Errorf("confirming should clear the completed task, got %d tasks", got)
	}
}

func TestOverdueTasksAreShown(t *testing.T) {
	h := newHarness(t, seedThree)
	if !strings.Contains(h.screen(), "2d ago") {
		t.Errorf("an overdue task should show how late it is:\n%s", h.screen())
	}
}

// tuki keeps quiet until things get genuinely silly.
func TestJudgementOnlyWhenEarned(t *testing.T) {
	late := task.StartOfDay(time.Now()).AddDate(0, 0, -5)

	quiet := newHarness(t, func(d *store.Data) {
		d.Add("one", "work", &late)
	})
	if strings.Contains(quiet.screen(), "overdue") {
		t.Error("tuki should not comment on a single late task")
	}

	loud := newHarness(t, func(d *store.Data) {
		for range 12 {
			d.Add("late thing", "work", &late)
		}
	})
	if !strings.Contains(loud.screen(), "overdue") {
		t.Errorf("tuki should say something about 12 overdue tasks:\n%s", loud.screen())
	}
}

// The frame must always fit the terminal it was given, in every state that
// puts long text on screen.
func TestFrameFitsTerminal(t *testing.T) {
	late := task.StartOfDay(time.Now()).AddDate(0, 0, -5)

	states := map[string]struct {
		seed  func(*store.Data)
		setup func(*harness)
	}{
		"normal": {seedThree, nil},
		"empty":  {nil, nil},
		"adding": {seedThree, func(h *harness) {
			h.press("a")
			h.typeText("a fairly long task description that keeps going")
		}},
		"searching": {seedThree, func(h *harness) {
			h.press("/")
			h.typeText("nothing will match this")
		}},
		"confirming": {seedThree, func(h *harness) { h.press("space"); h.press("c") }},
		"full help":  {seedThree, func(h *harness) { h.press("?") }},
		"judged": {
			func(d *store.Data) {
				for range 30 {
					d.Add("a task that is late and has a long name", "work", &late)
				}
			},
			nil,
		},
		"long text": {
			func(d *store.Data) {
				d.Add(strings.Repeat("very long task ", 20), "work", &late)
			},
			nil,
		},
	}

	sizes := []tea.WindowSizeMsg{
		{Width: 90, Height: 24},
		{Width: 40, Height: 12},
		{Width: 200, Height: 60},
		{Width: 30, Height: 8},
		{Width: 20, Height: 6},
	}

	for name, state := range states {
		for _, size := range sizes {
			h := newHarness(t, state.seed)
			h.send(size)
			if state.setup != nil {
				state.setup(h)
			}

			lines := strings.Split(h.screen(), "\n")
			if len(lines) > size.Height {
				t.Errorf("%s at %dx%d: frame is %d lines, want at most %d",
					name, size.Width, size.Height, len(lines), size.Height)
			}
			for i, l := range lines {
				if w := ansi.StringWidth(l); w > size.Width {
					t.Errorf("%s at %dx%d: line %d is %d cells wide, want at most %d\n%q",
						name, size.Width, size.Height, i, w, size.Width, l)
				}
			}
		}
	}
}

func TestQuitKeys(t *testing.T) {
	for _, k := range []string{"q", "ctrl+c"} {
		h := newHarness(t, seedThree)
		_, quit := h.m.handleKey(keyPressOrCtrlC(k))
		if !quit {
			t.Errorf("%q should quit", k)
		}
	}
}

func keyPressOrCtrlC(name string) tea.KeyPressMsg {
	if name == "ctrl+c" {
		return tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl}
	}
	return keyPress(name)
}
