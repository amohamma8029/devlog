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

const eventTimeLayout = "2006-01-02 15:04"

// SessionEvent represents a structured event parsed from a session Markdown body.
type SessionEvent struct {
	Type string // Start, Note, Blocker, Stop
	Time time.Time
	Body string
}

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

// ReadSessionBody reads the session file and returns the Markdown body (everything
// after the YAML front-matter delimiters).
func (s *Store) ReadSessionBody(sessionID string) (string, error) {
	if err := validateSessionID("ReadSessionBody", sessionID); err != nil {
		return "", err
	}

	content, err := s.ReadSessionContent(sessionID)
	if err != nil {
		return "", err
	}

	body, err := ExtractMarkdownBody(content)
	if err != nil {
		return "", fmt.Errorf("ReadSessionBody: %w", err)
	}

	return body, nil
}

// ReadSessionStartMessage returns the first non-empty line from a session's Start event body.
func (s *Store) ReadSessionStartMessage(sessionID string) (string, error) {
	body, err := s.ReadSessionBody(sessionID)
	if err != nil {
		return "", err
	}

	for _, event := range ParseSessionEvents(body) {
		if event.Type == "Start" {
			return firstNonEmptyLine(event.Body), nil
		}
	}

	return "", nil
}

// ReadSessionContent reads the raw session file content (including YAML front-matter).
func (s *Store) ReadSessionContent(sessionID string) (string, error) {
	if err := validateSessionID("ReadSessionContent", sessionID); err != nil {
		return "", err
	}

	path, err := s.sessionPath(sessionID)
	if err != nil {
		return "", fmt.Errorf("ReadSessionContent: resolve session path: %w", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("ReadSessionContent: session file not found: %s", path)
		}
		return "", fmt.Errorf("ReadSessionContent: read file: %w", err)
	}

	content := strings.ReplaceAll(string(data), "\r\n", "\n")
	return content, nil
}

// ParseSessionEvents parses a Markdown body (from a session file) into structured events.
func ParseSessionEvents(body string) []SessionEvent {
	body = strings.ReplaceAll(body, "\r\n", "\n")
	events, _ := parseSessionEvents(body)
	return events
}

// parseSessionEvents parses events and returns them along with whether a Start event was found.
func parseSessionEvents(body string) ([]SessionEvent, bool) {
	lines := strings.Split(body, "\n")
	var events []SessionEvent
	var current *SessionEvent
	var bodyLines []string
	hasStart := false

	flush := func() {
		if current == nil {
			return
		}
		current.Body = strings.TrimSpace(strings.Join(bodyLines, "\n"))
		events = append(events, *current)
	}

	for _, line := range lines {
		eventType, at, ok := parseEventHeading(line)
		if ok {
			flush()
			current = &SessionEvent{Type: eventType, Time: at}
			bodyLines = nil
			if eventType == "Start" {
				hasStart = true
			}
			continue
		}

		if current != nil {
			bodyLines = append(bodyLines, line)
		}
	}

	flush()
	return events, hasStart
}

func parseEventHeading(line string) (string, time.Time, bool) {
	if !strings.HasPrefix(line, "## ") {
		return "", time.Time{}, false
	}

	heading := strings.TrimSpace(strings.TrimPrefix(line, "## "))
	for _, eventType := range []string{"Start", "Note", "Blocker", "Stop"} {
		if heading == eventType {
			return eventType, time.Time{}, true
		}

		prefix := eventType + " - "
		if strings.HasPrefix(heading, prefix) {
			at, err := time.Parse(eventTimeLayout+" UTC", strings.TrimSpace(strings.TrimPrefix(heading, prefix)))
			if err != nil {
				return "", time.Time{}, false
			}
			return eventType, at, true
		}
	}

	return "", time.Time{}, false
}

// ExtractMarkdownBody returns everything after the YAML front-matter delimiters.
func ExtractMarkdownBody(content string) (string, error) {
	const delim = "---\n"
	if !strings.HasPrefix(content, delim) {
		return "", fmt.Errorf("missing opening front-matter delimiter")
	}

	parts := strings.SplitN(content[len(delim):], delim, 2)
	if len(parts) < 2 {
		return "", fmt.Errorf("missing closing front-matter delimiter")
	}

	return parts[1], nil
}

func firstNonEmptyLine(body string) string {
	for _, line := range strings.Split(body, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed != "" {
			return trimmed
		}
	}
	return ""
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
	section.WriteString(fmt.Sprintf("\n## %s - %s UTC\n", eventType, at.UTC().Format(eventTimeLayout)))
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
		return records[i].Started.After(records[j].Started)
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

	closed := false
	events, _ := parseSessionEvents(parts[1])
	for _, e := range events {
		if e.Type == "Stop" {
			closed = true
			break
		}
	}

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
