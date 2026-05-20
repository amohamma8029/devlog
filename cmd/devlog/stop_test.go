package main

import (
	"bytes"
	"strings"
	"testing"

	"github.com/amo/devlog/internal/store"
)

func TestStopCommandClosesActiveSession(t *testing.T) {
	requireCmdTestGit(t)

	root := initCmdTestRepo(t)
	sess := writeCmdTestSession(t, root)
	t.Chdir(root)

	out, err := executeStopCommand()
	if err != nil {
		t.Fatalf("stop command failed: %v", err)
	}

	wantOut := "Session " + sess.ID + " stopped.\n"
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
		t.Fatal("expected session file to contain stop body")
	}
}

func TestStopCommandFailsWithoutActiveSession(t *testing.T) {
	requireCmdTestGit(t)

	root := initCmdTestRepo(t)
	t.Chdir(root)

	_, err := executeStopCommand()
	if err == nil || !strings.Contains(err.Error(), "no active session found") {
		t.Fatalf("expected no active session error, got: %v", err)
	}
}

func TestRootCommandIncludesStopCommand(t *testing.T) {
	cmd, _, err := newRootCommand().Find([]string{"stop"})
	if err != nil {
		t.Fatalf("find stop command failed: %v", err)
	}
	if cmd == nil || cmd.Name() != "stop" {
		t.Fatalf("expected root command to include stop, got %v", cmd)
	}
}

func executeStopCommand(args ...string) (string, error) {
	cmd := newStopCommand()
	var out bytes.Buffer
	var errOut bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&errOut)
	cmd.SetArgs(args)

	err := cmd.Execute()
	return out.String(), err
}
