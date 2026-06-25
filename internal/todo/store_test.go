package todo

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func newTestStore(t *testing.T, root string) *Store {
	t.Helper()
	store, err := NewStore(root)
	if err != nil {
		t.Fatalf("NewStore(%q) failed: %v", root, err)
	}
	return store
}

func newTestStoreAt(t *testing.T, at time.Time) *Store {
	t.Helper()
	store := newTestStore(t, t.TempDir())
	store.now = func() time.Time { return at.UTC() }
	return store
}

func withTestIDs(store *Store, ids ...string) {
	store.newID = func() (string, error) {
		if len(ids) == 0 {
			return "", nil
		}
		id := ids[0]
		ids = ids[1:]
		return id, nil
	}
}

func readFile(t *testing.T, root string) string {
	t.Helper()
	path := filepath.Join(root, LogPath())
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read file: %v", err)
	}
	return string(data)
}

func TestNewStoreRequiresRoot(t *testing.T) {
	if _, err := NewStore(""); err == nil || !strings.Contains(err.Error(), "root is empty") {
		t.Fatalf("expected empty root error, got: %v", err)
	}
}

func TestNewStoreRequiresDirectory(t *testing.T) {
	file := filepath.Join(t.TempDir(), "root.txt")
	if err := os.WriteFile(file, []byte("not a directory"), 0644); err != nil {
		t.Fatalf("write temp file failed: %v", err)
	}
	if _, err := NewStore(file); err == nil || !strings.Contains(err.Error(), "not a directory") {
		t.Fatalf("expected root directory error, got: %v", err)
	}
}

func TestLoadMissingFileReturnsEmpty(t *testing.T) {
	store := newTestStore(t, t.TempDir())
	items, err := store.Load()
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if len(items) != 0 {
		t.Fatalf("expected no items, got: %v", items)
	}
}

func TestAddCreatesFileAndDirectory(t *testing.T) {
	root := t.TempDir()
	store := newTestStore(t, root)
	at := time.Date(2026, 1, 15, 14, 30, 22, 0, time.UTC)
	store.now = func() time.Time { return at }
	withTestIDs(store, "opaque-alpha")

	item, err := store.Add(AddInput{Text: "refactor parser", SessionID: "sess-1", Branch: "feat/todo"})
	if err != nil {
		t.Fatalf("Add failed: %v", err)
	}
	if item.ID == "" {
		t.Fatal("expected non-empty ID")
	}
	if item.ID != "opaque-alpha" {
		t.Fatalf("ID = %q, want opaque-alpha", item.ID)
	}
	if item.Status != StatusOpen {
		t.Fatalf("expected open status, got: %v", item.Status)
	}

	content := readFile(t, root)
	if !strings.Contains(content, "id: opaque-alpha") {
		t.Errorf("expected file to contain generated opaque id, got: %q", content)
	}
	if !strings.Contains(content, "refactor parser") {
		t.Errorf("expected file to contain text, got: %q", content)
	}
	if !strings.Contains(content, "session_id: sess-1") {
		t.Errorf("expected file to contain session_id, got: %q", content)
	}
	if !strings.Contains(content, "branch: feat/todo") {
		t.Errorf("expected file to contain branch, got: %q", content)
	}
}

func TestAddRejectsEmptyText(t *testing.T) {
	store := newTestStore(t, t.TempDir())
	if _, err := store.Add(AddInput{Text: "   "}); err == nil || !strings.Contains(err.Error(), "text is empty") {
		t.Fatalf("expected empty text error, got: %v", err)
	}
}

func TestLoadRoundTripsAddedItem(t *testing.T) {
	root := t.TempDir()
	store := newTestStore(t, root)
	at := time.Date(2026, 1, 15, 14, 30, 22, 0, time.UTC)
	store.now = func() time.Time { return at }
	withTestIDs(store, "opaque-alpha")

	added, err := store.Add(AddInput{Text: "ship auth", SessionID: "sess-1", Branch: "feat/todo"})
	if err != nil {
		t.Fatalf("Add failed: %v", err)
	}

	items, err := store.Load()
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("expected one item, got: %d", len(items))
	}
	got := items[0]
	if got.ID != added.ID {
		t.Errorf("ID = %q, want %q", got.ID, added.ID)
	}
	if got.Text != "ship auth" {
		t.Errorf("Text = %q, want %q", got.Text, "ship auth")
	}
	if got.Status != StatusOpen {
		t.Errorf("Status = %q, want open", got.Status)
	}
	if got.SessionID != "sess-1" {
		t.Errorf("SessionID = %q, want sess-1", got.SessionID)
	}
	if got.Branch != "feat/todo" {
		t.Errorf("Branch = %q, want feat/todo", got.Branch)
	}
	if !got.CreatedAt.Equal(at) {
		t.Errorf("CreatedAt = %v, want %v", got.CreatedAt, at)
	}
}

