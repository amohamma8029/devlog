package todo

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// Store persists todo events in an append-only repo-local log and projects
// the current state for callers. All mutating methods append a new event to
// the log; they never rewrite the full file for a single mutation.
type Store struct {
	root string
	now  func() time.Time
}

// NewStore creates a Store scoped to a repository root. The todo log lives
// under the provided root and is created on first mutation.
func NewStore(root string) (*Store, error) {
	if strings.TrimSpace(root) == "" {
		return nil, fmt.Errorf("todo.NewStore: root is empty")
	}

	absRoot, err := filepath.Abs(root)
	if err != nil {
		return nil, fmt.Errorf("todo.NewStore: resolve root: %w", err)
	}
	info, err := os.Stat(absRoot)
	if err != nil {
		return nil, fmt.Errorf("todo.NewStore: inspect root: %w", err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("todo.NewStore: root is not a directory: %s", absRoot)
	}

	return &Store{root: absRoot, now: func() time.Time { return time.Now().UTC() }}, nil
}

// Root returns the absolute repository root the Store is scoped to.
func (s *Store) Root() string {
	return s.root
}

// AddInput is the information a caller provides to create a new todo.
type AddInput struct {
	Text      string
	SessionID string
	Branch    string
}

// Add appends an add event and returns the projected item.
func (s *Store) Add(in AddInput) (Item, error) {
	text := strings.TrimSpace(in.Text)
	if text == "" {
		return Item{}, fmt.Errorf("todo.Store.Add: text is empty")
	}

	at := s.now()
	item := Item{
		ID:        NewID(at),
		Text:      text,
		Status:    StatusOpen,
		CreatedAt: at,
		UpdatedAt: at,
		SessionID: strings.TrimSpace(in.SessionID),
		Branch:    strings.TrimSpace(in.Branch),
	}

	if err := s.appendEntry(entry{Action: ActionAdd, ID: item.ID, At: at, SessionID: item.SessionID, Branch: item.Branch, Status: string(item.Status), Text: text}); err != nil {
		return Item{}, err
	}
	return item, nil
}

// UpdateText changes the body of an existing open todo.
func (s *Store) UpdateText(id, text string) error {
	return s.mutate(id, "todo.Store.UpdateText", func(item Item) (Item, error) {
		newText := strings.TrimSpace(text)
		if newText == "" {
			return Item{}, fmt.Errorf("todo.Store.UpdateText: text is empty")
		}
		if item.Status == StatusDone {
			return Item{}, fmt.Errorf("todo.Store.UpdateText: todo %q is done", id)
		}
		if item.Deleted {
			return Item{}, fmt.Errorf("todo.Store.UpdateText: todo %q is deleted", id)
		}
		item.Text = newText
		item.UpdatedAt = s.now()
		return item, nil
	}, entry{Action: ActionUpdate, ID: id, At: s.now(), Text: text})
}

// Complete marks an open todo as done.
func (s *Store) Complete(id string) error {
	return s.transition(id, ActionComplete, "todo.Store.Complete", func(item Item, at time.Time) (Item, error) {
		if item.Deleted {
			return Item{}, fmt.Errorf("todo.Store.Complete: todo %q is deleted", id)
		}
		if item.Status == StatusDone {
			return Item{}, fmt.Errorf("todo.Store.Complete: todo %q is already done", id)
		}
		item.Status = StatusDone
		item.Completed = &at
		item.UpdatedAt = at
		return item, nil
	})
}

// Reopen moves a done todo back to open.
func (s *Store) Reopen(id string) error {
	return s.transition(id, ActionReopen, "todo.Store.Reopen", func(item Item, at time.Time) (Item, error) {
		if item.Deleted {
			return Item{}, fmt.Errorf("todo.Store.Reopen: todo %q is deleted", id)
		}
		if item.Status == StatusOpen {
			return Item{}, fmt.Errorf("todo.Store.Reopen: todo %q is already open", id)
		}
		item.Status = StatusOpen
		item.Completed = nil
		item.UpdatedAt = at
		return item, nil
	})
}

// Delete marks a todo as deleted; history is preserved in the log.
func (s *Store) Delete(id string) error {
	return s.transition(id, ActionDelete, "todo.Store.Delete", func(item Item, at time.Time) (Item, error) {
		if item.Deleted {
			return Item{}, fmt.Errorf("todo.Store.Delete: todo %q is already deleted", id)
		}
		item.Deleted = true
		item.UpdatedAt = at
		return item, nil
	})
}

// List returns projected todos that match the given filter, ordered by
// CreatedAt ascending and then by ID.
func (s *Store) List(filter Filter) ([]Item, error) {
	items, err := s.Load()
	if err != nil {
		return nil, err
	}
	var out []Item
	for _, it := range items {
		if filter.Matches(it) {
			out = append(out, it)
		}
	}
	return out, nil
}

// Load returns the full projected state of every todo in the log, including
// deleted items, sorted by CreatedAt then ID.
func (s *Store) Load() ([]Item, error) {
	path, err := s.logPath()
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, fmt.Errorf("todo.Store.Load: read log: %w", err)
	}

	entries, err := parseLog(data)
	if err != nil {
		return nil, fmt.Errorf("todo.Store.Load: %w", err)
	}

	items := project(entries)
	sortItems(items)
	return items, nil
}

