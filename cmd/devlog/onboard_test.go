package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNewOnboarder(t *testing.T) {
	o := newOnboarder()
	if o.configPath == "" {
		t.Error("newOnboarder: configPath is empty")
	}
	if !strings.HasSuffix(o.configPath, ".yml") {
		t.Errorf("newOnboarder: configPath %q should end with .yml", o.configPath)
	}
}

func TestShouldRunFalseWhenConfigExists(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.yml")
	if err := os.WriteFile(configPath, []byte("author:\n  default_profile: git\n"), 0644); err != nil {
		t.Fatalf("setup: %v", err)
	}

	o := &onboarder{configPath: configPath, gitProfile: noGitProfile}
	if o.shouldRun() {
		t.Error("shouldRun: expected false when config file exists, got true")
	}
}

func TestShouldRunTrueWhenConfigMissing(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.yml")

	o := &onboarder{configPath: configPath, gitProfile: noGitProfile}
	if !o.shouldRun() {
		t.Error("shouldRun: expected true when config file is missing, got false")
	}
}

func TestShouldRunFalseWhenConfigPathEmpty(t *testing.T) {
	o := &onboarder{configPath: "", gitProfile: noGitProfile}
	if o.shouldRun() {
		t.Error("shouldRun: expected false when configPath is empty, got true")
	}
}

func TestShouldRunFalseWhenSessionsExistButNoConfig(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.yml")

	sessionsDir := filepath.Join(dir, ".devlog", "sessions")
	if err := os.MkdirAll(sessionsDir, 0755); err != nil {
		t.Fatalf("setup: %v", err)
	}
	sessionFile := filepath.Join(sessionsDir, "2026-01-15T140000Z.md")
	if err := os.WriteFile(sessionFile, []byte("---\nid: 2026-01-15T140000Z\nstatus: closed\n---\n\n## Start\n\ntest\n"), 0644); err != nil {
		t.Fatalf("setup: %v", err)
	}

	o := &onboarder{configPath: configPath, repoRoot: dir}
	if o.shouldRun() {
		t.Error("shouldRun: expected false when sessions exist even without config, got true")
	}
}

func TestShouldRunTrueWhenNoSessionsAndNoConfig(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.yml")

	o := &onboarder{configPath: configPath, repoRoot: dir}
	if !o.shouldRun() {
		t.Error("shouldRun: expected true when no config and no sessions, got false")
	}
}

func TestShouldRunTrueWhenNoRepoRootAndNoConfig(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.yml")

	o := &onboarder{configPath: configPath, repoRoot: ""}
	if !o.shouldRun() {
		t.Error("shouldRun: expected true when no config and no repoRoot, got false")
	}
}

func TestWelcomeMessageContainsExpectedContent(t *testing.T) {
	o := &onboarder{configPath: "/tmp/config.yml", gitProfile: noGitProfile}
	msg := o.welcomeMessage()

	mustContain(t, msg, "Welcome to devlog")
	mustContain(t, msg, "Record structured coding session journals")
	mustContain(t, msg, "Quick start:")
	mustContain(t, msg, "devlog open")
	mustContain(t, msg, "devlog note")
	mustContain(t, msg, "devlog block")
	mustContain(t, msg, "devlog config edit")
}

func TestRunDeclineWizardWritesDefaultConfig(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.yml")
	o := &onboarder{configPath: configPath, gitProfile: noGitProfile}
	var buf bytes.Buffer

	if err := o.run(&buf, strings.NewReader("n\n\n")); err != nil {
		t.Fatalf("run: unexpected error: %v", err)
	}

	output := buf.String()
	mustContain(t, output, "Welcome to devlog")
	mustContain(t, output, "Would you like to configure your preferences?")
	mustContain(t, output, "No problem! A default config has been created")
	mustContain(t, output, "Press Enter to continue")

	if _, err := os.Stat(configPath); err != nil {
		t.Errorf("default config not written at %s: %v", configPath, err)
	}
}

func TestRunEnterDefaultsToYes(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.yml")
	o := &onboarder{configPath: configPath, gitProfile: noGitProfile}
	var buf bytes.Buffer

	input := "yes\nAlice\nalice@example.com\nnano\n\n\n"
	if err := o.run(&buf, strings.NewReader(input)); err != nil {
		t.Fatalf("run: unexpected error: %v", err)
	}

	output := buf.String()
	mustContain(t, output, "Welcome to devlog")
	mustContain(t, output, "Would you like to configure your preferences?")
	mustContain(t, output, "Configuration saved")
	mustContain(t, output, "Press Enter to continue")

	if _, err := os.Stat(configPath); err != nil {
		t.Errorf("config not written at %s: %v", configPath, err)
	}
}

func mustContain(t *testing.T, s, substr string) {
	t.Helper()
	if !strings.Contains(s, substr) {
		t.Errorf("expected output to contain %q", substr)
	}
}

var noGitProfile = func() (string, string, error) {
	return "", "", nil
}
