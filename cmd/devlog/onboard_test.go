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

	o := &onboarder{configPath: configPath}
	if o.shouldRun() {
		t.Error("shouldRun: expected false when config file exists, got true")
	}
}

func TestShouldRunTrueWhenConfigMissing(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.yml")

	o := &onboarder{configPath: configPath}
	if !o.shouldRun() {
		t.Error("shouldRun: expected true when config file is missing, got false")
	}
}

func TestShouldRunFalseWhenConfigPathEmpty(t *testing.T) {
	o := &onboarder{configPath: ""}
	if o.shouldRun() {
		t.Error("shouldRun: expected false when configPath is empty, got true")
	}
}

func TestWelcomeMessageContainsExpectedContent(t *testing.T) {
	o := &onboarder{configPath: "/tmp/config.yml"}
	msg := o.welcomeMessage()

	mustContain(t, msg, "Welcome to devlog")
	mustContain(t, msg, "Record structured coding session journals")
	mustContain(t, msg, "Quick start:")
	mustContain(t, msg, "devlog open")
	mustContain(t, msg, "devlog note")
	mustContain(t, msg, "devlog block")
	mustContain(t, msg, "devlog config edit")
}

func TestRunWritesToWriter(t *testing.T) {
	o := &onboarder{configPath: "/tmp/config.yml"}
	var buf bytes.Buffer

	if err := o.run(&buf, strings.NewReader("\n")); err != nil {
		t.Fatalf("run: unexpected error: %v", err)
	}

	output := buf.String()
	if output == "" {
		t.Error("run: expected non-empty output")
	}
	mustContain(t, output, "Welcome to devlog")
	mustContain(t, output, "Press Enter to continue")
}

func mustContain(t *testing.T, s, substr string) {
	t.Helper()
	if !strings.Contains(s, substr) {
		t.Errorf("expected output to contain %q", substr)
	}
}
