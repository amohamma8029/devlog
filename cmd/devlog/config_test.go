package main

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"

	internalconfig "github.com/amo/devlog/internal/config"
)

func TestConfigPathCommandPrintsResolvedPath(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)

	original := launchTUI
	defer func() { launchTUI = original }()
	launchTUI = func() error {
		t.Fatal("launchTUI should not be called for config path")
		return nil
	}

	cmd := newRootCommand()
	var out, errOut bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&errOut)
	cmd.SetArgs([]string{"config", "path"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("config path command failed: %v", err)
	}

	wantPath, err := internalconfig.Path()
	if err != nil {
		t.Fatalf("resolve config path failed: %v", err)
	}
	want := wantPath + "\n"
	if out.String() != want {
		t.Fatalf("config path output = %q, want %q", out.String(), want)
	}

	gotPath := strings.TrimSpace(out.String())
	wantSuffix := filepath.Join(".config", "devlog", "config.yml")
	if !strings.HasSuffix(gotPath, wantSuffix) {
		t.Fatalf("config path output = %q, want suffix %q", gotPath, wantSuffix)
	}
}

func TestRootCommandIncludesConfigPathCommand(t *testing.T) {
	cmd, _, err := newRootCommand().Find([]string{"config", "path"})
	if err != nil {
		t.Fatalf("find config path command failed: %v", err)
	}
	if cmd == nil || cmd.Name() != "path" {
		t.Fatalf("expected root command to include config path, got %v", cmd)
	}
	if cmd.Parent() == nil || cmd.Parent().Name() != "config" {
		t.Fatalf("expected path command parent to be config, got %v", cmd.Parent())
	}
}

func TestConfigHelpIncludesPathCommand(t *testing.T) {
	cmd := newRootCommand()
	var out, errOut bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&errOut)
	cmd.SetArgs([]string{"config", "--help"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("config help failed: %v", err)
	}
	assertContains(t, out.String(), "path")
}
