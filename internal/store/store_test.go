package store

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestNewRequiresRoot(t *testing.T) {
	_, err := New("")
	if err == nil || !strings.Contains(err.Error(), "root is empty") {
		t.Fatalf("expected empty root error, got: %v", err)
	}
}

func TestNewRequiresDirectoryRoot(t *testing.T) {
	file := filepath.Join(t.TempDir(), "root.txt")
	if err := os.WriteFile(file, []byte("not a directory"), 0644); err != nil {
		t.Fatalf("write temp file failed: %v", err)
	}

	_, err := New(file)
	if err == nil || !strings.Contains(err.Error(), "root is not a directory") {
		t.Fatalf("expected root directory error, got: %v", err)
	}
}

func TestWriteSession(t *testing.T) {
	root := t.TempDir()
	store := newTestStore(t, root)
	sess := testSession()

	if err := store.WriteSession(sess, "Implement auth middleware"); err != nil {
		t.Fatalf("WriteSession failed: %v", err)
	}

	content := readSessionFile(t, root, sess.ID)
	if !strings.Contains(content, "id: 2026-01-15T143022Z") {
		t.Errorf("expected front-matter to contain id")
	}
	if !strings.Contains(content, "status: active") {
		t.Errorf("expected front-matter to contain status active")
	}
	if !strings.Contains(content, "## Start\n\nImplement auth middleware") {
		t.Errorf("expected body to contain start message")
	}
}

func TestWriteSessionCreatesSessionsDirectory(t *testing.T) {
	root := t.TempDir()
	store := newTestStore(t, root)

	if err := store.WriteSession(testSession(), "Implement auth middleware"); err != nil {
		t.Fatalf("WriteSession failed: %v", err)
	}

	path := filepath.Join(root, sessionsDir)
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("expected sessions directory to exist: %v", err)
	}
	if !info.IsDir() {
		t.Fatalf("expected sessions path to be a directory")
	}
}

func TestWriteSessionDuplicateIDFails(t *testing.T) {
	root := t.TempDir()
	store := newTestStore(t, root)
	sess := testSession()

	if err := store.WriteSession(sess, "Implement auth middleware"); err != nil {
		t.Fatalf("WriteSession failed: %v", err)
	}
	if err := store.WriteSession(sess, "Implement auth middleware"); err == nil {
		t.Fatal("expected duplicate WriteSession to fail")
	}
}

func TestWriteSessionValidatesInput(t *testing.T) {
	store := newTestStore(t, t.TempDir())

	if err := store.WriteSession(Session{}, "message"); err == nil || !strings.Contains(err.Error(), "session ID is empty") {
		t.Fatalf("expected empty ID error, got: %v", err)
	}
	if err := store.WriteSession(testSession(), ""); err == nil || !strings.Contains(err.Error(), "start message is empty") {
		t.Fatalf("expected empty start message error, got: %v", err)
	}
	if err := store.WriteSession(Session{ID: "../outside"}, "message"); err == nil || !strings.Contains(err.Error(), "invalid session ID") {
		t.Fatalf("expected invalid session ID error, got: %v", err)
	}
}

func TestAppendEvent(t *testing.T) {
	root := t.TempDir()
	store := newTestStore(t, root)
	sess := testSession()

	if err := store.WriteSession(sess, "Implement auth middleware"); err != nil {
		t.Fatalf("WriteSession failed: %v", err)
	}
	if err := store.AppendEvent(sess.ID, "Note", "Refactored JWT package"); err != nil {
		t.Fatalf("AppendEvent failed: %v", err)
	}

	content := readSessionFile(t, root, sess.ID)
	if !strings.Contains(content, "## Note - ") {
		t.Errorf("expected body to contain Note event")
	}
	if !strings.Contains(content, "Refactored JWT package") {
		t.Errorf("expected body to contain note text")
	}
}

func TestAppendEventValidatesInput(t *testing.T) {
	store := newTestStore(t, t.TempDir())

	if err := store.AppendEvent("", "Note", "body"); err == nil || !strings.Contains(err.Error(), "session ID is empty") {
		t.Fatalf("expected empty ID error, got: %v", err)
	}
	if err := store.AppendEvent("../outside", "Note", "body"); err == nil || !strings.Contains(err.Error(), "invalid session ID") {
		t.Fatalf("expected invalid session ID error, got: %v", err)
	}
	if err := store.AppendEvent("missing", "", "body"); err == nil || !strings.Contains(err.Error(), "event type is empty") {
		t.Fatalf("expected empty event type error, got: %v", err)
	}
	if err := store.AppendEvent("missing", "Note", ""); err == nil || !strings.Contains(err.Error(), "event body is empty") {
		t.Fatalf("expected empty event body error, got: %v", err)
	}
}

func TestAppendEventMissingSession(t *testing.T) {
	store := newTestStore(t, t.TempDir())

	err := store.AppendEvent("nonexistent", "Note", "body")
	if err == nil || !strings.Contains(err.Error(), "session file not found") {
		t.Fatalf("expected not found error, got: %v", err)
	}
}

