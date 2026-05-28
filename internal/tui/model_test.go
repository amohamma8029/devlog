package tui

import (
	"strings"
	"testing"
	"time"

	"github.com/amo/devlog/internal/store"
	tea "github.com/charmbracelet/bubbletea"
)

func TestModelSatisfiesTeaModel(t *testing.T) {
	var m tea.Model = Model{}
	_ = m
}

func TestNewModel(t *testing.T) {
	s := &store.Store{}
	m := NewModel(s, "/tmp/test")
	if m.CurrentView != SessionList {
		t.Errorf("NewModel CurrentView = %v, want SessionList", m.CurrentView)
	}
	if m.Store != s {
		t.Error("NewModel Store not set")
	}
	if m.Root != "/tmp/test" {
		t.Errorf("NewModel Root = %s, want /tmp/test", m.Root)
	}
}

func TestModelInit(t *testing.T) {
	s := &store.Store{}
	m := NewModel(s, "/tmp")
	cmd := m.Init()
	if cmd == nil {
		t.Error("Init() returned nil cmd; expected a command to load active session")
	}
}

func TestModelUpdateQuitOnEsc(t *testing.T) {
	s := &store.Store{}
	m := NewModel(s, "/tmp")
	msg := tea.KeyMsg{Type: tea.KeyEsc}
	_, cmd := m.Update(msg)
	if cmd == nil {
		t.Fatal("expected quit cmd on Esc")
	}
}

func TestModelUpdateQuitOnCtrlC(t *testing.T) {
	s := &store.Store{}
	m := NewModel(s, "/tmp")
	msg := tea.KeyMsg{Type: tea.KeyCtrlC}
	_, cmd := m.Update(msg)
	if cmd == nil {
		t.Fatal("expected quit cmd on Ctrl+C")
	}
}

func TestModelUpdateNoQuitWhenPaletteOpen(t *testing.T) {
	p := NewCommandPalette()
	p.Open = true
	m := Model{
		Palette: &p,
	}
	msg := tea.KeyMsg{Type: tea.KeyEsc}
	_, cmd := m.Update(msg)
	if cmd != nil {
		t.Fatal("expected no quit cmd when palette is open")
	}
}

func TestModelUpdateWindowSize(t *testing.T) {
	s := &store.Store{}
	m := NewModel(s, "/tmp")
	msg := tea.WindowSizeMsg{Width: 120, Height: 40}
	nm, _ := m.Update(msg)
	updated, ok := nm.(Model)
	if !ok {
		t.Fatal("expected Model from Update")
	}
	if updated.Width != 120 {
		t.Errorf("Width = %d, want 120", updated.Width)
	}
	if updated.Height != 40 {
		t.Errorf("Height = %d, want 40", updated.Height)
	}
}

func TestModelUpdateNavigationMsgReturnsLoadCommand(t *testing.T) {
	s, root := newTestStore(t)
	const sessionID = "2026-01-15T140000Z"
	writeTestSession(t, s, sessionID, "feat/nav", "Alice", "Implement session navigation", time.Date(2026, 1, 15, 14, 0, 0, 0, time.UTC))
	rec, err := s.GetSession(sessionID)
	if err != nil {
		t.Fatalf("GetSession failed: %v", err)
	}

	m := NewModel(s, root)
	updatedModel, cmd := m.Update(NavigationMsg{Target: ActiveSession, Session: &rec})
	updated, ok := updatedModel.(Model)
	if !ok {
		t.Fatalf("expected Model from Update, got %T", updatedModel)
	}
	if updated.CurrentView != ActiveSession {
		t.Fatalf("CurrentView = %v, want ActiveSession", updated.CurrentView)
	}
	if updated.ActiveSession == nil || updated.ActiveSession.ID != sessionID {
		t.Fatalf("ActiveSession = %#v, want %s", updated.ActiveSession, sessionID)
	}
	if cmd == nil {
		t.Fatal("NavigationMsg with a selected session should return a load command")
	}
}

