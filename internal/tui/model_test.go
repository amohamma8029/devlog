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

func TestModelUpdateQuitOnNoSessionKeys(t *testing.T) {
	for _, tc := range []struct {
		name string
		msg  tea.KeyMsg
	}{
		{name: "esc", msg: tea.KeyMsg{Type: tea.KeyEsc}},
		{name: "q", msg: tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := &store.Store{}
			m := NewModel(s, "/tmp")
			m.CurrentView = ActiveSession

			_, cmd := m.Update(tc.msg)
			if cmd == nil {
				t.Fatal("expected quit cmd")
			}
		})
	}
}

func TestModelSessionListBackKeysReturnToNoSession(t *testing.T) {
	for _, tc := range []struct {
		name string
		msg  tea.KeyMsg
	}{
		{name: "esc", msg: tea.KeyMsg{Type: tea.KeyEsc}},
		{name: "q", msg: tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := &store.Store{}
			m := NewModel(s, "/tmp")
			m.CurrentView = SessionList
			m.ActiveSession = nil

			updatedModel, cmd := m.Update(tc.msg)
			if cmd != nil {
				t.Fatal("expected no quit cmd")
			}
			updated, ok := updatedModel.(Model)
			if !ok {
				t.Fatalf("expected Model from Update, got %T", updatedModel)
			}
			if updated.CurrentView != ActiveSession {
				t.Fatalf("CurrentView = %v, want ActiveSession", updated.CurrentView)
			}
			if updated.ActiveSession != nil {
				t.Fatalf("ActiveSession = %#v, want nil", updated.ActiveSession)
			}
		})
	}
}

func TestModelNoSessionOpenPromptKeyFlow(t *testing.T) {
	s, root := newTestStore(t)
	m := NewModel(s, root)
	m.CurrentView = ActiveSession
	m.Width = 80
	m.Height = 24

	updatedModel, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'o'}})
	updated, ok := updatedModel.(Model)
	if !ok {
		t.Fatalf("expected Model from Update, got %T", updatedModel)
	}
	if cmd == nil {
		t.Fatal("expected cursor tick command")
	}
	if !updated.OpenPromptOpen {
		t.Fatal("pressing o should open the session prompt")
	}
	if !strings.Contains(updated.View(), "Open session: ") {
		t.Fatalf("view should render open prompt, got:\n%s", updated.View())
	}

	updatedModel, _ = updated.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
	updated = updatedModel.(Model)
	updatedModel, _ = updated.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	updated = updatedModel.(Model)
	if updated.OpenInput != "aq" {
		t.Fatalf("OpenInput after typing = %q, want aq", updated.OpenInput)
	}

	updatedModel, _ = updated.Update(tea.KeyMsg{Type: tea.KeyBackspace})
	updated = updatedModel.(Model)
	if updated.OpenInput != "a" {
		t.Fatalf("OpenInput after backspace = %q, want a", updated.OpenInput)
	}

	updatedModel, cmd = updated.Update(tea.KeyMsg{Type: tea.KeyEsc})
	updated = updatedModel.(Model)
	if cmd != nil {
		t.Fatal("esc in open prompt should not quit")
	}
	if updated.OpenPromptOpen {
		t.Fatal("esc should close open prompt")
	}
	if updated.OpenInput != "" {
		t.Fatalf("OpenInput after esc = %q, want empty", updated.OpenInput)
	}
}

func TestModelNoSessionOpenPromptEmptyMessageSetsError(t *testing.T) {
	s, root := newTestStore(t)
	m := NewModel(s, root)
	m.CurrentView = ActiveSession
	m.OpenPromptOpen = true
	m.OpenInput = "   "

	updatedModel, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	updated, ok := updatedModel.(Model)
	if !ok {
		t.Fatalf("expected Model from Update, got %T", updatedModel)
	}
	if cmd != nil {
		t.Fatal("expected no command for empty open prompt")
	}
	if updated.OpenPromptOpen {
		t.Fatal("empty submit should close open prompt")
	}
	if updated.OpenInput != "" {
		t.Fatalf("OpenInput = %q, want empty", updated.OpenInput)
	}
	if updated.ErrorMessage != "Usage: type a session message" {
		t.Fatalf("ErrorMessage = %q, want usage error", updated.ErrorMessage)
	}
}

