// Package session manages devlog session lifecycle.
package session

import (
	"errors"
	"fmt"

	"github.com/amohamma8029/devlog/internal/store"
)

var errNoActiveSession = errors.New(`no active session is in progress. Run "devlog open <message>" to start one.`)

// OpenSession creates a new session only when no active session exists.
func OpenSession(s *store.Store, sess store.Session, startMessage string) error {
	active, err := FindActiveSession(s)
	if err == nil {
		return fmt.Errorf(`a session is already active (%s). Run "devlog close" to end it first.`, active.ID)
	}
	if !errors.Is(err, errNoActiveSession) {
		return err
	}

	if err := s.WriteSession(sess, startMessage); err != nil {
		return err
	}

	return nil
}

// AppendEventToActiveSession appends an event to the single active session.
func AppendEventToActiveSession(s *store.Store, eventType, body string) error {
	active, err := FindActiveSession(s)
	if err != nil {
		return err
	}

	if err := s.AppendEvent(active.ID, eventType, body); err != nil {
		return err
	}

	return nil
}

// CloseActiveSession closes the single active session.
func CloseActiveSession(s *store.Store) error {
	active, err := FindActiveSession(s)
	if err != nil {
		return err
	}

	if err := s.CloseSession(active.ID); err != nil {
		return err
	}

	return nil
}

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
				return nil, fmt.Errorf("more than one active session exists. This is unexpected — check .devlog/sessions/ for open sessions.")
			}
			active = &records[i]
		}
	}

	if active == nil {
		return nil, errNoActiveSession
	}

	return active, nil
}
