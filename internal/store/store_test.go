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

func readSessionFile(t *testing.T, root, sessionID string) string {
	t.Helper()

	path := filepath.Join(root, sessionsDir, sessionID+".md")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read session file failed: %v", err)
	}
	return string(data)
}
