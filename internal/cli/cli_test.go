package cli

import (
	"bytes"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rmpato/tuki/internal/store"
	"github.com/rmpato/tuki/internal/task"
)

// runner executes commands against a throwaway tasks file, capturing output.
// Output is never a terminal here, so every command takes its script-friendly
// path — which is exactly the surface these tests are about.
type runner struct {
	t    *testing.T
	path string
}

func newRunner(t *testing.T) *runner {
	t.Helper()
	return &runner{t: t, path: filepath.Join(t.TempDir(), "tasks.json")}
}

func (r *runner) run(args ...string) string {
	r.t.Helper()
	out, err := r.try(args...)
	if err != nil {
		r.t.Fatalf("tuki %s: %v", strings.Join(args, " "), err)
	}
	return out
}

func (r *runner) try(args ...string) (string, error) {
	r.t.Helper()

	var buf bytes.Buffer
	cmd := NewRoot()
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetIn(strings.NewReader(""))
	cmd.SetArgs(append([]string{"--file", r.path}, args...))

	err := cmd.Execute()
	return buf.String(), err
}

func (r *runner) tasks() []task.Task {
	r.t.Helper()
	d, err := store.New(r.path).Load()
	if err != nil {
		r.t.Fatal(err)
	}
	return d.Tasks
}

func TestAddAndList(t *testing.T) {
	r := newRunner(t)
	r.run("add", "buy oat milk", "--tag", "home", "--due", "tomorrow")
	r.run("add", "write the docs #work @fri")

	out := r.run("list")
	lines := strings.Split(strings.TrimSpace(out), "\n")
	if len(lines) != 2 {
		t.Fatalf("expected 2 lines, got %d:\n%s", len(lines), out)
	}

	// id \t state \t tag \t due \t text
	first := strings.Split(lines[0], "\t")
	if len(first) != 5 {
		t.Fatalf("expected 5 tab-separated fields, got %d: %q", len(first), lines[0])
	}
	if first[0] != "1" || first[1] != "todo" || first[2] != "home" || first[4] != "buy oat milk" {
		t.Errorf("unexpected row: %q", lines[0])
	}
	if first[3] == "" {
		t.Error("due date column should be filled in")
	}

	second := strings.Split(lines[1], "\t")
	if second[2] != "work" {
		t.Errorf("inline #work was not picked up: %q", lines[1])
	}
	if second[4] != "write the docs" {
		t.Errorf("inline markers should be stripped from the text: %q", second[4])
	}
}

// A flag and an inline marker disagreeing: the flag is the explicit one.
func TestFlagsBeatInlineMarkers(t *testing.T) {
	r := newRunner(t)
	r.run("add", "a thing #hobby", "--tag", "work")

	if got := r.tasks()[0].Tag; got != "work" {
		t.Errorf("tag = %q, want work", got)
	}
}

func TestDoneAndUndone(t *testing.T) {
	r := newRunner(t)
	r.run("add", "one")
	r.run("add", "two")

	r.run("done", "1", "2")
	for _, tk := range r.tasks() {
		if !tk.Done {
			t.Errorf("task %d should be done", tk.ID)
		}
	}

	r.run("undone", "2")
	if r.tasks()[1].Done {
		t.Error("task 2 should be back on the list")
	}
}

func TestUnknownIDIsAnError(t *testing.T) {
	r := newRunner(t)
	r.run("add", "one")

	if _, err := r.try("done", "99"); err == nil {
		t.Error("done on a missing task should fail")
	}
	if _, err := r.try("done", "abc"); err == nil {
		t.Error("done with a non-number should fail")
	}
	// The valid task must not have been touched by the failed calls.
	if r.tasks()[0].Done {
		t.Error("a failed command should not change anything")
	}
}

func TestEditChangesTextTagAndDue(t *testing.T) {
	r := newRunner(t)
	r.run("add", "wrte the docs", "--due", "tomorrow")

	r.run("edit", "1", "write the docs")
	if got := r.tasks()[0].Text; got != "write the docs" {
		t.Errorf("text = %q", got)
	}

	r.run("edit", "1", "--tag", "work")
	if got := r.tasks()[0].Tag; got != "work" {
		t.Errorf("tag = %q", got)
	}

	r.run("edit", "1", "--due", "none")
	if due := r.tasks()[0].Due; due != nil {
		t.Errorf("due = %v, want cleared", due)
	}
}

func TestRemove(t *testing.T) {
	r := newRunner(t)
	r.run("add", "one")
	r.run("add", "two")

	r.run("rm", "1")
	got := r.tasks()
	if len(got) != 1 || got[0].Text != "two" {
		t.Errorf("after rm: %+v", got)
	}
}

// Without a terminal to ask, clear refuses rather than guessing.
func TestClearNeedsConsentInScripts(t *testing.T) {
	r := newRunner(t)
	r.run("add", "one")
	r.run("done", "1")

	if _, err := r.try("clear"); err == nil {
		t.Error("clear without --yes and without a terminal should fail")
	}
	if len(r.tasks()) != 1 {
		t.Error("the refused clear should not have removed anything")
	}

	r.run("clear", "--yes")
	if len(r.tasks()) != 0 {
		t.Error("clear --yes should have removed the completed task")
	}
}