func TestModelNoSessionOpenPromptCreatesAndLoadsSession(t *testing.T) {
	t.Setenv("DEVLOG_AUTHOR_NAME", "TUI Tester")
	t.Setenv("DEVLOG_AUTHOR_EMAIL", "tui@example.com")

	s, root := newTestStore(t)
	m := NewModel(s, root)
	m.CurrentView = ActiveSession
	m.OpenPromptOpen = true
	m.OpenInput = "Start from the TUI"

	updatedModel, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	updated, ok := updatedModel.(Model)
	if !ok {
		t.Fatalf("expected Model from Update, got %T", updatedModel)
	}
	if cmd == nil {
		t.Fatal("expected command for non-empty open prompt")
	}
	if updated.OpenPromptOpen || updated.OpenInput != "" {
		t.Fatalf("prompt should close and clear after submit, open=%v input=%q", updated.OpenPromptOpen, updated.OpenInput)
	}

	msg := cmd()
	if errMsg, ok := msg.(CommandErrorMsg); ok {
		t.Fatalf("open prompt returned error: %v", errMsg.Error)
	}
	loaded, ok := msg.(ActiveSessionLoadedMsg)
	if !ok {
		t.Fatalf("open prompt returned %T, want ActiveSessionLoadedMsg", msg)
	}
	if loaded.Session == nil {
		t.Fatal("loaded session is nil")
	}
	if loaded.Title != "Start from the TUI" {
		t.Fatalf("Title = %q, want start message", loaded.Title)
	}
	if len(loaded.Events) != 1 || loaded.Events[0].Type != "Start" || loaded.Events[0].Body != "Start from the TUI" {
		t.Fatalf("Events = %#v, want single Start event", loaded.Events)
	}

	updatedModel, _ = updated.Update(loaded)
	updated, ok = updatedModel.(Model)
	if !ok {
		t.Fatalf("expected Model after ActiveSessionLoadedMsg, got %T", updatedModel)
	}
	if updated.CurrentView != ActiveSession {
		t.Fatalf("CurrentView = %v, want ActiveSession", updated.CurrentView)
	}
	if updated.ActiveSession == nil || updated.ActiveSession.ID != loaded.Session.ID {
		t.Fatalf("ActiveSession = %#v, want loaded session", updated.ActiveSession)
	}

	records, err := s.ListSessions()
	if err != nil {
		t.Fatalf("ListSessions failed: %v", err)
	}
	if len(records) != 1 {
		t.Fatalf("session count = %d, want 1", len(records))
	}
}

func TestModelNoSessionOpenPromptReportsExistingActiveSession(t *testing.T) {
	t.Setenv("DEVLOG_AUTHOR_NAME", "TUI Tester")
	t.Setenv("DEVLOG_AUTHOR_EMAIL", "tui@example.com")

	s, root := newTestStore(t)
	const activeID = "2026-02-20T090000Z"
	writeTestSession(t, s, activeID, "feat/current", "Alice", "Current work", time.Date(2026, 2, 20, 9, 0, 0, 0, time.UTC))

	m := NewModel(s, root)
	m.CurrentView = ActiveSession
	m.OpenPromptOpen = true
	m.OpenInput = "Another session"

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("expected command for non-empty open prompt")
	}
	msg := cmd()
	errMsg, ok := msg.(CommandErrorMsg)
	if !ok {
		t.Fatalf("open prompt returned %T, want CommandErrorMsg", msg)
	}
	if !strings.Contains(errMsg.Error.Error(), "a session is already active") {
		t.Fatalf("error = %q, want already active", errMsg.Error.Error())
	}
}

func TestModelSessionListFilterReceivesQ(t *testing.T) {
	s, root := newTestStore(t)
	writeTestSession(t, s, "2026-01-15T140000Z", "feat/a", "Alice", "quality pass", time.Date(2026, 1, 15, 14, 0, 0, 0, time.UTC))

	m := NewModel(s, root)
	m.CurrentView = SessionList
	m.SessionList = loadTestModel(t, s, root)

	updatedModel, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}})
	updated, ok := updatedModel.(Model)
	if !ok {
		t.Fatalf("expected Model after /, got %T", updatedModel)
	}
	if updated.CurrentView != SessionList {
		t.Fatalf("CurrentView after / = %v, want SessionList", updated.CurrentView)
	}
	if !updated.SessionList.filterMode {
		t.Fatal("expected session list filter mode after /")
	}

	updatedModel, cmd := updated.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	if cmd != nil {
		t.Fatal("expected q in filter mode not to return a command")
	}
	updated, ok = updatedModel.(Model)
	if !ok {
		t.Fatalf("expected Model after q, got %T", updatedModel)
	}
	if updated.CurrentView != SessionList {
		t.Fatalf("CurrentView after filter q = %v, want SessionList", updated.CurrentView)
	}
	if updated.SessionList.filterText != "q" {
		t.Fatalf("filterText after q = %q, want %q", updated.SessionList.filterText, "q")
	}
}