func TestAddRetriesWhenGeneratedIDCollides(t *testing.T) {
	root := t.TempDir()
	store := newTestStore(t, root)
	store.now = func() time.Time { return time.Date(2026, 1, 15, 14, 30, 22, 0, time.UTC) }
	withTestIDs(store, "opaque-alpha")

	first, err := store.Add(AddInput{Text: "first"})
	if err != nil {
		t.Fatalf("Add first failed: %v", err)
	}
	if first.ID != "opaque-alpha" {
		t.Fatalf("first ID = %q, want opaque-alpha", first.ID)
	}

	withTestIDs(store, "opaque-alpha", "opaque-beta")
	second, err := store.Add(AddInput{Text: "second"})
	if err != nil {
		t.Fatalf("Add second failed after collision retry: %v", err)
	}
	if second.ID != "opaque-beta" {
		t.Fatalf("second ID = %q, want opaque-beta", second.ID)
	}

	items, err := store.Load()
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(items))
	}
}

func TestMutationsRewriteFile(t *testing.T) {
	root := t.TempDir()
	at := time.Date(2026, 1, 15, 14, 30, 22, 0, time.UTC)
	store := newTestStore(t, root)
	store.now = func() time.Time { return at }
	withTestIDs(store, "opaque-alpha")

	first, err := store.Add(AddInput{Text: "first"})
	if err != nil {
		t.Fatalf("Add first failed: %v", err)
	}
	afterAdd := readFile(t, root)

	if !strings.Contains(afterAdd, "first") {
		t.Fatalf("expected file to contain 'first' after add, got: %q", afterAdd)
	}

	completedAt := at.Add(time.Minute)
	store.now = func() time.Time { return completedAt }
	if err := store.Complete(first.ID); err != nil {
		t.Fatalf("Complete failed: %v", err)
	}
	afterComplete := readFile(t, root)

	if !strings.Contains(afterComplete, "done") {
		t.Errorf("expected file to contain done status, got: %q", afterComplete)
	}
	if !strings.Contains(afterComplete, "first") {
		t.Errorf("expected file to still contain 'first', got: %q", afterComplete)
	}
}

func TestCompleteRequiresExistingOpenTodo(t *testing.T) {
	store := newTestStoreAt(t, time.Date(2026, 1, 15, 14, 30, 22, 0, time.UTC))
	withTestIDs(store, "opaque-alpha")
	if err := store.Complete("missing"); err == nil || !strings.Contains(err.Error(), "not found") {
		t.Fatalf("expected not found error, got: %v", err)
	}

	added, err := store.Add(AddInput{Text: "thing"})
	if err != nil {
		t.Fatalf("Add failed: %v", err)
	}
	if err := store.Complete(added.ID); err != nil {
		t.Fatalf("first Complete failed: %v", err)
	}
	if err := store.Complete(added.ID); err == nil || !strings.Contains(err.Error(), "already done") {
		t.Fatalf("expected already done error, got: %v", err)
	}
}

func TestCompleteSetsCompletedAt(t *testing.T) {
	created := time.Date(2026, 1, 15, 14, 30, 22, 0, time.UTC)
	store := newTestStoreAt(t, created)
	withTestIDs(store, "opaque-alpha")
	added, err := store.Add(AddInput{Text: "thing"})
	if err != nil {
		t.Fatalf("Add failed: %v", err)
	}

	completed := created.Add(2 * time.Minute)
	store.now = func() time.Time { return completed }
	if err := store.Complete(added.ID); err != nil {
		t.Fatalf("Complete failed: %v", err)
	}

	items, err := store.Load()
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("expected one item, got: %d", len(items))
	}
	got := items[0]
	if got.Status != StatusDone {
		t.Errorf("Status = %q, want done", got.Status)
	}
	if got.Completed == nil || !got.Completed.Equal(completed) {
		var gotTime time.Time
		if got.Completed != nil {
			gotTime = *got.Completed
		}
		t.Errorf("Completed = %v, want %v", gotTime, completed)
	}
}

