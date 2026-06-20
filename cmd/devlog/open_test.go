package main

import (
	"bytes"
	"strings"
	"testing"

	"github.com/amo/devlog/internal/store"
)

func TestOpenCommandShowsStyledConfirmation(t *testing.T) {
	requireCmdTestGit(t)
	setConfigTestHome(t)

	root := initCmdTestRepo(t)
	runCmdTestGit(t, root, "config", "user.name", "Test Author")
	runCmdTestGit(t, root, "config", "user.email", "test@example.com")
	t.Chdir(root)

	out, err := executeOpenCommand("CLI", "output", "polish")
	if err != nil {
		t.Fatalf("open command failed: %v", err)
	}

	assertContains(t, out, "Opened session")
	assertContains(t, out, "session: CLI output polish (")
	assertNotContains(t, out, "id: ")
	assertContains(t, out, "branch: feat/test (active)")
	assertNotContains(t, out, "╭")

	sess := readSingleOpenSession(t, root)
	if sess.Author != "Test Author" || sess.Email != "test@example.com" {
		t.Fatalf("session identity = %q <%s>, want Test Author <test@example.com>", sess.Author, sess.Email)
	}
}

func TestOpenCommandUsesConfiguredAuthorProfile(t *testing.T) {
	requireCmdTestGit(t)
	_, configPath := setConfigTestHome(t)
	writeConfigTestFile(t, configPath, `author:
  default_profile: opencode
  profiles:
    opencode:
      display: "OpenCode"
      email: "opencode@example.com"
`)

	root := initCmdTestRepo(t)
	t.Chdir(root)

	if _, err := executeOpenCommand("custom", "profile"); err != nil {
		t.Fatalf("open command failed: %v", err)
	}

	sess := readSingleOpenSession(t, root)
	if sess.Author != "OpenCode" || sess.Email != "opencode@example.com" {
		t.Fatalf("session identity = %q <%s>, want OpenCode <opencode@example.com>", sess.Author, sess.Email)
	}
}

func TestOpenCommandInvalidConfigDoesNotCreateSession(t *testing.T) {
	requireCmdTestGit(t)
	_, configPath := setConfigTestHome(t)
	writeConfigTestFile(t, configPath, `author:
  default_profile: missing
`)

	root := initCmdTestRepo(t)
	t.Chdir(root)

	_, err := executeOpenCommand("bad", "config")
	if err == nil || !strings.Contains(err.Error(), "author.default_profile") {
		t.Fatalf("expected author.default_profile error, got: %v", err)
	}
	if sessions := listOpenSessions(t, root); len(sessions) != 0 {
		t.Fatalf("session count = %d, want 0", len(sessions))
	}
}

func executeOpenCommand(args ...string) (string, error) {
	cmd := newOpenCommand()
	var out bytes.Buffer
	var errOut bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&errOut)
	cmd.SetArgs(args)

	err := cmd.Execute()
	return out.String(), err
}

func listOpenSessions(t *testing.T, root string) []store.SessionRecord {
	t.Helper()

	s, err := store.New(root)
	if err != nil {
		t.Fatalf("store.New failed: %v", err)
	}
	records, err := s.ListSessions()
	if err != nil {
		t.Fatalf("ListSessions failed: %v", err)
	}
	return records
}

func readSingleOpenSession(t *testing.T, root string) store.SessionRecord {
	t.Helper()

	records := listOpenSessions(t, root)
	if len(records) != 1 {
		t.Fatalf("session count = %d, want 1", len(records))
	}
	return records[0]
}
