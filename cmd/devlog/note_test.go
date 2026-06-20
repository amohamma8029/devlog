package main

import (
	"bytes"
	"os"
	"strings"
	"testing"
)

func TestNoteCommandShowsStyledConfirmation(t *testing.T) {
	requireCmdTestGit(t)

	root := initCmdTestRepo(t)
	sess := writeCmdTestSession(t, root)
	t.Chdir(root)

	out, err := executeNoteCommand("-m", "styled note")
	if err != nil {
		t.Fatalf("note command failed: %v", err)
	}

	content := readCmdTestSessionFile(t, root, sess.ID)
	if !strings.Contains(content, "styled note") {
		t.Fatal("expected session file to contain note body")
	}

	assertContains(t, out, "Added note")
	assertContains(t, out, "→")
	assertContains(t, out, "styled note")
	assertContains(t, out, "session: start message ("+sess.ID+")")
	assertNotContains(t, out, "id: "+sess.ID)
	assertContains(t, out, "branch: feat/test (active)")
}

func TestNoteCommandUsesConfiguredEditor(t *testing.T) {
	requireCmdTestGit(t)
	_, configPath := setConfigTestHome(t)
	t.Setenv("EDITOR", "env-editor --wait")
	writeConfigTestFile(t, configPath, `editor:
  command: configured-editor
  args: ["--wait", "--reuse-window"]
`)

	root := initCmdTestRepo(t)
	sess := writeCmdTestSession(t, root)
	t.Chdir(root)

	var calls []editorCall
	withBodyEditor(t, func(editor configEditor, path string) error {
		calls = append(calls, editorCall{editor: editor, path: path})
		return os.WriteFile(path, []byte("configured note\n"), 0644)
	})

	out, err := executeNoteCommand()
	if err != nil {
		t.Fatalf("note command failed: %v", err)
	}

	assertBodyEditorCall(t, calls, "configured-editor", []string{"--wait", "--reuse-window"})
	content := readCmdTestSessionFile(t, root, sess.ID)
	assertContains(t, content, "configured note")
	assertContains(t, out, "Added note")
}

func TestNoteCommandInvalidConfigDoesNotLaunchEditor(t *testing.T) {
	_, configPath := setConfigTestHome(t)
	t.Setenv("EDITOR", "env-editor")
	writeConfigTestFile(t, configPath, `display:
  timezone: "New York"
`)

	withBodyEditor(t, func(editor configEditor, path string) error {
		t.Fatal("body editor should not launch for invalid config")
		return nil
	})

	_, err := executeNoteCommand()
	if err == nil || !strings.Contains(err.Error(), "display.timezone") {
		t.Fatalf("expected invalid config error, got: %v", err)
	}
}

func executeNoteCommand(args ...string) (string, error) {
	cmd := newNoteCommand()
	var out bytes.Buffer
	var errOut bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&errOut)
	cmd.SetArgs(args)

	err := cmd.Execute()
	return out.String(), err
}
