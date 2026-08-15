package tui

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	internalconfig "github.com/amohamma8029/devlog/internal/config"
	"github.com/amohamma8029/devlog/internal/store"
	"github.com/amohamma8029/devlog/internal/todo"
	tea "github.com/charmbracelet/bubbletea"
	xansi "github.com/charmbracelet/x/ansi"
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

func runSessionListTestGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s failed: %v\n%s", strings.Join(args, " "), err, strings.TrimSpace(string(output)))
	}
}

func TestSessionListHandoffIncludesRelevantTodos(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skipf("git binary not available: %v", err)
	}
	root := t.TempDir()
	runSessionListTestGit(t, root, "init")
	runSessionListTestGit(t, root, "checkout", "-b", "feat/test")

	s, err := store.New(root)
	if err != nil {
		t.Fatalf("store.New failed: %v", err)
	}
	sess := store.Session{
		ID:      "2026-01-15T140000Z",
		Author:  "Alice",
		Started: time.Date(2026, 1, 15, 14, 0, 0, 0, time.UTC),
		Branch:  "feat/test",
		Status:  "active",
	}
	if err := s.WriteSession(sess, "start work"); err != nil {
		t.Fatalf("WriteSession failed: %v", err)
	}
	if err := s.AppendEvent(sess.ID, "Note", "implement feature"); err != nil {
		t.Fatalf("AppendEvent failed: %v", err)
	}

	todoStore, err := todo.NewStore(root)
	if err != nil {
		t.Fatalf("todo.NewStore failed: %v", err)
	}
	if _, err := todoStore.Add(todo.AddInput{Text: "follow up from list", SessionID: sess.ID, Branch: sess.Branch}); err != nil {
		t.Fatalf("todo.Add failed: %v", err)
	}

	m := NewSessionListModel(s, root, 80, 24)
	_, cmd := m.generateHandoff()
	if cmd == nil {
		t.Fatal("expected command from generateHandoff")
	}
	msg := cmd()
	gen, ok := msg.(HandoffGeneratedMsg)
	if !ok {
		t.Fatalf("expected HandoffGeneratedMsg, got %T", msg)
	}
	if gen.Error != nil {
		t.Fatalf("handoff generation failed: %v", gen.Error)
	}
	if !hasHandoffTodoListSection(gen.Content) {
		t.Errorf("expected ## Todos section in list handoff, got:\n%s", gen.Content)
	}
	if !handoffTodosSectionContains(gen.Content, "follow up from list") {
		t.Errorf("expected relevant open todo in list handoff, got:\n%s", gen.Content)
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

func TestSessionListPageNavigation(t *testing.T) {
	s, root := newTestStore(t)
	writeTestSession(t, s, "2026-01-15T140000Z", "feat/a", "Alice", "one", time.Date(2026, 1, 15, 14, 0, 0, 0, time.UTC))
	writeTestSession(t, s, "2026-01-15T140001Z", "feat/b", "Alice", "two", time.Date(2026, 1, 15, 14, 0, 1, 0, time.UTC))
	writeTestSession(t, s, "2026-01-15T140002Z", "feat/c", "Alice", "three", time.Date(2026, 1, 15, 14, 0, 2, 0, time.UTC))
	writeTestSession(t, s, "2026-01-15T140003Z", "feat/d", "Alice", "four", time.Date(2026, 1, 15, 14, 0, 3, 0, time.UTC))
	writeTestSession(t, s, "2026-01-15T140004Z", "feat/e", "Alice", "five", time.Date(2026, 1, 15, 14, 0, 4, 0, time.UTC))
	writeTestSession(t, s, "2026-01-15T140005Z", "feat/f", "Alice", "six", time.Date(2026, 1, 15, 14, 0, 5, 0, time.UTC))

	m := NewSessionListModel(s, root, 80, 6)
	updated, _ := m.Update(m.Init()())
	m, _ = updated.(SessionListModel)

	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyPgDown})
	m, _ = updated.(SessionListModel)
	if m.cursor != 3 {
		t.Fatalf("cursor should be 3 after PgDn, got %d", m.cursor)
	}

	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyPgUp})
	m, _ = updated.(SessionListModel)
	if m.cursor != 0 {
		t.Fatalf("cursor should be 0 after PgUp, got %d", m.cursor)
	}

	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnd})
	m, _ = updated.(SessionListModel)
	if m.cursor != len(m.filtered)-1 {
		t.Fatalf("cursor should jump to end, got %d", m.cursor)
	}

	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyHome})
	m, _ = updated.(SessionListModel)
	if m.cursor != 0 {
		t.Fatalf("cursor should jump home, got %d", m.cursor)
	}

	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	m, _ = updated.(SessionListModel)
	if m.cursor != 0 {
		t.Fatalf("j should not move cursor after removal, got %d", m.cursor)
	}

	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}})
	m, _ = updated.(SessionListModel)
	if m.cursor != 0 {
		t.Fatalf("k should not move cursor after removal, got %d", m.cursor)
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
	for _, col := range []string{"TITLE", "BRANCH", "AUTHOR", "STARTED", "STATUS"} {
		if !strings.Contains(v, col) {
			t.Fatalf("expected column %q in view, got: %s", col, v)
		}
	}
	if !strings.Contains(v, "auth") {
		t.Fatalf("expected title 'auth' in view: %s", v)
	}
	if !strings.Contains(v, "Alice") {
		t.Fatalf("expected author in view: %s", v)
	}
	if !strings.Contains(v, "active") {
		t.Fatalf("expected status in view: %s", v)
	}
}

