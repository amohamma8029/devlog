package session

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/amo/devlog/internal/store"
)

func TestFindActiveSession(t *testing.T) {
	root := t.TempDir()
	s, err := store.New(root)
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}

	sess := store.Session{
		ID:      "2026-01-15T140000Z",
		Author:  "Alice",
		Started: time.Date(2026, 1, 15, 14, 0, 0, 0, time.UTC),
		Branch:  "main",
		Status:  "active",
	}
	if err := s.WriteSession(sess, "start message"); err != nil {
		t.Fatalf("WriteSession failed: %v", err)
	}

	active, err := FindActiveSession(s)
	if err != nil {
		t.Fatalf("FindActiveSession failed: %v", err)
	}
	if active.ID != sess.ID {
		t.Errorf("expected ID %q, got %q", sess.ID, active.ID)
	}
	if active.Closed {
		t.Error("expected active session to have Closed == false")
	}
}

func TestFindActiveSessionNone(t *testing.T) {
	s, err := store.New(t.TempDir())
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}

	_, err = FindActiveSession(s)
	if err == nil || !strings.Contains(err.Error(), "no active session is in progress") {
		t.Fatalf("expected no active session error, got: %v", err)
	}
}

func TestFindActiveSessionMultiple(t *testing.T) {
	root := t.TempDir()
	s, err := store.New(root)
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}

	sess1 := store.Session{
		ID:      "2026-01-15T140000Z",
		Author:  "Alice",
		Started: time.Date(2026, 1, 15, 14, 0, 0, 0, time.UTC),
		Branch:  "main",
		Status:  "active",
	}
	sess2 := store.Session{
		ID:      "2026-01-15T150000Z",
		Author:  "Bob",
		Started: time.Date(2026, 1, 15, 15, 0, 0, 0, time.UTC),
		Branch:  "feat/widgets",
		Status:  "active",
	}

	if err := s.WriteSession(sess1, "first"); err != nil {
		t.Fatalf("WriteSession 1 failed: %v", err)
	}
	if err := s.WriteSession(sess2, "second"); err != nil {
		t.Fatalf("WriteSession 2 failed: %v", err)
	}

	_, err = FindActiveSession(s)
	if err == nil || !strings.Contains(err.Error(), "more than one active session exists") {
		t.Fatalf("expected multiple active sessions error, got: %v", err)
	}
}

func TestFindActiveSessionAllClosed(t *testing.T) {
	root := t.TempDir()
	s, err := store.New(root)
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}

	sess := store.Session{
		ID:      "2026-01-15T140000Z",
		Author:  "Alice",
		Started: time.Date(2026, 1, 15, 14, 0, 0, 0, time.UTC),
		Branch:  "main",
		Status:  "active",
	}
	if err := s.WriteSession(sess, "start"); err != nil {
		t.Fatalf("WriteSession failed: %v", err)
	}
	if err := s.CloseSession(sess.ID); err != nil {
		t.Fatalf("CloseSession failed: %v", err)
	}

	_, err = FindActiveSession(s)
	if err == nil || !strings.Contains(err.Error(), "no active session is in progress") {
		t.Fatalf("expected no active session error, got: %v", err)
	}
}

func TestFindActiveSessionOneActiveAmongClosed(t *testing.T) {
	root := t.TempDir()
	s, err := store.New(root)
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}

	closed := store.Session{
		ID:      "2026-01-15T100000Z",
		Author:  "Alice",
		Started: time.Date(2026, 1, 15, 10, 0, 0, 0, time.UTC),
		Branch:  "main",
		Status:  "active",
	}
	active := store.Session{
		ID:      "2026-01-15T140000Z",
		Author:  "Bob",
		Started: time.Date(2026, 1, 15, 14, 0, 0, 0, time.UTC),
		Branch:  "feat/widgets",
		Status:  "active",
	}

	if err := s.WriteSession(closed, "old session"); err != nil {
		t.Fatalf("WriteSession closed failed: %v", err)
	}
	if err := s.CloseSession(closed.ID); err != nil {
		t.Fatalf("CloseSession failed: %v", err)
	}
	if err := s.WriteSession(active, "current session"); err != nil {
		t.Fatalf("WriteSession active failed: %v", err)
	}

	got, err := FindActiveSession(s)
	if err != nil {
		t.Fatalf("FindActiveSession failed: %v", err)
	}
	if got.ID != active.ID {
		t.Errorf("expected active session %q, got %q", active.ID, got.ID)
	}
	if got.Closed {
		t.Error("expected active session to have Closed == false")
	}
}

func TestFindActiveSessionNilStore(t *testing.T) {
	_, err := FindActiveSession(nil)
	if err == nil || !strings.Contains(err.Error(), "store is nil") {
		t.Fatalf("expected nil store error, got: %v", err)
	}
}

func TestOpenSessionSucceeds(t *testing.T) {
	root := t.TempDir()
	s := newTestStore(t, root)
	sess := testSession("2026-01-15T140000Z", time.Date(2026, 1, 15, 14, 0, 0, 0, time.UTC))

	if err := OpenSession(s, sess, "start message"); err != nil {
		t.Fatalf("OpenSession failed: %v", err)
	}

	active, err := FindActiveSession(s)
	if err != nil {
		t.Fatalf("FindActiveSession failed: %v", err)
	}
	if active.ID != sess.ID {
		t.Errorf("expected active session %q, got %q", sess.ID, active.ID)
	}
}

