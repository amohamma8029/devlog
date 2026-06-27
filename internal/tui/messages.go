package tui

import (
	"github.com/amo/devlog/internal/store"
	"github.com/amo/devlog/internal/todo"
)

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

type TodoLoadedMsg struct {
	Items      []todo.Item
	Error      error
	Selection  int
	SelectedID string
	Message    string
}

type ActiveSessionRefreshTickMsg struct{}

type ActiveSessionRefreshResultMsg struct {
	SessionID string
	Metadata  store.SessionFileMetadata
	Changed   bool
	Events    []store.SessionEvent
	Title     string
	Closed    bool
	Error     error
}

type CursorTickMsg struct{}

type NavigationMsg struct {
	Target  View
	Session *store.SessionRecord
}

type HandoffSavedMsg struct {
	Path  string
	Error error
}

type HandoffCopiedMsg struct {
	Error error
}

type MultiLineNoteMsg struct {
	Body      string
	IsBlocker bool
}

type pasteMsg struct {
	text string
}

type ClipboardActionMsg struct {
	Action string
}
