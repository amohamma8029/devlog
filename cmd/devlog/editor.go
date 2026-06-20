package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

var runBodyEditor = runBodyEditorProcess

func resolveBody(cmd *cobra.Command, flagMsg string, args []string, bodyLabel string) (string, error) {
	if strings.TrimSpace(flagMsg) != "" {
		return strings.TrimSpace(flagMsg), nil
	}

	positional := strings.TrimSpace(strings.Join(args, " "))
	if positional != "" {
		return positional, nil
	}

	editor, err := resolveBodyEditor()
	if err != nil {
		return "", err
	}

	return openEditor(cmd, editor, bodyLabel)
}

func resolveBodyEditor() (configEditor, error) {
	cfg, err := loadRuntimeConfig()
	if err != nil {
		return configEditor{}, err
	}

	if command := strings.TrimSpace(cfg.Editor.Command); command != "" {
		return configEditor{Command: command, Args: append([]string(nil), cfg.Editor.Args...)}, nil
	}

	editor := strings.TrimSpace(os.Getenv("EDITOR"))
	if editor == "" {
		return configEditor{}, fmt.Errorf(`no message provided and no editor is configured. Use -m <message> to provide text, set editor.command in the config, or set $EDITOR to launch an editor.`)
	}

	parts := strings.Fields(editor)
	if len(parts) == 0 {
		return configEditor{}, fmt.Errorf("$EDITOR is empty")
	}
	return configEditor{Command: parts[0], Args: parts[1:]}, nil
}

func openEditor(cmd *cobra.Command, editor configEditor, bodyLabel string) (string, error) {
	tmpDir, err := os.MkdirTemp("", "devlog-note-*")
	if err != nil {
		return "", fmt.Errorf("create temp directory: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	tmpPath := filepath.Join(tmpDir, "NOTE.md")
	if err := os.WriteFile(tmpPath, []byte{}, 0644); err != nil {
		return "", fmt.Errorf("create temp file: %w", err)
	}

	if err := runBodyEditor(cmd, editor, tmpPath); err != nil {
		return "", err
	}

	data, err := os.ReadFile(tmpPath)
	if err != nil {
		return "", fmt.Errorf("read temp file: %w", err)
	}

	body := strings.TrimSpace(string(data))
	if body == "" {
		return "", fmt.Errorf("%s body is empty", bodyLabel)
	}

	return body, nil
}

func runBodyEditorProcess(cmd *cobra.Command, editor configEditor, path string) error {
	return runEditorProcess(cmd, editor, path)
}

func runEditorProcess(cmd *cobra.Command, editor configEditor, path string) error {
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
