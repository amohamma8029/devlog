package main

import (
	"bytes"
	"testing"
)

func TestOpenCommandShowsStyledConfirmation(t *testing.T) {
	requireCmdTestGit(t)
	t.Setenv("DEVLOG_AUTHOR_NAME", "Test Author")
	t.Setenv("DEVLOG_AUTHOR_EMAIL", "test@example.com")

	root := initCmdTestRepo(t)
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
