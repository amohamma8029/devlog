// Package session manages devlog session lifecycle.
package session

import (
	"fmt"

	"github.com/amo/devlog/internal/store"
)

// FindActiveSession returns the single active session from the store.
// It returns an error if no active session exists or if multiple are found.
// Active means the session has not been closed (no ## Stop event in the Markdown body).
func FindActiveSession(s *store.Store) (*store.SessionRecord, error) {
	if s == nil {
		return nil, fmt.Errorf("FindActiveSession: store is nil")
	}

	records, err := s.ListSessions()
	if err != nil {
		return nil, fmt.Errorf("FindActiveSession: list sessions: %w", err)
	}

	var active *store.SessionRecord
	for i := range records {
		if !records[i].Closed {
			if active != nil {
				return nil, fmt.Errorf("FindActiveSession: multiple active sessions found")
			}
			active = &records[i]
		}
	}

	if active == nil {
		return nil, fmt.Errorf("FindActiveSession: no active session found")
	}

	return active, nil
}
