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
		StatusOpen:  true,
		StatusDone:  true,
		Status(""):  false,
		Status("x"): false,
	}
	for s, want := range cases {
		if got := s.Valid(); got != want {
			t.Errorf("Status(%q).Valid() = %v, want %v", s, got, want)
		}
	}
}

func TestItemValidateOpen(t *testing.T) {
	it := Item{
		ID:        "opaque-alpha",
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
		ID:        "opaque-beta",
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
}

func TestFilterRespectsIncludeFlags(t *testing.T) {
	open := validOpen()
	completed := time.Date(2026, 1, 15, 15, 0, 0, 0, time.UTC)
	done := open
	done.Status = StatusDone
	done.Completed = &completed

	cases := []struct {
		name   string
		filter Filter
		openOK bool
		doneOK bool
	}{
		{"default", DefaultFilter("", ""), true, false},
		{"open only", Filter{IncludeOpen: true, MatchSessionAny: true, MatchBranchAny: true}, true, false},
		{"open and done", Filter{IncludeOpen: true, IncludeDone: true, MatchSessionAny: true, MatchBranchAny: true}, true, true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.filter.Matches(open); got != tc.openOK {
				t.Errorf("open match = %v, want %v", got, tc.openOK)
			}
			if got := tc.filter.Matches(done); got != tc.doneOK {
				t.Errorf("done match = %v, want %v", got, tc.doneOK)
			}
		})
	}
}

func TestFilterIncludesRepoWideTodos(t *testing.T) {
	repoWide := validOpen()
	repoWide.SessionID = ""
	repoWide.Branch = ""

	if !DefaultFilter("sess-1", "feat/a").Matches(repoWide) {
		t.Fatal("repo-wide todo (empty SessionID/Branch) should match scoped default filter")
	}

	sessionFilter := Filter{IncludeOpen: true, SessionID: "sess-1", Branch: "feat/a"}
	if !sessionFilter.Matches(repoWide) {
		t.Fatal("repo-wide todo should match filter with specific session and branch")
	}

	repoSessionOnly := validOpen()
	repoSessionOnly.SessionID = ""
	repoSessionOnly.Branch = "feat/a"
	if !sessionFilter.Matches(repoSessionOnly) {
		t.Fatal("repo-session todo (empty SessionID) should match scoped filter")
	}

	repoBranchOnly := validOpen()
	repoBranchOnly.SessionID = "sess-1"
	repoBranchOnly.Branch = ""
	if !sessionFilter.Matches(repoBranchOnly) {
		t.Fatal("repo-branch todo (empty Branch) should match scoped filter")
	}

	scoped := validOpen()
	scoped.SessionID = "sess-1"
	scoped.Branch = "feat/a"
	if !sessionFilter.Matches(scoped) {
		t.Fatal("fully scoped todo should still match")
	}

	wrongSession := validOpen()
	wrongSession.SessionID = "sess-2"
	wrongSession.Branch = "feat/a"
	if sessionFilter.Matches(wrongSession) {
		t.Fatal("wrong session non-repo todo should not match")
	}
}

func validOpen() Item {
	return Item{
		ID:        "opaque-alpha",
		Text:      "refactor parser",
		Status:    StatusOpen,
		CreatedAt: time.Date(2026, 1, 15, 14, 30, 22, 0, time.UTC),
		UpdatedAt: time.Date(2026, 1, 15, 14, 30, 22, 0, time.UTC),
		SessionID: "2026-01-15T143022Z",
		Branch:    "feat/todo",
	}
}