func TestModelSessionListArrowNavigation(t *testing.T) {
	s, root := newTestStore(t)
	writeTestSession(t, s, "2026-01-15T140000Z", "feat/a", "Alice", "auth", time.Date(2026, 1, 15, 14, 0, 0, 0, time.UTC))
	writeTestSession(t, s, "2026-02-20T090000Z", "feat/b", "Bob", "tests", time.Date(2026, 2, 20, 9, 0, 0, 0, time.UTC))

	m := NewModel(s, root)
	m.CurrentView = SessionList
	m.SessionList = loadTestModel(t, s, root)

	updatedModel, cmd := m.Update(tea.KeyMsg{Type: tea.KeyDown})
	if cmd != nil {
		t.Fatal("expected down arrow not to return a command")
	}
	updated, ok := updatedModel.(Model)
	if !ok {
		t.Fatalf("expected Model after down, got %T", updatedModel)
	}
	if updated.CurrentView != SessionList {
		t.Fatalf("CurrentView after down = %v, want SessionList", updated.CurrentView)
	}
	if updated.SessionList.cursor != 1 {
		t.Fatalf("cursor after down = %d, want 1", updated.SessionList.cursor)
	}

	updatedModel, cmd = updated.Update(tea.KeyMsg{Type: tea.KeyUp})
	if cmd != nil {
		t.Fatal("expected up arrow not to return a command")
	}
	updated, ok = updatedModel.(Model)
	if !ok {
		t.Fatalf("expected Model after up, got %T", updatedModel)
	}
	if updated.SessionList.cursor != 0 {
		t.Fatalf("cursor after up = %d, want 0", updated.SessionList.cursor)
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

func TestHandleCommandNoteOnClosedSessionSetsError(t *testing.T) {
	s, root := newTestStore(t)
	const closedID = "2026-01-15T140000Z"
	writeTestSession(t, s, closedID, "feat/old", "Alice", "Old work", time.Date(2026, 1, 15, 14, 0, 0, 0, time.UTC))
	closeTestSession(t, s, closedID)

	rec, err := s.GetSession(closedID)
	if err != nil {
		t.Fatalf("GetSession failed: %v", err)
	}

	p := NewCommandPalette()
	m := Model{
		Store:         s,
		Root:          root,
		ActiveSession: &rec,
		Palette:       &p,
	}

	bodyBefore, err := s.ReadSessionBody(closedID)
	if err != nil {
		t.Fatalf("ReadSessionBody failed: %v", err)
	}

	updatedModel, cmd := m.Update(CommandExecutedMsg{Input: "/note should not append"})
	updated, ok := updatedModel.(Model)
	if !ok {
		t.Fatalf("expected Model, got %T", updatedModel)
	}
	if cmd != nil {
		t.Fatal("expected no command for /note on closed session")
	}
	if updated.ErrorMessage == "" {
		t.Fatal("expected error message for /note on closed session")
	}

	bodyAfter, err := s.ReadSessionBody(closedID)
	if err != nil {
		t.Fatalf("ReadSessionBody after failed: %v", err)
	}
	if bodyAfter != bodyBefore {
		t.Fatal("closed session body was modified by /note")
	}
}

func TestHandleCommandBlockOnClosedSessionSetsError(t *testing.T) {
	s, root := newTestStore(t)
	const closedID = "2026-01-15T140000Z"
	writeTestSession(t, s, closedID, "feat/old", "Alice", "Old work", time.Date(2026, 1, 15, 14, 0, 0, 0, time.UTC))
	closeTestSession(t, s, closedID)

	rec, err := s.GetSession(closedID)
	if err != nil {
		t.Fatalf("GetSession failed: %v", err)
	}

	p := NewCommandPalette()
	m := Model{
		Store:         s,
		Root:          root,
		ActiveSession: &rec,
		Palette:       &p,
	}

	updatedModel, cmd := m.Update(CommandExecutedMsg{Input: "/block should not append"})
	updated, ok := updatedModel.(Model)
	if !ok {
		t.Fatalf("expected Model, got %T", updatedModel)
	}
	if cmd != nil {
		t.Fatal("expected no command for /block on closed session")
	}
	if updated.ErrorMessage == "" {
		t.Fatal("expected error message for /block on closed session")
	}
}

func TestHandleCommandCloseOnClosedSessionSetsError(t *testing.T) {
	s, root := newTestStore(t)
	const closedID = "2026-01-15T140000Z"
	writeTestSession(t, s, closedID, "feat/old", "Alice", "Old work", time.Date(2026, 1, 15, 14, 0, 0, 0, time.UTC))
	closeTestSession(t, s, closedID)

	rec, err := s.GetSession(closedID)
	if err != nil {
		t.Fatalf("GetSession failed: %v", err)
	}

	p := NewCommandPalette()
	m := Model{
		Store:         s,
		Root:          root,
		ActiveSession: &rec,
		Palette:       &p,
	}

	updatedModel, cmd := m.Update(CommandExecutedMsg{Input: "/close"})
	updated, ok := updatedModel.(Model)
	if !ok {
		t.Fatalf("expected Model, got %T", updatedModel)
	}
	if cmd != nil {
		t.Fatal("expected no command for /close on closed session")
	}
	if updated.ErrorMessage == "" {
		t.Fatal("expected error message for /close on closed session")
	}
}

func TestHandleCommandNoteWithNoSessionSetsError(t *testing.T) {
	p := NewCommandPalette()
	m := Model{
		Palette: &p,
	}

	updatedModel, cmd := m.Update(CommandExecutedMsg{Input: "/note orphan note"})
	updated, ok := updatedModel.(Model)
	if !ok {
		t.Fatalf("expected Model, got %T", updatedModel)
	}
	if cmd != nil {
		t.Fatal("expected no command for /note with no session")
	}
	if updated.ErrorMessage == "" {
		t.Fatal("expected error message for /note with no session")
	}
}

func TestHandleCommandNoteDoesNotMutateGlobalActiveWhenViewingClosed(t *testing.T) {
	s, root := newTestStore(t)
	const activeID = "2026-02-20T090000Z"
	const closedID = "2026-01-15T140000Z"

	writeTestSession(t, s, closedID, "feat/old", "Alice", "Old work", time.Date(2026, 1, 15, 14, 0, 0, 0, time.UTC))
	closeTestSession(t, s, closedID)

	writeTestSession(t, s, activeID, "feat/new", "Alice", "Current work", time.Date(2026, 2, 20, 9, 0, 0, 0, time.UTC))

	closedRec, err := s.GetSession(closedID)
	if err != nil {
		t.Fatalf("GetSession closed failed: %v", err)
	}

	activeBodyBefore, err := s.ReadSessionBody(activeID)
	if err != nil {
		t.Fatalf("ReadSessionBody active failed: %v", err)
	}

	p := NewCommandPalette()
	m := Model{
		Store:         s,
		Root:          root,
		ActiveSession: &closedRec,
		Palette:       &p,
	}

	updatedModel, cmd := m.Update(CommandExecutedMsg{Input: "/note stray note"})
	updated, ok := updatedModel.(Model)
	if !ok {
		t.Fatalf("expected Model, got %T", updatedModel)
	}
	if cmd != nil {
		t.Fatal("expected no command")
	}
	if updated.ErrorMessage == "" {
		t.Fatal("expected error message")
	}

	activeBodyAfter, err := s.ReadSessionBody(activeID)
	if err != nil {
		t.Fatalf("ReadSessionBody active after failed: %v", err)
	}
	if activeBodyAfter != activeBodyBefore {
		t.Fatal("global active session was mutated while viewing a closed session")
	}
}

func TestHandleCommandNoteAppendsToDisplayedOpenSession(t *testing.T) {
	s, root := newTestStore(t)
	const openID = "2026-02-20T090000Z"
	writeTestSession(t, s, openID, "feat/new", "Alice", "Current work", time.Date(2026, 2, 20, 9, 0, 0, 0, time.UTC))

	rec, err := s.GetSession(openID)
	if err != nil {
		t.Fatalf("GetSession failed: %v", err)
	}

	p := NewCommandPalette()
	m := Model{
		Store:         s,
		Root:          root,
		ActiveSession: &rec,
		Palette:       &p,
	}

	updatedModel, cmd := m.Update(CommandExecutedMsg{Input: "/note visible session note"})
	updated, ok := updatedModel.(Model)
	if !ok {
		t.Fatalf("expected Model, got %T", updatedModel)
	}
	if cmd == nil {
		t.Fatal("expected command for /note on open session")
	}
	if updated.ErrorMessage != "" {
		t.Fatalf("unexpected error: %s", updated.ErrorMessage)
	}

	msg := cmd()
	if _, isErr := msg.(CommandErrorMsg); isErr {
		t.Fatalf("command returned error: %v", msg)
	}

	body, err := s.ReadSessionBody(openID)
	if err != nil {
		t.Fatalf("ReadSessionBody failed: %v", err)
	}
	if !strings.Contains(body, "visible session note") {
		t.Fatal("note was not appended to the displayed open session")
	}
}

func TestHandleViewKeyEscClosesHandoffSavePrompt(t *testing.T) {
	s, root := newTestStore(t)
	const sessionID = "2026-01-15T140000Z"
	writeTestSession(t, s, sessionID, "feat/a", "Alice", "auth", time.Date(2026, 1, 15, 14, 0, 0, 0, time.UTC))

	m := NewModel(s, root)
	m.CurrentView = HandoffPreview
	m.SavePromptOpen = true
	m.SaveInput = "test-file.md"
	m.ActiveSession = &store.SessionRecord{
		Session: store.Session{ID: sessionID, Author: "Alice", Branch: "feat/a"},
	}

	msg := tea.KeyMsg{Type: tea.KeyEsc}
	updatedModel, cmd := m.Update(msg)
	if cmd != nil {
		t.Fatal("esc with save prompt open should not quit")
	}
	updated, ok := updatedModel.(Model)
	if !ok {
		t.Fatalf("expected Model, got %T", updatedModel)
	}
	if updated.SavePromptOpen {
		t.Error("esc should close save prompt")
	}
	if updated.CurrentView != HandoffPreview {
		t.Errorf("should stay on HandoffPreview, got %v", updated.CurrentView)
	}
}

func TestHandleViewKeyQInHandoffSavePrompt(t *testing.T) {
	s, root := newTestStore(t)
	const sessionID = "2026-01-15T140000Z"
	writeTestSession(t, s, sessionID, "feat/a", "Alice", "auth", time.Date(2026, 1, 15, 14, 0, 0, 0, time.UTC))

	m := NewModel(s, root)
	m.CurrentView = HandoffPreview
	m.SavePromptOpen = true
	m.SaveInput = "file.md"
	m.ActiveSession = &store.SessionRecord{
		Session: store.Session{ID: sessionID, Author: "Alice", Branch: "feat/a"},
	}

	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}}
	updatedModel, cmd := m.Update(msg)
	if cmd != nil {
		t.Fatal("q with save prompt open should not navigate or quit")
	}
	updated, ok := updatedModel.(Model)
	if !ok {
		t.Fatalf("expected Model, got %T", updatedModel)
	}
	if !updated.SavePromptOpen {
		t.Error("save prompt should stay open after typing q")
	}
	if !strings.Contains(updated.SaveInput, "q") {
		t.Errorf("q should be appended to filename, got %q", updated.SaveInput)
	}
	if updated.CurrentView != HandoffPreview {
		t.Errorf("should stay on HandoffPreview, got %v", updated.CurrentView)
	}
}

