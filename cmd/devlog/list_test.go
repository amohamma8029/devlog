package main

import (
	"bytes"
	"strings"
	"testing"
	"time"

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
