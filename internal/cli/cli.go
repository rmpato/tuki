// Package cli is tuki's command-line half: the part that works in scripts,
// pipes, and one-liners, without ever opening the full-screen interface.
package cli

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/colorprofile"
	"github.com/charmbracelet/x/term"
	"github.com/spf13/cobra"

	"github.com/rmpato/tuki/internal/store"
	"github.com/rmpato/tuki/internal/task"
	"github.com/rmpato/tuki/internal/ui"
)

// env carries everything a command needs: where the tasks live, where output
// goes, and whether anyone is actually looking at it.
type env struct {
	store *store.Store
	out   io.Writer
	w     io.Writer // colour-aware wrapper around out
	theme ui.Theme
	tty   bool
	now   time.Time
}

// newEnv resolves the store and works out how much decoration is appropriate.
func newEnv(cmd *cobra.Command) (*env, error) {
	path, _ := cmd.Flags().GetString("file")

	var s *store.Store
	if path != "" {
		s = store.New(path)
	} else {
		var err error
		if s, err = store.Default(); err != nil {
			return nil, err
		}
	}

	out := cmd.OutOrStdout()
	return &env{
		store: s,
		out:   out,
		w:     colorprofile.NewWriter(out, os.Environ()),
		theme: ui.New(ui.PrefersDark()),
		tty:   isTerminal(out),
		now:   time.Now(),
	}, nil
}

func isTerminal(w io.Writer) bool {
	f, ok := w.(*os.File)
	if !ok {
		return false
	}
	return term.IsTerminal(f.Fd())
}

func (e *env) load() (*store.Data, error) { return e.store.Load() }

func (e *env) printf(format string, a ...any) {
	fmt.Fprintf(e.w, format, a...)
}

func (e *env) println(a ...any) {
	fmt.Fprintln(e.w, a...)
}

// parseIDs turns command arguments into task IDs, complaining usefully.
func parseIDs(args []string) ([]int, error) {
	ids := make([]int, 0, len(args))
	for _, a := range args {
		n, err := strconv.Atoi(strings.TrimSpace(a))
		if err != nil {
			return nil, fmt.Errorf("%q is not a task number", a)
		}
		ids = append(ids, n)
	}
	return ids, nil
}

// mark renders the ○ / ✓ in front of a task.
func (e *env) mark(t task.Task) string {
	if t.Done {
		return e.theme.MarkDone.Render("✓")
	}
	return e.theme.Mark.Render("○")
}

// text renders a task's words, crossed out when it's done.
func (e *env) text(t task.Task) string {
	if t.Done {
		return e.theme.Completed.Render(t.Text)
	}
	return e.theme.Pending.Render(t.Text)
}

// due renders the small date chip, or nothing.
func (e *env) due(t task.Task) string {
	if t.Due == nil {
		return ""
	}
	label := task.HumanDue(*t.Due, e.now)
	switch {
	case t.Done:
		return e.theme.Hint.Render(label)
	case t.Overdue(e.now):
		return e.theme.DueLate.Render(label)
	case t.DueToday(e.now):
		return e.theme.DueSoon.Render(label)
	default:
		return e.theme.Due.Render(label)
	}
}

// id renders a task number, padded so columns line up.
func (e *env) id(t task.Task, width int) string {
	return e.theme.Hint.Render(fmt.Sprintf("%*d", width, t.ID))
}

// plainLine is the scripting format: tab-separated, stable, greppable.
//
//	id	done|todo	tag	2026-08-20	text
func plainLine(t task.Task) string {
	state := "todo"
	if t.Done {
		state = "done"
	}
	dueStr := ""
	if t.Due != nil {
		dueStr = t.Due.Format("2006-01-02")
	}
	return strings.Join([]string{
		strconv.Itoa(t.ID), state, task.NormalizeTag(t.Tag), dueStr, t.Text,
	}, "\t")
}

func writeJSON(w io.Writer, v any) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

// confirm asks a yes/no question, but only when there's a human to ask.
func confirm(e *env, in io.Reader, question string) (bool, error) {
	if !e.tty {
		return false, fmt.Errorf("%s — rerun with --yes", question)
	}
	e.printf("%s %s ", e.theme.Prompt.Render(question), e.theme.Hint.Render("[y/N]"))
	line, err := bufio.NewReader(in).ReadString('\n')
	if err != nil && line == "" {
		return false, nil
	}
	answer := strings.ToLower(strings.TrimSpace(line))
	return answer == "y" || answer == "yes", nil
}

func plural(n int, one, many string) string {
	if n == 1 {
		return fmt.Sprintf("%d %s", n, one)
	}
	return fmt.Sprintf("%d %s", n, many)
}

// idWidth is how wide the ID column needs to be for a given set of tasks.
func idWidth(tasks []task.Task) int {
	w := 1
	for _, t := range tasks {
		if n := len(strconv.Itoa(t.ID)); n > w {
			w = n
		}
	}
	return w
}

// rule draws the thin line in a group heading.
func (e *env) rule(width int) string {
	if width < 1 {
		width = 1
	}
	return e.theme.Rule.Render(strings.Repeat("─", width))
}
