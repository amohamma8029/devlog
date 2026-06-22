package store

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestNewRequiresRoot(t *testing.T) {
	_, err := New("")
	if err == nil || !strings.Contains(err.Error(), "root is empty") {
		t.Fatalf("expected empty root error, got: %v", err)
	}
}

func TestNewRequiresDirectoryRoot(t *testing.T) {
	file := filepath.Join(t.TempDir(), "root.txt")
	if err := os.WriteFile(file, []byte("not a directory"), 0644); err != nil {
		t.Fatalf("write temp file failed: %v", err)
	}

	_, err := New(file)
	if err == nil || !strings.Contains(err.Error(), "root is not a directory") {
		t.Fatalf("expected root directory error, got: %v", err)
	}
}

func TestWriteSession(t *testing.T) {
	root := t.TempDir()
	store := newTestStore(t, root)
	sess := testSession()

	if err := store.WriteSession(sess, "Implement auth middleware"); err != nil {
		t.Fatalf("WriteSession failed: %v", err)
	}

	content := readSessionFile(t, root, sess.ID)
	if !strings.Contains(content, "id: 2026-01-15T143022Z") {
		t.Errorf("expected front-matter to contain id")
	}
	if !strings.Contains(content, "status: active") {
		t.Errorf("expected front-matter to contain status active")
	}
	if !strings.Contains(content, "email: test@example.com") {
		t.Errorf("expected front-matter to contain email")
	}
	if !strings.Contains(content, "## Start\n\nImplement auth middleware") {
		t.Errorf("expected body to contain start message")
	}
}

func TestWriteSessionCreatesSessionsDirectory(t *testing.T) {
	root := t.TempDir()
	store := newTestStore(t, root)

	if err := store.WriteSession(testSession(), "Implement auth middleware"); err != nil {
		t.Fatalf("WriteSession failed: %v", err)
	}

	path := filepath.Join(root, sessionsDir)
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("expected sessions directory to exist: %v", err)
	}
	if !info.IsDir() {
		t.Fatalf("expected sessions path to be a directory")
	}
}

func TestWriteSessionDuplicateIDFails(t *testing.T) {
	root := t.TempDir()
	store := newTestStore(t, root)
	sess := testSession()

	if err := store.WriteSession(sess, "Implement auth middleware"); err != nil {
		t.Fatalf("WriteSession failed: %v", err)
	}
	if err := store.WriteSession(sess, "Implement auth middleware"); err == nil {
		t.Fatal("expected duplicate WriteSession to fail")
	}
}

func TestWriteSessionValidatesInput(t *testing.T) {
	store := newTestStore(t, t.TempDir())

	if err := store.WriteSession(Session{}, "message"); err == nil || !strings.Contains(err.Error(), "session ID is empty") {
		t.Fatalf("expected empty ID error, got: %v", err)
	}
	if err := store.WriteSession(testSession(), ""); err == nil || !strings.Contains(err.Error(), "start message is empty") {
		t.Fatalf("expected empty start message error, got: %v", err)
	}
	if err := store.WriteSession(Session{ID: "../outside"}, "message"); err == nil || !strings.Contains(err.Error(), "invalid session ID") {
		t.Fatalf("expected invalid session ID error, got: %v", err)
	}
}

func TestAppendEvent(t *testing.T) {
	root := t.TempDir()
	store := newTestStore(t, root)
	sess := testSession()

	if err := store.WriteSession(sess, "Implement auth middleware"); err != nil {
		t.Fatalf("WriteSession failed: %v", err)
	}
	if err := store.AppendEvent(sess.ID, "Note", "Refactored JWT package"); err != nil {
		t.Fatalf("AppendEvent failed: %v", err)
	}

	content := readSessionFile(t, root, sess.ID)
	if !strings.Contains(content, "## Note - ") {
		t.Errorf("expected body to contain Note event")
	}
	if !strings.Contains(content, "Refactored JWT package") {
		t.Errorf("expected body to contain note text")
	}
}

func TestAppendEventWritesFullTimestamp(t *testing.T) {
	root := t.TempDir()
	store := newTestStore(t, root)
	sess := testSession()

	if err := store.WriteSession(sess, "Implement auth middleware"); err != nil {
		t.Fatalf("WriteSession failed: %v", err)
	}
	at := time.Date(2026, 1, 15, 16, 45, 0, 0, time.UTC)
	if err := store.appendEvent("test", sess.ID, "Note", "Refactored JWT package", at); err != nil {
		t.Fatalf("appendEvent failed: %v", err)
	}

	content := readSessionFile(t, root, sess.ID)
	if !strings.Contains(content, "## Note - 2026-01-15 16:45 UTC") {
		t.Fatalf("expected full timestamp heading, got:\n%s", content)
	}
}

