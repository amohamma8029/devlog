package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	internalgit "github.com/amo/devlog/internal/git"
	"github.com/amo/devlog/internal/session"
	"github.com/amo/devlog/internal/store"
	"github.com/spf13/cobra"
)

func newNoteCommand() *cobra.Command {
	var message string

	cmd := &cobra.Command{
		Use:          "note [message]",
		Short:        "Append a note to the active devlog session.",
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			body, err := resolveNoteBody(cmd, message, args)
			if err != nil {
				return err
			}

			root, err := internalgit.RepoRoot()
			if err != nil {
				return err
			}

			s, err := store.New(root)
			if err != nil {
				return err
			}

			active, err := session.FindActiveSession(s)
			if err != nil {
				return err
			}

			if err := session.AppendEventToActiveSession(s, "Note", body); err != nil {
				return err
			}

			_, err = fmt.Fprintf(cmd.OutOrStdout(), "Recorded note in session %s\n", active.ID)
			return err
		},
	}

	cmd.Flags().StringVarP(&message, "message", "m", "", "Note message to append")

	return cmd
}

func resolveNoteBody(cmd *cobra.Command, flagMsg string, args []string) (string, error) {
	if strings.TrimSpace(flagMsg) != "" {
		return strings.TrimSpace(flagMsg), nil
	}

	positional := strings.TrimSpace(strings.Join(args, " "))
	if positional != "" {
		return positional, nil
	}

	editor := os.Getenv("EDITOR")
	if strings.TrimSpace(editor) == "" {
		return "", fmt.Errorf("no message provided and $EDITOR not set")
	}

	return openEditor(cmd, strings.TrimSpace(editor))
}

func openEditor(cmd *cobra.Command, editor string) (string, error) {
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
		return "", fmt.Errorf("note body is empty")
	}

	return body, nil
}
