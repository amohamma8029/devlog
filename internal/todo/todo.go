// Package todo defines the repo-wide todo domain contract used by the CLI,
// TUI, and handoff flows. It declares the persisted model, status vocabulary,
// filter options, and storage path so later slices can build commands and
// views behind a stable interface.
package todo

import (
	"fmt"
	"strings"
	"time"
)

// FileName is the repo-local filename that holds the todo list.
// The full path is the repository root joined with FileName by the storage
// layer.
const FileName = "todos.md"

// DirName is the devlog directory that contains the todo file alongside the
// existing session files.
const DirName = ".devlog"

// LogPath returns the slash-separated repo-relative path to the todo file.
// It performs no filesystem checks; the storage layer owns validation.
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

// Item is a todo as stored in the single-state YAML file.
type Item struct {
	ID        string     `yaml:"id"`
	Text      string     `yaml:"text"`
	Status    Status     `yaml:"status"`
	CreatedAt time.Time  `yaml:"created_at"`
	UpdatedAt time.Time  `yaml:"updated_at"`
	Completed *time.Time `yaml:"completed_at,omitempty"`
	SessionID string     `yaml:"session_id,omitempty"`
	Branch    string     `yaml:"branch,omitempty"`
}

// Filter narrows which todos a view or command should surface.
// The zero value is intentionally useful: it returns open todos scoped
// to the active session/branch.
type Filter struct {
	IncludeOpen     bool
	IncludeDone     bool
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

// AllFilter returns a filter that surfaces every todo regardless of state.
func AllFilter() Filter {
	return Filter{
		IncludeOpen: true,
		IncludeDone: true,
	}
}

// Matches reports whether the item passes the filter.
func (f Filter) Matches(it Item) bool {
	if !f.IncludeOpen && it.Status == StatusOpen {
		return false
	}
	if !f.IncludeDone && it.Status == StatusDone {
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

// Validate reports whether the item satisfies the contract.
// Empty IDs or text are rejected; unknown statuses are rejected; CreatedAt
// must be non-zero.
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
