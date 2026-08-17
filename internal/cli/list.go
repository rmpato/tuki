package cli

import (
	"fmt"
	"strings"
	"time"

	"charm.land/lipgloss/v2"
	"github.com/spf13/cobra"

	"github.com/rmpato/tuki/internal/store"
	"github.com/rmpato/tuki/internal/task"
	"github.com/rmpato/tuki/internal/ui"
)

// listWidth is the column tuki prints into. Same narrow column as the TUI.
const listWidth = 64

type listOptions struct {
	tag      string
	onlyDone bool
	onlyTodo bool
	asJSON   bool
	plain    bool
}

func newListCmd() *cobra.Command {
	var opts listOptions

	cmd := &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "show your tasks",
		Long: `Show your tasks, grouped by tag.

When the output is a terminal you get the pretty version. When it's a pipe you
get tab-separated columns instead:

  id	done|todo	tag	due	text`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			e, err := newEnv(cmd)
			if err != nil {
				return err
			}
			return runList(e, opts)
		},
	}

	cmd.Flags().StringVarP(&opts.tag, "tag", "t", "", "only show one group")
	cmd.Flags().BoolVar(&opts.onlyDone, "done", false, "only completed tasks")
	cmd.Flags().BoolVar(&opts.onlyTodo, "todo", false, "only unfinished tasks")
	cmd.Flags().BoolVar(&opts.asJSON, "json", false, "print JSON")
	cmd.Flags().BoolVar(&opts.plain, "plain", false, "force the script-friendly format")

	return cmd
}

func runList(e *env, opts listOptions) error {
	d, err := e.load()
	if err != nil {
		return err
	}

	tasks := filterTasks(d.Tasks, opts)

	if opts.asJSON {
		return writeJSON(e.out, tasks)
	}
	if opts.plain || !e.tty {
		for _, t := range tasks {
			fmt.Fprintln(e.out, plainLine(t))
		}
		return nil
	}

	printPretty(e, d, tasks)
	return nil
}

func filterTasks(tasks []task.Task, opts listOptions) []task.Task {
	want := task.NormalizeTag(opts.tag)
	out := make([]task.Task, 0, len(tasks))
	for _, t := range tasks {
		if opts.tag != "" && task.NormalizeTag(t.Tag) != want {
			continue
		}
		if opts.onlyDone && !t.Done {
			continue
		}
		if opts.onlyTodo && t.Done {
			continue
		}
		out = append(out, t)
	}
	return out
}

// printPretty is the version a person reads: grouped, spaced, quietly
// coloured, and with the task numbers you need for `tuki done`.
func printPretty(e *env, d *store.Data, tasks []task.Task) {
	done, total := task.Stats(tasks)

	e.println()
	head := e.theme.Face.Render(headFace(d, tasks, e.now)) + "  " + e.theme.Logo.Render("tuki")
	tail := ""
	if total > 0 {
		tail = e.theme.Count.Render(fmt.Sprintf("%d/%d done", done, total))
	}
	e.println(spread(head, tail))

	if total == 0 {
		e.println()
		for _, line := range strings.Split(ui.Blob(ui.FaceSleepy), "\n") {
			e.println("  " + e.theme.Face.Render(line))
		}
		e.println()
		e.println("  " + e.theme.Aside.Render(ui.Empty()))
		e.println("  " + e.theme.Hint.Render(`try: tuki add "buy milk"`))
		e.println()
		return
	}

	width := idWidth(tasks)
	tags, byTag := task.Group(tasks)

	for _, tag := range tags {
		group := byTag[tag]
		gdone, gtotal := task.Stats(group)

		label := e.theme.TagStyle(tag).Render(strings.ToUpper(tag))
		count := e.theme.Hint.Render(fmt.Sprintf("%d/%d", gdone, gtotal))
		fill := listWidth - lipgloss.Width(label) - lipgloss.Width(count) - 4
		e.println()
		e.println("  " + label + " " + e.rule(fill) + " " + count)
		e.println()

		for _, t := range group {
			left := "   " + e.id(t, width) + "  " + e.mark(t) + "  " + e.text(t)
			e.println(spread(left, e.due(t)))
		}
	}

	if d.Settings.Judge {
		if line := ui.Judge(task.CountOverdue(tasks, e.now)); line != "" {
			e.println()
			e.println("  " + e.theme.Aside.Render(line))
		}
	} else if done == total {
		e.println()
		e.println("  " + e.theme.Aside.Render(ui.AllDone()))
	}
	e.println()
}

// headFace picks tuki's expression for a one-shot listing.
func headFace(d *store.Data, tasks []task.Task, now time.Time) string {
	done, total := task.Stats(tasks)
	switch {
	case total == 0:
		return ui.FaceSleepy
	case done == total:
		return ui.FaceCheer
	case d.Settings.Judge && task.CountOverdue(tasks, now) >= 6:
		return ui.FaceSide
	default:
		return ui.FaceIdle
	}
}

// spread pushes right up against the right edge of tuki's column.
func spread(left, right string) string {
	if right == "" {
		return left
	}
	gap := listWidth - lipgloss.Width(left) - lipgloss.Width(right)
	if gap < 1 {
		gap = 1
	}
	return left + strings.Repeat(" ", gap) + right
}
