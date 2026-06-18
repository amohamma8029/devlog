package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestHandoffCommandWritesToDefaultPath(t *testing.T) {
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

	expectedPath := filepath.Join(root, ".devlog", "handoffs", sess.ID+".md")
	expectedDisplayPath := ".devlog/handoffs/" + sess.ID + ".md"
	assertContains(t, out, "Handoff written")
	assertContains(t, out, "path: "+expectedDisplayPath)
	assertContains(t, out, "session: start message ("+sess.ID+")")
	assertNotContains(t, out, "id: "+sess.ID)
	assertContains(t, out, "branch: feat/test (active)")
	if strings.Index(out, "path:") < strings.Index(out, "branch:") {
		t.Fatalf("path should render below session metadata, got:\n%s", out)
	}
	assertNotContains(t, out, "# Handoff:")
	assertNotContains(t, out, "## Summary")

	data, err := os.ReadFile(expectedPath)
	if err != nil {
		t.Fatalf("read output file failed: %v", err)
	}
	content := string(data)
	assertContains(t, content, "# Handoff: feat/test — "+sess.ID+" (Test Author) [active]")
	assertContains(t, content, "## Summary")
	assertContains(t, content, "Progress: finished auth module")
	assertContains(t, content, "Blockers: waiting for PR review")
	assertContains(t, content, "## Changes")
}

func TestHandoffCommandWithSessionID(t *testing.T) {
	requireCmdTestGit(t)

	root := initCmdTestRepo(t)
	sess := writeCmdTestSession(t, root)
	appendCmdTestEvent(t, root, sess.ID, "Note", "worked on feature X")
	t.Chdir(root)

	_, err := executeHandoffCommand(sess.ID)
	if err != nil {
		t.Fatalf("handoff command with session id failed: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(root, ".devlog", "handoffs", sess.ID+".md"))
	if err != nil {
		t.Fatalf("read output file failed: %v", err)
	}
	content := string(data)
	assertContains(t, content, sess.ID)
	assertContains(t, content, "Progress: worked on feature X")
}

func TestHandoffCommandWritesToCustomFilename(t *testing.T) {
	requireCmdTestGit(t)

	root := initCmdTestRepo(t)
	sess := writeCmdTestSession(t, root)
	appendCmdTestEvent(t, root, sess.ID, "Note", "wrote handoff tests")
	t.Chdir(root)

	out, err := executeHandoffCommand("-o", "my-handoff")
	if err != nil {
		t.Fatalf("handoff command with output flag failed: %v", err)
	}

	expectedPath := filepath.Join(root, ".devlog", "handoffs", "my-handoff.md")
	assertContains(t, out, "Handoff written")
	assertContains(t, out, "path: .devlog/handoffs/my-handoff.md")

	data, err := os.ReadFile(expectedPath)
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

	_, err := executeHandoffCommand()
	if err != nil {
		t.Fatalf("handoff command with code changes failed: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(root, ".devlog", "handoffs", sess.ID+".md"))
	if err != nil {
		t.Fatalf("read output file failed: %v", err)
	}
	content := string(data)
	assertContains(t, content, "#### feature.go")
	assertContains(t, content, "```diff")
}

func TestHandoffCommandNoDiffFlagExcludesRawDiff(t *testing.T) {
	requireCmdTestGit(t)

	root := initCmdTestRepo(t)
	sess := writeCmdTestSession(t, root)
	appendCmdTestEvent(t, root, sess.ID, "Note", "implemented feature Y")
	t.Chdir(root)

	if err := os.WriteFile(filepath.Join(root, "feature.go"), []byte("package main\n"), 0644); err != nil {
		t.Fatalf("write file failed: %v", err)
	}

	_, err := executeHandoffCommand("--no-diff")
	if err != nil {
		t.Fatalf("handoff command with --no-diff failed: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(root, ".devlog", "handoffs", sess.ID+".md"))
	if err != nil {
		t.Fatalf("read output file failed: %v", err)
	}
	content := string(data)
	assertContains(t, content, "## Changes")
	assertContains(t, content, "feature.go")
	if strings.Contains(content, "```diff") {
		t.Fatalf("--no-diff output should omit raw diff blocks, got:\n%s", content)
	}
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

func TestHandoffCommandRejectsPathTraversal(t *testing.T) {
	requireCmdTestGit(t)

	root := initCmdTestRepo(t)
	writeCmdTestSession(t, root)
	t.Chdir(root)

	_, err := executeHandoffCommand("-o", "../../escape")
	if err == nil || !strings.Contains(err.Error(), "invalid filename") {
		t.Fatalf("expected invalid filename error, got: %v", err)
	}
}

func TestHandoffCommandRejectsPathSeparator(t *testing.T) {
	requireCmdTestGit(t)

	root := initCmdTestRepo(t)
	writeCmdTestSession(t, root)
	t.Chdir(root)

	_, err := executeHandoffCommand("-o", "path/to/file")
	if err == nil || !strings.Contains(err.Error(), "invalid filename") {
		t.Fatalf("expected invalid filename error, got: %v", err)
	}
}

func TestHandoffCommandRejectsBackslash(t *testing.T) {
	requireCmdTestGit(t)

	root := initCmdTestRepo(t)
	writeCmdTestSession(t, root)
	t.Chdir(root)

	_, err := executeHandoffCommand("-o", "path\\to\\file")
	if err == nil || !strings.Contains(err.Error(), "invalid filename") {
		t.Fatalf("expected invalid filename error, got: %v", err)
	}
}

func TestHandoffCommandRejectsOverwrite(t *testing.T) {
	requireCmdTestGit(t)

	root := initCmdTestRepo(t)
	writeCmdTestSession(t, root)
	t.Chdir(root)

	_, err := executeHandoffCommand("-o", "my-handoff")
	if err != nil {
		t.Fatalf("first handoff command failed: %v", err)
	}

	_, err = executeHandoffCommand("-o", "my-handoff")
	if err == nil || !strings.Contains(err.Error(), "file already exists") {
		t.Fatalf("expected file already exists error, got: %v", err)
	}
}

func TestHandoffCommandRejectsOverwriteDefault(t *testing.T) {
	requireCmdTestGit(t)

	root := initCmdTestRepo(t)
	writeCmdTestSession(t, root)
	t.Chdir(root)

	_, err := executeHandoffCommand()
	if err != nil {
		t.Fatalf("first handoff command failed: %v", err)
	}

	_, err = executeHandoffCommand()
	if err == nil || !strings.Contains(err.Error(), "file already exists") {
		t.Fatalf("expected file already exists error, got: %v", err)
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
