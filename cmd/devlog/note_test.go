package main

import (
	"bytes"
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
