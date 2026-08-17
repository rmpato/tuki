package tui

import "charm.land/bubbles/v2/key"

// KeyMap is every key tuki listens for. It satisfies help.KeyMap so the footer
// and the '?' screen are generated from exactly these bindings — they can't
// drift out of sync with what actually works.
type KeyMap struct {
	Up     key.Binding
	Down   key.Binding
	Top    key.Binding
	Bottom key.Binding

	Toggle key.Binding
	Add    key.Binding
	Edit   key.Binding
	Delete key.Binding
	Undo   key.Binding
	Tag    key.Binding
	Due    key.Binding

	Filter key.Binding
	Search key.Binding
	Clear  key.Binding

	Help   key.Binding
	Quit   key.Binding
	Cancel key.Binding
	Submit key.Binding
}

// DefaultKeys is keyboard-first and vim-shaped, with arrows for everyone else.
func DefaultKeys() KeyMap {
	return KeyMap{
		Up:     key.NewBinding(key.WithKeys("up", "k", "ctrl+p"), key.WithHelp("↑/k", "up")),
		Down:   key.NewBinding(key.WithKeys("down", "j", "ctrl+n"), key.WithHelp("↓/j", "down")),
		Top:    key.NewBinding(key.WithKeys("g", "home"), key.WithHelp("g", "top")),
		Bottom: key.NewBinding(key.WithKeys("G", "end"), key.WithHelp("G", "bottom")),

		Toggle: key.NewBinding(key.WithKeys("space", "enter", "x"), key.WithHelp("space", "done")),
		Add:    key.NewBinding(key.WithKeys("a", "n", "+"), key.WithHelp("a", "add")),
		Edit:   key.NewBinding(key.WithKeys("e"), key.WithHelp("e", "edit")),
		Delete: key.NewBinding(key.WithKeys("d", "delete"), key.WithHelp("d", "delete")),
		Undo:   key.NewBinding(key.WithKeys("u"), key.WithHelp("u", "undo")),
		Tag:    key.NewBinding(key.WithKeys("t"), key.WithHelp("t", "cycle tag")),
		Due:    key.NewBinding(key.WithKeys("D"), key.WithHelp("D", "set due")),

		Filter: key.NewBinding(key.WithKeys("tab", "left", "right", "h", "l"), key.WithHelp("tab", "filter group")),
		Search: key.NewBinding(key.WithKeys("/"), key.WithHelp("/", "search")),
		Clear:  key.NewBinding(key.WithKeys("c"), key.WithHelp("c", "clear done")),

		Help:   key.NewBinding(key.WithKeys("?"), key.WithHelp("?", "keys")),
		Quit:   key.NewBinding(key.WithKeys("q", "ctrl+c"), key.WithHelp("q", "quit")),
		Cancel: key.NewBinding(key.WithKeys("esc", "ctrl+c")),
		Submit: key.NewBinding(key.WithKeys("enter")),
	}
}

// ShortHelp is the one-line footer.
func (k KeyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Up, k.Down, k.Toggle, k.Add, k.Delete, k.Help, k.Quit}
}

// FullHelp is the '?' view.
func (k KeyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.Up, k.Down, k.Top, k.Bottom},
		{k.Toggle, k.Add, k.Edit, k.Delete, k.Undo},
		{k.Tag, k.Due, k.Filter, k.Search, k.Clear},
		{k.Help, k.Quit},
	}
}
