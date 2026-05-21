package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

const version = "0.1.0"

func Execute() error {
	return newRootCommand().Execute()
}

func newRootCommand() *cobra.Command {
	var showVersion bool

	cmd := &cobra.Command{
		Use:           "devlog",
		Short:         "Record structured coding session journals inside git repos.",
		SilenceErrors: true,
		Long: `devlog records structured coding session journals inside a git repository.

Open a session to capture context, add notes and blockers as you work,
check status to see where things stand, and close when done. Sessions are
stored as Markdown files under .devlog/sessions/.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if showVersion {
				_, err := fmt.Fprintf(cmd.OutOrStdout(), "devlog version %s\n", version)
				return err
			}

			return cmd.Help()
		},
	}

	cmd.Flags().BoolVarP(&showVersion, "version", "v", false, "Print the version and exit")
	cmd.AddCommand(newOpenCommand())
	cmd.AddCommand(newNoteCommand())
	cmd.AddCommand(newBlockCommand())
	cmd.AddCommand(newStatusCommand())
	cmd.AddCommand(newCloseCommand())

	return cmd
}