func TestCloseSessionAppendsStopEvent(t *testing.T) {
	root := t.TempDir()
	store := newTestStore(t, root)
	sess := testSession()

	if err := store.WriteSession(sess, "Implement auth middleware"); err != nil {
		t.Fatalf("WriteSession failed: %v", err)
	}
	if err := store.CloseSession(sess.ID); err != nil {
		t.Fatalf("CloseSession failed: %v", err)
	}

	content := readSessionFile(t, root, sess.ID)
	if !strings.Contains(content, "status: active") {
		t.Errorf("expected front-matter to remain append-only metadata")
	}
	if !strings.Contains(content, "## Stop - ") {
		t.Errorf("expected body to contain Stop event")
	}
	if !strings.Contains(content, "Session closed.") {
		t.Errorf("expected body to contain close message")
	}
}

func TestCloseSessionValidatesInput(t *testing.T) {
	store := newTestStore(t, t.TempDir())

	if err := store.CloseSession(""); err == nil || !strings.Contains(err.Error(), "session ID is empty") {
		t.Fatalf("expected empty ID error, got: %v", err)
	}
	if err := store.CloseSession("../outside"); err == nil || !strings.Contains(err.Error(), "invalid session ID") {
		t.Fatalf("expected invalid session ID error, got: %v", err)
	}
}

func TestCloseSessionMissingFile(t *testing.T) {
	store := newTestStore(t, t.TempDir())

	err := store.CloseSession("nonexistent")
	if err == nil || !strings.Contains(err.Error(), "session file not found") {
		t.Fatalf("expected not found error, got: %v", err)
	}
}

func newTestStore(t *testing.T, root string) *Store {
	t.Helper()

	store, err := New(root)
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}
	return store
}

func testSession() Session {
	return Session{
		ID:      "2026-01-15T143022Z",
		Author:  "Test Author",
		Started: time.Date(2026, 1, 15, 14, 30, 22, 0, time.UTC),
		Branch:  "main",
		Status:  "active",
	}
}

func TestGetSession(t *testing.T) {
	root := t.TempDir()
	store := newTestStore(t, root)
	sess := testSession()

	if err := store.WriteSession(sess, "Implement auth middleware"); err != nil {
		t.Fatalf("WriteSession failed: %v", err)
	}

	rec, err := store.GetSession(sess.ID)
	if err != nil {
		t.Fatalf("GetSession failed: %v", err)
	}
	if rec.ID != sess.ID {
		t.Errorf("expected ID %q, got %q", sess.ID, rec.ID)
	}
	if rec.Author != sess.Author {
		t.Errorf("expected Author %q, got %q", sess.Author, rec.Author)
	}
	if !rec.Started.Equal(sess.Started) {
		t.Errorf("expected Started %v, got %v", sess.Started, rec.Started)
	}
	if rec.Branch != sess.Branch {
		t.Errorf("expected Branch %q, got %q", sess.Branch, rec.Branch)
	}
	if rec.Status != sess.Status {
		t.Errorf("expected Status %q, got %q", sess.Status, rec.Status)
	}
	if rec.Closed {
		t.Error("expected Closed to be false for active session")
	}
}

func TestGetSessionClosed(t *testing.T) {
	root := t.TempDir()
	store := newTestStore(t, root)
	sess := testSession()

	if err := store.WriteSession(sess, "Implement auth middleware"); err != nil {
		t.Fatalf("WriteSession failed: %v", err)
	}
	if err := store.CloseSession(sess.ID); err != nil {
		t.Fatalf("CloseSession failed: %v", err)
	}

	rec, err := store.GetSession(sess.ID)
	if err != nil {
		t.Fatalf("GetSession failed: %v", err)
	}
	if !rec.Closed {
		t.Error("expected Closed to be true after CloseSession")
	}
	if rec.Status != "active" {
		t.Errorf("expected Status to remain active, got %q", rec.Status)
	}
}

func TestGetSessionMissing(t *testing.T) {
	store := newTestStore(t, t.TempDir())

	_, err := store.GetSession("nonexistent")
	if err == nil || !strings.Contains(err.Error(), "session file not found") {
		t.Fatalf("expected not found error, got: %v", err)
	}
}

func TestGetSessionInvalidID(t *testing.T) {
	store := newTestStore(t, t.TempDir())

	if _, err := store.GetSession(""); err == nil || !strings.Contains(err.Error(), "session ID is empty") {
		t.Fatalf("expected empty ID error, got: %v", err)
	}
	if _, err := store.GetSession("../outside"); err == nil || !strings.Contains(err.Error(), "invalid session ID") {
		t.Fatalf("expected invalid session ID error, got: %v", err)
	}
}

