package main

import (
	"bufio"
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/amo/devlog/internal/config"
)

func TestParseYesNoDefault(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{"", true},
		{"y", true},
		{"Y", true},
		{"yes", true},
		{"Yes", true},
		{"YES", true},
		{"n", false},
		{"N", false},
		{"no", false},
		{"No", false},
		{"  y  ", true},
		{"  n  ", false},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := parseYesNoDefault(strings.TrimSpace(tt.input))
			// The function already trims, but we pre-trim for consistency with run()
			got2 := parseYesNoDefault(strings.TrimSpace(tt.input))
			if got != tt.want || got2 != tt.want {
				t.Errorf("parseYesNoDefault(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestWriteDefaultConfig(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.yml")
	o := &onboarder{configPath: configPath, gitProfile: noGitProfile}

	if err := o.writeDefaultConfig(); err != nil {
		t.Fatalf("writeDefaultConfig: %v", err)
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("read config: %v", err)
	}

	cfg, err := config.Parse(data)
	if err != nil {
		t.Fatalf("parse config: %v", err)
	}

	if cfg.Author.DefaultProfile != "git" {
		t.Errorf("default author profile = %q, want %q", cfg.Author.DefaultProfile, "git")
	}
}

func TestWriteConfigCreatesParentDirs(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "sub", "nested", "config.yml")
	o := &onboarder{configPath: configPath, gitProfile: noGitProfile}

	cfg := config.Default()
	cfg.Editor.Command = "nano"

	if err := o.writeConfig(cfg); err != nil {
		t.Fatalf("writeConfig: %v", err)
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("read config: %v", err)
	}

	parsed, err := config.Parse(data)
	if err != nil {
		t.Fatalf("parse config: %v", err)
	}

	if parsed.Editor.Command != "nano" {
		t.Errorf("editor command = %q, want %q", parsed.Editor.Command, "nano")
	}
}

func TestRunWizardAllValues(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.yml")
	o := &onboarder{configPath: configPath, gitProfile: noGitProfile}

	var buf bytes.Buffer
	input := "Alice\nalice@example.com\ncode\nAmerica/New_York\n12h\n"

	if err := o.runWizard(&buf, bufio.NewReader(strings.NewReader(input))); err != nil {
		t.Fatalf("runWizard: %v", err)
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("read config: %v", err)
	}

	cfg, err := config.Parse(data)
	if err != nil {
		t.Fatalf("parse config: %v", err)
	}

	if cfg.Author.DefaultProfile != "default" {
		t.Errorf("default_profile = %q, want %q", cfg.Author.DefaultProfile, "default")
	}
	if cfg.Author.Profiles["default"].Display != "Alice" {
		t.Errorf("display = %q, want %q", cfg.Author.Profiles["default"].Display, "Alice")
	}
	if cfg.Author.Profiles["default"].Email != "alice@example.com" {
		t.Errorf("email = %q, want %q", cfg.Author.Profiles["default"].Email, "alice@example.com")
	}
	if cfg.Editor.Command != "code" {
		t.Errorf("editor = %q, want %q", cfg.Editor.Command, "code")
	}
	if cfg.Display.Timezone != "America/New_York" {
		t.Errorf("timezone = %q, want %q", cfg.Display.Timezone, "America/New_York")
	}
	if cfg.Display.ClockFormat != "12h" {
		t.Errorf("clock_format = %q, want %q", cfg.Display.ClockFormat, "12h")
	}
}

func TestRunWizardAcceptsDefaults(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.yml")
	o := &onboarder{configPath: configPath, gitProfile: noGitProfile}

	var buf bytes.Buffer
	// Name: enter "Bob", Email: enter (accept default), Editor: enter (accept default... wait, editor has no default, so empty rejected)
	// Let me provide explicit values for all required fields
	input := "Bob\nbob@test.com\nvim\n\n\n"

	if err := o.runWizard(&buf, bufio.NewReader(strings.NewReader(input))); err != nil {
		t.Fatalf("runWizard: %v", err)
	}

	cfg, err := config.LoadFile(configPath)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}

	if cfg.Display.Timezone != "UTC" {
		t.Errorf("timezone default = %q, want UTC", cfg.Display.Timezone)
	}
	if cfg.Editor.Command != "vim" {
		t.Errorf("editor = %q, want vim", cfg.Editor.Command)
	}
	if cfg.Display.ClockFormat != "24h" {
		t.Errorf("clock_format default = %q, want 24h", cfg.Display.ClockFormat)
	}
}

