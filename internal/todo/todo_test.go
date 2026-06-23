package todo

import (
	"strings"
	"testing"
	"time"
)

func TestLogPathStable(t *testing.T) {
	if got, want := LogPath(), ".devlog/todos.md"; got != want {
		t.Fatalf("LogPath() = %q, want %q", got, want)
	}
}

func TestStatusValid(t *testing.T) {
	cases := map[Status]bool{
		StatusOpen: true,
		StatusDone: true,
		Status(""):  false,
		Status("x"): false,
	}
	for s, want := range cases {
		if got := s.Valid(); got != want {
			t.Errorf("Status(%q).Valid() = %v, want %v", s, got, want)
		}
	}
}

func TestActionValid(t *testing.T) {
	cases := map[Action]bool{
		ActionAdd:      true,
		ActionUpdate:   true,
		ActionComplete: true,
		ActionReopen:   true,
		ActionDelete:   true,
		Action(""):     false,
		Action("x"):    false,
	}
	for a, want := range cases {
		if got := a.Valid(); got != want {
			t.Errorf("Action(%q).Valid() = %v, want %v", a, got, want)
		}
	}
}

func TestItemValidateOpen(t *testing.T) {
	it := Item{
		ID:        "2026-01-15T143022Z-000001",
		Text:      "refactor parser",
		Status:    StatusOpen,
		CreatedAt: time.Date(2026, 1, 15, 14, 30, 22, 0, time.UTC),
		UpdatedAt: time.Date(2026, 1, 15, 14, 30, 22, 0, time.UTC),
	}
	if err := it.Validate(); err != nil {
		t.Fatalf("open item should validate, got: %v", err)
	}
}

func TestItemValidateDone(t *testing.T) {
	completed := time.Date(2026, 1, 15, 15, 0, 0, 0, time.UTC)
	it := Item{
		ID:        "2026-01-15T143022Z-000002",
		Text:      "ship auth",
		Status:    StatusDone,
		CreatedAt: time.Date(2026, 1, 15, 14, 30, 22, 0, time.UTC),
		UpdatedAt: completed,
		Completed: &completed,
	}
	if err := it.Validate(); err != nil {
		t.Fatalf("done item with completed_at should validate, got: %v", err)
	}
}

func TestItemValidateRejectsEmptyID(t *testing.T) {
	it := validOpen()
	it.ID = "  "
	if err := it.Validate(); err == nil || !strings.Contains(err.Error(), "id is empty") {
		t.Fatalf("expected id empty error, got: %v", err)
	}
}

func TestItemValidateRejectsEmptyText(t *testing.T) {
	it := validOpen()
	it.Text = "  "
	if err := it.Validate(); err == nil || !strings.Contains(err.Error(), "text is empty") {
		t.Fatalf("expected text empty error, got: %v", err)
	}
}

func TestItemValidateRejectsUnknownStatus(t *testing.T) {
	it := validOpen()
	it.Status = Status("archived")
	if err := it.Validate(); err == nil || !strings.Contains(err.Error(), "status") {
		t.Fatalf("expected status error, got: %v", err)
	}
}

func TestItemValidateRejectsZeroCreatedAt(t *testing.T) {
	it := validOpen()
	it.CreatedAt = time.Time{}
	if err := it.Validate(); err == nil || !strings.Contains(err.Error(), "created_at") {
		t.Fatalf("expected created_at error, got: %v", err)
	}
}

func TestItemValidateDoneRequiresCompleted(t *testing.T) {
	it := validOpen()
	it.Status = StatusDone
	if err := it.Validate(); err == nil || !strings.Contains(err.Error(), "completed_at is nil") {
		t.Fatalf("expected completed_at required error, got: %v", err)
	}
}

func TestItemValidateOpenRejectsCompleted(t *testing.T) {
	completed := time.Date(2026, 1, 15, 15, 0, 0, 0, time.UTC)
	it := validOpen()
	it.Completed = &completed
	if err := it.Validate(); err == nil || !strings.Contains(err.Error(), "completed_at is set") {
		t.Fatalf("expected completed_at conflict error, got: %v", err)
	}
}

