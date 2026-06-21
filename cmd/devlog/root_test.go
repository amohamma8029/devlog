package main

import (
	"bytes"
	"fmt"
	"testing"
)

func TestRootCommandLaunchesTUIWithoutArgs(t *testing.T) {
	original := launchTUI
	defer func() { launchTUI = original }()

	called := false
	launchTUI = func() error {
		called = true
		return nil
	}

	cmd := newRootCommand()
	var out, errOut bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&errOut)
	cmd.SetArgs(nil)

	if err := cmd.Execute(); err != nil {
		t.Fatalf("root command failed: %v", err)
	}
	if !called {
		t.Fatal("expected launchTUI to be called when no args provided")
	}
}

func TestRootCommandShowsVersionWhenFlagSet(t *testing.T) {
	original := launchTUI
	defer func() { launchTUI = original }()

	launchTUI = func() error {
		t.Fatal("launchTUI should not be called when --version is set")
		return nil
	}

	cmd := newRootCommand()
	var out, errOut bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&errOut)
	cmd.SetArgs([]string{"--version"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("root command with --version failed: %v", err)
	}
	if out.String() == "" {
		t.Fatal("expected version output, got empty")
	}
	assertContains(t, out.String(), "devlog")
	assertContains(t, out.String(), "version: "+version)
}

func TestRootCommandRuntimeErrorDoesNotPrintUsage(t *testing.T) {
	original := launchTUI
	defer func() { launchTUI = original }()

	launchTUI = func() error {
		return fmt.Errorf("display.timezone %q must be %q, %q, or a valid IANA timezone", "New York", "UTC", "local")
	}

	cmd := newRootCommand()
	var out, errOut bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&errOut)
	cmd.SetArgs(nil)

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected runtime error, got nil")
	}
	assertContains(t, err.Error(), "display.timezone")
	assertNotContains(t, errOut.String(), "Usage:")
	assertNotContains(t, errOut.String(), "Flags:")
}

func TestRootHelpUsesStaticStyleAndHidesCompletion(t *testing.T) {
	original := launchTUI
	defer func() { launchTUI = original }()
	launchTUI = func() error {
		t.Fatal("launchTUI should not be called for help")
		return nil
	}

	cmd := newRootCommand()
	var out, errOut bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&errOut)
	cmd.SetArgs([]string{"--help"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("root help failed: %v", err)
	}
	assertContains(t, out.String(), "devlog")
	assertContains(t, out.String(), "Commands")
	assertContains(t, out.String(), "Session workflow")
	assertContains(t, out.String(), cliHelpGroupStyle.Render("Session workflow"))
	assertContains(t, out.String(), "Review")
	assertContains(t, out.String(), cliHelpGroupStyle.Render("Review"))
	assertContains(t, out.String(), "open")
	assertContains(t, out.String(), "status")
	assertContains(t, out.String(), "handoff")
	assertNotContains(t, out.String(), "completion")
	assertNotContains(t, out.String(), "╭")
}

func TestCommandHelpUsesStaticStyle(t *testing.T) {
	cmd := newRootCommand()
	var out, errOut bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&errOut)
	cmd.SetArgs([]string{"status", "--help"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("status help failed: %v", err)
	}
	assertContains(t, out.String(), "devlog status")
	assertContains(t, out.String(), "usage:")
	assertContains(t, out.String(), "--number")
	assertNotContains(t, out.String(), "╭")
}
