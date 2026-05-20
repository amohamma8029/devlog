// Package store handles reading and writing devlog session files.
package store

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// Session represents a devlog session with YAML front-matter metadata.
type Session struct {
	ID      string    `yaml:"id"`
	Author  string    `yaml:"author"`
	Email   string    `yaml:"email,omitempty"`
	Started time.Time `yaml:"started"`
	Branch  string    `yaml:"branch"`
	Status  string    `yaml:"status"`
}

// SessionRecord wraps a Session with read-time derived state.
// Closed is derived from the Markdown body (presence of ## Stop event),
// not from the YAML front-matter.
type SessionRecord struct {
	Session
	Closed bool
}

const sessionsDir = ".devlog/sessions"

// Store reads and writes devlog files under a repository root.
type Store struct {
	root string
}

// New creates a Store scoped to a repository root.
func New(root string) (*Store, error) {
	if strings.TrimSpace(root) == "" {
		return nil, fmt.Errorf("New: root is empty")
	}

	absRoot, err := filepath.Abs(root)
	if err != nil {
		return nil, fmt.Errorf("New: resolve root: %w", err)
	}
	info, err := os.Stat(absRoot)
	if err != nil {
		return nil, fmt.Errorf("New: inspect root: %w", err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("New: root is not a directory: %s", absRoot)
	}

	return &Store{root: absRoot}, nil
}

// WriteSession creates a new session file with YAML front-matter and an initial Markdown body.
func (s *Store) WriteSession(sess Session, startMessage string) error {
	if err := validateSessionID("WriteSession", sess.ID); err != nil {
		return err
	}
	if strings.TrimSpace(startMessage) == "" {
		return fmt.Errorf("WriteSession: start message is empty")
	}

	dir, err := s.sessionsPath()
	if err != nil {
		return fmt.Errorf("WriteSession: resolve sessions directory: %w", err)
	}

	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("WriteSession: create sessions directory: %w", err)
	}

	path, err := s.sessionPath(sess.ID)
	if err != nil {
		return fmt.Errorf("WriteSession: resolve session path: %w", err)
	}

	front, err := yaml.Marshal(sess)
	if err != nil {
		return fmt.Errorf("WriteSession: marshal front-matter: %w", err)
	}

	var body strings.Builder
	body.WriteString("---\n")
	body.Write(front)
	body.WriteString("---\n\n")
	body.WriteString("## Start\n")
	body.WriteString("\n")
	body.WriteString(strings.TrimRight(startMessage, "\n") + "\n")

	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0644)
	if err != nil {
		if os.IsExist(err) {
			return fmt.Errorf("WriteSession: session file already exists: %s", path)
		}
		return fmt.Errorf("WriteSession: create file: %w", err)
	}

	if _, err := f.WriteString(body.String()); err != nil {
		closeErr := f.Close()
		if closeErr != nil {
			return fmt.Errorf("WriteSession: write file: %w (close failed: %v)", err, closeErr)
		}
		return fmt.Errorf("WriteSession: write file: %w", err)
	}

	if err := f.Close(); err != nil {
		return fmt.Errorf("WriteSession: close file: %w", err)
	}

	return nil
}

// AppendEvent appends a Markdown section to an existing session file.
func (s *Store) AppendEvent(sessionID, eventType, body string) error {
	if err := validateSessionID("AppendEvent", sessionID); err != nil {
		return err
	}
	return s.appendEvent("AppendEvent", sessionID, eventType, body, time.Now().UTC())
}

// CloseSession appends a stop marker; readers derive closed state from the event log.
func (s *Store) CloseSession(sessionID string) error {
	if err := validateSessionID("CloseSession", sessionID); err != nil {
		return err
	}
	return s.appendEvent("CloseSession", sessionID, "Stop", "Session closed.", time.Now().UTC())
}

