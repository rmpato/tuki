package cli

import (
	"context"
	"runtime/debug"

	"github.com/charmbracelet/fang"
	"github.com/spf13/cobra"

	"github.com/rmpato/tuki/internal/tui"
)

// Version is stamped at build time with -ldflags. When it isn't, tuki asks the
// Go build info, which is populated by `go install`.
var Version = ""

func version() string {
	if Version != "" {
		return Version
	}
	if info, ok := debug.ReadBuildInfo(); ok && info.Main.Version != "" && info.Main.Version != "(devel)" {
		return info.Main.Version
	}
	return "dev"
}

// NewRoot builds the whole command tree.
func NewRoot() *cobra.Command {
	root := &cobra.Command{
		Use:   "tuki",
		Short: "a tiny terminal companion that remembers things for you",
		Long: `tuki keeps the things you need to do.

Running tuki with no arguments opens the full-screen interface. Everything
else is here so tuki works in scripts and pipes too.

  tuki add "buy milk" --tag home --due tomorrow
  tuki list --tag work
  tuki done 12
  tuki clear

Tasks live in one local JSON file. Nothing is sent anywhere.`,
		Args:          cobra.NoArgs,
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			e, err := newEnv(cmd)
			if err != nil {
				return err
			}
			// Piping `tuki` somewhere should behave like `tuki list`, not open
			// an interface nobody can see.
			if !e.tty {
				return runList(e, listOptions{})
			}
			return tui.Run(e.store, e.out)
		},
	}

	root.PersistentFlags().String("file", "", "use a specific tasks file instead of the default")

	root.AddCommand(
		newAddCmd(),
		newListCmd(),
		newDoneCmd(),
		newUndoneCmd(),
		newEditCmd(),
		newRemoveCmd(),
		newClearCmd(),
		newTagsCmd(),
		newPathCmd(),
	)
	return root
}

// Execute runs tuki, wrapped in fang for the styled help, errors, manpage, and
// shell completions.
func Execute(ctx context.Context) error {
	return fang.Execute(ctx, NewRoot(), fang.WithVersion(version()))
}
