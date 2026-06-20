package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	internalconfig "github.com/amo/devlog/internal/config"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

const starterConfigYAML = `author:
  default_profile: git
  profiles: {}

editor:
  command: ""
  args: []

display:
  timezone: UTC
  clock_format: 24h

handoff:
  diff_context_lines: 3

tui:
  handoff_preview:
    diff_line_limit: 100
`

var runConfigEditor = runConfigEditorProcess

type configEditor struct {
	Command string
	Args    []string
}

func newConfigCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "Inspect devlog configuration.",
	}
	cmd.AddCommand(newConfigPathCommand())
	cmd.AddCommand(newConfigEditCommand())

	return cmd
}

func newConfigPathCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "path",
		Short: "Print the global config file path.",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			path, err := internalconfig.Path()
			if err != nil {
				return err
			}

			_, err = fmt.Fprintln(cmd.OutOrStdout(), path)
			return err
		},
	}
}

func newConfigEditCommand() *cobra.Command {
	return &cobra.Command{
		Use:          "edit",
		Short:        "Open the global config file in an editor.",
		Args:         cobra.NoArgs,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			path, err := internalconfig.Path()
			if err != nil {
				return err
			}

			created, err := ensureConfigFile(path)
			if err != nil {
				return err
			}
			if created {
				if _, err := fmt.Fprint(cmd.OutOrStdout(), renderCLIConfirmation("Config created", cliField{"path", path})); err != nil {
					return err
				}
			}

			editor, err := configEditEditor(path)
			if err != nil {
				return err
			}
			if _, err := fmt.Fprint(cmd.OutOrStdout(), renderCLIConfirmation("Opening editor", cliField{"editor", editor.String()}, cliField{"path", path})); err != nil {
				return err
			}

			if err := runConfigEditor(cmd, editor, path); err != nil {
				return &cliStructuredError{
					title:  "Editor launch failed",
					fields: []cliField{{"editor", editor.String()}, {"path", path}},
					cause:  err.Error(),
					hint:   "Set editor.command in the config or set VISUAL/EDITOR",
				}
			}

			data, err := os.ReadFile(path)
			if err != nil {
				return &cliStructuredError{
					title:  "Config validation failed",
					fields: []cliField{{"path", path}},
					cause:  fmt.Sprintf("read: %s", err),
				}
			}
			if _, err := internalconfig.Parse(data); err != nil {
				return &cliStructuredError{
					title:  "Config validation failed",
					fields: []cliField{{"path", path}},
					cause:  err.Error(),
					hint:   "Run `devlog config edit` again to fix the file",
				}
			}

			_, err = fmt.Fprint(cmd.OutOrStdout(), renderCLIConfirmation("Config validated", cliField{"path", path}))
			return err
		},
	}
}

func ensureConfigFile(path string) (bool, error) {
	info, err := os.Stat(path)
	if err == nil {
		if info.IsDir() {
			return false, fmt.Errorf("config edit: %s is a directory", path)
		}
		return false, nil
	}
	if !os.IsNotExist(err) {
		return false, fmt.Errorf("config edit: stat %s: %w", path, err)
	}

	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return false, fmt.Errorf("config edit: create config directory: %w", err)
	}
	if err := os.WriteFile(path, []byte(starterConfigYAML), 0600); err != nil {
		return false, fmt.Errorf("config edit: create %s: %w", path, err)
	}

	return true, nil
}

func configEditEditor(path string) (configEditor, error) {
	if editor, ok := configEditEditorFromFile(path); ok {
		return editor, nil
	}

	return configEditEditorFromEnv(path)
}

func configEditEditorFromFile(path string) (configEditor, bool) {
	data, err := os.ReadFile(path)
	if err != nil {
		return configEditor{}, false
	}

	var cfg struct {
		Editor internalconfig.EditorConfig `yaml:"editor,omitempty"`
	}
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return configEditor{}, false
	}

	command := strings.TrimSpace(cfg.Editor.Command)
	if command == "" {
		return configEditor{}, false
	}

	return configEditor{Command: command, Args: append([]string(nil), cfg.Editor.Args...)}, true
}

func configEditEditorFromEnv(path string) (configEditor, error) {
	if editor := strings.TrimSpace(os.Getenv("VISUAL")); editor != "" {
		return parseConfigEditor(editor, "VISUAL")
	}
	if editor := strings.TrimSpace(os.Getenv("EDITOR")); editor != "" {
		return parseConfigEditor(editor, "EDITOR")
	}

	return defaultConfigEditor(), nil
}

func parseConfigEditor(editor, source string) (configEditor, error) {
	parts := strings.Fields(editor)
	if len(parts) == 0 {
		return configEditor{}, fmt.Errorf("config edit: %s is empty", source)
	}
	return configEditor{Command: parts[0], Args: parts[1:]}, nil
}

func defaultConfigEditor() configEditor {
	if runtime.GOOS == "windows" {
		return configEditor{Command: "notepad.exe"}
	}
	return configEditor{Command: "vi"}
}

func (e configEditor) String() string {
	parts := append([]string{e.Command}, e.Args...)
	return strings.Join(parts, " ")
}

func runConfigEditorProcess(cmd *cobra.Command, editor configEditor, path string) error {
	args := append(append([]string(nil), editor.Args...), path)
	c := exec.Command(editor.Command, args...)
	c.Stdin = os.Stdin
	c.Stdout = cmd.OutOrStdout()
	c.Stderr = cmd.ErrOrStderr()

	if err := c.Run(); err != nil {
		return fmt.Errorf("editor exited with error: %w", err)
	}

	return nil
}