func TestAppendEventValidatesInput(t *testing.T) {
	store := newTestStore(t, t.TempDir())

	if err := store.AppendEvent("", "Note", "body"); err == nil || !strings.Contains(err.Error(), "session ID is empty") {
		t.Fatalf("expected empty ID error, got: %v", err)
	}
	if err := store.AppendEvent("../outside", "Note", "body"); err == nil || !strings.Contains(err.Error(), "invalid session ID") {
		t.Fatalf("expected invalid session ID error, got: %v", err)
	}
	if err := store.AppendEvent("missing", "", "body"); err == nil || !strings.Contains(err.Error(), "event type is empty") {
		t.Fatalf("expected empty event type error, got: %v", err)
	}
	if err := store.AppendEvent("missing", "Note", ""); err == nil || !strings.Contains(err.Error(), "event body is empty") {
		t.Fatalf("expected empty event body error, got: %v", err)
	}
}

func TestAppendEventMissingSession(t *testing.T) {
	store := newTestStore(t, t.TempDir())

	err := store.AppendEvent("nonexistent", "Note", "body")
	if err == nil || !strings.Contains(err.Error(), "session file not found") {
		t.Fatalf("expected not found error, got: %v", err)
	}
}

func TestCloseSessionAppendsStopEvent(t *testing.T) {
	root := t.TempDir()
	store := newTestStore(t, root)
	sess := testSession()

	if err := store.WriteSession(sess, "Implement auth middleware"); err != nil {
		t.Fatalf("WriteSession failed: %v", err)
	}
	if err := store.CloseSession(sess.ID); err != nil {
		t.Fatalf("CloseSession failed: %v", err)
	}

	content := readSessionFile(t, root, sess.ID)
	if !strings.Contains(content, "status: active") {
		t.Errorf("expected front-matter to remain append-only metadata")
	}
	if !strings.Contains(content, "## Stop - ") {
		t.Errorf("expected body to contain Stop event")
	}
	if !strings.Contains(content, "Session closed.") {
		t.Errorf("expected body to contain close message")
	}
}

func TestCloseSessionValidatesInput(t *testing.T) {
	store := newTestStore(t, t.TempDir())

	if err := store.CloseSession(""); err == nil || !strings.Contains(err.Error(), "session ID is empty") {
		t.Fatalf("expected empty ID error, got: %v", err)
	}
	if err := store.CloseSession("../outside"); err == nil || !strings.Contains(err.Error(), "invalid session ID") {
		t.Fatalf("expected invalid session ID error, got: %v", err)
	}
}

func TestCloseSessionMissingFile(t *testing.T) {
	store := newTestStore(t, t.TempDir())

	err := store.CloseSession("nonexistent")
	if err == nil || !strings.Contains(err.Error(), "session file not found") {
		t.Fatalf("expected not found error, got: %v", err)
	}
}

func newTestStore(t *testing.T, root string) *Store {
	t.Helper()

	store, err := New(root)
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}
	return store
}

func testSession() Session {
	return Session{
		ID:      "2026-01-15T143022Z",
		Author:  "Test Author",
		Email:   "test@example.com",
		Started: time.Date(2026, 1, 15, 14, 30, 22, 0, time.UTC),
		Branch:  "main",
		Status:  "active",
	}
}

func TestGetSession(t *testing.T) {
	root := t.TempDir()
	store := newTestStore(t, root)
	sess := testSession()

	if err := store.WriteSession(sess, "Implement auth middleware"); err != nil {
		t.Fatalf("WriteSession failed: %v", err)
	}

	rec, err := store.GetSession(sess.ID)
	if err != nil {
		t.Fatalf("GetSession failed: %v", err)
	}
	if rec.ID != sess.ID {
		t.Errorf("expected ID %q, got %q", sess.ID, rec.ID)
	}
	if rec.Author != sess.Author {
		t.Errorf("expected Author %q, got %q", sess.Author, rec.Author)
	}
	if rec.Email != sess.Email {
		t.Errorf("expected Email %q, got %q", sess.Email, rec.Email)
	}
	if !rec.Started.Equal(sess.Started) {
		t.Errorf("expected Started %v, got %v", sess.Started, rec.Started)
	}
	if rec.Branch != sess.Branch {
		t.Errorf("expected Branch %q, got %q", sess.Branch, rec.Branch)
	}
	if rec.Status != sess.Status {
		t.Errorf("expected Status %q, got %q", sess.Status, rec.Status)
	}
	if rec.Closed {
		t.Error("expected Closed to be false for active session")
	}
}

