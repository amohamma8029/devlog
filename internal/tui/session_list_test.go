package tui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/amo/devlog/internal/store"
)

func newTestStore(t *testing.T) (*store.Store, string) {
	t.Helper()
	root := t.TempDir()
	s, err := store.New(root)
	if err != nil {
		t.Fatalf("store.New failed: %v", err)
	}
	return s, root
}

func writeTestSession(t *testing.T, s *store.Store, id, branch, author, startMessage string, started time.Time) {
	t.Helper()
	sess := store.Session{
		ID:      id,
		Author:  author,
		Started: started,
		Branch:  branch,
		Status:  "active",
	}
	if err := s.WriteSession(sess, startMessage); err != nil {
		t.Fatalf("WriteSession failed: %v", err)
	}
}

func closeTestSession(t *testing.T, s *store.Store, id string) {
	t.Helper()
	if err := s.CloseSession(id); err != nil {
		t.Fatalf("CloseSession failed: %v", err)
	}
}

func loadTestModel(t *testing.T, s *store.Store, root string) SessionListModel {
	t.Helper()
	m := NewSessionListModel(s, root, 80, 24)
	cmd := m.Init()
	msg := cmd()
	updated, _ := m.Update(msg)
	sl, _ := updated.(SessionListModel)
	return sl
}

func TestSessionListLoadsSessions(t *testing.T) {
	s, root := newTestStore(t)
	writeTestSession(t, s, "2026-01-15T140000Z", "feat/a", "Alice", "auth work", time.Date(2026, 1, 15, 14, 0, 0, 0, time.UTC))
	writeTestSession(t, s, "2026-02-20T090000Z", "feat/b", "Bob", "tests pass", time.Date(2026, 2, 20, 9, 0, 0, 0, time.UTC))

	m := loadTestModel(t, s, root)

	if !m.loaded {
		t.Fatal("expected sessions to be loaded")
	}
	if len(m.sessions) != 2 {
		t.Fatalf("expected 2 sessions, got %d", len(m.sessions))
	}
	if len(m.filtered) != 2 {
		t.Fatalf("expected 2 filtered, got %d", len(m.filtered))
	}
}

func TestSessionListCursorNavigation(t *testing.T) {
	s, root := newTestStore(t)
	writeTestSession(t, s, "2026-01-15T140000Z", "feat/a", "Alice", "auth", time.Date(2026, 1, 15, 14, 0, 0, 0, time.UTC))
	writeTestSession(t, s, "2026-02-20T090000Z", "feat/b", "Bob", "tests", time.Date(2026, 2, 20, 9, 0, 0, 0, time.UTC))

	m := loadTestModel(t, s, root)

	if m.cursor != 0 {
		t.Fatalf("cursor should start at 0, got %d", m.cursor)
	}

	down := tea.KeyMsg{Type: tea.KeyDown}
	updated, _ := m.Update(down)
	m, _ = updated.(SessionListModel)
	if m.cursor != 1 {
		t.Fatalf("cursor should be 1 after down, got %d", m.cursor)
	}

	down = tea.KeyMsg{Type: tea.KeyDown}
	updated, _ = m.Update(down)
	m, _ = updated.(SessionListModel)
	if m.cursor != 1 {
		t.Fatalf("cursor should be clamped at 1, got %d", m.cursor)
	}

	up := tea.KeyMsg{Type: tea.KeyUp}
	updated, _ = m.Update(up)
	m, _ = updated.(SessionListModel)
	if m.cursor != 0 {
		t.Fatalf("cursor should be 0 after up, got %d", m.cursor)
	}
}

func TestSessionListJKNavigation(t *testing.T) {
	s, root := newTestStore(t)
	writeTestSession(t, s, "2026-01-15T140000Z", "feat/a", "Alice", "auth", time.Date(2026, 1, 15, 14, 0, 0, 0, time.UTC))
	writeTestSession(t, s, "2026-02-20T090000Z", "feat/b", "Bob", "tests", time.Date(2026, 2, 20, 9, 0, 0, 0, time.UTC))

	m := loadTestModel(t, s, root)

	j := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}}
	updated, _ := m.Update(j)
	m, _ = updated.(SessionListModel)
	if m.cursor != 1 {
		t.Fatalf("cursor should be 1 after j, got %d", m.cursor)
	}

	k := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}}
	updated, _ = m.Update(k)
	m, _ = updated.(SessionListModel)
	if m.cursor != 0 {
		t.Fatalf("cursor should be 0 after k, got %d", m.cursor)
	}
}