func TestHandleViewKeyQNavigatesBackFromHandoffNoPrompt(t *testing.T) {
	s, root := newTestStore(t)
	const sessionID = "2026-01-15T140000Z"
	writeTestSession(t, s, sessionID, "feat/a", "Alice", "auth", time.Date(2026, 1, 15, 14, 0, 0, 0, time.UTC))

	m := NewModel(s, root)
	m.CurrentView = HandoffPreview
	m.SavePromptOpen = false
	m.ActiveSession = &store.SessionRecord{
		Session: store.Session{ID: sessionID, Author: "Alice", Branch: "feat/a"},
	}

	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}}
	updatedModel, cmd := m.Update(msg)
	if cmd != nil {
		t.Fatal("q without save prompt should not quit")
	}
	updated, ok := updatedModel.(Model)
	if !ok {
		t.Fatalf("expected Model, got %T", updatedModel)
	}
	if updated.CurrentView != ActiveSession {
		t.Errorf("should navigate to ActiveSession, got %v", updated.CurrentView)
	}
}

func TestHandleViewKeyEscNavigatesBackFromHandoffNoPrompt(t *testing.T) {
	s, root := newTestStore(t)
	const sessionID = "2026-01-15T140000Z"
	writeTestSession(t, s, sessionID, "feat/a", "Alice", "auth", time.Date(2026, 1, 15, 14, 0, 0, 0, time.UTC))

	m := NewModel(s, root)
	m.CurrentView = HandoffPreview
	m.SavePromptOpen = false
	m.ActiveSession = &store.SessionRecord{
		Session: store.Session{ID: sessionID, Author: "Alice", Branch: "feat/a"},
	}

	msg := tea.KeyMsg{Type: tea.KeyEsc}
	updatedModel, cmd := m.Update(msg)
	if cmd != nil {
		t.Fatal("esc without save prompt should not quit")
	}
	updated, ok := updatedModel.(Model)
	if !ok {
		t.Fatalf("expected Model, got %T", updatedModel)
	}
	if updated.CurrentView != ActiveSession {
		t.Errorf("should navigate to ActiveSession, got %v", updated.CurrentView)
	}
}