func TestOpenSessionFailsWhenActiveExists(t *testing.T) {
	root := t.TempDir()
	s := newTestStore(t, root)
	active := testSession("2026-01-15T140000Z", time.Date(2026, 1, 15, 14, 0, 0, 0, time.UTC))
	next := testSession("2026-01-15T150000Z", time.Date(2026, 1, 15, 15, 0, 0, 0, time.UTC))

	if err := OpenSession(s, active, "active session"); err != nil {
		t.Fatalf("OpenSession active failed: %v", err)
	}

	err := OpenSession(s, next, "new session")
	if err == nil || !strings.Contains(err.Error(), "a session is already active ("+active.ID+")") {
		t.Fatalf("expected active session error, got: %v", err)
	}
	if _, err := os.Stat(sessionFilePath(root, next.ID)); !os.IsNotExist(err) {
		t.Fatalf("expected new session file not to exist, stat error: %v", err)
	}
}

func TestOpenSessionDuplicateIDFails(t *testing.T) {
	root := t.TempDir()
	s := newTestStore(t, root)
	sess := testSession("2026-01-15T140000Z", time.Date(2026, 1, 15, 14, 0, 0, 0, time.UTC))

	if err := OpenSession(s, sess, "original session"); err != nil {
		t.Fatalf("OpenSession failed: %v", err)
	}
	if err := CloseActiveSession(s); err != nil {
		t.Fatalf("CloseActiveSession failed: %v", err)
	}
	before := readSessionFile(t, root, sess.ID)

	err := OpenSession(s, sess, "duplicate session")
	if err == nil || !strings.Contains(err.Error(), "session file already exists") {
		t.Fatalf("expected duplicate ID error, got: %v", err)
	}
	after := readSessionFile(t, root, sess.ID)
	if after != before {
		t.Fatal("expected duplicate OpenSession to leave existing session file unchanged")
	}
}

func TestAppendEventToActiveSessionSucceeds(t *testing.T) {
	root := t.TempDir()
	s := newTestStore(t, root)
	sess := testSession("2026-01-15T140000Z", time.Date(2026, 1, 15, 14, 0, 0, 0, time.UTC))

	if err := OpenSession(s, sess, "start message"); err != nil {
		t.Fatalf("OpenSession failed: %v", err)
	}
	if err := AppendEventToActiveSession(s, "Note", "Refactored JWT package"); err != nil {
		t.Fatalf("AppendEventToActiveSession failed: %v", err)
	}

	content := readSessionFile(t, root, sess.ID)
	if !strings.Contains(content, "## Note - ") {
		t.Errorf("expected body to contain Note event")
	}
	if !strings.Contains(content, "Refactored JWT package") {
		t.Errorf("expected body to contain note text")
	}
}

func TestAppendEventToActiveSessionFailsWhenNoneActive(t *testing.T) {
	s := newTestStore(t, t.TempDir())

	err := AppendEventToActiveSession(s, "Note", "body")
	if err == nil || !strings.Contains(err.Error(), "no active session is in progress") {
		t.Fatalf("expected no active session error, got: %v", err)
	}
}

func TestAppendEventToActiveSessionFailsWhenSessionClosed(t *testing.T) {
	root := t.TempDir()
	s := newTestStore(t, root)
	sess := testSession("2026-01-15T140000Z", time.Date(2026, 1, 15, 14, 0, 0, 0, time.UTC))

	if err := OpenSession(s, sess, "start message"); err != nil {
		t.Fatalf("OpenSession failed: %v", err)
	}
	if err := CloseActiveSession(s); err != nil {
		t.Fatalf("CloseActiveSession failed: %v", err)
	}
	before := readSessionFile(t, root, sess.ID)

	err := AppendEventToActiveSession(s, "Note", "body")
	if err == nil || !strings.Contains(err.Error(), "no active session is in progress") {
		t.Fatalf("expected no active session error, got: %v", err)
	}
	after := readSessionFile(t, root, sess.ID)
	if after != before {
		t.Fatal("expected append after close to leave session file unchanged")
	}
}

func TestCloseActiveSessionFailsWhenNoneActive(t *testing.T) {
	s := newTestStore(t, t.TempDir())

	err := CloseActiveSession(s)
	if err == nil || !strings.Contains(err.Error(), "no active session is in progress") {
		t.Fatalf("expected no active session error, got: %v", err)
	}
}

func TestCloseActiveSessionSucceeds(t *testing.T) {
	s := newTestStore(t, t.TempDir())
	sess := testSession("2026-01-15T140000Z", time.Date(2026, 1, 15, 14, 0, 0, 0, time.UTC))

	if err := OpenSession(s, sess, "start message"); err != nil {
		t.Fatalf("OpenSession failed: %v", err)
	}
	if err := CloseActiveSession(s); err != nil {
		t.Fatalf("CloseActiveSession failed: %v", err)
	}

	rec, err := s.GetSession(sess.ID)
	if err != nil {
		t.Fatalf("GetSession failed: %v", err)
	}
	if !rec.Closed {
		t.Error("expected session to be closed")
	}
}

func newTestStore(t *testing.T, root string) *store.Store {
	t.Helper()

	s, err := store.New(root)
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}
	return s
}

func testSession(id string, started time.Time) store.Session {
	return store.Session{
		ID:      id,
		Author:  "Test Author",
		Started: started,
		Branch:  "main",
		Status:  "active",
	}
}

func readSessionFile(t *testing.T, root, sessionID string) string {
	t.Helper()

	data, err := os.ReadFile(sessionFilePath(root, sessionID))
	if err != nil {
		t.Fatalf("read session file failed: %v", err)
	}
	return string(data)
}

func sessionFilePath(root, sessionID string) string {
	return filepath.Join(root, ".devlog", "sessions", sessionID+".md")
}