func TestListSessions(t *testing.T) {
	root := t.TempDir()
	store := newTestStore(t, root)

	sess1 := Session{
		ID:      "2026-01-15T140000Z",
		Author:  "Alice",
		Started: time.Date(2026, 1, 15, 14, 0, 0, 0, time.UTC),
		Branch:  "main",
		Status:  "active",
	}
	sess2 := Session{
		ID:      "2026-01-15T150000Z",
		Author:  "Bob",
		Started: time.Date(2026, 1, 15, 15, 0, 0, 0, time.UTC),
		Branch:  "feat/widgets",
		Status:  "active",
	}
	sess3 := Session{
		ID:      "2026-01-15T160000Z",
		Author:  "Carol",
		Started: time.Date(2026, 1, 15, 16, 0, 0, 0, time.UTC),
		Branch:  "fix/bug",
		Status:  "active",
	}

	if err := store.WriteSession(sess1, "first"); err != nil {
		t.Fatalf("WriteSession 1 failed: %v", err)
	}
	if err := store.WriteSession(sess2, "second"); err != nil {
		t.Fatalf("WriteSession 2 failed: %v", err)
	}
	if err := store.WriteSession(sess3, "third"); err != nil {
		t.Fatalf("WriteSession 3 failed: %v", err)
	}

	records, err := store.ListSessions()
	if err != nil {
		t.Fatalf("ListSessions failed: %v", err)
	}
	if len(records) != 3 {
		t.Fatalf("expected 3 records, got %d", len(records))
	}

	if records[0].ID != sess1.ID {
		t.Errorf("expected first record %q, got %q", sess1.ID, records[0].ID)
	}
	if records[1].ID != sess2.ID {
		t.Errorf("expected second record %q, got %q", sess2.ID, records[1].ID)
	}
	if records[2].ID != sess3.ID {
		t.Errorf("expected third record %q, got %q", sess3.ID, records[2].ID)
	}
}

func TestListSessionsEmpty(t *testing.T) {
	store := newTestStore(t, t.TempDir())

	records, err := store.ListSessions()
	if err != nil {
		t.Fatalf("ListSessions failed: %v", err)
	}
	if len(records) != 0 {
		t.Fatalf("expected 0 records, got %d", len(records))
	}
}

func TestListSessionsChronological(t *testing.T) {
	root := t.TempDir()
	store := newTestStore(t, root)

	sessEarly := Session{
		ID:      "2026-01-15T080000Z",
		Author:  "Alice",
		Started: time.Date(2026, 1, 15, 8, 0, 0, 0, time.UTC),
		Branch:  "main",
		Status:  "active",
	}
	sessLate := Session{
		ID:      "2026-01-15T180000Z",
		Author:  "Bob",
		Started: time.Date(2026, 1, 15, 18, 0, 0, 0, time.UTC),
		Branch:  "feat/z",
		Status:  "active",
	}

	if err := store.WriteSession(sessLate, "late"); err != nil {
		t.Fatalf("WriteSession late failed: %v", err)
	}
	if err := store.WriteSession(sessEarly, "early"); err != nil {
		t.Fatalf("WriteSession early failed: %v", err)
	}

	records, err := store.ListSessions()
	if err != nil {
		t.Fatalf("ListSessions failed: %v", err)
	}
	if len(records) != 2 {
		t.Fatalf("expected 2 records, got %d", len(records))
	}

	if records[0].ID != sessEarly.ID {
		t.Errorf("expected first record %q (earlier), got %q", sessEarly.ID, records[0].ID)
	}
	if records[1].ID != sessLate.ID {
		t.Errorf("expected second record %q (later), got %q", sessLate.ID, records[1].ID)
	}
}

func TestListSessionsSkipsNonMD(t *testing.T) {
	root := t.TempDir()
	store := newTestStore(t, root)

	sess := testSession()
	if err := store.WriteSession(sess, "start"); err != nil {
		t.Fatalf("WriteSession failed: %v", err)
	}

	gitkeep := filepath.Join(root, sessionsDir, ".gitkeep")
	if err := os.WriteFile(gitkeep, []byte{}, 0644); err != nil {
		t.Fatalf("write .gitkeep failed: %v", err)
	}

	records, err := store.ListSessions()
	if err != nil {
		t.Fatalf("ListSessions failed: %v", err)
	}
	if len(records) != 1 {
		t.Fatalf("expected 1 record (skipping .gitkeep), got %d", len(records))
	}
}

func TestParseSessionFileMissingOpeningDelimiter(t *testing.T) {
	_, err := parseSessionFile([]byte("no delimiter here"))
	if err == nil || !strings.Contains(err.Error(), "missing opening front-matter delimiter") {
		t.Fatalf("expected missing opening delimiter error, got: %v", err)
	}
}

func TestParseSessionFileMissingClosingDelimiter(t *testing.T) {
	_, err := parseSessionFile([]byte("---\nid: x\n"))
	if err == nil || !strings.Contains(err.Error(), "missing closing front-matter delimiter") {
		t.Fatalf("expected missing closing delimiter error, got: %v", err)
	}
}

func readSessionFile(t *testing.T, root, sessionID string) string {
	t.Helper()

	path := filepath.Join(root, sessionsDir, sessionID+".md")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read session file failed: %v", err)
	}
	return string(data)
}
