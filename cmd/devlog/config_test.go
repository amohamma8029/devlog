package main

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	internalconfig "github.com/amo/devlog/internal/config"
	"github.com/spf13/cobra"
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

func TestConfigEditCommandCreatesMissingConfigAndUsesEnvEditor(t *testing.T) {
	_, configPath := setConfigTestHome(t)
	t.Setenv("VISUAL", "")
	t.Setenv("EDITOR", "env-editor --wait")

	var calls []editorCall
	withConfigEditor(t, func(cmd *cobra.Command, editor configEditor, path string) error {
		calls = append(calls, editorCall{editor: editor, path: path})
		return nil
	})

	out, err := executeRootCommand("config", "edit")
	if err != nil {
		t.Fatalf("config edit command failed: %v", err)
	}

	assertEditorCall(t, calls, "env-editor", []string{"--wait"}, configPath)
	if _, err := internalconfig.LoadFile(configPath); err != nil {
		t.Fatalf("starter config did not validate: %v", err)
	}
	assertContains(t, out, "Config created")
	assertContains(t, out, "Opening editor")
	assertContains(t, out, "env-editor --wait")
	assertContains(t, out, "Config validated")
	assertContains(t, out, configPath)
}

func TestConfigEditCommandUsesConfiguredEditor(t *testing.T) {
	_, configPath := setConfigTestHome(t)
	t.Setenv("VISUAL", "visual-editor")
	t.Setenv("EDITOR", "env-editor")
	writeConfigTestFile(t, configPath, `editor:
  command: configured-editor
  args: ["--wait"]
`)

	var calls []editorCall
	withConfigEditor(t, func(cmd *cobra.Command, editor configEditor, path string) error {
		calls = append(calls, editorCall{editor: editor, path: path})
		return nil
	})

	if _, err := executeRootCommand("config", "edit"); err != nil {
		t.Fatalf("config edit command failed: %v", err)
	}

	assertEditorCall(t, calls, "configured-editor", []string{"--wait"}, configPath)
	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("read config file failed: %v", err)
	}
	assertContains(t, string(data), "configured-editor")
}

func TestConfigEditCommandUsesConfiguredEditorFromSemanticallyInvalidConfig(t *testing.T) {
	_, configPath := setConfigTestHome(t)
	t.Setenv("VISUAL", "visual-editor")
	t.Setenv("EDITOR", "env-editor")
	writeConfigTestFile(t, configPath, `editor:
  command: configured-editor
  args: ["--wait"]

display:
  timezone: "New York"
`)

	var calls []editorCall
	withConfigEditor(t, func(cmd *cobra.Command, editor configEditor, path string) error {
		calls = append(calls, editorCall{editor: editor, path: path})
		return os.WriteFile(path, []byte(starterConfigYAML), 0600)
	})

	if _, err := executeRootCommand("config", "edit"); err != nil {
		t.Fatalf("config edit command failed: %v", err)
	}

	assertEditorCall(t, calls, "configured-editor", []string{"--wait"}, configPath)
}

func TestConfigEditCommandUsesVisualBeforeEditor(t *testing.T) {
	_, configPath := setConfigTestHome(t)
	t.Setenv("VISUAL", "visual-editor --wait")
	t.Setenv("EDITOR", "env-editor")

	var calls []editorCall
	withConfigEditor(t, func(cmd *cobra.Command, editor configEditor, path string) error {
		calls = append(calls, editorCall{editor: editor, path: path})
		return nil
	})

	if _, err := executeRootCommand("config", "edit"); err != nil {
		t.Fatalf("config edit command failed: %v", err)
	}

	assertEditorCall(t, calls, "visual-editor", []string{"--wait"}, configPath)
}

func TestConfigEditCommandFallsBackToEnvEditorForInvalidExistingConfig(t *testing.T) {
	_, configPath := setConfigTestHome(t)
	t.Setenv("VISUAL", "")
	t.Setenv("EDITOR", "env-editor")
	writeConfigTestFile(t, configPath, `display:
  timezone: "New York"
`)

	var calls []editorCall
	withConfigEditor(t, func(cmd *cobra.Command, editor configEditor, path string) error {
		calls = append(calls, editorCall{editor: editor, path: path})
		return os.WriteFile(path, []byte(starterConfigYAML), 0600)
	})

	if _, err := executeRootCommand("config", "edit"); err != nil {
		t.Fatalf("config edit command failed: %v", err)
	}

	assertEditorCall(t, calls, "env-editor", nil, configPath)
	if _, err := internalconfig.LoadFile(configPath); err != nil {
		t.Fatalf("repaired config did not validate: %v", err)
	}
}

