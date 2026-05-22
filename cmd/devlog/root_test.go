package main

import (
	"bytes"
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
}