func TestRunWizardRejectsEmptyName(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.yml")
	o := &onboarder{configPath: configPath, gitProfile: noGitProfile}

	var buf bytes.Buffer
	input := "\nAlice\nalice@example.com\nnano\nUTC\n24h\n"

	if err := o.runWizard(&buf, bufio.NewReader(strings.NewReader(input))); err != nil {
		t.Fatalf("runWizard: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "display name is required") {
		t.Error("expected validation error for empty display name")
	}

	cfg, err := config.LoadFile(configPath)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if cfg.Author.Profiles["default"].Display != "Alice" {
		t.Errorf("display = %q, want Alice (retried after validation error)", cfg.Author.Profiles["default"].Display)
	}
}

func TestRunWizardRejectsInvalidEmail(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.yml")
	o := &onboarder{configPath: configPath, gitProfile: noGitProfile}

	var buf bytes.Buffer
	input := "Alice\nbad\nbob@test.com\nnano\nUTC\n24h\n"

	if err := o.runWizard(&buf, bufio.NewReader(strings.NewReader(input))); err != nil {
		t.Fatalf("runWizard: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "email must contain @ and .") {
		t.Error("expected validation error for invalid email")
	}

	cfg, err := config.LoadFile(configPath)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if cfg.Author.Profiles["default"].Email != "bob@test.com" {
		t.Errorf("email = %q, want bob@test.com (retried)", cfg.Author.Profiles["default"].Email)
	}
}

func TestRunWizardAcceptsEmptyEditor(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.yml")
	o := &onboarder{configPath: configPath, gitProfile: noGitProfile}

	var buf bytes.Buffer
	input := "Alice\nalice@example.com\n\nUTC\n24h\n"

	if err := o.runWizard(&buf, bufio.NewReader(strings.NewReader(input))); err != nil {
		t.Fatalf("runWizard: %v", err)
	}

	cfg, err := config.LoadFile(configPath)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if cfg.Editor.Command != "" {
		t.Errorf("editor = %q, want empty (skipped)", cfg.Editor.Command)
	}
}

func TestRunWizardRejectsInvalidTimezone(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.yml")
	o := &onboarder{configPath: configPath, gitProfile: noGitProfile}

	var buf bytes.Buffer
	input := "Alice\nalice@example.com\nnano\nMars\nUTC\n24h\n"

	if err := o.runWizard(&buf, bufio.NewReader(strings.NewReader(input))); err != nil {
		t.Fatalf("runWizard: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "is not a valid timezone") {
		t.Error("expected validation error for invalid timezone")
	}

	cfg, err := config.LoadFile(configPath)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if cfg.Display.Timezone != "UTC" {
		t.Errorf("timezone = %q, want UTC (retried)", cfg.Display.Timezone)
	}
}

func TestRunWizardAcceptsLocalTimezone(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.yml")
	o := &onboarder{configPath: configPath, gitProfile: noGitProfile}

	var buf bytes.Buffer
	input := "Alice\nalice@example.com\nnano\nlocal\n12h\n"

	if err := o.runWizard(&buf, bufio.NewReader(strings.NewReader(input))); err != nil {
		t.Fatalf("runWizard: %v", err)
	}

	cfg, err := config.LoadFile(configPath)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if cfg.Display.Timezone != "local" {
		t.Errorf("timezone = %q, want local", cfg.Display.Timezone)
	}
}

func TestRunWizardAbortedByEOF(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.yml")
	o := &onboarder{configPath: configPath, gitProfile: noGitProfile}

	var buf bytes.Buffer
	// EOF during first prompt — simulates Ctrl+C
	input := ""

	if err := o.runWizard(&buf, bufio.NewReader(strings.NewReader(input))); err != errWizardAborted {
		t.Fatalf("runWizard: expected errWizardAborted, got %v", err)
	}

	// Config must not exist
	if _, err := os.Stat(configPath); !os.IsNotExist(err) {
		t.Error("config file was written despite abort")
	}
}

func TestRunWizardAbortedMidway(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.yml")
	o := &onboarder{configPath: configPath, gitProfile: noGitProfile}

	var buf bytes.Buffer
	// Name provided, email aborted
	input := "Alice\n"

	if err := o.runWizard(&buf, bufio.NewReader(strings.NewReader(input))); err != errWizardAborted {
		t.Fatalf("runWizard: expected errWizardAborted, got %v", err)
	}

	if _, err := os.Stat(configPath); !os.IsNotExist(err) {
		t.Error("config file was written despite mid-wizard abort")
	}
}
func TestRunWizardAcceptsEmptyEmail(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.yml")
	o := &onboarder{configPath: configPath, gitProfile: noGitProfile}

	var buf bytes.Buffer
	input := "Alice\n\nnano\nUTC\n24h\n"

	if err := o.runWizard(&buf, bufio.NewReader(strings.NewReader(input))); err != nil {
		t.Fatalf("runWizard: %v", err)
	}

	cfg, err := config.LoadFile(configPath)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if cfg.Author.Profiles["default"].Email != "" {
		t.Errorf("email = %q, want empty", cfg.Author.Profiles["default"].Email)
	}
}

func TestRunWizardKeepsGitProfileWhenDefaultsMatch(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.yml")
	o := &onboarder{configPath: configPath, gitProfile: func() (string, string, error) {
		return "Alice", "alice@example.com", nil
	}}

	var buf bytes.Buffer
	// Accept all git defaults: Enter for name, Enter for email, Enter for editor, Enter for timezone, Enter for clock
	input := "\n\n\n\n\n"

	if err := o.runWizard(&buf, bufio.NewReader(strings.NewReader(input))); err != nil {
		t.Fatalf("runWizard: %v", err)
	}

	cfg, err := config.LoadFile(configPath)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if cfg.Author.DefaultProfile != "git" {
		t.Errorf("default_profile = %q, want git (matched git defaults)", cfg.Author.DefaultProfile)
	}
	if len(cfg.Author.Profiles) != 0 {
		t.Errorf("profiles should be empty when accepting git defaults, got %v", cfg.Author.Profiles)
	}
}

func TestRunWizardCreatesProfileWhenValuesDiffer(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.yml")
	o := &onboarder{configPath: configPath, gitProfile: func() (string, string, error) {
		return "Alice", "alice@example.com", nil
	}}

	var buf bytes.Buffer
	// Override name with custom value, accept git email, skip editor, default timezone, default clock
	input := "Bob\n\n\n\n\n"

	if err := o.runWizard(&buf, bufio.NewReader(strings.NewReader(input))); err != nil {
		t.Fatalf("runWizard: %v", err)
	}

	cfg, err := config.LoadFile(configPath)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if cfg.Author.DefaultProfile != "default" {
		t.Errorf("default_profile = %q, want default (custom value differs)", cfg.Author.DefaultProfile)
	}
	if cfg.Author.Profiles["default"].Display != "Bob" {
		t.Errorf("display = %q, want Bob", cfg.Author.Profiles["default"].Display)
	}
}