func TestGetSessionClosed(t *testing.T) {
	root := t.TempDir()
	store := newTestStore(t, root)
	sess := testSession()

	if err := store.WriteSession(sess, "Implement auth middleware"); err != nil {
		t.Fatalf("WriteSession failed: %v", err)
	}
	if err := store.CloseSession(sess.ID); err != nil {
		t.Fatalf("CloseSession failed: %v", err)
	}

	rec, err := store.GetSession(sess.ID)
	if err != nil {
		t.Fatalf("GetSession failed: %v", err)
	}
	if !rec.Closed {
		t.Error("expected Closed to be true after CloseSession")
	}
	if rec.Status != "active" {
		t.Errorf("expected Status to remain active, got %q", rec.Status)
	}
}

func TestGetSessionNotClosedWhenStopInBody(t *testing.T) {
	root := t.TempDir()
	store := newTestStore(t, root)
	sess := testSession()

	if err := store.WriteSession(sess, "Implement auth middleware"); err != nil {
		t.Fatalf("WriteSession failed: %v", err)
	}
	if err := store.AppendEvent(sess.ID, "Note", "We should ## Stop this approach and reconsider"); err != nil {
		t.Fatalf("AppendEvent failed: %v", err)
	}

	rec, err := store.GetSession(sess.ID)
	if err != nil {
		t.Fatalf("GetSession failed: %v", err)
	}
	if rec.Closed {
		t.Error("expected Closed to be false when ## Stop appears only in note body")
	}
}

func TestGetSessionMissing(t *testing.T) {
	store := newTestStore(t, t.TempDir())

	_, err := store.GetSession("nonexistent")
	if err == nil || !strings.Contains(err.Error(), "session file not found") {
		t.Fatalf("expected not found error, got: %v", err)
	}
}

func TestGetSessionInvalidID(t *testing.T) {
	store := newTestStore(t, t.TempDir())

	if _, err := store.GetSession(""); err == nil || !strings.Contains(err.Error(), "session ID is empty") {
		t.Fatalf("expected empty ID error, got: %v", err)
	}
	if _, err := store.GetSession("../outside"); err == nil || !strings.Contains(err.Error(), "invalid session ID") {
		t.Fatalf("expected invalid session ID error, got: %v", err)
	}
}

func TestListSessions(t *testing.T) {
	root := t.TempDir()
	store := newTestStore(t, root)

	sess1 := Session{
		ID:      "2026-01-15T140000Z",
		Author:  "Alice",
		Started: time.Date(2026, 1, 15, 14, 0, 0, 0, time.UTC),
		Branch:  "main",
		Status:  "active",
	}
	sess2 := Session{
		ID:      "2026-01-15T150000Z",
		Author:  "Bob",
		Started: time.Date(2026, 1, 15, 15, 0, 0, 0, time.UTC),
		Branch:  "feat/widgets",
		Status:  "active",
	}
	sess3 := Session{
		ID:      "2026-01-15T160000Z",
		Author:  "Carol",
		Started: time.Date(2026, 1, 15, 16, 0, 0, 0, time.UTC),
		Branch:  "fix/bug",
		Status:  "active",
	}

	if err := store.WriteSession(sess1, "first"); err != nil {
		t.Fatalf("WriteSession 1 failed: %v", err)
	}
	if err := store.WriteSession(sess2, "second"); err != nil {
		t.Fatalf("WriteSession 2 failed: %v", err)
	}
	if err := store.WriteSession(sess3, "third"); err != nil {
		t.Fatalf("WriteSession 3 failed: %v", err)
	}

	records, err := store.ListSessions()
	if err != nil {
		t.Fatalf("ListSessions failed: %v", err)
	}
	if len(records) != 3 {
		t.Fatalf("expected 3 records, got %d", len(records))
	}

	if records[0].ID != sess3.ID {
		t.Errorf("expected first record %q (most recent), got %q", sess3.ID, records[0].ID)
	}
	if records[1].ID != sess2.ID {
		t.Errorf("expected second record %q, got %q", sess2.ID, records[1].ID)
	}
	if records[2].ID != sess1.ID {
		t.Errorf("expected third record %q (oldest), got %q", sess1.ID, records[2].ID)
	}
}