func TestConfigEditCommandReturnsValidationErrorAfterEditor(t *testing.T) {
	_, configPath := setConfigTestHome(t)
	t.Setenv("VISUAL", "")
	t.Setenv("EDITOR", "env-editor")

	withConfigEditor(t, func(cmd *cobra.Command, editor configEditor, path string) error {
		return os.WriteFile(path, []byte(`display:
  timezone: "New York"
`), 0600)
	})

	_, err := executeRootCommand("config", "edit")
	if err == nil {
		t.Fatal("expected validation error, got nil")
	}
	if !strings.Contains(err.Error(), "Config validation failed") || !strings.Contains(err.Error(), "display.timezone") || !strings.Contains(err.Error(), configPath) {
		t.Fatalf("expected config validation error with path and cause, got: %v", err)
	}
	if _, statErr := os.Stat(configPath); statErr != nil {
		t.Fatalf("expected config file to exist after edit, got: %v", statErr)
	}
}

func TestConfigEditCommandValidationErrorDoesNotPrintUsage(t *testing.T) {
	setConfigTestHome(t)
	t.Setenv("VISUAL", "")
	t.Setenv("EDITOR", "env-editor")

	withConfigEditor(t, func(cmd *cobra.Command, editor configEditor, path string) error {
		return os.WriteFile(path, []byte(`display:
  timezone: "New York"
`), 0600)
	})

	cmd := newRootCommand()
	var out, errOut bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&errOut)
	cmd.SetArgs([]string{"config", "edit"})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected validation error, got nil")
	}
	assertNotContains(t, errOut.String(), "Usage:")
	assertNotContains(t, errOut.String(), "Flags:")
}

func TestConfigEditCommandUsesDefaultEditor(t *testing.T) {
	_, configPath := setConfigTestHome(t)
	t.Setenv("VISUAL", "")
	t.Setenv("EDITOR", "")

	var calls []editorCall
	withConfigEditor(t, func(cmd *cobra.Command, editor configEditor, path string) error {
		calls = append(calls, editorCall{editor: editor, path: path})
		return nil
	})

	if _, err := executeRootCommand("config", "edit"); err != nil {
		t.Fatalf("config edit command failed: %v", err)
	}

	want := defaultConfigEditor()
	assertEditorCall(t, calls, want.Command, want.Args, configPath)
}

func TestConfigEditCommandWrapsEditorLaunchError(t *testing.T) {
	_, configPath := setConfigTestHome(t)
	t.Setenv("VISUAL", "")
	t.Setenv("EDITOR", "env-editor")

	withConfigEditor(t, func(cmd *cobra.Command, editor configEditor, path string) error {
		return os.ErrPermission
	})

	_, err := executeRootCommand("config", "edit")
	if err == nil {
		t.Fatal("expected editor launch error, got nil")
	}
	for _, want := range []string{"Editor launch failed", "env-editor", configPath, "Set editor.command", "VISUAL/EDITOR"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("expected error to contain %q, got: %v", want, err)
		}
	}
}

func TestConfigEditCommandFallsBackToEnvEditorWhenConfiguredEditorFails(t *testing.T) {
	_, configPath := setConfigTestHome(t)
	t.Setenv("VISUAL", "")
	t.Setenv("EDITOR", "env-editor")
	writeConfigTestFile(t, configPath, `editor:
  command: missing-editor
  args: ["--wait"]
`)

	var calls []editorCall
	withConfigEditor(t, func(cmd *cobra.Command, editor configEditor, path string) error {
		calls = append(calls, editorCall{editor: editor, path: path})
		if editor.Command == "missing-editor" {
			return os.ErrPermission
		}
		return nil
	})

	out, err := executeRootCommand("config", "edit")
	if err != nil {
		t.Fatalf("config edit command failed: %v", err)
	}

	if len(calls) != 2 {
		t.Fatalf("editor calls = %d, want 2 (configured + fallback)", len(calls))
	}
	if calls[0].editor.Command != "missing-editor" {
		t.Fatalf("first editor = %q, want missing-editor", calls[0].editor.Command)
	}
	if calls[1].editor.Command != "env-editor" {
		t.Fatalf("fallback editor = %q, want env-editor", calls[1].editor.Command)
	}
	assertContains(t, out, "Editor launch failed")
	assertContains(t, out, "missing-editor")
	assertContains(t, out, "Falling back to editor")
	assertContains(t, out, "env-editor")
	assertNotContains(t, out, "Config validated")
	assertContains(t, out, "Warning")
	assertContains(t, out, "missing-editor")
	assertContains(t, out, "not found in PATH")
}

