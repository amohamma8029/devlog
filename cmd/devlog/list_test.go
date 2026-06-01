package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/amo/devlog/internal/store"
)

func TestListCommandListsAllSessions(t *testing.T) {
	requireCmdTestGit(t)

	root := initCmdTestRepo(t)
	_ = writeCmdTestSession(t, root)
	_ = writeCmdTestClosedSession(t, root)
	t.Chdir(root)

	out, err := executeListCommand()
	if err != nil {
		t.Fatalf("list command failed: %v", err)
	}

	if !strings.Contains(out, "start message") {
		t.Fatalf("expected output to contain title 'start message', got: %s", out)
	}
	if !strings.Contains(out, "start message") {
		t.Fatalf("expected output to contain title 'start message', got: %s", out)
	}
	if !strings.Contains(out, "active") {
		t.Fatal("expected output to contain active status")
	}
	if !strings.Contains(out, "closed") {
		t.Fatal("expected output to contain closed status")
	}
	for _, col := range []string{"TITLE", "BRANCH", "STATUS", "STARTED", "DURATION"} {
		if !strings.Contains(out, col) {
			t.Fatalf("expected table to contain %q column, got: %s", col, out)
		}
	}
}

func TestComputeListDurationUsesFullStopTimestampAcrossMidnight(t *testing.T) {
	root := t.TempDir()
	s := newCmdTestStore(t, root)
	sess := store.Session{
		ID:      "2026-01-15T230000Z",
		Author:  "Test Author",
		Started: time.Date(2026, 1, 15, 23, 0, 0, 0, time.UTC),
		Branch:  "feat/duration",
		Status:  "active",
	}

	if err := s.WriteSession(sess, "start message"); err != nil {
		t.Fatalf("WriteSession failed: %v", err)
	}
	appendCmdTestSessionBody(t, root, sess.ID, "\n## Stop - 2026-01-16 02:00 UTC\n\nSession closed.\n")

	rec, err := s.GetSession(sess.ID)
	if err != nil {
		t.Fatalf("GetSession failed: %v", err)
	}
	if !rec.Closed {
		t.Fatal("expected session to be closed")
	}

	got := computeListDuration(rec, root, time.Date(2026, 1, 16, 3, 0, 0, 0, time.UTC))
	if got != "3h" {
		t.Fatalf("expected duration 3h, got %q", got)
	}
}

func TestComputeListDurationUsesLastFullStopTimestamp(t *testing.T) {
	root := t.TempDir()
	s := newCmdTestStore(t, root)
	sess := store.Session{
		ID:      "2026-01-15T100000Z",
		Author:  "Test Author",
		Started: time.Date(2026, 1, 15, 10, 0, 0, 0, time.UTC),
		Branch:  "feat/duration",
		Status:  "active",
	}

	if err := s.WriteSession(sess, "start message"); err != nil {
		t.Fatalf("WriteSession failed: %v", err)
	}
	appendCmdTestSessionBody(t, root, sess.ID, "\n## Stop - 2026-01-16 11:00 UTC\n\nSession closed.\n")
	appendCmdTestSessionBody(t, root, sess.ID, "\n## Stop - 2026-01-17 12:00 UTC\n\nSession closed again.\n")

	rec, err := s.GetSession(sess.ID)
	if err != nil {
		t.Fatalf("GetSession failed: %v", err)
	}

	got := computeListDuration(rec, root, time.Date(2026, 1, 17, 13, 0, 0, 0, time.UTC))
	if got != "2d 2h" {
		t.Fatalf("expected duration 2d 2h, got %q", got)
	}
}

func TestOldHHMMStopHeadingDoesNotMarkSessionClosed(t *testing.T) {
	root := t.TempDir()
	s := newCmdTestStore(t, root)
	sess := store.Session{
		ID:      "2026-01-15T100000Z",
		Author:  "Test Author",
		Started: time.Date(2026, 1, 15, 10, 0, 0, 0, time.UTC),
		Branch:  "feat/legacy",
		Status:  "active",
	}

	if err := s.WriteSession(sess, "start message"); err != nil {
		t.Fatalf("WriteSession failed: %v", err)
	}
	appendCmdTestSessionBody(t, root, sess.ID, "\n## Stop - 14:30 UTC\n\nSession closed.\n")

	rec, err := s.GetSession(sess.ID)
	if err != nil {
		t.Fatalf("GetSession failed: %v", err)
	}
	if rec.Closed {
		t.Fatal("expected old HH:MM Stop heading not to mark session closed")
	}
}

func TestListCommandActiveFlag(t *testing.T) {
	requireCmdTestGit(t)

	root := initCmdTestRepo(t)
	_ = writeCmdTestSession(t, root)
	_ = writeCmdTestClosedSession(t, root)
	t.Chdir(root)

	out, err := executeListCommand("--active")
	if err != nil {
		t.Fatalf("list --active failed: %v", err)
	}

	if !strings.Contains(out, "start message") {
		t.Fatalf("expected output to contain title 'start message', got: %s", out)
	}
	if strings.Contains(out, "closed") {
		t.Fatal("expected --active output to not contain closed sessions")
	}
}

func TestListCommandBranchFlag(t *testing.T) {
	requireCmdTestGit(t)

	root := initCmdTestRepo(t)
	writeCmdTestSession(t, root)
	writeCmdTestSessionWithBranch(t, root, "feat/cli-ux")
	t.Chdir(root)

	out, err := executeListCommand("--branch", "cli-ux")
	if err != nil {
		t.Fatalf("list --branch failed: %v", err)
	}

	if !strings.Contains(out, "feat/cli-ux") {
		t.Fatalf("expected output to contain branch feat/cli-ux, got: %s", out)
	}
	if strings.Count(out, "feat/") > 1 {
		t.Fatalf("expected only one session with branch containing cli-ux, got: %s", out)
	}
}

