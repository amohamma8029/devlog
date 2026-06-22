package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"

	internalconfig "github.com/amo/devlog/internal/config"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

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

			if err := runConfigEditEditor(cmd, path); err != nil {
				return err
			}

			data, err := os.ReadFile(path)
			if err != nil {
				return &cliStructuredError{
					title:  "Config validation failed",
					fields: []cliField{{"path", path}},
					cause:  fmt.Sprintf("read: %s", err),
				}
			}
			cfg, err := internalconfig.Parse(data)
			if err != nil {
				return &cliStructuredError{
					title:  "Config validation failed",
					fields: []cliField{{"path", path}},
					cause:  err.Error(),
					hint:   "Run `devlog config edit` again to fix the file",
				}
			}

			if command := strings.TrimSpace(cfg.Editor.Command); command != "" {
				if _, err := exec.LookPath(command); err != nil {
					var warn strings.Builder
					warn.WriteString(cliWarningStyle.Render("Warning"))
					warn.WriteByte('\n')
					writeCLIField(&warn, "editor", command)
					writeCLIField(&warn, "message", fmt.Sprintf("editor command not found in PATH; verify editor.command or remove it to use $EDITOR"))
					_, err := fmt.Fprint(cmd.OutOrStdout(), warn.String())
					return err
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
	body, err := renderConfigYAML(internalconfig.Default())
	if err != nil {
		return false, fmt.Errorf("config edit: render starter config: %w", err)
	}
	if err := os.WriteFile(path, []byte(body), 0600); err != nil {
		return false, fmt.Errorf("config edit: create %s: %w", path, err)
	}

	return true, nil
}

func renderConfigYAML(cfg internalconfig.Config) (string, error) {
	var b strings.Builder

	b.WriteString("# devlog global config. Run `devlog config edit` to reopen this file.\n")
	b.WriteString("# Lines beginning with # are comments and can be left in place.\n\n")

	b.WriteString("author:\n")
	b.WriteString("  # Use \"git\" to read user.name/user.email from git config, or use a profile ID below.\n")
	fmt.Fprintf(&b, "  default_profile: %s\n", quoteYAMLString(cfg.Author.DefaultProfile))
	b.WriteString("  # Add profiles by ID. Each profile supports display and optional email; \"git\" is reserved.\n")
	if len(cfg.Author.Profiles) == 0 {
		b.WriteString("  profiles:\n")
		b.WriteString("    # work:\n")
		b.WriteString("    #   display: \"Your Name\"\n")
		b.WriteString("    #   email: \"you@example.com\"\n")
	} else {
		b.WriteString("  profiles:\n")
		profileIDs := make([]string, 0, len(cfg.Author.Profiles))
		for id := range cfg.Author.Profiles {
			profileIDs = append(profileIDs, id)
		}
		sort.Strings(profileIDs)
		for _, id := range profileIDs {
			profile := cfg.Author.Profiles[id]
			fmt.Fprintf(&b, "    %s:\n", id)
			fmt.Fprintf(&b, "      display: %s\n", quoteYAMLString(profile.Display))
			fmt.Fprintf(&b, "      email: %s\n", quoteYAMLString(profile.Email))
		}
	}

	b.WriteString("\neditor:\n")
	b.WriteString("  # Leave command empty to use $VISUAL, $EDITOR, then the platform default editor.\n")
	fmt.Fprintf(&b, "  command: %s\n", quoteYAMLString(cfg.Editor.Command))
	b.WriteString("  # Extra arguments passed before the file path, for example [\"--wait\"].\n")
	if len(cfg.Editor.Args) == 0 {
		b.WriteString("  args: []\n")
	} else {
		b.WriteString("  args:\n")
		for _, arg := range cfg.Editor.Args {
			fmt.Fprintf(&b, "    - %s\n", quoteYAMLString(arg))
		}
	}

	b.WriteString("\ndisplay:\n")
	b.WriteString("  # Use \"UTC\", \"local\", or an IANA timezone such as \"America/New_York\".\n")
	fmt.Fprintf(&b, "  timezone: %s\n", quoteYAMLString(cfg.Display.Timezone))
	b.WriteString("  # Use \"24h\" for ISO-like times or \"12h\" for AM/PM times.\n")
	fmt.Fprintf(&b, "  clock_format: %s\n", quoteYAMLString(cfg.Display.ClockFormat))

	b.WriteString("\nhandoff:\n")
	b.WriteString("  # Number of unchanged context lines included around handoff diffs.\n")
	fmt.Fprintf(&b, "  diff_context_lines: %d\n", cfg.Handoff.DiffContextLines)

	b.WriteString("\ntui:\n")
	b.WriteString("  handoff_preview:\n")
	b.WriteString("    # Maximum raw diff lines shown in the TUI handoff preview; 0 disables truncation.\n")
	fmt.Fprintf(&b, "    diff_line_limit: %d\n", cfg.TUI.HandoffPreview.DiffLineLimit)

	if _, err := internalconfig.Parse([]byte(b.String())); err != nil {
		return "", fmt.Errorf("generated config did not validate: %w", err)
	}

	return b.String(), nil
}

func quoteYAMLString(value string) string {
	return strconv.Quote(value)
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

func runConfigEditEditor(cmd *cobra.Command, path string) error {
	editor, err := configEditEditor(path)
	if err != nil {
		return err
	}

	if _, err := fmt.Fprint(cmd.OutOrStdout(), renderCLIConfirmation("Opening editor", cliField{"editor", editor.String()}, cliField{"path", path})); err != nil {
		return err
	}

	if err := runConfigEditor(cmd, editor, path); err != nil {
		return launchConfigEditorFallback(cmd, path, editor, err)
	}

	return nil
}

func launchConfigEditorFallback(cmd *cobra.Command, path string, failed configEditor, launchErr error) error {
	if _, ok := configEditEditorFromFile(path); !ok {
		return &cliStructuredError{
			title:  "Editor launch failed",
			fields: []cliField{{"editor", failed.String()}, {"path", path}},
			cause:  launchErr.Error(),
			hint:   "Set editor.command in the config or set VISUAL/EDITOR",
		}
	}

	fallback, fbErr := configEditEditorFromEnv(path)
	if fbErr != nil || fallback.String() == failed.String() {
		return &cliStructuredError{
			title:  "Editor launch failed",
			fields: []cliField{{"editor", failed.String()}, {"path", path}},
			cause:  launchErr.Error(),
			hint:   "Set editor.command in the config or set VISUAL/EDITOR",
		}
	}

	var warn strings.Builder
	warn.WriteString(cliErrorStyle.Render("Editor launch failed"))
	warn.WriteByte('\n')
	writeCLIField(&warn, "editor", failed.String())
	writeCLIField(&warn, "error", launchErr.Error())
	if _, err := fmt.Fprint(cmd.OutOrStdout(), warn.String()); err != nil {
		return err
	}

	var notice strings.Builder
	notice.WriteString(cliWarningStyle.Render("Falling back to editor"))
	notice.WriteByte('\n')
	writeCLIField(&notice, "editor", fallback.String())
	writeCLIField(&notice, "path", path)
	if _, err := fmt.Fprint(cmd.OutOrStdout(), notice.String()); err != nil {
		return err
	}

	if err := runConfigEditor(cmd, fallback, path); err != nil {
		return &cliStructuredError{
			title:  "Editor launch failed",
			fields: []cliField{{"editor", fallback.String()}, {"path", path}},
			cause:  err.Error(),
			hint:   "Set editor.command in the config or set VISUAL/EDITOR",
		}
	}

	return nil
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
	return runEditorProcess(cmd, editor, path)
}