func (s *Store) mutate(id, op string, transform func(Item) (Item, error), ent entry) error {
	if strings.TrimSpace(id) == "" {
		return fmt.Errorf("%s: id is empty", op)
	}

	items, err := s.Load()
	if err != nil {
		return err
	}

	var match *Item
	for i := range items {
		if items[i].ID == id {
			match = &items[i]
			break
		}
	}
	if match == nil {
		return fmt.Errorf("%s: todo %q not found", op, id)
	}

	next, err := transform(*match)
	if err != nil {
		return err
	}
	if err := next.Validate(); err != nil {
		return fmt.Errorf("%s: %w", op, err)
	}

	ent.ID = id
	ent.At = next.UpdatedAt
	if ent.Text == "" {
		ent.Text = next.Text
	}
	return s.appendEntry(ent)
}

func (s *Store) transition(id string, action Action, op string, transform func(Item, time.Time) (Item, error)) error {
	if strings.TrimSpace(id) == "" {
		return fmt.Errorf("%s: id is empty", op)
	}

	items, err := s.Load()
	if err != nil {
		return err
	}

	var match *Item
	for i := range items {
		if items[i].ID == id {
			match = &items[i]
			break
		}
	}
	if match == nil {
		return fmt.Errorf("%s: todo %q not found", op, id)
	}

	at := s.now()
	next, err := transform(*match, at)
	if err != nil {
		return err
	}
	if err := next.Validate(); err != nil {
		return fmt.Errorf("%s: %w", op, err)
	}

	ent := entry{Action: action, ID: id, At: at, Status: string(next.Status), Text: next.Text}
	if next.Completed != nil {
		ent.Completed = next.Completed
	}
	return s.appendEntry(ent)
}

func (s *Store) appendEntry(ent entry) error {
	if !ent.Action.Valid() {
		return fmt.Errorf("todo.Store.appendEntry: action %q is not recognized", ent.Action)
	}
	if strings.TrimSpace(ent.ID) == "" {
		return fmt.Errorf("todo.Store.appendEntry: id is empty")
	}
	if ent.At.IsZero() {
		return fmt.Errorf("todo.Store.appendEntry: at is zero")
	}
	if err := s.ensureDir(); err != nil {
		return err
	}

	path, err := s.logPath()
	if err != nil {
		return err
	}

	var buf bytes.Buffer
	if err := yaml.NewEncoder(&buf).Encode(ent); err != nil {
		return fmt.Errorf("todo.Store.appendEntry: marshal entry: %w", err)
	}
	buf.WriteString("---\n\n")

	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0644)
	if err != nil {
		return fmt.Errorf("todo.Store.appendEntry: open log: %w", err)
	}
	if _, err := f.Write(buf.Bytes()); err != nil {
		closeErr := f.Close()
		if closeErr != nil {
			return fmt.Errorf("todo.Store.appendEntry: write log: %w (close failed: %v)", err, closeErr)
		}
		return fmt.Errorf("todo.Store.appendEntry: write log: %w", err)
	}
	if err := f.Close(); err != nil {
		return fmt.Errorf("todo.Store.appendEntry: close log: %w", err)
	}
	return nil
}