func TestDefaultFilterScopesToSessionAndBranch(t *testing.T) {
	f := DefaultFilter("2026-01-15T143022Z", "feat/todo")
	open := validOpen()
	open.SessionID = "2026-01-15T143022Z"
	open.Branch = "feat/todo"
	if !f.Matches(open) {
		t.Fatal("default filter should match an open todo in the active session/branch")
	}

	otherSession := validOpen()
	otherSession.SessionID = "2026-01-16T090000Z"
	otherSession.Branch = "feat/todo"
	if f.Matches(otherSession) {
		t.Fatal("default filter should not match a different session")
	}

	otherBranch := validOpen()
	otherBranch.SessionID = "2026-01-15T143022Z"
	otherBranch.Branch = "feat/other"
	if f.Matches(otherBranch) {
		t.Fatal("default filter should not match a different branch")
	}

	done := validOpen()
	done.SessionID = "2026-01-15T143022Z"
	done.Branch = "feat/todo"
	completed := time.Date(2026, 1, 15, 15, 0, 0, 0, time.UTC)
	done.Status = StatusDone
	done.Completed = &completed
	if f.Matches(done) {
		t.Fatal("default filter should not match done todos")
	}
}

func TestAllFilterSurfacesEverything(t *testing.T) {
	f := AllFilter()
	open := validOpen()
	if !f.Matches(open) {
		t.Fatal("all filter should match open items")
	}

	completed := time.Date(2026, 1, 15, 15, 0, 0, 0, time.UTC)
	done := open
	done.Status = StatusDone
	done.Completed = &completed
	if !f.Matches(done) {
		t.Fatal("all filter should match done items")
	}

	deleted := open
	deleted.Deleted = true
	if !f.Matches(deleted) {
		t.Fatal("all filter should match deleted items")
	}
}

func TestFilterRespectsIncludeFlags(t *testing.T) {
	open := validOpen()
	deleted := open
	deleted.Deleted = true
	completed := time.Date(2026, 1, 15, 15, 0, 0, 0, time.UTC)
	done := open
	done.Status = StatusDone
	done.Completed = &completed

	cases := []struct {
		name    string
		filter  Filter
		openOK  bool
		doneOK  bool
		delOK   bool
	}{
		{"default", DefaultFilter("", ""), true, false, false},
		{"open only", Filter{IncludeOpen: true, MatchSessionAny: true, MatchBranchAny: true}, true, false, false},
		{"open and done", Filter{IncludeOpen: true, IncludeDone: true, MatchSessionAny: true, MatchBranchAny: true}, true, true, false},
		{"include deleted", Filter{IncludeOpen: true, IncludeDone: true, IncludeDeleted: true, MatchSessionAny: true, MatchBranchAny: true}, true, true, true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.filter.Matches(open); got != tc.openOK {
				t.Errorf("open match = %v, want %v", got, tc.openOK)
			}
			if got := tc.filter.Matches(done); got != tc.doneOK {
				t.Errorf("done match = %v, want %v", got, tc.doneOK)
			}
			if got := tc.filter.Matches(deleted); got != tc.delOK {
				t.Errorf("deleted match = %v, want %v", got, tc.delOK)
			}
		})
	}
}

func TestNewIDIsUniqueAndFormatted(t *testing.T) {
	at := time.Date(2026, 1, 15, 14, 30, 22, 0, time.UTC)
	seen := make(map[string]struct{}, 1000)
	for i := 0; i < 1000; i++ {
		id := NewID(at)
		if !strings.HasPrefix(id, "2026-01-15T143022Z-") {
			t.Fatalf("NewID = %q, missing timestamp prefix", id)
		}
		if _, dup := seen[id]; dup {
			t.Fatalf("NewID %q returned twice in a row", id)
		}
		seen[id] = struct{}{}
	}
}

func validOpen() Item {
	return Item{
		ID:        "2026-01-15T143022Z-000001",
		Text:      "refactor parser",
		Status:    StatusOpen,
		CreatedAt: time.Date(2026, 1, 15, 14, 30, 22, 0, time.UTC),
		UpdatedAt: time.Date(2026, 1, 15, 14, 30, 22, 0, time.UTC),
		SessionID: "2026-01-15T143022Z",
		Branch:    "feat/todo",
	}
}