func TestListSessionsEmpty(t *testing.T) {
	store := newTestStore(t, t.TempDir())

	records, err := store.ListSessions()
	if err != nil {
		t.Fatalf("ListSessions failed: %v", err)
	}
	if len(records) != 0 {
		t.Fatalf("expected 0 records, got %d", len(records))
	}
}

func TestListSessionsChronological(t *testing.T) {
	root := t.TempDir()
	store := newTestStore(t, root)

	sessEarly := Session{
		ID:      "2026-01-15T080000Z",
		Author:  "Alice",
		Started: time.Date(2026, 1, 15, 8, 0, 0, 0, time.UTC),
		Branch:  "main",
		Status:  "active",
	}
	sessLate := Session{
		ID:      "2026-01-15T180000Z",
		Author:  "Bob",
		Started: time.Date(2026, 1, 15, 18, 0, 0, 0, time.UTC),
		Branch:  "feat/z",
		Status:  "active",
	}

	if err := store.WriteSession(sessLate, "late"); err != nil {
		t.Fatalf("WriteSession late failed: %v", err)
	}
	if err := store.WriteSession(sessEarly, "early"); err != nil {
		t.Fatalf("WriteSession early failed: %v", err)
	}

	records, err := store.ListSessions()
	if err != nil {
		t.Fatalf("ListSessions failed: %v", err)
	}
	if len(records) != 2 {
		t.Fatalf("expected 2 records, got %d", len(records))
	}

	if records[0].ID != sessLate.ID {
		t.Errorf("expected first record %q (most recent), got %q", sessLate.ID, records[0].ID)
	}
	if records[1].ID != sessEarly.ID {
		t.Errorf("expected second record %q (older), got %q", sessEarly.ID, records[1].ID)
	}
}

func TestListSessionsSkipsNonMD(t *testing.T) {
	root := t.TempDir()
	store := newTestStore(t, root)

	sess := testSession()
	if err := store.WriteSession(sess, "start"); err != nil {
		t.Fatalf("WriteSession failed: %v", err)
	}

	gitkeep := filepath.Join(root, sessionsDir, ".gitkeep")
	if err := os.WriteFile(gitkeep, []byte{}, 0644); err != nil {
		t.Fatalf("write .gitkeep failed: %v", err)
	}

	records, err := store.ListSessions()
	if err != nil {
		t.Fatalf("ListSessions failed: %v", err)
	}
	if len(records) != 1 {
		t.Fatalf("expected 1 record (skipping .gitkeep), got %d", len(records))
	}
}

func TestParseSessionFileMissingOpeningDelimiter(t *testing.T) {
	_, err := parseSessionFile([]byte("no delimiter here"))
	if err == nil || !strings.Contains(err.Error(), "missing opening front-matter delimiter") {
		t.Fatalf("expected missing opening delimiter error, got: %v", err)
	}
}

func TestParseSessionFileMissingClosingDelimiter(t *testing.T) {
	_, err := parseSessionFile([]byte("---\nid: x\n"))
	if err == nil || !strings.Contains(err.Error(), "missing closing front-matter delimiter") {
		t.Fatalf("expected missing closing delimiter error, got: %v", err)
	}
}

func TestReadSessionBody(t *testing.T) {
	root := t.TempDir()
	store := newTestStore(t, root)
	sess := testSession()

	if err := store.WriteSession(sess, "Implement auth middleware"); err != nil {
		t.Fatalf("WriteSession failed: %v", err)
	}

	body, err := store.ReadSessionBody(sess.ID)
	if err != nil {
		t.Fatalf("ReadSessionBody failed: %v", err)
	}
	if !strings.Contains(body, "## Start") {
		t.Errorf("expected body to contain '## Start', got: %s", body)
	}
	if !strings.Contains(body, "Implement auth middleware") {
		t.Errorf("expected body to contain start message, got: %s", body)
	}
	if strings.Contains(body, "id:") {
		t.Errorf("body should not contain YAML front-matter, got: %s", body)
	}
}

