package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

func resolveBody(cmd *cobra.Command, flagMsg string, args []string, bodyLabel string) (string, error) {
	if strings.TrimSpace(flagMsg) != "" {
		return strings.TrimSpace(flagMsg), nil
	}

	positional := strings.TrimSpace(strings.Join(args, " "))
	if positional != "" {
		return positional, nil
	}

	editor := os.Getenv("EDITOR")
	if strings.TrimSpace(editor) == "" {
		return "", fmt.Errorf(`no message provided and $EDITOR is not set. Use -m <message> to provide text, or set $EDITOR to launch an editor.`)
	}

	return openEditor(cmd, strings.TrimSpace(editor), bodyLabel)
}

func openEditor(cmd *cobra.Command, editor, bodyLabel string) (string, error) {
	tmpDir, err := os.MkdirTemp("", "devlog-note-*")
	if err != nil {
		return "", fmt.Errorf("create temp directory: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	tmpPath := filepath.Join(tmpDir, "NOTE.md")
	if err := os.WriteFile(tmpPath, []byte{}, 0644); err != nil {
		return "", fmt.Errorf("create temp file: %w", err)
	}

	editorArgs := strings.Fields(editor)
	if len(editorArgs) == 0 {
		return "", fmt.Errorf("$EDITOR is empty")
	}

	c := exec.Command(editorArgs[0], append(editorArgs[1:], tmpPath)...)
	c.Stdin = os.Stdin
	c.Stdout = cmd.OutOrStdout()
	c.Stderr = cmd.ErrOrStderr()

	if err := c.Run(); err != nil {
		return "", fmt.Errorf("editor exited with error: %w", err)
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
