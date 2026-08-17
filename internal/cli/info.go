package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/rmpato/tuki/internal/task"
)

func newTagsCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "tags",
		Short: "list the groups you're using",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			e, err := newEnv(cmd)
			if err != nil {
				return err
			}
			d, err := e.load()
			if err != nil {
				return err
			}

			tags, byTag := task.Group(d.Tasks)
			if !e.tty {
				for _, tag := range tags {
					done, total := task.Stats(byTag[tag])
					fmt.Fprintf(e.out, "%s\t%d\t%d\n", tag, done, total)
				}
				return nil
			}

			e.println()
			for _, tag := range tags {
				done, total := task.Stats(byTag[tag])
				label := e.theme.TagStyle(tag).Render(strings.ToUpper(tag))
				count := e.theme.Hint.Render(fmt.Sprintf("%d/%d", done, total))
				e.println(spread("  "+label, count))
			}
			e.println()
			return nil
		},
	}
}

func newPathCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "path",
		Short: "print where tuki keeps your tasks",
		Long: `Print the path to tuki's task file.

Handy for backups and for peeking:

  cat "$(tuki path)"`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			e, err := newEnv(cmd)
			if err != nil {
				return err
			}
			fmt.Fprintln(e.out, e.store.Path())
			return nil
		},
	}
}