func TestReadSessionFileMetadata(t *testing.T) {
	root := t.TempDir()
	store := newTestStore(t, root)
	sess := testSession()

	if err := store.WriteSession(sess, "Implement auth middleware"); err != nil {
		t.Fatalf("WriteSession failed: %v", err)
	}

	before, err := store.ReadSessionFileMetadata(sess.ID)
	if err != nil {
		t.Fatalf("ReadSessionFileMetadata failed: %v", err)
	}
	if before.Size <= 0 {
		t.Fatalf("metadata size = %d, want positive size", before.Size)
	}
	if before.ModTime.IsZero() {
		t.Fatal("metadata mod time should be set")
	}

	if err := store.AppendEvent(sess.ID, "Note", "Refactored JWT package"); err != nil {
		t.Fatalf("AppendEvent failed: %v", err)
	}
	after, err := store.ReadSessionFileMetadata(sess.ID)
	if err != nil {
		t.Fatalf("ReadSessionFileMetadata after append failed: %v", err)
	}
	if after.Size <= before.Size {
		t.Fatalf("metadata size after append = %d, want > %d", after.Size, before.Size)
	}
	if before.Equal(after) {
		t.Fatal("metadata should change after append")
	}
}

func TestExtractMarkdownBody(t *testing.T) {
	content := "---\nid: test\n---\n\n## Start\n\nImplement auth middleware\n"

	body, err := ExtractMarkdownBody(content)
	if err != nil {
		t.Fatalf("ExtractMarkdownBody failed: %v", err)
	}
	if body != "\n## Start\n\nImplement auth middleware\n" {
		t.Fatalf("expected markdown body, got %q", body)
	}
}

func TestExtractMarkdownBodyRequiresDelimiters(t *testing.T) {
	if _, err := ExtractMarkdownBody("no delimiter here"); err == nil || !strings.Contains(err.Error(), "missing opening front-matter delimiter") {
		t.Fatalf("expected missing opening delimiter error, got: %v", err)
	}
	if _, err := ExtractMarkdownBody("---\nid: x\n"); err == nil || !strings.Contains(err.Error(), "missing closing front-matter delimiter") {
		t.Fatalf("expected missing closing delimiter error, got: %v", err)
	}
}

func TestReadSessionStartMessage(t *testing.T) {
	root := t.TempDir()
	store := newTestStore(t, root)
	sess := testSession()

	if err := store.WriteSession(sess, "\n\nImplement auth middleware\nSecond line"); err != nil {
		t.Fatalf("WriteSession failed: %v", err)
	}

	message, err := store.ReadSessionStartMessage(sess.ID)
	if err != nil {
		t.Fatalf("ReadSessionStartMessage failed: %v", err)
	}
	if message != "Implement auth middleware" {
		t.Fatalf("expected first non-empty start line, got %q", message)
	}
}

func TestReadSessionStartMessageEmptyWhenMissing(t *testing.T) {
	root := t.TempDir()
	store := newTestStore(t, root)
	sess := testSession()

	if err := store.WriteSession(sess, "temporary"); err != nil {
		t.Fatalf("WriteSession failed: %v", err)
	}

	path := filepath.Join(root, sessionsDir, sess.ID+".md")
	content := strings.Replace(readSessionFile(t, root, sess.ID), "temporary", "", 1)
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("write session file failed: %v", err)
	}

	message, err := store.ReadSessionStartMessage(sess.ID)
	if err != nil {
		t.Fatalf("ReadSessionStartMessage failed: %v", err)
	}
	if message != "" {
		t.Fatalf("expected empty start message, got %q", message)
	}
}

func TestReadSessionBodyMissing(t *testing.T) {
	store := newTestStore(t, t.TempDir())

	_, err := store.ReadSessionBody("nonexistent")
	if err == nil || !strings.Contains(err.Error(), "session file not found") {
		t.Fatalf("expected not found error, got: %v", err)
	}
}

func TestReadSessionBodyInvalidID(t *testing.T) {
	store := newTestStore(t, t.TempDir())

	if _, err := store.ReadSessionBody(""); err == nil || !strings.Contains(err.Error(), "session ID is empty") {
		t.Fatalf("expected empty ID error, got: %v", err)
	}
	if _, err := store.ReadSessionBody("../outside"); err == nil || !strings.Contains(err.Error(), "invalid session ID") {
		t.Fatalf("expected invalid session ID error, got: %v", err)
	}
}