func TestListCommandBranchFlagNoMatch(t *testing.T) {
	requireCmdTestGit(t)

	root := initCmdTestRepo(t)
	writeCmdTestSession(t, root)
	t.Chdir(root)

	out, err := executeListCommand("--branch", "nonexistent")
	if err != nil {
		t.Fatalf("list --branch failed: %v", err)
	}

	if !strings.Contains(out, "No sessions found.") {
		t.Fatalf("expected no sessions message with unmatched branch, got: %s", out)
	}
}

func TestListCommandEmptyRepo(t *testing.T) {
	requireCmdTestGit(t)

	root := initCmdTestRepo(t)
	t.Chdir(root)

	out, err := executeListCommand()
	if err != nil {
		t.Fatalf("list command failed: %v", err)
	}

	if !strings.Contains(out, "No sessions found.") {
		t.Fatalf("expected no sessions message for empty repo, got: %s", out)
	}
}

func TestListCommandNoArgs(t *testing.T) {
	cmd := newListCommand()
	if cmd.Args == nil {
		t.Fatal("expected list command to define Args validator")
	}
}

func TestRootCommandIncludesListCommand(t *testing.T) {
	cmd, _, err := newRootCommand().Find([]string{"list"})
	if err != nil {
		t.Fatalf("find list command failed: %v", err)
	}
	if cmd == nil || cmd.Name() != "list" {
		t.Fatalf("expected root command to include list, got %v", cmd)
	}
}

func writeCmdTestClosedSession(t *testing.T, root string) store.Session {
	t.Helper()

	sess := store.Session{
		ID:      "2026-02-20T090000Z",
		Author:  "Test Author",
		Started: time.Date(2026, 2, 20, 9, 0, 0, 0, time.UTC),
		Branch:  "feat/closed",
		Status:  "active",
	}

	s, err := store.New(root)
	if err != nil {
		t.Fatalf("store.New failed: %v", err)
	}

	if err := s.WriteSession(sess, "start message"); err != nil {
		t.Fatalf("WriteSession failed: %v", err)
	}
	if err := s.CloseSession(sess.ID); err != nil {
		t.Fatalf("CloseSession failed: %v", err)
	}

	return sess
}

func newCmdTestStore(t *testing.T, root string) *store.Store {
	t.Helper()

	s, err := store.New(root)
	if err != nil {
		t.Fatalf("store.New failed: %v", err)
	}
	return s
}

func appendCmdTestSessionBody(t *testing.T, root, sessionID, body string) {
	t.Helper()

	path := filepath.Join(root, ".devlog", "sessions", sessionID+".md")
	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		t.Fatalf("open session file failed: %v", err)
	}
	if _, err := f.WriteString(body); err != nil {
		closeErr := f.Close()
		if closeErr != nil {
			t.Fatalf("append session body failed: %v (close failed: %v)", err, closeErr)
		}
		t.Fatalf("append session body failed: %v", err)
	}
	if err := f.Close(); err != nil {
		t.Fatalf("close session file failed: %v", err)
	}
}

func writeCmdTestSessionWithBranch(t *testing.T, root, branch string) store.Session {
	t.Helper()

	sess := store.Session{
		ID:      "2026-03-21T120000Z",
		Author:  "Test Author",
		Started: time.Date(2026, 3, 21, 12, 0, 0, 0, time.UTC),
		Branch:  branch,
		Status:  "active",
	}

	s, err := store.New(root)
	if err != nil {
		t.Fatalf("store.New failed: %v", err)
	}

	if err := s.WriteSession(sess, "start message"); err != nil {
		t.Fatalf("WriteSession failed: %v", err)
	}

	return sess
}

func executeListCommand(args ...string) (string, error) {
	cmd := newListCommand()
	var out bytes.Buffer
	var errOut bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&errOut)
	cmd.SetArgs(args)

	err := cmd.Execute()
	return out.String(), err
}

func TestTruncateListFieldASCII(t *testing.T) {
	tests := []struct {
		name string
		s    string
		max  int
		want string
	}{
		{"shorter than max", "hello", 10, "hello"},
		{"exactly max", "hello", 5, "hello"},
		{"longer than max", "hello world", 6, "hello\u2026"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := truncateListField(tt.s, tt.max)
			if got != tt.want {
				t.Errorf("truncateListField(%q, %d) = %q, want %q", tt.s, tt.max, got, tt.want)
			}
		})
	}
}

func TestTruncateListFieldUnicode(t *testing.T) {
	tests := []struct {
		name string
		s    string
		max  int
	}{
		{"multi-byte shorter than max in runes", "café", 10},
		{"multi-byte longer than max in runes", "café résumé", 5},
		{"emoji shorter", "hello 🚀", 10},
		{"emoji longer", "hello 🚀 world", 8},
		{"CJK shorter", "日本語テスト", 10},
		{"CJK longer", "日本語テスト文章", 4},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := truncateListField(tt.s, tt.max)
			if !utf8.ValidString(got) {
				t.Errorf("truncateListField(%q, %d) = %q is not valid UTF-8", tt.s, tt.max, got)
			}
			if utf8.RuneCountInString(got) > tt.max {
				t.Errorf("truncateListField(%q, %d) = %q has %d runes, exceeds max %d", tt.s, tt.max, got, utf8.RuneCountInString(got), tt.max)
			}
		})
	}
}
