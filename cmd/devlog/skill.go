package main

import (
	"fmt"

	"github.com/amohamma8029/devlog/internal/skill"
	"github.com/spf13/cobra"
)

func newSkillCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "skill",
		Short: "Manage devlog agent skills.",
	}
	cmd.AddCommand(newSkillInstallCommand())
	cmd.AddCommand(newSkillUninstallCommand())
	return cmd
}

func newSkillInstallCommand() *cobra.Command {
	var force bool

	cmd := &cobra.Command{
		Use:          "install <tool|all>",
		Short:        "Install the devlog skill into a coding agent's skill directory.",
		Args:         cobra.ExactArgs(1),
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			tool := args[0]

			installer, err := skill.NewInstaller()
			if err != nil {
				return err
			}

			if tool == "all" {
				return runSkillInstallAll(cmd, installer, force)
			}

			return runSkillInstallOne(cmd, installer, tool, force)
		},
	}

	cmd.Flags().BoolVar(&force, "force", false, "Overwrite an existing SKILL.md")

	return cmd
}

func runSkillInstallOne(cmd *cobra.Command, installer *skill.Installer, tool string, force bool) error {
	path, err := installer.Install(tool, force)
	if err != nil {
		return err
	}

	_, err = fmt.Fprint(cmd.OutOrStdout(), renderCLIConfirmation("Skill installed",
		cliField{"tool", tool},
		cliField{"path", path}))
	return err
}

func runSkillInstallAll(cmd *cobra.Command, installer *skill.Installer, force bool) error {
	results, err := installer.InstallAll(force)

	for _, r := range results {
		if r.Err != nil {
			fmt.Fprint(cmd.OutOrStdout(), renderCLIError(r.Err))
			continue
		}
		fmt.Fprint(cmd.OutOrStdout(), renderCLIConfirmation("Skill installed",
			cliField{"tool", r.Tool},
			cliField{"path", r.Path}))
	}

	return err
}

func newSkillUninstallCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:          "uninstall <tool|all>",
		Short:        "Remove the devlog skill from a coding agent's skill directory.",
		Args:         cobra.ExactArgs(1),
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			tool := args[0]

			installer, err := skill.NewInstaller()
			if err != nil {
				return err
			}

			if tool == "all" {
				return runSkillUninstallAll(cmd, installer)
			}

			return runSkillUninstallOne(cmd, installer, tool)
		},
	}

	return cmd
}

func runSkillUninstallOne(cmd *cobra.Command, installer *skill.Installer, tool string) error {
	path, removed, err := installer.Uninstall(tool)
	if err != nil {
		return err
	}

	if !removed {
		_, err = fmt.Fprint(cmd.OutOrStdout(), renderCLIConfirmation("Skill not installed",
			cliField{"tool", tool}))
		return err
	}

	_, err = fmt.Fprint(cmd.OutOrStdout(), renderCLIConfirmation("Skill removed",
		cliField{"tool", tool},
		cliField{"path", path}))
	return err
}

func runSkillUninstallAll(cmd *cobra.Command, installer *skill.Installer) error {
	results, err := installer.UninstallAll()

	for _, r := range results {
		if r.Err != nil {
			fmt.Fprint(cmd.OutOrStdout(), renderCLIError(r.Err))
			continue
		}
		if !r.Removed {
			fmt.Fprint(cmd.OutOrStdout(), renderCLIConfirmation("Skill not installed",
				cliField{"tool", r.Tool}))
			continue
		}
		fmt.Fprint(cmd.OutOrStdout(), renderCLIConfirmation("Skill removed",
			cliField{"tool", r.Tool},
			cliField{"path", r.Path}))
	}

	return err
}