func TestSessionListUsesConfiguredDisplayTime(t *testing.T) {
	s, root := newTestStore(t)
	writeTestSession(t, s, "2026-01-15T140000Z", "feat/a", "Alice", "auth", time.Date(2026, 1, 15, 14, 0, 0, 0, time.UTC))
	cfg := internalconfig.Default()
	cfg.Display.Timezone = "America/New_York"
	cfg.Display.ClockFormat = internalconfig.ClockFormat12h

	m := NewSessionListModelWithConfig(s, root, 140, 24, cfg)
	updated, _ := m.Update(m.Init()())
	m = updated.(SessionListModel)

	v := m.View()
	if !strings.Contains(v, "2026-01-15 9:00:00 AM EST") {
		t.Fatalf("session list should use configured display time, got:\n%s", v)
	}
}

func TestSessionListRetainsHandoffDiffContextConfig(t *testing.T) {
	s, root := newTestStore(t)
	cfg := internalconfig.Default()
	cfg.Handoff.DiffContextLines = 0

	m := NewSessionListModelWithConfig(s, root, 140, 24, cfg)

	if m.config.Handoff.DiffContextLines != 0 {
		t.Fatalf("handoff diff context = %d, want 0", m.config.Handoff.DiffContextLines)
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

func TestSessionListReloadsAfterClose(t *testing.T) {
	s, root := newTestStore(t)
	writeTestSession(t, s, "2026-01-15T140000Z", "feat/a", "Alice", "auth work", time.Date(2026, 1, 15, 14, 0, 0, 0, time.UTC))

	m := loadTestModel(t, s, root)

	if m.sessions[0].Closed {
		t.Fatal("expected session to be active initially")
	}

	closeTestSession(t, s, "2026-01-15T140000Z")

	cmd := m.Init()
	msg := cmd()
	updated, _ := m.Update(msg)
	m = updated.(SessionListModel)

	if !m.sessions[0].Closed {
		t.Fatal("expected session to be closed after reload")
	}
	if len(m.filtered) != 1 {
		t.Fatalf("expected 1 filtered session after reload, got %d", len(m.filtered))
	}
}

func TestTruncateCellASCII(t *testing.T) {
	tests := []struct {
		name  string
		s     string
		width int
		want  string
	}{
		{"shorter than width", "hello", 10, "hello"},
		{"exactly width", "hello", 5, "hello"},
		{"longer than width", "hello world", 6, "hello\u2026"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := truncateCell(tt.s, tt.width)
			if got != tt.want {
				t.Errorf("truncateCell(%q, %d) = %q, want %q", tt.s, tt.width, got, tt.want)
			}
		})
	}
}

func TestTruncateCellUnicode(t *testing.T) {
	tests := []struct {
		name  string
		s     string
		width int
	}{
		{"multi-byte shorter", "café", 10},
		{"multi-byte longer", "café résumé", 5},
		{"emoji shorter", "hello 🚀", 10},
		{"emoji longer", "hello 🚀 world", 12},
		{"CJK shorter", "日本語テスト", 10},
		{"CJK longer", "日本語テスト文章", 8},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := truncateCell(tt.s, tt.width)
			sw := xansi.StringWidth(got)
			if sw > tt.width {
				t.Errorf("truncateCell(%q, %d) = %q has display width %d, exceeds max %d", tt.s, tt.width, got, sw, tt.width)
			}
			if !strings.Contains(got, "\u2026") && xansi.StringWidth(tt.s) > tt.width {
				t.Errorf("truncateCell(%q, %d) = %q should contain ellipsis for truncation", tt.s, tt.width, got)
			}
		})
	}
}
