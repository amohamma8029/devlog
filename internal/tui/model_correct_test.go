package tui

import (
	"testing"

	internalconfig "github.com/amo/devlog/internal/config"
	"github.com/amo/devlog/internal/store"
	tea "github.com/charmbracelet/bubbletea"
)

func TestModelHandleEditEventOpensComposerPreFilled(t *testing.T) {
	s := &store.Store{}
	m := NewModel(s, "/tmp")

	m.ActiveSession = &store.SessionRecord{Session: store.Session{ID: "2026-01-15T143000Z", Author: "Test", Branch: "feat/test", Status: "active"}}
	m.Events = []store.SessionEvent{
		{Type: "Start", Body: "title"},
		{Type: "Note", Time: testEventTime(14, 30), Body: "original note body"},
	}
	m.CurrentView = ActiveSession
	m.SelectedEvent = 0
	m.refreshActiveTimelineCache()

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'e'}})
	if cmd != nil {
		t.Fatalf("expected no cmd, got cmd")
	}

	um := updated.(Model)
	if !um.Palette.Open {
		t.Fatal("palette should be open after pressing e")
	}
	if !um.Palette.MultiLine {
		t.Fatal("palette should be in multi-line mode")
	}
	if len(um.Palette.MultiLineLines) == 0 || um.Palette.MultiLineLines[0] != "original note body" {
		t.Fatalf("composer should be pre-filled, got %v", um.Palette.MultiLineLines)
	}
	if um.EditingEvent != 1 {
		t.Fatalf("EditingEvent = %d, want 1 (full index in Events)", um.EditingEvent)
	}
}

func TestModelHandleCorrectionSubmit(t *testing.T) {
	s := &store.Store{}
	m := NewModel(s, "/tmp")

	m.ActiveSession = &store.SessionRecord{Session: store.Session{ID: "2026-01-15T143000Z", Author: "Test", Branch: "feat/test", Status: "active"}}
	m.Events = []store.SessionEvent{
		{Type: "Start", Body: "title"},
		{Type: "Note", Time: testEventTime(14, 30), Body: "original"},
	}
	m.CurrentView = ActiveSession
	m.SelectedEvent = 0
	m.EditingEvent = 1 // index 1 in m.Events (0 = Start)
	m.Palette.Open = true
	m.Palette.MultiLine = true
	m.Palette.MultiLineLines = []string{"original"}
	m.Palette.MultiLineCursorRow = 0
	m.Palette.MultiLineCursorCol = len("original")

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("expected MultiLineNoteMsg cmd, got nil")
	}

	msg := cmd()
	updated2, _ := updated.Update(msg)
	um := updated2.(Model)
	if um.EditingEvent != -1 {
		t.Errorf("EditingEvent should be cleared after submit, got %d", um.EditingEvent)
	}
	if um.SelectedEvent != -1 {
		t.Errorf("SelectedEvent should be cleared after submit, got %d", um.SelectedEvent)
	}
}

func TestModelHandleEditEventNoSelectionNoPalette(t *testing.T) {
	s := &store.Store{}
	m := NewModelWithConfig(s, "/tmp", internalconfig.Default())

	m.ActiveSession = &store.SessionRecord{Session: store.Session{ID: "2026-01-15T143000Z", Author: "Test", Branch: "feat/test", Status: "active"}}
	m.Events = []store.SessionEvent{
		{Type: "Start", Body: "title"},
		{Type: "Note", Time: testEventTime(14, 30), Body: "note"},
	}
	m.CurrentView = ActiveSession
	m.SelectedEvent = -1

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'e'}})
	if cmd != nil {
		t.Fatalf("expected no cmd when no selection, got cmd")
	}
	um := updated.(Model)
	if um.Palette.Open {
		t.Fatal("palette should not open when no event selected")
	}
}

func TestHandleEditEventFullIndexMapping(t *testing.T) {
	s := &store.Store{}
	m := NewModel(s, "/tmp")
	m.ActiveSession = &store.SessionRecord{Session: store.Session{ID: "test", Author: "Test", Branch: "feat/test", Status: "active"}}
	m.Events = []store.SessionEvent{
		{Type: "Start", Body: "test"},
		{Type: "Note", Time: testEventTime(20, 01), Body: "Note one"},
		{Type: "Note", Time: testEventTime(20, 02), Body: "Note two"},
		{Type: "Blocker", Time: testEventTime(20, 03), Body: "Blocker three"},
		{Type: "Note", Time: testEventTime(20, 04), Body: "Note four"},
		{Type: "Blocker", Time: testEventTime(20, 05), Body: "Blocker five"},
	}
	m.CurrentView = ActiveSession

	tests := []struct {
		selected int
		wantIdx  int
	}{
		{0, 1}, // first visible → Note one
		{1, 2}, // second visible → Note two
		{2, 3}, // third visible → Blocker three
		{3, 4}, // fourth visible → Note four
		{4, 5}, // fifth visible → Blocker five
	}

	for _, tt := range tests {
		m.SelectedEvent = tt.selected
		updated, _ := m.handleEditEvent()
		um := updated.(Model)
		if um.EditingEvent != tt.wantIdx {
			t.Errorf("visible %d → fullIdx %d, want %d", tt.selected, um.EditingEvent, tt.wantIdx)
		}
	}
}