func TestModelUpdateNavigationMsgLoadCommandParsesEvents(t *testing.T) {
	s, root := newTestStore(t)
	const sessionID = "2026-01-15T140000Z"
	writeTestSession(t, s, sessionID, "feat/nav", "Alice", "Implement session navigation", time.Date(2026, 1, 15, 14, 0, 0, 0, time.UTC))
	if err := s.AppendEvent(sessionID, "Note", "Loaded selected session timeline"); err != nil {
		t.Fatalf("AppendEvent failed: %v", err)
	}
	rec, err := s.GetSession(sessionID)
	if err != nil {
		t.Fatalf("GetSession failed: %v", err)
	}

	m := NewModel(s, root)
	_, cmd := m.Update(NavigationMsg{Target: ActiveSession, Session: &rec})
	if cmd == nil {
		t.Fatal("NavigationMsg with a selected session should return a load command")
	}

	loadMsg := cmd()
	loaded, ok := loadMsg.(ActiveSessionLoadedMsg)
	if !ok {
		t.Fatalf("navigation load command returned %T, want ActiveSessionLoadedMsg", loadMsg)
	}
	if loaded.Session == nil || loaded.Session.ID != sessionID {
		t.Fatalf("loaded Session = %#v, want %s", loaded.Session, sessionID)
	}
	if loaded.Title != "Implement session navigation" {
		t.Fatalf("Title = %q, want start message", loaded.Title)
	}
	if len(loaded.Events) != 2 {
		t.Fatalf("Events length = %d, want 2", len(loaded.Events))
	}
	if loaded.Events[1].Type != "Note" || loaded.Events[1].Body != "Loaded selected session timeline" {
		t.Fatalf("second event = %#v, want parsed note", loaded.Events[1])
	}
}

func TestModelSessionListEnterLoadsClosedSessionView(t *testing.T) {
	s, root := newTestStore(t)
	const sessionID = "2026-01-15T140000Z"
	writeTestSession(t, s, sessionID, "feat/nav", "Alice", "Review closed session", time.Date(2026, 1, 15, 14, 0, 0, 0, time.UTC))
	if err := s.AppendEvent(sessionID, "Note", "Closed session details should be visible"); err != nil {
		t.Fatalf("AppendEvent failed: %v", err)
	}
	closeTestSession(t, s, sessionID)

	m := NewModel(s, root)
	m.Width = 80
	m.Height = 24
	list := loadTestModel(t, s, root)
	m.SessionList = list
	m.CurrentView = SessionList

	updatedModel, navCmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if navCmd == nil {
		t.Fatal("session list Enter should return a navigation command")
	}
	updated, ok := updatedModel.(Model)
	if !ok {
		t.Fatalf("expected Model from Update, got %T", updatedModel)
	}

	updatedModel, loadCmd := updated.Update(navCmd())
	if loadCmd == nil {
		t.Fatal("NavigationMsg should return a session load command")
	}
	updated, ok = updatedModel.(Model)
	if !ok {
		t.Fatalf("expected Model after NavigationMsg, got %T", updatedModel)
	}

	updatedModel, _ = updated.Update(loadCmd())
	updated, ok = updatedModel.(Model)
	if !ok {
		t.Fatalf("expected Model after ActiveSessionLoadedMsg, got %T", updatedModel)
	}

	v := updated.View()
	if !strings.Contains(v, "Review closed session") {
		t.Fatalf("closed session view should show title, got:\n%s", v)
	}
	if !strings.Contains(v, "Closed session details should be visible") {
		t.Fatalf("closed session view should show note event, got:\n%s", v)
	}
	if !strings.Contains(v, "Session closed.") {
		t.Fatalf("closed session view should show stop event, got:\n%s", v)
	}
}

func TestModelUpdateHandoffGeneratedClearsScreen(t *testing.T) {
	m := testModel()
	m.CurrentView = ActiveSession
	m.ScrollOffset = 5

	updatedModel, cmd := m.Update(HandoffGeneratedMsg{Content: "# Handoff"})
	updated, ok := updatedModel.(Model)
	if !ok {
		t.Fatalf("expected Model from Update, got %T", updatedModel)
	}
	if updated.CurrentView != HandoffPreview {
		t.Fatalf("CurrentView = %v, want HandoffPreview", updated.CurrentView)
	}
	if updated.ScrollOffset != 0 {
		t.Errorf("ScrollOffset = %d, want 0", updated.ScrollOffset)
	}
	if cmd == nil {
		t.Fatal("expected clear-screen command when entering handoff preview")
	}
}

func TestModelViewNonEmpty(t *testing.T) {
	s := &store.Store{}
	m := NewModel(s, "/tmp")
	v := m.View()
	if v == "" {
		t.Error("View() returned empty string")
	}
}

func TestModelViewIncludesPaletteWhenOpen(t *testing.T) {
	p := NewCommandPalette()
	p.Open = true
	p.Input = "test"
	sess := &store.SessionRecord{
		Session: store.Session{ID: "x", Author: "a", Branch: "b"},
	}
	m := Model{
		CurrentView:   ActiveSession,
		ActiveSession: sess,
		Palette:       &p,
		Width:         80,
		Height:        24,
	}
	v := m.View()
	if !strings.Contains(v, "test") {
		t.Errorf("View() does not contain palette input: %s", v)
	}
}
