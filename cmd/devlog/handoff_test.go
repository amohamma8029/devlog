package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestHandoffCommandPrintsToStdout(t *testing.T) {
	requireCmdTestGit(t)

	root := initCmdTestRepo(t)
	sess := writeCmdTestSession(t, root)
	appendCmdTestEvent(t, root, sess.ID, "Note", "finished auth module")
	appendCmdTestEvent(t, root, sess.ID, "Blocker", "waiting for PR review")
	t.Chdir(root)

	out, err := executeHandoffCommand()
	if err != nil {
		t.Fatalf("handoff command failed: %v", err)
	}

	assertContains(t, out, "# Handoff: feat/test — "+sess.ID+" (Test Author) [active]")
	assertContains(t, out, "## Summary")
	assertContains(t, out, "Progress: finished auth module")
	assertContains(t, out, "Blockers: waiting for PR review")
	assertContains(t, out, "## Changes")
}

func TestHandoffCommandWithSessionID(t *testing.T) {
	requireCmdTestGit(t)

	root := initCmdTestRepo(t)
	sess := writeCmdTestSession(t, root)
	appendCmdTestEvent(t, root, sess.ID, "Note", "worked on feature X")
	t.Chdir(root)

	out, err := executeHandoffCommand(sess.ID)
	if err != nil {
		t.Fatalf("handoff command with session id failed: %v", err)
	}

	assertContains(t, out, sess.ID)
	assertContains(t, out, "Progress: worked on feature X")
}

func TestHandoffCommandWritesToFile(t *testing.T) {
	requireCmdTestGit(t)

	root := initCmdTestRepo(t)
	sess := writeCmdTestSession(t, root)
	appendCmdTestEvent(t, root, sess.ID, "Note", "wrote handoff tests")
	t.Chdir(root)

	outFile := filepath.Join(root, "handoff-output.md")
	out, err := executeHandoffCommand("-o", outFile)
	if err != nil {
		t.Fatalf("handoff command with output flag failed: %v", err)
	}

	assertContains(t, out, "Handoff written to "+outFile)

	data, err := os.ReadFile(outFile)
	if err != nil {
		t.Fatalf("read output file failed: %v", err)
	}
	if !strings.Contains(string(data), "# Handoff:") {
		t.Errorf("output file should contain handoff content, got:\n%s", string(data))
	}
}

func TestHandoffCommandFailsWithoutActiveSession(t *testing.T) {
	requireCmdTestGit(t)

	root := initCmdTestRepo(t)
	t.Chdir(root)

	_, err := executeHandoffCommand()
	if err == nil || !strings.Contains(err.Error(), "no active session is in progress") {
		t.Fatalf("expected no active session error, got: %v", err)
	}
}

func TestHandoffCommandFailsWithNonexistentID(t *testing.T) {
	requireCmdTestGit(t)

	root := initCmdTestRepo(t)
	t.Chdir(root)

	_, err := executeHandoffCommand("nonexistent-id")
	if err == nil || !strings.Contains(err.Error(), "session file not found") {
		t.Fatalf("expected session not found error, got: %v", err)
	}
}

func TestHandoffCommandFailsWithTooManyArgs(t *testing.T) {
	requireCmdTestGit(t)

	root := initCmdTestRepo(t)
	t.Chdir(root)

	_, err := executeHandoffCommand("id1", "id2")
	if err == nil || !strings.Contains(err.Error(), "too many arguments") {
		t.Fatalf("expected too many arguments error, got: %v", err)
	}
}

func TestHandoffCommandWithCodeChanges(t *testing.T) {
	requireCmdTestGit(t)

	root := initCmdTestRepo(t)
	sess := writeCmdTestSession(t, root)
	appendCmdTestEvent(t, root, sess.ID, "Note", "implemented feature Y")
	t.Chdir(root)

	// Make a code change so the diff is non-empty.
	if err := os.WriteFile(filepath.Join(root, "feature.go"), []byte("package main\n"), 0644); err != nil {
		t.Fatalf("write file failed: %v", err)
	}

	out, err := executeHandoffCommand()
	if err != nil {
		t.Fatalf("handoff command with code changes failed: %v", err)
	}

	assertContains(t, out, "feature.go")
}

func TestRootCommandIncludesHandoffCommand(t *testing.T) {
	cmd, _, err := newRootCommand().Find([]string{"handoff"})
	if err != nil {
		t.Fatalf("find handoff command failed: %v", err)
	}
	if cmd == nil || cmd.Name() != "handoff" {
		t.Fatalf("expected root command to include handoff, got %v", cmd)
	}
}

func executeHandoffCommand(args ...string) (string, error) {
	cmd := newHandoffCommand()
	var out bytes.Buffer
	var errOut bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&errOut)
	cmd.SetArgs(args)

	err := cmd.Execute()
	return out.String(), err
}
