package tui

import "github.com/amo/devlog/internal/store"

type MouseActionMsg struct {
	Action MouseAction
	X      int
	Y      int
}

type StoreResultMsg struct {
	Error error
}

type PaletteToggleMsg struct {
	Open bool
}

type HandoffGeneratedMsg struct {
	Content string
	Error   error
}

type CommandExecutedMsg struct {
	Input string
}

type CommandErrorMsg struct {
	Error error
}

type ActiveSessionLoadedMsg struct {
	Session *store.SessionRecord
	Events  []store.SessionEvent
	Title   string
}

type CursorTickMsg struct{}

type NavigationMsg struct {
	Target  View
	Session *store.SessionRecord
}
