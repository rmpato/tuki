// Package ui holds everything about how tuki looks and what tuki says: the
// palette, the styles, the little face, and the lines it occasionally offers.
package ui

import (
	"hash/fnv"
	"image/color"
	"os"
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/rmpato/tuki/internal/task"
)

// Theme is a resolved palette plus the styles built from it. Build one with
// New and pass it around; it is cheap to copy.
type Theme struct {
	IsDark bool

	// Palette.
	Brand   color.Color // tuki's own color
	Text    color.Color // normal task text
	Muted   color.Color // secondary information
	Faint   color.Color // barely-there details, rules, completed text
	Overdue color.Color // gentle alarm
	Gold    color.Color // celebrations

	// Text styles.
	Logo      lipgloss.Style
	Face      lipgloss.Style
	Section   lipgloss.Style
	Rule      lipgloss.Style
	Count     lipgloss.Style
	Pending   lipgloss.Style
	Completed lipgloss.Style
	Selected  lipgloss.Style
	Mark      lipgloss.Style
	MarkDone  lipgloss.Style
	Cursor    lipgloss.Style
	Due       lipgloss.Style
	DueSoon   lipgloss.Style
	DueLate   lipgloss.Style
	Hint      lipgloss.Style
	Aside     lipgloss.Style
	Cheer     lipgloss.Style
	Prompt    lipgloss.Style
	Warn      lipgloss.Style

	tagColors map[string]color.Color
	extra     []color.Color
}

// New builds the theme for a light or dark terminal.
func New(isDark bool) Theme {
	c := lipgloss.LightDark(isDark)

	t := Theme{
		IsDark: isDark,

		Brand:   c(lipgloss.Color("#B4761A"), lipgloss.Color("#F5C26B")),
		Text:    c(lipgloss.Color("#1F2328"), lipgloss.Color("#DCDEE0")),
		Muted:   c(lipgloss.Color("#6B7075"), lipgloss.Color("#9198A0")),
		Faint:   c(lipgloss.Color("#B5BAC0"), lipgloss.Color("#5A6067")),
		Overdue: c(lipgloss.Color("#B3352F"), lipgloss.Color("#F08C8C")),
		Gold:    c(lipgloss.Color("#9A6B00"), lipgloss.Color("#FFD479")),
	}

	t.tagColors = map[string]color.Color{
		task.TagWork:  c(lipgloss.Color("#2A5DB0"), lipgloss.Color("#7FA9F0")),
		task.TagHome:  c(lipgloss.Color("#1F7A5A"), lipgloss.Color("#79D3AE")),
		task.TagHobby: c(lipgloss.Color("#9B3D8C"), lipgloss.Color("#E5A3D8")),
		task.TagMisc:  t.Muted,
	}
	// Colors for tags tuki has never met before.
	t.extra = []color.Color{
		c(lipgloss.Color("#A05A1E"), lipgloss.Color("#E8B07A")),
		c(lipgloss.Color("#1C7A78"), lipgloss.Color("#7ED6D3")),
		c(lipgloss.Color("#7A4FBF"), lipgloss.Color("#BFA3F0")),
		c(lipgloss.Color("#8A6A00"), lipgloss.Color("#DCC96B")),
	}

	t.Logo = lipgloss.NewStyle().Foreground(t.Brand).Bold(true)
	t.Face = lipgloss.NewStyle().Foreground(t.Brand)
	t.Section = lipgloss.NewStyle().Bold(true)
	t.Rule = lipgloss.NewStyle().Foreground(t.Faint)
	t.Count = lipgloss.NewStyle().Foreground(t.Muted)
	t.Pending = lipgloss.NewStyle().Foreground(t.Text)
	// StrikethroughSpaces keeps the line continuous across words, and lets
	// Lip Gloss emit one escape sequence for the string instead of one per rune.
	t.Completed = lipgloss.NewStyle().Foreground(t.Faint).Strikethrough(true).StrikethroughSpaces(true)
	t.Selected = lipgloss.NewStyle().Foreground(t.Brand).Bold(true)
	t.Mark = lipgloss.NewStyle().Foreground(t.Faint)
	t.MarkDone = lipgloss.NewStyle().Foreground(t.Brand)
	t.Cursor = lipgloss.NewStyle().Foreground(t.Brand).Bold(true)
	t.Due = lipgloss.NewStyle().Foreground(t.Muted)
	t.DueSoon = lipgloss.NewStyle().Foreground(t.Gold)
	t.DueLate = lipgloss.NewStyle().Foreground(t.Overdue)
	t.Hint = lipgloss.NewStyle().Foreground(t.Faint)
	t.Aside = lipgloss.NewStyle().Foreground(t.Muted).Italic(true)
	t.Cheer = lipgloss.NewStyle().Foreground(t.Gold)
	t.Prompt = lipgloss.NewStyle().Foreground(t.Brand)
	t.Warn = lipgloss.NewStyle().Foreground(t.Overdue)

	return t
}

// Tag returns the color for a tag, inventing a stable one for tags tuki
// doesn't already know.
func (t Theme) Tag(name string) color.Color {
	name = task.NormalizeTag(name)
	if c, ok := t.tagColors[name]; ok {
		return c
	}
	h := fnv.New32a()
	_, _ = h.Write([]byte(name))
	return t.extra[int(h.Sum32())%len(t.extra)]
}

// TagStyle is the section-heading style for a tag.
func (t Theme) TagStyle(name string) lipgloss.Style {
	return t.Section.Foreground(t.Tag(name))
}

// Fade blends from tuki's brand color back to a resting color. It drives the
// small glow a task gives off right after you complete it.
func (t Theme) Fade(rest color.Color, step, steps int) color.Color {
	if steps < 2 {
		return rest
	}
	ramp := lipgloss.Blend1D(steps, t.Gold, rest)
	if step < 0 {
		step = 0
	}
	if step >= len(ramp) {
		step = len(ramp) - 1
	}
	return ramp[step]
}

// PrefersDark guesses whether the terminal has a dark background without
// asking it, which keeps `tuki list` instant. TUKI_THEME wins; then the
// COLORFGBG convention that many terminals set; otherwise dark, because most
// terminals are.
//
// The full-screen TUI doesn't use this: it asks the terminal properly and
// asynchronously via Bubble Tea.
func PrefersDark() bool {
	switch strings.ToLower(os.Getenv("TUKI_THEME")) {
	case "light":
		return false
	case "dark":
		return true
	}
	// COLORFGBG looks like "15;0" — foreground;background, as ANSI indices.
	if v := os.Getenv("COLORFGBG"); v != "" {
		parts := strings.Split(v, ";")
		switch parts[len(parts)-1] {
		case "7", "15":
			return false
		case "0", "8":
			return true
		}
	}
	return true
}