func TestReadSessionContent(t *testing.T) {
	root := t.TempDir()
	store := newTestStore(t, root)
	sess := testSession()

	if err := store.WriteSession(sess, "Implement auth middleware"); err != nil {
		t.Fatalf("WriteSession failed: %v", err)
	}

	content, err := store.ReadSessionContent(sess.ID)
	if err != nil {
		t.Fatalf("ReadSessionContent failed: %v", err)
	}
	if !strings.Contains(content, "id: 2026-01-15T143022Z") {
		t.Errorf("expected content to contain YAML id field, got: %s", content)
	}
	if !strings.Contains(content, "## Start") {
		t.Errorf("expected content to contain markdown body, got: %s", content)
	}
}

func TestParseSessionEventsBasic(t *testing.T) {
	body := "\n## Start\n\nImplement auth middleware\n\n## Note - 2026-01-15 14:30 UTC\n\nRefactored JWT package\nMulti-line note.\n\n## Stop - 2026-01-15 15:00 UTC\n\nSession closed.\n"

	events := ParseSessionEvents(body)
	if len(events) != 3 {
		t.Fatalf("expected 3 events, got %d", len(events))
	}

	if events[0].Type != "Start" {
		t.Errorf("expected first event to be Start, got %s", events[0].Type)
	}
	if !events[0].Time.IsZero() {
		t.Errorf("expected Start event to have zero Time, got %v", events[0].Time)
	}
	if events[0].Body != "Implement auth middleware" {
		t.Errorf("expected Start body, got %q", events[0].Body)
	}

	if events[1].Type != "Note" {
		t.Errorf("expected second event to be Note, got %s", events[1].Type)
	}
	expectedNoteTime := mustParseEventTime(t, "2026-01-15 14:30 UTC")
	if !events[1].Time.Equal(expectedNoteTime) {
		t.Errorf("expected Note time %v, got %v", expectedNoteTime, events[1].Time)
	}
	if events[1].Body != "Refactored JWT package\nMulti-line note." {
		t.Errorf("expected Note body, got %q", events[1].Body)
	}

	if events[2].Type != "Stop" {
		t.Errorf("expected third event to be Stop, got %s", events[2].Type)
	}
	expectedStopTime := mustParseEventTime(t, "2026-01-15 15:00 UTC")
	if !events[2].Time.Equal(expectedStopTime) {
		t.Errorf("expected Stop time %v, got %v", expectedStopTime, events[2].Time)
	}
	if events[2].Body != "Session closed." {
		t.Errorf("expected Stop body, got %q", events[2].Body)
	}
}

func TestParseSessionEventsEmptyBody(t *testing.T) {
	events := ParseSessionEvents("")
	if len(events) != 0 {
		t.Fatalf("expected 0 events for empty body, got %d", len(events))
	}
}

func TestParseSessionEventsNoHeadings(t *testing.T) {
	events := ParseSessionEvents("just some text\nno headings here\n")
	if len(events) != 0 {
		t.Fatalf("expected 0 events, got %d", len(events))
	}
}

func TestParseSessionEventsBlocker(t *testing.T) {
	body := "\n## Start\n\nStarted work.\n\n## Blocker - 2026-01-15 15:30 UTC\n\nWaiting for API key approval.\n"

	events := ParseSessionEvents(body)
	if len(events) != 2 {
		t.Fatalf("expected 2 events, got %d", len(events))
	}
	if events[1].Type != "Blocker" {
		t.Errorf("expected second event to be Blocker, got %s", events[1].Type)
	}
	expectedBlockerTime := mustParseEventTime(t, "2026-01-15 15:30 UTC")
	if !events[1].Time.Equal(expectedBlockerTime) {
		t.Errorf("expected Blocker time %v, got %v", expectedBlockerTime, events[1].Time)
	}
	if events[1].Body != "Waiting for API key approval." {
		t.Errorf("expected Blocker body, got %q", events[1].Body)
	}
}

func TestParseSessionEventsSupportsMultiDayTimestamps(t *testing.T) {
	body := "\n## Start\n\nStarted work.\n\n## Stop - 2026-01-17 12:00 UTC\n\nSession closed.\n"

	events := ParseSessionEvents(body)
	if len(events) != 2 {
		t.Fatalf("expected 2 events, got %d", len(events))
	}

	expectedStopTime := mustParseEventTime(t, "2026-01-17 12:00 UTC")
	if !events[1].Time.Equal(expectedStopTime) {
		t.Errorf("expected Stop time %v, got %v", expectedStopTime, events[1].Time)
	}
}

