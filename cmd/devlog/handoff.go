package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	internalgit "github.com/amo/devlog/internal/git"
	"github.com/amo/devlog/internal/handoff"
	"github.com/amo/devlog/internal/session"
	"github.com/amo/devlog/internal/store"
	"github.com/spf13/cobra"
)

func newHandoffCommand() *cobra.Command {
	var output string

	cmd := &cobra.Command{
		Use:          "handoff [session-id]",
		Short:        "Generate a narrative handoff summary of a session.",
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			root, err := internalgit.RepoRoot()
			if err != nil {
				return err
			}

			s, err := store.New(root)
			if err != nil {
				return err
			}

			var rec store.SessionRecord

			if len(args) > 0 {
				if len(args) > 1 {
					return fmt.Errorf(`too many arguments. Run "devlog handoff" with no arguments for the active session, or "devlog handoff <session-id>" for a specific session.`)
				}
				rec, err = s.GetSession(args[0])
				if err != nil {
					return err
				}
			} else {
				active, err := session.FindActiveSession(s)
				if err != nil {
					return err
				}
				rec = *active
			}

			sessionContent, err := readSessionContent(root, rec.ID)
			if err != nil {
				return err
			}

			diff, err := internalgit.DiffSince(rec.Started.UTC())
			if err != nil {
				return err
			}

			handoffText, err := handoff.Generate(sessionContent, diff)
			if err != nil {
				return fmt.Errorf("handoff: generate: %w", err)
			}

			if output != "" {
				if err := os.WriteFile(output, []byte(handoffText), 0644); err != nil {
					return fmt.Errorf("handoff: write output file: %w", err)
				}
				_, err = fmt.Fprintf(cmd.OutOrStdout(), "Handoff written to %s\n", output)
				return err
			}

			_, err = fmt.Fprint(cmd.OutOrStdout(), handoffText)
			return err
		},
	}

	cmd.Flags().StringVarP(&output, "output", "o", "", "Write handoff to a file instead of stdout")

	return cmd
}

func readSessionContent(root, sessionID string) (string, error) {
	path, err := sessionFilePath(root, sessionID)
	if err != nil {
		return "", err
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("handoff: session file not found: %s", path)
		}
		return "", fmt.Errorf("handoff: read session file: %w", err)
	}

	content := strings.ReplaceAll(string(data), "\r\n", "\n")
	return content, nil
}

func sessionFilePath(root, sessionID string) (string, error) {
	if strings.TrimSpace(sessionID) == "" {
		return "", fmt.Errorf("handoff: session ID is empty")
	}
	return filepath.Join(root, ".devlog", "sessions", sessionID+".md"), nil
}
