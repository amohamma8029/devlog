// Package todo defines the repo-wide todo domain contract used by the CLI,
// TUI, and handoff flows. This file is the production-safe BBA stub: it
// declares the persisted model, status/action vocabulary, filter options,
// and storage path so later slices can build storage, commands, and views
// behind a stable interface. It performs no file I/O.
package todo

import (
	"fmt"
	"strings"
	"time"
)

// FileName is the repo-local filename that holds the append-only todo log.
// The full path is the repository root joined with FileName by the storage
// layer; that responsibility is intentionally out of scope for this stub.
const FileName = "todos.md"

// DirName is the devlog directory that contains the todo log alongside the
// existing session files. Storage layout is owned by the storage slice.
const DirName = ".devlog"

// LogPath returns the slash-separated repo-relative path to the todo log.
// It performs no filesystem checks; the storage slice owns validation.
func LogPath() string {
	return DirName + "/" + FileName
}

// Status is the persisted state of a todo item.
type Status string

const (
	// StatusOpen is the default state for a newly added todo.
	StatusOpen Status = "open"
	// StatusDone marks a todo that has been completed.
	StatusDone Status = "done"
)

// Valid reports whether s is a recognized todo status.
func (s Status) Valid() bool {
	switch s {
	case StatusOpen, StatusDone:
		return true
	}
	return false
}

// Action is the kind of mutation recorded against a todo. The storage layer
// uses actions to model full CRUD history in an append-only log.
type Action string

const (
	// ActionAdd records a new todo.
	ActionAdd Action = "add"
	// ActionUpdate records a body edit on an existing todo.
	ActionUpdate Action = "update"
	// ActionComplete records a transition from open to done.
	ActionComplete Action = "complete"
	// ActionReopen records a transition from done to open.
	ActionReopen Action = "reopen"
	// ActionDelete records a hide/delete against a todo. The storage layer
	// keeps the original item visible with Deleted=true so history is
	// preserved.
	ActionDelete Action = "delete"
)

// Valid reports whether a is a recognized todo action.
func (a Action) Valid() bool {
	switch a {
	case ActionAdd, ActionUpdate, ActionComplete, ActionReopen, ActionDelete:
		return true
	}
	return false
}

// Item is the projected view of a todo after applying the append-only log.
// Deleted items remain present so status and handoff output can surface
// deletions as annotations rather than silently dropping history.
type Item struct {
	ID        string     `yaml:"id"`
	Text      string     `yaml:"text"`
	Status    Status     `yaml:"status"`
	CreatedAt time.Time  `yaml:"created_at"`
	UpdatedAt time.Time  `yaml:"updated_at"`
	Completed *time.Time `yaml:"completed_at,omitempty"`
	SessionID string     `yaml:"session_id,omitempty"`
	Branch    string     `yaml:"branch,omitempty"`
	Deleted   bool       `yaml:"deleted,omitempty"`
}

// Filter narrows which projected todos a view or command should surface.
// The zero value is intentionally useful: it returns open, non-deleted
// todos scoped to the active session/branch.
type Filter struct {
	IncludeOpen     bool
	IncludeDone     bool
	IncludeDeleted  bool
	SessionID       string
	Branch          string
	MatchSessionAny bool
	MatchBranchAny  bool
}

// DefaultFilter returns the filter used by status and active-session views:
// open, non-deleted, scoped to the given session or branch.
func DefaultFilter(sessionID, branch string) Filter {
	return Filter{
		IncludeOpen:     true,
		SessionID:       sessionID,
		Branch:          branch,
		MatchSessionAny: sessionID == "",
		MatchBranchAny:  branch == "",
	}
}

// AllFilter returns a filter that surfaces every projected todo regardless
// of state. Deleted items are still excluded unless IncludeDeleted is set.
func AllFilter() Filter {
	return Filter{
		IncludeOpen:    true,
		IncludeDone:    true,
		IncludeDeleted: true,
	}
}

// Matches reports whether the projected item passes the filter.
func (f Filter) Matches(it Item) bool {
	if !f.IncludeOpen && it.Status == StatusOpen {
		return false
	}
	if !f.IncludeDone && it.Status == StatusDone {
		return false
	}
	if !f.IncludeDeleted && it.Deleted {
		return false
	}
	if !f.MatchSessionAny && f.SessionID != "" && it.SessionID != f.SessionID {
		return false
	}
	if !f.MatchBranchAny && f.Branch != "" && it.Branch != f.Branch {
		return false
	}
	return true
}

// Validate reports whether the projected item satisfies the contract.
// Empty IDs or text are rejected; unknown statuses are rejected; CreatedAt
// must be non-zero so the storage layer can correlate log entries.
func (it Item) Validate() error {
	if strings.TrimSpace(it.ID) == "" {
		return fmt.Errorf("todo: id is empty")
	}
	if strings.TrimSpace(it.Text) == "" {
		return fmt.Errorf("todo %q: text is empty", it.ID)
	}
	if !it.Status.Valid() {
		return fmt.Errorf("todo %q: status %q is not recognized", it.ID, it.Status)
	}
	if it.CreatedAt.IsZero() {
		return fmt.Errorf("todo %q: created_at is zero", it.ID)
	}
	if it.Status == StatusDone && it.Completed == nil {
		return fmt.Errorf("todo %q: status is done but completed_at is nil", it.ID)
	}
	if it.Status == StatusOpen && it.Completed != nil {
		return fmt.Errorf("todo %q: status is open but completed_at is set", it.ID)
	}
	return nil
}