func TestReopenRestoresOpenStatus(t *testing.T) {
	created := time.Date(2026, 1, 15, 14, 30, 22, 0, time.UTC)
	store := newTestStoreAt(t, created)
	withTestIDs(store, "opaque-alpha")
	added, err := store.Add(AddInput{Text: "thing"})
	if err != nil {
		t.Fatalf("Add failed: %v", err)
	}
	if err := store.Complete(added.ID); err != nil {
		t.Fatalf("Complete failed: %v", err)
	}
	if err := store.Reopen(added.ID); err != nil {
		t.Fatalf("Reopen failed: %v", err)
	}

	items, err := store.Load()
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if items[0].Status != StatusOpen {
		t.Errorf("Status = %q, want open", items[0].Status)
	}
	if items[0].Completed != nil {
		t.Errorf("Completed should be nil after reopen, got: %v", items[0].Completed)
	}
	if err := store.Reopen(added.ID); err == nil || !strings.Contains(err.Error(), "already open") {
		t.Fatalf("expected already open error, got: %v", err)
	}
}

func TestUpdateTextRevisesBody(t *testing.T) {
	created := time.Date(2026, 1, 15, 14, 30, 22, 0, time.UTC)
	store := newTestStoreAt(t, created)
	withTestIDs(store, "opaque-alpha")
	added, err := store.Add(AddInput{Text: "thing"})
	if err != nil {
		t.Fatalf("Add failed: %v", err)
	}

	updated := created.Add(time.Minute)
	store.now = func() time.Time { return updated }
	if err := store.UpdateText(added.ID, "new body"); err != nil {
		t.Fatalf("UpdateText failed: %v", err)
	}

	items, err := store.Load()
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if items[0].Text != "new body" {
		t.Errorf("Text = %q, want %q", items[0].Text, "new body")
	}
	if !items[0].UpdatedAt.Equal(updated) {
		t.Errorf("UpdatedAt = %v, want %v", items[0].UpdatedAt, updated)
	}
}

func TestUpdateTextRejectsEmpty(t *testing.T) {
	store := newTestStoreAt(t, time.Date(2026, 1, 15, 14, 30, 22, 0, time.UTC))
	withTestIDs(store, "opaque-alpha")
	added, err := store.Add(AddInput{Text: "thing"})
	if err != nil {
		t.Fatalf("Add failed: %v", err)
	}
	if err := store.UpdateText(added.ID, "   "); err == nil || !strings.Contains(err.Error(), "text is empty") {
		t.Fatalf("expected empty text error, got: %v", err)
	}
}

func TestUpdateTextRejectsDone(t *testing.T) {
	created := time.Date(2026, 1, 15, 14, 30, 22, 0, time.UTC)
	store := newTestStoreAt(t, created)
	withTestIDs(store, "opaque-alpha")
	added, err := store.Add(AddInput{Text: "thing"})
	if err != nil {
		t.Fatalf("Add failed: %v", err)
	}
	if err := store.Complete(added.ID); err != nil {
		t.Fatalf("Complete failed: %v", err)
	}
	if err := store.UpdateText(added.ID, "new body"); err == nil || !strings.Contains(err.Error(), "is done") {
		t.Fatalf("expected done error, got: %v", err)
	}
}

func TestDeleteRemovesItem(t *testing.T) {
	store := newTestStoreAt(t, time.Date(2026, 1, 15, 14, 30, 22, 0, time.UTC))
	withTestIDs(store, "opaque-alpha", "opaque-beta")
	first, err := store.Add(AddInput{Text: "first"})
	if err != nil {
		t.Fatalf("Add first failed: %v", err)
	}
	if _, err := store.Add(AddInput{Text: "second"}); err != nil {
		t.Fatalf("Add second failed: %v", err)
	}

	if err := store.Delete(first.ID); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	items, err := store.Load()
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("expected 1 item after delete, got: %d", len(items))
	}
	if items[0].Text != "second" {
		t.Errorf("expected remaining item to be 'second', got: %q", items[0].Text)
	}

	if err := store.Delete(first.ID); err == nil || !strings.Contains(err.Error(), "not found") {
		t.Fatalf("expected not found error on re-delete, got: %v", err)
	}
}

func TestPruneCompletedRemovesOnlyDoneItems(t *testing.T) {
	store := newTestStoreAt(t, time.Date(2026, 1, 15, 14, 30, 22, 0, time.UTC))
	withTestIDs(store, "opaque-alpha", "opaque-beta", "opaque-gamma")
	open, err := store.Add(AddInput{Text: "open"})
	if err != nil {
		t.Fatalf("Add open failed: %v", err)
	}
	firstDone, err := store.Add(AddInput{Text: "first done"})
	if err != nil {
		t.Fatalf("Add first done failed: %v", err)
	}
	secondDone, err := store.Add(AddInput{Text: "second done"})
	if err != nil {
		t.Fatalf("Add second done failed: %v", err)
	}
	if err := store.Complete(firstDone.ID); err != nil {
		t.Fatalf("Complete first done failed: %v", err)
	}
	if err := store.Complete(secondDone.ID); err != nil {
		t.Fatalf("Complete second done failed: %v", err)
	}

	removed, err := store.PruneCompleted()
	if err != nil {
		t.Fatalf("PruneCompleted failed: %v", err)
	}
	if removed != 2 {
		t.Fatalf("expected 2 pruned items, got %d", removed)
	}

	items, err := store.Load()
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if len(items) != 1 || items[0].ID != open.ID {
		t.Fatalf("expected only open item to remain, got: %+v", items)
	}

	removed, err = store.PruneCompleted()
	if err != nil {
		t.Fatalf("second PruneCompleted failed: %v", err)
	}
	if removed != 0 {
		t.Fatalf("expected no pruned items on second run, got %d", removed)
	}
}