func TestClearKeepsUnfinished(t *testing.T) {
	r := newRunner(t)
	r.run("add", "done one")
	r.run("add", "still todo")
	r.run("done", "1")

	r.run("clear", "--yes")
	got := r.tasks()
	if len(got) != 1 || got[0].Text != "still todo" {
		t.Errorf("after clear: %+v", got)
	}

	r.run("clear", "--all", "--yes")
	if len(r.tasks()) != 0 {
		t.Error("clear --all should remove everything")
	}
}

func TestListFilters(t *testing.T) {
	r := newRunner(t)
	r.run("add", "work thing", "--tag", "work")
	r.run("add", "home thing", "--tag", "home")
	r.run("done", "1")

	if out := r.run("list", "--tag", "work"); !strings.Contains(out, "work thing") || strings.Contains(out, "home thing") {
		t.Errorf("--tag work gave:\n%s", out)
	}
	if out := r.run("list", "--todo"); strings.Contains(out, "work thing") {
		t.Errorf("--todo should hide completed tasks:\n%s", out)
	}
	if out := r.run("list", "--done"); strings.Contains(out, "home thing") {
		t.Errorf("--done should hide unfinished tasks:\n%s", out)
	}
}

func TestListJSON(t *testing.T) {
	r := newRunner(t)
	r.run("add", "buy oat milk", "--tag", "home")

	var got []task.Task
	if err := json.Unmarshal([]byte(r.run("list", "--json")), &got); err != nil {
		t.Fatalf("--json did not produce valid JSON: %v", err)
	}
	if len(got) != 1 || got[0].Text != "buy oat milk" || got[0].Tag != "home" {
		t.Errorf("--json gave %+v", got)
	}
}

// Piping `tuki` with no subcommand should list, not try to open an interface.
func TestBareCommandListsWhenPiped(t *testing.T) {
	r := newRunner(t)
	r.run("add", "buy oat milk")

	if out := r.run(); !strings.Contains(out, "buy oat milk") {
		t.Errorf("bare tuki should have listed:\n%s", out)
	}
}

func TestTagsAndPath(t *testing.T) {
	r := newRunner(t)
	r.run("add", "a", "--tag", "work")
	r.run("add", "b", "--tag", "work")
	r.run("add", "c", "--tag", "home")
	r.run("done", "1")

	out := r.run("tags")
	if !strings.Contains(out, "work\t1\t2") {
		t.Errorf("tags gave:\n%s", out)
	}

	if got := strings.TrimSpace(r.run("path")); got != r.path {
		t.Errorf("path = %q, want %q", got, r.path)
	}
}

func TestBadDueIsRejected(t *testing.T) {
	r := newRunner(t)
	if _, err := r.try("add", "a thing", "--due", "whenever"); err == nil {
		t.Error("an unparseable --due should fail")
	}
	if len(r.tasks()) != 0 {
		t.Error("a rejected add should not have saved anything")
	}
}

func TestEmptyAddIsRejected(t *testing.T) {
	r := newRunner(t)
	if _, err := r.try("add", "   "); err == nil {
		t.Error("adding nothing should fail")
	}
}

// Typos should be reported, not silently open the interface.
func TestUnknownCommandFails(t *testing.T) {
	r := newRunner(t)
	if _, err := r.try("lsit"); err == nil {
		t.Error("an unknown command should fail")
	}
}

func TestNotesPreviewStripsMarkdown(t *testing.T) {
	notes := "## Changelog\n" +
		"* 37ba49827febfc12a7a9af6243c87528cbb71127: Add releases and self-update (@rmpato)\n" +
		"* deadbeef: Fix a thing (@someone)\n" +
		"\n## Install\n\n```sh\ncurl -fsSL https://example.test/install.sh | sh\n```\n" +
		"Some trailing prose.\n"

	got := notesPreview(notes, 8)
	// Everything from the "## Install" heading onward is boilerplate appended
	// to every release, so it stops there rather than trailing prose into the
	// terminal.
	want := []string{
		"· Add releases and self-update",
		"· Fix a thing",
	}

	if len(got) != len(want) {
		t.Fatalf("got %d lines %q, want %d", len(got), got, len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("line %d = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestNotesPreviewCaps(t *testing.T) {
	var b strings.Builder
	for i := 0; i < 30; i++ {
		b.WriteString("* a note\n")
	}
	got := notesPreview(b.String(), 5)
	if len(got) != 6 || got[5] != "· …" {
		t.Errorf("expected 5 lines plus an ellipsis, got %d: %q", len(got), got)
	}
}

// Notes with no headings at all should still come through.
func TestNotesPreviewWithoutHeadings(t *testing.T) {
	got := notesPreview("just one line about the release\n", 5)
	if len(got) != 1 || got[0] != "· just one line about the release" {
		t.Errorf("notesPreview = %q", got)
	}
}