func (s *Store) ensureDir() error {
	dir := filepath.Join(s.root, DirName)
	info, err := os.Stat(dir)
	if err == nil {
		if info.IsDir() {
			return nil
		}
		return fmt.Errorf("todo.Store: %s exists and is not a directory", dir)
	}
	if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("todo.Store: stat %s: %w", dir, err)
	}
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("todo.Store: create %s: %w", dir, err)
	}
	return nil
}

func (s *Store) logPath() (string, error) {
	if s == nil || strings.TrimSpace(s.root) == "" {
		return "", fmt.Errorf("todo.Store: root is empty")
	}
	return filepath.Join(s.root, LogPath()), nil
}

type entry struct {
	Action    Action     `yaml:"action"`
	ID        string     `yaml:"id"`
	At        time.Time  `yaml:"at"`
	Status    string     `yaml:"status,omitempty"`
	Text      string     `yaml:"text,omitempty"`
	SessionID string     `yaml:"session_id,omitempty"`
	Branch    string     `yaml:"branch,omitempty"`
	Completed *time.Time `yaml:"completed_at,omitempty"`
}

func parseLog(data []byte) ([]entry, error) {
	text := strings.ReplaceAll(string(data), "\r\n", "\n")
	if strings.TrimSpace(text) == "" {
		return nil, nil
	}

	parts := strings.Split(text, "\n---")
	if len(parts) == 0 {
		return nil, nil
	}

	var entries []entry
	for _, part := range parts {
		raw := strings.TrimSpace(part)
		if raw == "" {
			continue
		}
		raw = strings.TrimPrefix(raw, "---")
		raw = strings.TrimSpace(raw)
		if raw == "" {
			return nil, fmt.Errorf("entry has no YAML body")
		}
		var ent entry
		if err := yaml.Unmarshal([]byte(raw), &ent); err != nil {
			return nil, fmt.Errorf("parse entry: %w", err)
		}
		if !ent.Action.Valid() {
			return nil, fmt.Errorf("entry has invalid action %q", ent.Action)
		}
		if strings.TrimSpace(ent.ID) == "" {
			return nil, fmt.Errorf("entry has empty id")
		}
		if ent.At.IsZero() {
			return nil, fmt.Errorf("entry %q has zero at", ent.ID)
		}
		entries = append(entries, ent)
	}
	return entries, nil
}

func project(entries []entry) []Item {
	byID := make(map[string]Item, len(entries))
	for _, ent := range entries {
		switch ent.Action {
		case ActionAdd:
			item := Item{
				ID:        ent.ID,
				Text:      ent.Text,
				Status:    Status(ent.Status),
				CreatedAt: ent.At,
				UpdatedAt: ent.At,
				SessionID: ent.SessionID,
				Branch:    ent.Branch,
			}
			if item.Status == "" {
				item.Status = StatusOpen
			}
			byID[ent.ID] = item
		case ActionUpdate:
			item, ok := byID[ent.ID]
			if !ok {
				continue
			}
			item.Text = ent.Text
			item.UpdatedAt = ent.At
			byID[ent.ID] = item
		case ActionComplete:
			item, ok := byID[ent.ID]
			if !ok {
				continue
			}
			completed := ent.At
			if ent.Completed != nil {
				completed = *ent.Completed
			}
			item.Status = StatusDone
			item.Completed = &completed
			item.UpdatedAt = ent.At
			byID[ent.ID] = item
		case ActionReopen:
			item, ok := byID[ent.ID]
			if !ok {
				continue
			}
			item.Status = StatusOpen
			item.Completed = nil
			item.UpdatedAt = ent.At
			byID[ent.ID] = item
		case ActionDelete:
			item, ok := byID[ent.ID]
			if !ok {
				continue
			}
			item.Deleted = true
			item.UpdatedAt = ent.At
			byID[ent.ID] = item
		}
	}

	items := make([]Item, 0, len(byID))
	for _, item := range byID {
		if item.Status == "" {
			item.Status = StatusOpen
		}
		items = append(items, item)
	}
	return items
}

func sortItems(items []Item) {
	sort.Slice(items, func(i, j int) bool {
		if items[i].CreatedAt.Equal(items[j].CreatedAt) {
			return items[i].ID < items[j].ID
		}
		return items[i].CreatedAt.Before(items[j].CreatedAt)
	})
}
