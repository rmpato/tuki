package tui

import (
	"fmt"
	"io"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/rmpato/tuki/internal/store"
	"github.com/rmpato/tuki/internal/task"
	"github.com/rmpato/tuki/internal/ui"
)

// Run opens tuki's full-screen interface and returns once you leave it.
func Run(s *store.Store, out io.Writer) error {
	d, err := s.Load()
	if err != nil {
		return err
	}

	m := New(s, d)
	final, err := tea.NewProgram(m).Run()
	if err != nil {
		return err
	}

	if fm, ok := final.(*Model); ok {
		if fm.saveErr != nil {
			return fm.saveErr
		}
		farewell(out, fm)
	}
	return nil
}

// farewell leaves a single quiet line in your scrollback, so closing tuki
// still tells you where you stand.
func farewell(out io.Writer, m *Model) {
	done, total := task.Stats(m.data.Tasks)
	if total == 0 {
		return
	}
	face := ui.FaceIdle
	if done == total {
		face = ui.FaceCheer
	}
	line := lipgloss.NewStyle().Foreground(m.theme.Brand).Render(face) +
		lipgloss.NewStyle().Foreground(m.theme.Muted).Render(fmt.Sprintf("  %d/%d done", done, total))
	fmt.Fprintln(out, line)
}