func TestSessionListEnterEmitsNavigation(t *testing.T) {
	s, root := newTestStore(t)
	writeTestSession(t, s, "2026-01-15T140000Z", "feat/a", "Alice", "auth", time.Date(2026, 1, 15, 14, 0, 0, 0, time.UTC))

	m := loadTestModel(t, s, root)

	enter := tea.KeyMsg{Type: tea.KeyEnter}
	_, cmd := m.Update(enter)
	if cmd == nil {
		t.Fatal("expected navigation cmd on enter")
	}

	msg := cmd()
	nav, ok := msg.(NavigationMsg)
	if !ok {
		t.Fatalf("expected NavigationMsg, got %T", msg)
	}
	if nav.Target != ActiveSession {
		t.Errorf("expected Target = ActiveSession, got %v", nav.Target)
	}
	if nav.Session == nil {
		t.Fatal("expected non-nil Session")
	}
	if nav.Session.ID != "2026-01-15T140000Z" {
		t.Errorf("expected session ID 2026-01-15T140000Z, got %s", nav.Session.ID)
	}
}

func TestSessionListEnterNoSessions(t *testing.T) {
	s, root := newTestStore(t)
	m := loadTestModel(t, s, root)

	enter := tea.KeyMsg{Type: tea.KeyEnter}
	_, cmd := m.Update(enter)
	if cmd != nil {
		t.Fatal("expected no cmd on enter with empty list")
	}
}

func TestSessionListFilterByID(t *testing.T) {
	s, root := newTestStore(t)
	writeTestSession(t, s, "2026-01-15T140000Z", "feat/a", "Alice", "auth", time.Date(2026, 1, 15, 14, 0, 0, 0, time.UTC))
	writeTestSession(t, s, "2026-02-20T090000Z", "feat/b", "Bob", "tests", time.Date(2026, 2, 20, 9, 0, 0, 0, time.UTC))

	m := loadTestModel(t, s, root)

	slash := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}}
	updated, _ := m.Update(slash)
	m, _ = updated.(SessionListModel)
	if !m.filterMode {
		t.Fatal("expected filter mode after /")
	}

	for _, r := range "2026-02" {
		k := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}}
		updated, _ = m.Update(k)
		m, _ = updated.(SessionListModel)
	}

	if len(m.filtered) != 1 {
		t.Fatalf("expected 1 filtered session, got %d", len(m.filtered))
	}
	if m.sessions[m.filtered[0]].ID != "2026-02-20T090000Z" {
		t.Fatalf("expected filtered session 2026-02-20T090000Z, got %s", m.sessions[m.filtered[0]].ID)
	}
}

func TestSessionListFilterByStartMessage(t *testing.T) {
	s, root := newTestStore(t)
	writeTestSession(t, s, "2026-01-15T140000Z", "feat/a", "Alice", "implement auth middleware", time.Date(2026, 1, 15, 14, 0, 0, 0, time.UTC))
	writeTestSession(t, s, "2026-02-20T090000Z", "feat/b", "Bob", "fix tests pass", time.Date(2026, 2, 20, 9, 0, 0, 0, time.UTC))

	m := loadTestModel(t, s, root)

	slash := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}}
	updated, _ := m.Update(slash)
	m, _ = updated.(SessionListModel)

	for _, r := range "middle" {
		k := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}}
		updated, _ = m.Update(k)
		m, _ = updated.(SessionListModel)
	}

	if len(m.filtered) != 1 {
		t.Fatalf("expected 1 filtered session, got %d", len(m.filtered))
	}
}

func TestSessionListFilterEscClears(t *testing.T) {
	s, root := newTestStore(t)
	writeTestSession(t, s, "2026-01-15T140000Z", "feat/a", "Alice", "auth", time.Date(2026, 1, 15, 14, 0, 0, 0, time.UTC))
	writeTestSession(t, s, "2026-02-20T090000Z", "feat/b", "Bob", "tests", time.Date(2026, 2, 20, 9, 0, 0, 0, time.UTC))

	m := loadTestModel(t, s, root)

	slash := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}}
	updated, _ := m.Update(slash)
	m, _ = updated.(SessionListModel)

	for _, r := range "2026-01" {
		k := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}}
		updated, _ = m.Update(k)
		m, _ = updated.(SessionListModel)
	}

	if len(m.filtered) != 1 {
		t.Fatalf("expected 1 filtered session before esc, got %d", len(m.filtered))
	}

	esc := tea.KeyMsg{Type: tea.KeyEsc}
	updated, _ = m.Update(esc)
	m, _ = updated.(SessionListModel)

	if m.filterMode {
		t.Fatal("expected filter mode to be off after esc")
	}
	if len(m.filtered) != 2 {
		t.Fatalf("expected all sessions after esc, got %d", len(m.filtered))
	}
}

func TestSessionListEmptyList(t *testing.T) {
	s, root := newTestStore(t)
	m := loadTestModel(t, s, root)

	v := m.View()
	if !strings.Contains(v, "No sessions found.") {
		t.Fatalf("expected empty state message, got: %s", v)
	}
}

