package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/rmpato/tuki/internal/update"
)

func newUpdateCmd() *cobra.Command {
	var (
		yes       bool
		checkOnly bool
	)

	cmd := &cobra.Command{
		Use:     "update",
		Aliases: []string{"upgrade", "self-update"},
		Short:   "check for a newer tuki and install it",
		Long: `Check GitHub for a newer tuki.

Nothing is installed without asking first. The download is checked against the
release's own sha256 checksum before it replaces anything, and the binary it's
replacing is kept aside until the new one is in place.

  tuki update           # check, then ask
  tuki update --check   # only report what's available
  tuki update --yes     # don't ask (for scripts)`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			e, err := newEnv(cmd)
			if err != nil {
				return err
			}

			current := version()
			client := &update.Client{}

			rel, err := client.Latest(cmd.Context())
			if err != nil {
				return err
			}

			if !update.IsNewer(current, rel.Version) {
				e.println("  " + e.theme.Face.Render("(o.o)") + "  " +
					e.theme.Hint.Render(fmt.Sprintf("tuki %s is the latest.", current)))
				return nil
			}

			e.println()
			e.println("  " + e.theme.Logo.Render(rel.Version) + "  " +
				e.theme.Hint.Render("is out — you have "+current))
			if rel.Notes != "" {
				e.println()
				for _, line := range notesPreview(rel.Notes, 8) {
					e.println("  " + e.theme.Aside.Render(line))
				}
			}
			e.println()

			if checkOnly {
				e.println("  " + e.theme.Hint.Render("run 'tuki update' to install it"))
				return nil
			}

			exe, err := update.ExecutablePath()
			if err != nil {
				return err
			}

			if !yes {
				ok, err := confirm(e, cmd.InOrStdin(), fmt.Sprintf("replace %s?", exe))
				if err != nil {
					return err
				}
				if !ok {
					e.println("  " + e.theme.Hint.Render("left alone"))
					return nil
				}
			}

			goos, goarch := update.Platform()
			binary, err := client.Binary(cmd.Context(), rel, goos, goarch)
			if err != nil {
				return err
			}
			if err := update.Replace(exe, binary); err != nil {
				return err
			}

			e.println("  " + e.theme.MarkDone.Render("✓") + "  " +
				e.theme.Hint.Render(fmt.Sprintf("tuki %s installed", rel.Version)))
			return nil
		},
	}

	cmd.Flags().BoolVarP(&yes, "yes", "y", false, "install without asking")
	cmd.Flags().BoolVar(&checkOnly, "check", false, "only report whether an update exists")
	return cmd
}

// notesPreview turns GitHub's markdown release notes into a few plain lines.
// Fenced code blocks and headings are dropped: what's wanted before an update
// is the gist, not the release page rendered badly in a terminal.
func notesPreview(notes string, max int) []string {
	const width = 68

	var out []string
	var inFence bool

	for _, line := range strings.Split(notes, "\n") {
		line = strings.TrimSpace(line)

		if strings.HasPrefix(line, "```") {
			inFence = !inFence
			continue
		}
		if strings.HasPrefix(line, "#") {
			// A heading before anything else is the changelog's own ("##
			// Changelog"). A heading after it starts a different section —
			// the install boilerplate we append to every release — and none
			// of that belongs in a terminal.
			if len(out) > 0 {
				break
			}
			continue
		}
		if inFence || line == "" {
			continue
		}

		// "* abcdef123: Did a thing (@someone)" reads better as "· Did a thing".
		line = strings.TrimLeft(line, "*-+ ")
		if i := strings.Index(line, ": "); i > 0 && i < 48 && !strings.Contains(line[:i], " ") {
			line = line[i+2:]
		}
		if i := strings.LastIndex(line, " (@"); i > 0 && strings.HasSuffix(line, ")") {
			line = line[:i]
		}
		if line == "" {
			continue
		}

		if len(line) > width {
			line = line[:width-1] + "…"
		}
		if len(out) == max {
			out = append(out, "· …")
			break
		}
		out = append(out, "· "+line)
	}
	return out
}