func TestRootCommandIncludesConfigEditCommand(t *testing.T) {
	cmd, _, err := newRootCommand().Find([]string{"config", "edit"})
	if err != nil {
		t.Fatalf("find config edit command failed: %v", err)
	}
	if cmd == nil || cmd.Name() != "edit" {
		t.Fatalf("expected root command to include config edit, got %v", cmd)
	}
	if cmd.Parent() == nil || cmd.Parent().Name() != "config" {
		t.Fatalf("expected edit command parent to be config, got %v", cmd.Parent())
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
	assertContains(t, out.String(), "edit")
}

type editorCall struct {
	editor configEditor
	path   string
}

func setConfigTestHome(t *testing.T) (string, string) {
	t.Helper()

	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)

	path, err := internalconfig.Path()
	if err != nil {
		t.Fatalf("resolve config path failed: %v", err)
	}

	return home, path
}

func writeConfigTestFile(t *testing.T, path, body string) {
	t.Helper()

	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatalf("create config dir failed: %v", err)
	}
	if err := os.WriteFile(path, []byte(body), 0600); err != nil {
		t.Fatalf("write config file failed: %v", err)
	}
}

func withConfigEditor(t *testing.T, run func(*cobra.Command, configEditor, string) error) {
	t.Helper()

	original := runConfigEditor
	runConfigEditor = run
	t.Cleanup(func() { runConfigEditor = original })
}

func withBodyEditor(t *testing.T, run func(configEditor, string) error) {
	t.Helper()

	original := runBodyEditor
	runBodyEditor = func(cmd *cobra.Command, editor configEditor, path string) error {
		return run(editor, path)
	}
	t.Cleanup(func() { runBodyEditor = original })
}

func executeRootCommand(args ...string) (string, error) {
	cmd := newRootCommand()
	var out, errOut bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&errOut)
	cmd.SetArgs(args)

	err := cmd.Execute()
	return out.String(), err
}

func assertEditorCall(t *testing.T, calls []editorCall, command string, args []string, path string) {
	t.Helper()

	if len(calls) != 1 {
		t.Fatalf("editor calls = %d, want 1", len(calls))
	}
	if calls[0].editor.Command != command {
		t.Fatalf("editor command = %q, want %q", calls[0].editor.Command, command)
	}
	if strings.Join(calls[0].editor.Args, "\x00") != strings.Join(args, "\x00") {
		t.Fatalf("editor args = %#v, want %#v", calls[0].editor.Args, args)
	}
	if calls[0].path != path {
		t.Fatalf("editor path = %q, want %q", calls[0].path, path)
	}
}

func assertBodyEditorCall(t *testing.T, calls []editorCall, command string, args []string) {
	t.Helper()

	if len(calls) != 1 {
		t.Fatalf("editor calls = %d, want 1", len(calls))
	}
	if calls[0].editor.Command != command {
		t.Fatalf("editor command = %q, want %q", calls[0].editor.Command, command)
	}
	if strings.Join(calls[0].editor.Args, "\x00") != strings.Join(args, "\x00") {
		t.Fatalf("editor args = %#v, want %#v", calls[0].editor.Args, args)
	}
	if filepath.Base(calls[0].path) != "NOTE.md" {
		t.Fatalf("editor path = %q, want NOTE.md temp file", calls[0].path)
	}
}

func TestRenderCLIErrorStructured(t *testing.T) {
	err := &cliStructuredError{
		title:  "Config validation failed",
		fields: []cliField{{"path", "/tmp/config.yml"}},
		cause:  `display.timezone "New York" must be "UTC", "local", or a valid IANA timezone`,
		hint:   "Run `devlog config edit` again to fix the file",
	}

	got := renderCLIError(err)
	assertContains(t, got, "Config validation failed")
	assertContains(t, got, "path:")
	assertContains(t, got, "/tmp/config.yml")
	assertContains(t, got, "error:")
	assertContains(t, got, `display.timezone "New York"`)
	assertContains(t, got, "Run `devlog config edit` again to fix the file")
}

func TestRenderCLIErrorPlain(t *testing.T) {
	got := renderCLIError(fmt.Errorf("no active session is in progress"))
	assertContains(t, got, "Error")
	assertContains(t, got, "no active session is in progress")
}

func TestRenderCLIErrorNil(t *testing.T) {
	if got := renderCLIError(nil); got != "" {
		t.Fatalf("renderCLIError(nil) = %q, want empty", got)
	}
}