func TestListAppliesFilter(t *testing.T) {
	created := time.Date(2026, 1, 15, 14, 30, 22, 0, time.UTC)
	store := newTestStoreAt(t, created)
	withTestIDs(store, "opaque-alpha", "opaque-beta", "opaque-gamma")

	if _, err := store.Add(AddInput{Text: "first", SessionID: "sess-1", Branch: "feat/a"}); err != nil {
		t.Fatalf("Add first failed: %v", err)
	}
	second, err := store.Add(AddInput{Text: "second", SessionID: "sess-2", Branch: "feat/b"})
	if err != nil {
		t.Fatalf("Add second failed: %v", err)
	}
	createdPlus := created.Add(time.Minute)
	store.now = func() time.Time { return createdPlus }
	if err := store.Complete(second.ID); err != nil {
		t.Fatalf("Complete failed: %v", err)
	}
	createdPlus2 := created.Add(2 * time.Minute)
	store.now = func() time.Time { return createdPlus2 }
	if _, err := store.Add(AddInput{Text: "third", SessionID: "sess-1", Branch: "feat/a"}); err != nil {
		t.Fatalf("Add third failed: %v", err)
	}

	open, err := store.List(Filter{IncludeOpen: true, MatchSessionAny: true, MatchBranchAny: true})
	if err != nil {
		t.Fatalf("List open failed: %v", err)
	}
	if len(open) != 2 {
		t.Errorf("expected 2 open items, got: %d", len(open))
	}

	done, err := store.List(Filter{IncludeDone: true, MatchSessionAny: true, MatchBranchAny: true})
	if err != nil {
		t.Fatalf("List done failed: %v", err)
	}
	if len(done) != 1 {
		t.Errorf("expected 1 done item, got: %d", len(done))
	}

	scoped, err := store.List(DefaultFilter("sess-1", "feat/a"))
	if err != nil {
		t.Fatalf("List scoped failed: %v", err)
	}
	if len(scoped) != 2 {
		t.Errorf("expected 2 scoped open items, got: %d", len(scoped))
	}
}

func TestListIsDeterministicByCreatedAtThenID(t *testing.T) {
	created := time.Date(2026, 1, 15, 14, 30, 22, 0, time.UTC)
	store := newTestStoreAt(t, created)
	withTestIDs(store, "opaque-alpha", "opaque-beta", "opaque-gamma")
	for _, text := range []string{"alpha", "beta", "gamma"} {
		if _, err := store.Add(AddInput{Text: text}); err != nil {
			t.Fatalf("Add %q failed: %v", text, err)
		}
	}
	first, err := store.Load()
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	second, err := store.Load()
	if err != nil {
		t.Fatalf("Load second failed: %v", err)
	}
	if len(first) != len(second) {
		t.Fatalf("expected consistent length, got %d vs %d", len(first), len(second))
	}
	for i := range first {
		if first[i].ID != second[i].ID {
			t.Errorf("item %d ID differs: %q vs %q", i, first[i].ID, second[i].ID)
		}
	}
}

func TestLoadRejectsMalformedFile(t *testing.T) {
	root := t.TempDir()
	store := newTestStore(t, root)
	dir := filepath.Join(root, DirName)
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, LogPath()), []byte("not: valid: yaml: :::\n"), 0644); err != nil {
		t.Fatalf("write bad file: %v", err)
	}
	if _, err := store.Load(); err == nil {
		t.Fatal("expected Load to fail on malformed file")
	}
}

func TestStoreRejectsFileAsDirectory(t *testing.T) {
	root := t.TempDir()
	devlogPath := filepath.Join(root, DirName)
	if err := os.WriteFile(devlogPath, []byte("not a directory"), 0644); err != nil {
		t.Fatalf("write file: %v", err)
	}
	store := newTestStore(t, root)
	withTestIDs(store, "opaque-alpha")
	if _, err := store.Add(AddInput{Text: "thing"}); err == nil || !strings.Contains(err.Error(), "not a directory") {
		t.Fatalf("expected not a directory error, got: %v", err)
	}
}
