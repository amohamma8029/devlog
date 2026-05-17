package session

import (
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
	if err == nil || !strings.Contains(err.Error(), "no active session found") {
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
	if err == nil || !strings.Contains(err.Error(), "multiple active sessions found") {
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
	if err == nil || !strings.Contains(err.Error(), "no active session found") {
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
