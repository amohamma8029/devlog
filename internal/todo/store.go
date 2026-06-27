package todo

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// Store persists the current state of all todos in a repo-local single-state
// YAML file. Every mutation loads the file, modifies the in-memory list, and
// rewrites the full file with current state.
type Store struct {
	root  string
	now   func() time.Time
	newID func() (string, error)
}

// NewStore creates a Store scoped to a repository root. The todo file lives
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

	return &Store{root: absRoot, now: func() time.Time { return time.Now().UTC() }, newID: randomID}, nil
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

// Add creates a new todo, appends it to the item list, and rewrites the file.
func (s *Store) Add(in AddInput) (Item, error) {
	text := strings.TrimSpace(in.Text)
	if text == "" {
		return Item{}, fmt.Errorf("todo.Store.Add: text is empty")
	}

	items, err := s.Load()
	if err != nil {
		return Item{}, err
	}

	id, err := s.nextID(items)
	if err != nil {
		return Item{}, err
	}

	at := s.now()
	item := Item{
		ID:        id,
		Text:      text,
		Status:    StatusOpen,
		CreatedAt: at,
		UpdatedAt: at,
		SessionID: strings.TrimSpace(in.SessionID),
		Branch:    strings.TrimSpace(in.Branch),
	}

	items = append(items, item)
	if err := s.writeFile(items); err != nil {
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
		item.Text = newText
		item.UpdatedAt = s.now()
		return item, nil
	})
}

// Complete marks an open todo as done.
func (s *Store) Complete(id string) error {
	return s.mutate(id, "todo.Store.Complete", func(item Item) (Item, error) {
		if item.Status == StatusDone {
			return Item{}, fmt.Errorf("todo.Store.Complete: todo %q is already done", id)
		}
		at := s.now()
		item.Status = StatusDone
		item.Completed = &at
		item.UpdatedAt = at
		return item, nil
	})
}

// Reopen moves a done todo back to open.
func (s *Store) Reopen(id string) error {
	return s.mutate(id, "todo.Store.Reopen", func(item Item) (Item, error) {
		if item.Status == StatusOpen {
			return Item{}, fmt.Errorf("todo.Store.Reopen: todo %q is already open", id)
		}
		item.Status = StatusOpen
		item.Completed = nil
		item.UpdatedAt = s.now()
		return item, nil
	})
}

// Delete removes a todo from the file entirely.
func (s *Store) Delete(id string) error {
	if strings.TrimSpace(id) == "" {
		return fmt.Errorf("todo.Store.Delete: id is empty")
	}

	items, err := s.Load()
	if err != nil {
		return err
	}

	idx := -1
	for i := range items {
		if items[i].ID == id {
			idx = i
			break
		}
	}
	if idx == -1 {
		return fmt.Errorf("todo.Store.Delete: todo %q not found", id)
	}

	items = append(items[:idx], items[idx+1:]...)
	return s.writeFile(items)
}

// PruneCompleted removes every completed todo from the file and returns the
// number of items removed.
func (s *Store) PruneCompleted() (int, error) {
	items, err := s.Load()
	if err != nil {
		return 0, err
	}

	kept := make([]Item, 0, len(items))
	removed := 0
	for _, item := range items {
		if item.Status == StatusDone {
			removed++
			continue
		}
		kept = append(kept, item)
	}
	if removed == 0 {
		return 0, nil
	}

	if err := s.writeFile(kept); err != nil {
		return 0, err
	}
	return removed, nil
}

// ClearSessionAttribution blanks SessionID and Branch on all open todos
// attributed to the given session, making them repo-wide. Closed (done) todos
// for that session are left unchanged.
func (s *Store) ClearSessionAttribution(sessionID string) (int, error) {
	if strings.TrimSpace(sessionID) == "" {
		return 0, fmt.Errorf("todo.Store.ClearSessionAttribution: sessionID is empty")
	}

	items, err := s.Load()
	if err != nil {
		return 0, err
	}

	changed := 0
	for i := range items {
		if items[i].Status != StatusOpen {
			continue
		}
		if items[i].SessionID != sessionID {
			continue
		}
		items[i].SessionID = ""
		items[i].Branch = ""
		items[i].UpdatedAt = s.now()
		changed++
	}
	if changed == 0 {
		return 0, nil
	}

	if err := s.writeFile(items); err != nil {
		return 0, err
	}
	return changed, nil
}

// List returns todos that match the given filter, ordered by CreatedAt
// ascending and then by ID.
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

// Load returns the full current state of every todo in the file, sorted by
// CreatedAt then ID.
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
		return nil, fmt.Errorf("todo.Store.Load: read file: %w", err)
	}

	var items []Item
	if err := yaml.Unmarshal(data, &items); err != nil {
		return nil, fmt.Errorf("todo.Store.Load: parse: %w", err)
	}

	sortItems(items)
	return items, nil
}

func (s *Store) mutate(id, op string, transform func(Item) (Item, error)) error {
	if strings.TrimSpace(id) == "" {
		return fmt.Errorf("%s: id is empty", op)
	}

	items, err := s.Load()
	if err != nil {
		return err
	}

	idx := -1
	for i := range items {
		if items[i].ID == id {
			idx = i
			break
		}
	}
	if idx == -1 {
		return fmt.Errorf("%s: todo %q not found", op, id)
	}

	next, err := transform(items[idx])
	if err != nil {
		return err
	}
	if err := next.Validate(); err != nil {
		return fmt.Errorf("%s: %w", op, err)
	}

	items[idx] = next
	return s.writeFile(items)
}

func (s *Store) nextID(existing []Item) (string, error) {
	if s == nil || s.newID == nil {
		return "", fmt.Errorf("todo.Store.nextID: id generator is nil")
	}

	ids := make(map[string]struct{}, len(existing))
	for _, item := range existing {
		ids[item.ID] = struct{}{}
	}

	for attempt := 0; attempt < 100; attempt++ {
		id, err := s.newID()
		if err != nil {
			return "", err
		}
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		if _, exists := ids[id]; exists {
			continue
		}
		return id, nil
	}
	return "", fmt.Errorf("todo.Store.nextID: could not generate a unique todo id")
}

func (s *Store) writeFile(items []Item) error {
	if err := s.ensureDir(); err != nil {
		return err
	}

	path, err := s.logPath()
	if err != nil {
		return err
	}

	data, err := yaml.Marshal(items)
	if err != nil {
		return fmt.Errorf("todo.Store.writeFile: marshal: %w", err)
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("todo.Store.writeFile: write: %w", err)
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

func sortItems(items []Item) {
	sort.Slice(items, func(i, j int) bool {
		if items[i].CreatedAt.Equal(items[j].CreatedAt) {
			return items[i].ID < items[j].ID
		}
		return items[i].CreatedAt.Before(items[j].CreatedAt)
	})
}

func randomID() (string, error) {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", fmt.Errorf("todo.Store.randomID: %w", err)
	}
	return hex.EncodeToString(b[:]), nil
}