func (s *Store) appendEvent(op, sessionID, eventType, body string, at time.Time) error {
	if strings.TrimSpace(eventType) == "" {
		return fmt.Errorf("%s: event type is empty", op)
	}
	if strings.TrimSpace(body) == "" {
		return fmt.Errorf("%s: event body is empty", op)
	}

	path, err := s.sessionPath(sessionID)
	if err != nil {
		return fmt.Errorf("%s: resolve session path: %w", op, err)
	}

	var section strings.Builder
	section.WriteString(fmt.Sprintf("\n## %s - %s UTC\n", eventType, at.Format("15:04")))
	section.WriteString("\n")
	section.WriteString(strings.TrimRight(body, "\n") + "\n")

	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("%s: session file not found: %s", op, path)
		}
		return fmt.Errorf("%s: open file: %w", op, err)
	}

	if _, err := f.WriteString(section.String()); err != nil {
		closeErr := f.Close()
		if closeErr != nil {
			return fmt.Errorf("%s: write to file: %w (close failed: %v)", op, err, closeErr)
		}
		return fmt.Errorf("%s: write to file: %w", op, err)
	}

	if err := f.Close(); err != nil {
		return fmt.Errorf("%s: close file: %w", op, err)
	}

	return nil
}

// GetSession reads a single session file and returns its metadata plus derived Closed state.
func (s *Store) GetSession(sessionID string) (SessionRecord, error) {
	if err := validateSessionID("GetSession", sessionID); err != nil {
		return SessionRecord{}, err
	}

	path, err := s.sessionPath(sessionID)
	if err != nil {
		return SessionRecord{}, fmt.Errorf("GetSession: resolve session path: %w", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return SessionRecord{}, fmt.Errorf("GetSession: session file not found: %s", path)
		}
		return SessionRecord{}, fmt.Errorf("GetSession: read file: %w", err)
	}

	return parseSessionFile(data)
}

// ListSessions reads all session files from the sessions directory and returns them
// in chronological order (ascending Started time).
func (s *Store) ListSessions() ([]SessionRecord, error) {
	dir, err := s.sessionsPath()
	if err != nil {
		return nil, fmt.Errorf("ListSessions: resolve sessions directory: %w", err)
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return []SessionRecord{}, nil
		}
		return nil, fmt.Errorf("ListSessions: read directory: %w", err)
	}

	var records []SessionRecord
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}
		sessionID := strings.TrimSuffix(entry.Name(), ".md")
		rec, err := s.GetSession(sessionID)
		if err != nil {
			return nil, fmt.Errorf("ListSessions: read %s: %w", sessionID, err)
		}
		records = append(records, rec)
	}

	sort.Slice(records, func(i, j int) bool {
		return records[i].Started.Before(records[j].Started)
	})

	return records, nil
}

// parseSessionFile parses raw session file bytes into a SessionRecord.
func parseSessionFile(data []byte) (SessionRecord, error) {
	content := string(data)
	const delim = "---\n"

	if !strings.HasPrefix(content, delim) {
		return SessionRecord{}, fmt.Errorf("parseSessionFile: missing opening front-matter delimiter")
	}

	parts := strings.SplitN(content[len(delim):], delim, 2)
	if len(parts) < 2 {
		return SessionRecord{}, fmt.Errorf("parseSessionFile: missing closing front-matter delimiter")
	}

	var sess Session
	if err := yaml.Unmarshal([]byte(parts[0]), &sess); err != nil {
		return SessionRecord{}, fmt.Errorf("parseSessionFile: unmarshal front-matter: %w", err)
	}

	closed := strings.Contains(parts[1], "## Stop")

	return SessionRecord{Session: sess, Closed: closed}, nil
}

func (s *Store) sessionsPath() (string, error) {
	if s == nil || strings.TrimSpace(s.root) == "" {
		return "", fmt.Errorf("Store: root is empty")
	}
	return filepath.Join(s.root, sessionsDir), nil
}

func (s *Store) sessionPath(sessionID string) (string, error) {
	dir, err := s.sessionsPath()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, sessionID+".md"), nil
}

func validateSessionID(op, sessionID string) error {
	if strings.TrimSpace(sessionID) == "" {
		return fmt.Errorf("%s: session ID is empty", op)
	}
	if sessionID == "." || sessionID == ".." || strings.ContainsAny(sessionID, `/\`) {
		return fmt.Errorf("%s: invalid session ID: %s", op, sessionID)
	}
	return nil
}