func TestSessionListViewContainsColumns(t *testing.T) {
	s, root := newTestStore(t)
	writeTestSession(t, s, "2026-01-15T140000Z", "feat/a", "Alice", "auth", time.Date(2026, 1, 15, 14, 0, 0, 0, time.UTC))

	m := loadTestModel(t, s, root)

	v := m.View()
	for _, col := range []string{"ID", "BRANCH", "AUTHOR", "STARTED", "STATUS"} {
		if !strings.Contains(v, col) {
			t.Fatalf("expected column %q in view, got: %s", col, v)
		}
	}
	if !strings.Contains(v, "2026-01-15T140000Z") {
		t.Fatalf("expected session ID in view: %s", v)
	}
	if !strings.Contains(v, "Alice") {
		t.Fatalf("expected author in view: %s", v)
	}
	if !strings.Contains(v, "active") {
		t.Fatalf("expected status in view: %s", v)
	}
}

func TestSessionListViewContainsClosedStatus(t *testing.T) {
	s, root := newTestStore(t)
	writeTestSession(t, s, "2026-02-20T090000Z", "feat/b", "Bob", "tests", time.Date(2026, 2, 20, 9, 0, 0, 0, time.UTC))
	closeTestSession(t, s, "2026-02-20T090000Z")

	m := loadTestModel(t, s, root)

	v := m.View()
	if !strings.Contains(v, "closed") {
		t.Fatalf("expected closed status in view: %s", v)
	}
}

func TestSessionListMouseScroll(t *testing.T) {
	s, root := newTestStore(t)
	writeTestSession(t, s, "2026-01-15T140000Z", "feat/a", "Alice", "auth", time.Date(2026, 1, 15, 14, 0, 0, 0, time.UTC))
	writeTestSession(t, s, "2026-02-20T090000Z", "feat/b", "Bob", "tests", time.Date(2026, 2, 20, 9, 0, 0, 0, time.UTC))

	m := loadTestModel(t, s, root)

	scrollDown := tea.MouseMsg{Button: tea.MouseButtonWheelDown}
	updated, _ := m.Update(scrollDown)
	m, _ = updated.(SessionListModel)
	if m.cursor != 1 {
		t.Fatalf("cursor should be 1 after scroll down, got %d", m.cursor)
	}

	scrollUp := tea.MouseMsg{Button: tea.MouseButtonWheelUp}
	updated, _ = m.Update(scrollUp)
	m, _ = updated.(SessionListModel)
	if m.cursor != 0 {
		t.Fatalf("cursor should be 0 after scroll up, got %d", m.cursor)
	}
}

func TestSessionListFilterEnterExits(t *testing.T) {
	s, root := newTestStore(t)
	writeTestSession(t, s, "2026-01-15T140000Z", "feat/a", "Alice", "auth", time.Date(2026, 1, 15, 14, 0, 0, 0, time.UTC))

	m := loadTestModel(t, s, root)

	slash := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}}
	updated, _ := m.Update(slash)
	m, _ = updated.(SessionListModel)

	if !m.filterMode {
		t.Fatal("expected filter mode to be on")
	}

	enter := tea.KeyMsg{Type: tea.KeyEnter}
	updated, _ = m.Update(enter)
	m, _ = updated.(SessionListModel)

	if m.filterMode {
		t.Fatal("expected filter mode to be off after enter")
	}
}

func TestSessionListSessionFilesReadable(t *testing.T) {
	s, root := newTestStore(t)
	writeTestSession(t, s, "2026-01-15T140000Z", "feat/a", "Alice", "implement auth", time.Date(2026, 1, 15, 14, 0, 0, 0, time.UTC))

	m := loadTestModel(t, s, root)

	msg, ok := m.startMessages["2026-01-15T140000Z"]
	if !ok {
		t.Fatal("expected start message for session")
	}
	if msg != "implement auth" {
		t.Fatalf("expected start message 'implement auth', got %q", msg)
	}
}

func TestSessionListFileMissingStartMessage(t *testing.T) {
	s, root := newTestStore(t)
	writeTestSession(t, s, "2026-01-15T140000Z", "feat/a", "Alice", "temp", time.Date(2026, 1, 15, 14, 0, 0, 0, time.UTC))

	path := filepath.Join(root, ".devlog", "sessions", "2026-01-15T140000Z.md")
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read file: %v", err)
	}
	withoutBody := strings.Replace(string(content), "temp", "", 1)
	if err := os.WriteFile(path, []byte(withoutBody), 0644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	m := loadTestModel(t, s, root)
	msg, ok := m.startMessages["2026-01-15T140000Z"]
	if !ok {
		t.Fatal("expected start message entry")
	}
	if msg != "" {
		t.Fatalf("expected empty start message, got %q", msg)
	}
}