func TestParseSessionEventsSkipsOldHHMMHeadings(t *testing.T) {
	body := "\n## Start\n\nStarted work.\n\n## Stop - 15:30 UTC\n\nSession closed.\n"

	events := ParseSessionEvents(body)
	if len(events) != 1 {
		t.Fatalf("expected only Start event, got %d", len(events))
	}
	if events[0].Type != "Start" {
		t.Fatalf("expected only parsed event to be Start, got %s", events[0].Type)
	}
}

func mustParseEventTime(t *testing.T, value string) time.Time {
	t.Helper()

	at, err := time.Parse(eventTimeLayout+" UTC", value)
	if err != nil {
		t.Fatalf("parse event time %q failed: %v", value, err)
	}
	return at
}

func TestParseSessionEventsEditReplacesBody(t *testing.T) {
	body := "\n## Start\n\nStarted work.\n\n## Note - 2026-01-15 14:30 UTC\n\nOriginal note text\n\n## Edit - 2026-01-15 14:35 UTC\n\nNote 14:30\nCorrected note text\n"

	events := ParseSessionEvents(body)
	if len(events) != 2 {
		t.Fatalf("expected 2 events (Start + corrected Note), got %d", len(events))
	}
	if events[1].Type != "Note" {
		t.Errorf("expected Note, got %s", events[1].Type)
	}
	if events[1].Body != "Corrected note text" {
		t.Errorf("expected corrected body %q, got %q", "Corrected note text", events[1].Body)
	}
	if events[1].IsDeleted {
		t.Error("IsDeleted should be false for corrected event")
	}
}

func TestParseSessionEventsStructuredEditReplacesBody(t *testing.T) {
	body := "\n## Start\n\nStarted work.\n\n## Note - 2026-01-15 14:30 UTC\n\nOriginal note text\n\n## Edit - 2026-01-15 14:35 UTC\n\nTarget: Note 14:30\nAction: update\n\nOriginal:\nOriginal note text\n\nNew:\nCorrected note text\n"

	events := ParseSessionEvents(body)
	if len(events) != 2 {
		t.Fatalf("expected 2 events (Start + corrected Note), got %d", len(events))
	}
	if events[1].Body != "Corrected note text" {
		t.Errorf("expected corrected body %q, got %q", "Corrected note text", events[1].Body)
	}
	if events[1].IsDeleted {
		t.Error("IsDeleted should be false for corrected event")
	}
	expectedModified := mustParseEventTime(t, "2026-01-15 14:35 UTC")
	if !events[1].CorrectedAt.Equal(expectedModified) {
		t.Errorf("CorrectedAt = %v, want %v", events[1].CorrectedAt, expectedModified)
	}
}

func TestParseSessionEventsStructuredEditDeletesBodyMatchedEvent(t *testing.T) {
	body := "\n## Start\n\nStarted work.\n\n## Note - 2026-01-15 14:30 UTC\n\nFirst note\n\n## Note - 2026-01-15 14:30 UTC\n\nSecond note\n\n## Edit - 2026-01-15 14:35 UTC\n\nTarget: Note 14:30\nAction: delete\n\nOriginal:\nFirst note\n"

	events := ParseSessionEvents(body)
	if len(events) != 3 {
		t.Fatalf("expected 3 events, got %d", len(events))
	}
	if !events[1].IsDeleted {
		t.Error("first matching note should be deleted")
	}
	if events[2].IsDeleted {
		t.Error("second same-minute note should not be deleted")
	}
	if events[2].Body != "Second note" {
		t.Errorf("second note body = %q, want %q", events[2].Body, "Second note")
	}
}

func TestParseSessionEventsEditDeletesEvent(t *testing.T) {
	body := "\n## Start\n\nStarted work.\n\n## Note - 2026-01-15 14:30 UTC\n\nOriginal note\n\n## Edit - 2026-01-15 14:35 UTC\n\nNote 14:30\n"

	events := ParseSessionEvents(body)
	if len(events) != 2 {
		t.Fatalf("expected 2 events (Start + deleted Note), got %d", len(events))
	}
	if events[1].Type != "Note" {
		t.Errorf("expected Note, got %s", events[1].Type)
	}
	if !events[1].IsDeleted {
		t.Error("IsDeleted should be true for deleted event")
	}
	if events[1].Body != "" {
		t.Errorf("deleted event body should be empty, got %q", events[1].Body)
	}
}

