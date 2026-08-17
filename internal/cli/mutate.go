package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/rmpato/tuki/internal/task"
	"github.com/rmpato/tuki/internal/ui"
)

func newAddCmd() *cobra.Command {
	var tag, due string

	cmd := &cobra.Command{
		Use:     "add <text>",
		Aliases: []string{"a", "new"},
		Short:   "remember something new",
		Long: `Add a task.

The tag and due date can be flags, or written inline:

  tuki add "buy milk" --tag home --due tomorrow
  tuki add "buy milk #home @tomorrow"

Due dates understand today, tomorrow, weekday names, +3d, 2w, and 2026-08-20.`,
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			e, err := newEnv(cmd)
			if err != nil {
				return err
			}
			d, err := e.load()
			if err != nil {
				return err
			}

			in := task.ParseInline(strings.Join(args, " "), e.now)
			if in.Text == "" {
				return fmt.Errorf("tuki needs something to remember")
			}

			// Explicit flags win over anything written inline.
			if tag != "" {
				in.Tag = task.NormalizeTag(tag)
			}
			if due != "" {
				parsed, err := task.ParseDue(due, e.now)
				if err != nil {
					return err
				}
				in.Due = parsed
			}

			t := d.Add(in.Text, in.Tag, in.Due)
			if err := e.store.Save(d); err != nil {
				return err
			}

			if !e.tty {
				fmt.Fprintln(e.out, plainLine(t))
				return nil
			}
			e.println(spread(
				"  "+e.mark(t)+"  "+e.text(t)+"  "+e.theme.Hint.Render(fmt.Sprintf("#%d", t.ID)),
				e.due(t),
			))
			return nil
		},
	}

	cmd.Flags().StringVarP(&tag, "tag", "t", "", "group it under work, home, hobby, misc, or your own")
	cmd.Flags().StringVarP(&due, "due", "d", "", "when it's due (today, fri, +3d, 2026-08-20)")
	return cmd
}

func newDoneCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "done <id>...",
		Aliases: []string{"do", "x"},
		Short:   "cross something off",
		Args:    cobra.MinimumNArgs(1),
		RunE:    func(cmd *cobra.Command, args []string) error { return setDone(cmd, args, true) },
	}
}

func newUndoneCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "undone <id>...",
		Aliases: []string{"undo"},
		Short:   "put something back on the list",
		Args:    cobra.MinimumNArgs(1),
		RunE:    func(cmd *cobra.Command, args []string) error { return setDone(cmd, args, false) },
	}
}

func setDone(cmd *cobra.Command, args []string, done bool) error {
	e, err := newEnv(cmd)
	if err != nil {
		return err
	}
	d, err := e.load()
	if err != nil {
		return err
	}
	ids, err := parseIDs(args)
	if err != nil {
		return err
	}

	var changed []task.Task
	for _, id := range ids {
		t, ok := d.SetDone(id, done)
		if !ok {
			return fmt.Errorf("tuki has no task %d", id)
		}
		changed = append(changed, t)
	}
	if err := e.store.Save(d); err != nil {
		return err
	}

	for _, t := range changed {
		if !e.tty {
			fmt.Fprintln(e.out, plainLine(t))
			continue
		}
		line := "  " + e.mark(t) + "  " + e.text(t)
		if done && d.Settings.Celebrate {
			line += "   " + e.theme.Cheer.Render(ui.Cheer())
		}
		e.println(line)
	}
	return nil
}

func newEditCmd() *cobra.Command {
	var tag, due string

	cmd := &cobra.Command{
		Use:   "edit <id> [text]",
		Short: "reword a task, or change its tag or due date",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			e, err := newEnv(cmd)
			if err != nil {
				return err
			}
			d, err := e.load()
			if err != nil {
				return err
			}
			ids, err := parseIDs(args[:1])
			if err != nil {
				return err
			}
			t, ok := d.Find(ids[0])
			if !ok {
				return fmt.Errorf("tuki has no task %d", ids[0])
			}

			if len(args) > 1 {
				in := task.ParseInline(strings.Join(args[1:], " "), e.now)
				if in.Text != "" {
					t.Text = in.Text
				}
				if in.Tag != "" {
					t.Tag = in.Tag
				}
				if in.Due != nil {
					t.Due = in.Due
				}
			}
			if tag != "" {
				t.Tag = task.NormalizeTag(tag)
			}
			if cmd.Flags().Changed("due") {
				parsed, err := task.ParseDue(due, e.now)
				if err != nil {
					return err
				}
				t.Due = parsed
			}

			if err := e.store.Save(d); err != nil {
				return err
			}
			if !e.tty {
				fmt.Fprintln(e.out, plainLine(*t))
				return nil
			}
			e.println(spread("  "+e.mark(*t)+"  "+e.text(*t), e.due(*t)))
			return nil
		},
	}

	cmd.Flags().StringVarP(&tag, "tag", "t", "", "move it to another group")
	cmd.Flags().StringVarP(&due, "due", "d", "", "set the due date, or 'none' to clear it")
	return cmd
}

func newRemoveCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "rm <id>...",
		Aliases: []string{"remove", "del"},
		Short:   "forget a task entirely",
		Args:    cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			e, err := newEnv(cmd)
			if err != nil {
				return err
			}
			d, err := e.load()
			if err != nil {
				return err
			}
			ids, err := parseIDs(args)
			if err != nil {
				return err
			}

			var gone []task.Task
			for _, id := range ids {
				t, _, ok := d.Remove(id)
				if !ok {
					return fmt.Errorf("tuki has no task %d", id)
				}
				gone = append(gone, t)
			}
			if err := e.store.Save(d); err != nil {
				return err
			}

			if !e.tty {
				return nil
			}
			for _, t := range gone {
				e.println("  " + e.theme.Hint.Render("forgot") + "  " + e.theme.Completed.Render(t.Text))
			}
			return nil
		},
	}
}

func newClearCmd() *cobra.Command {
	var yes, all bool

	cmd := &cobra.Command{
		Use:   "clear",
		Short: "remove completed tasks",
		Long: `Remove every completed task.

With --all, remove everything, finished or not. Both ask first when there's a
terminal to ask; in a script, pass --yes.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			e, err := newEnv(cmd)
			if err != nil {
				return err
			}
			d, err := e.load()
			if err != nil {
				return err
			}

			n := len(d.Tasks)
			if !all {
				n, _ = task.Stats(d.Tasks)
			}
			if n == 0 {
				if e.tty {
					e.println("  " + e.theme.Hint.Render("nothing to clear"))
				}
				return nil
			}

			if !yes {
				what := plural(n, "completed task", "completed tasks")
				if all {
					what = plural(n, "task", "tasks")
				}
				ok, err := confirm(e, cmd.InOrStdin(), fmt.Sprintf("forget %s?", what))
				if err != nil {
					return err
				}
				if !ok {
					return nil
				}
			}

			if all {
				d.Tasks = nil
			} else {
				n = d.ClearDone()
			}
			if err := e.store.Save(d); err != nil {
				return err
			}
			if e.tty {
				e.println("  " + e.theme.Hint.Render(fmt.Sprintf("cleared %s", plural(n, "task", "tasks"))))
			}
			return nil
		},
	}

	cmd.Flags().BoolVarP(&yes, "yes", "y", false, "don't ask")
	cmd.Flags().BoolVar(&all, "all", false, "clear unfinished tasks too")
	return cmd
}
