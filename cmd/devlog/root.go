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
		Use:               "devlog",
		Short:             "Record structured coding session journals inside git repos.",
		SilenceErrors:     true,
		SilenceUsage:      true,
		CompletionOptions: cobra.CompletionOptions{HiddenDefaultCmd: true},
		Long: `devlog records structured coding session journals inside a git repository.

Open a session to capture context, add notes and blockers as you work,
check status to see where things stand, and close when done. Sessions are
stored as Markdown files under .devlog/sessions/.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if showVersion {
				_, err := fmt.Fprint(cmd.OutOrStdout(), renderCLIConfirmation("devlog", cliField{"version", version}))
				return err
			}

			onboarder := newOnboarder()
			if onboarder.shouldRun() {
				if err := onboarder.run(cmd.OutOrStdout(), cmd.InOrStdin()); err != nil {
					return err
				}
			}

			return launchTUI()
		},
	}

	cmd.Flags().BoolVarP(&showVersion, "version", "v", false, "Print the version and exit")
	configureCLIHelp(cmd)
	cmd.AddCommand(newOpenCommand())
	cmd.AddCommand(newNoteCommand())
	cmd.AddCommand(newBlockCommand())
	cmd.AddCommand(newStatusCommand())
	cmd.AddCommand(newCloseCommand())
	cmd.AddCommand(newListCommand())
	cmd.AddCommand(newHandoffCommand())
	cmd.AddCommand(newConfigCommand())

	return cmd
}