func TestParseSessionEventsEditDoesNotMatchStart(t *testing.T) {
	body := "\n## Start\n\nStarted work.\n\n## Edit - 2026-01-15 14:35 UTC\n\nStart \nNew start\n"

	events := ParseSessionEvents(body)
	if len(events) != 1 {
		t.Fatalf("expected 1 event (Start, Edit should not match), got %d", len(events))
	}
	if events[0].Type != "Start" {
		t.Errorf("expected Start, got %s", events[0].Type)
	}
	if events[0].Body != "Started work." {
		t.Errorf("Start body should be unchanged, got %q", events[0].Body)
	}
}

func TestParseSessionEventsEditMultipleEvents(t *testing.T) {
	body := "\n## Start\n\nStarted work.\n\n## Note - 2026-01-15 14:30 UTC\n\nFirst note\n\n## Note - 2026-01-15 14:35 UTC\n\nSecond note\n\n## Edit - 2026-01-15 14:40 UTC\n\nNote 14:30\nCorrected first note\n"

	events := ParseSessionEvents(body)
	if len(events) != 3 {
		t.Fatalf("expected 3 events, got %d", len(events))
	}
	if events[1].Body != "Corrected first note" {
		t.Errorf("first note should be corrected, got %q", events[1].Body)
	}
	if events[2].Body != "Second note" {
		t.Errorf("second note should be unchanged, got %q", events[2].Body)
	}
}

func TestParseSessionEventsEditLastOneWins(t *testing.T) {
	body := "\n## Start\n\nStarted work.\n\n## Note - 2026-01-15 14:30 UTC\n\nOriginal\n\n## Edit - 2026-01-15 14:35 UTC\n\nNote 14:30\nFirst correction\n\n## Edit - 2026-01-15 14:40 UTC\n\nNote 14:30\nSecond correction\n"

	events := ParseSessionEvents(body)
	if len(events) != 2 {
		t.Fatalf("expected 2 events, got %d", len(events))
	}
	if events[1].Body != "Second correction" {
		t.Errorf("last correction should win, got %q", events[1].Body)
	}
}

func TestParseSessionEventsEditBlocker(t *testing.T) {
	body := "\n## Start\n\nStarted work.\n\n## Blocker - 2026-01-15 14:30 UTC\n\nOriginal blocker\n\n## Edit - 2026-01-15 14:35 UTC\n\nBlocker 14:30\nUpdated blocker\n"

	events := ParseSessionEvents(body)
	if len(events) != 2 {
		t.Fatalf("expected 2 events, got %d", len(events))
	}
	if events[1].Type != "Blocker" {
		t.Errorf("expected Blocker, got %s", events[1].Type)
	}
	if events[1].Body != "Updated blocker" {
		t.Errorf("expected corrected blocker body, got %q", events[1].Body)
	}
}

func TestParseSessionEventsEditNoMatch(t *testing.T) {
	body := "\n## Start\n\nStarted work.\n\n## Note - 2026-01-15 14:30 UTC\n\nOriginal note\n\n## Edit - 2026-01-15 14:35 UTC\n\nNote 99:99\nDoes not match\n"

	events := ParseSessionEvents(body)
	if len(events) != 2 {
		t.Fatalf("expected 2 events (bad edit ignored), got %d", len(events))
	}
	if events[1].Body != "Original note" {
		t.Errorf("note body should be unchanged, got %q", events[1].Body)
	}
}

func TestParseSessionEventsEditEmptyBodyNoop(t *testing.T) {
	body := "\n## Start\n\nStarted work.\n\n## Note - 2026-01-15 14:30 UTC\n\nOriginal note\n\n## Edit - 2026-01-15 14:35 UTC\n"

	events := ParseSessionEvents(body)
	if len(events) != 2 {
		t.Fatalf("expected 2 events, got %d", len(events))
	}
	if events[1].Body != "Original note" {
		t.Errorf("note body should be unchanged when correction body is empty, got %q", events[1].Body)
	}
}

func readSessionFile(t *testing.T, root, sessionID string) string {
	t.Helper()

	path := filepath.Join(root, sessionsDir, sessionID+".md")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read session file failed: %v", err)
	}
	return string(data)
}
