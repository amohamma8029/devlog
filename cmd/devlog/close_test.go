package main

import (
	"bytes"
	"strings"
	"testing"

	"github.com/amo/devlog/internal/store"
)

func TestCloseCommandClosesActiveSession(t *testing.T) {
	requireCmdTestGit(t)

	root := initCmdTestRepo(t)
	sess := writeCmdTestSession(t, root)
	t.Chdir(root)

	out, err := executeCloseCommand()
	if err != nil {
		t.Fatalf("close command failed: %v", err)
	}

	wantOut := "Session " + sess.ID + " closed.\n"
	if out != wantOut {
		t.Fatalf("expected output %q, got %q", wantOut, out)
	}

	s, err := store.New(root)
	if err != nil {
		t.Fatalf("store.New failed: %v", err)
	}
	rec, err := s.GetSession(sess.ID)
	if err != nil {
		t.Fatalf("GetSession failed: %v", err)
	}
	if !rec.Closed {
		t.Fatal("expected session to be closed")
	}

	content := readCmdTestSessionFile(t, root, sess.ID)
	if !strings.Contains(content, "## Stop - ") {
		t.Fatal("expected session file to contain Stop event")
	}
	if !strings.Contains(content, "Session closed.") {
		t.Fatal("expected session file to contain close body")
	}
}

func TestCloseCommandFailsWithoutActiveSession(t *testing.T) {
	requireCmdTestGit(t)

	root := initCmdTestRepo(t)
	t.Chdir(root)

	_, err := executeCloseCommand()
	if err == nil || !strings.Contains(err.Error(), "no active session is in progress") {
		t.Fatalf("expected no active session error, got: %v", err)
	}
}

func TestRootCommandIncludesCloseCommand(t *testing.T) {
	cmd, _, err := newRootCommand().Find([]string{"close"})
	if err != nil {
		t.Fatalf("find close command failed: %v", err)
	}
	if cmd == nil || cmd.Name() != "close" {
		t.Fatalf("expected root command to include close, got %v", cmd)
	}
}

func executeCloseCommand(args ...string) (string, error) {
	cmd := newCloseCommand()
	var out bytes.Buffer
	var errOut bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&errOut)
	cmd.SetArgs(args)

	err := cmd.Execute()
	return out.String(), err
}
