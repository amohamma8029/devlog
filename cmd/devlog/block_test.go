package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/amo/devlog/internal/store"
)

func TestBlockCommandAppendsBlockerWithFlag(t *testing.T) {
	requireCmdTestGit(t)

	root := initCmdTestRepo(t)
	sess := writeCmdTestSession(t, root)
	t.Chdir(root)

	out, err := executeBlockCommand("-m", "waiting for API review")
	if err != nil {
		t.Fatalf("block command failed: %v", err)
	}

	content := readCmdTestSessionFile(t, root, sess.ID)
	if !strings.Contains(content, "## Blocker - ") {
		t.Fatal("expected session file to contain Blocker event")
	}
	if !strings.Contains(content, "waiting for API review") {
		t.Fatal("expected session file to contain blocker body")
	}

	wantOut := "Recorded blocker in session " + sess.ID + "\n"
	if out != wantOut {
		t.Fatalf("expected output %q, got %q", wantOut, out)
	}
}

func TestBlockCommandAppendsBlockerWithPositionalMessage(t *testing.T) {
	requireCmdTestGit(t)

	root := initCmdTestRepo(t)
	sess := writeCmdTestSession(t, root)
	t.Chdir(root)

	_, err := executeBlockCommand("DB", "migration", "failing")
	if err != nil {
		t.Fatalf("block command failed: %v", err)
	}

	content := readCmdTestSessionFile(t, root, sess.ID)
	if !strings.Contains(content, "DB migration failing") {
		t.Fatal("expected positional args to be joined into blocker body")
	}
}

func TestBlockCommandFailsWithoutMessageOrEditor(t *testing.T) {
	t.Setenv("EDITOR", "")

	_, err := executeBlockCommand()
	if err == nil || !strings.Contains(err.Error(), "no message provided and $EDITOR") {
		t.Fatalf("expected missing message error, got: %v", err)
	}
}

func TestBlockCommandFailsWithoutActiveSession(t *testing.T) {
	requireCmdTestGit(t)

	root := initCmdTestRepo(t)
	t.Chdir(root)

	_, err := executeBlockCommand("-m", "blocked")
	if err == nil || !strings.Contains(err.Error(), "no active session is in progress") {
		t.Fatalf("expected no active session error, got: %v", err)
	}
}

func executeBlockCommand(args ...string) (string, error) {
	cmd := newBlockCommand()
	var out bytes.Buffer
	var errOut bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&errOut)
	cmd.SetArgs(args)

	err := cmd.Execute()
	return out.String(), err
}

func requireCmdTestGit(t *testing.T) {
	t.Helper()

	if _, err := exec.LookPath("git"); err != nil {
		t.Skipf("git binary not available: %v", err)
	}
}

func initCmdTestRepo(t *testing.T) string {
	t.Helper()

	root := t.TempDir()
	runCmdTestGit(t, root, "init")
	runCmdTestGit(t, root, "checkout", "-b", "feat/test")
	return root
}

func runCmdTestGit(t *testing.T, dir string, args ...string) {
	t.Helper()

	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s failed: %v\n%s", strings.Join(args, " "), err, strings.TrimSpace(string(output)))
	}
}

func writeCmdTestSession(t *testing.T, root string) store.Session {
	t.Helper()

	s, err := store.New(root)
	if err != nil {
		t.Fatalf("store.New failed: %v", err)
	}

	sess := store.Session{
		ID:      "2026-01-15T140000Z",
		Author:  "Test Author",
		Started: time.Date(2026, 1, 15, 14, 0, 0, 0, time.UTC),
		Branch:  "feat/test",
		Status:  "active",
	}
	if err := s.WriteSession(sess, "start message"); err != nil {
		t.Fatalf("WriteSession failed: %v", err)
	}

	return sess
}

func readCmdTestSessionFile(t *testing.T, root, sessionID string) string {
	t.Helper()

	data, err := os.ReadFile(filepath.Join(root, ".devlog", "sessions", sessionID+".md"))
	if err != nil {
		t.Fatalf("read session file failed: %v", err)
	}
	return string(data)
}
