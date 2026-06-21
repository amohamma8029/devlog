package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/amo/devlog/internal/config"
	internalgit "github.com/amo/devlog/internal/git"
	"gopkg.in/yaml.v3"
)

type onboarder struct {
	configPath string
	repoRoot   string
	gitProfile func() (string, string, error)
}

func newOnboarder() *onboarder {
	path, err := config.Path()
	if err != nil {
		return &onboarder{configPath: "", gitProfile: internalgit.AuthorIdentity}
	}

	root, err := internalgit.RepoRoot()
	if err != nil {
		root = ""
	}

	return &onboarder{configPath: path, repoRoot: root, gitProfile: internalgit.AuthorIdentity}
}

func (o *onboarder) shouldRun() bool {
	if o.configPath == "" {
		return false
	}
	if _, err := os.Stat(o.configPath); err == nil {
		return false
	} else if !os.IsNotExist(err) {
		return false
	}

	if o.repoRoot != "" {
		sessionsDir := filepath.Join(o.repoRoot, ".devlog", "sessions")
		entries, err := os.ReadDir(sessionsDir)
		if err == nil {
			for _, entry := range entries {
				if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".md") {
					return false
				}
			}
		}
	}

	return true
}

func (o *onboarder) run(out io.Writer, in io.Reader) error {
	if _, err := fmt.Fprint(out, o.welcomeMessage()); err != nil {
		return err
	}

	reader := bufio.NewReader(in)

	fmt.Fprintf(out, "\n  %s %s", cliLabelStyle.Render("Would you like to configure your preferences?"), cliMutedStyle.Render("[Y/n]: "))
	yn, err := reader.ReadString('\n')
	if err != nil && err != io.EOF {
		return err
	}

	if parseYesNoDefault(strings.TrimSpace(yn)) {
		if err := o.runWizard(out, reader); err != nil {
			if err == errWizardAborted {
				fmt.Fprintf(out, "\n  %s\n", cliValueStyle.Render("Setup cancelled. Nothing was saved. Run 'devlog config edit' to configure later."))
			} else {
				return err
			}
		} else {
			fmt.Fprintf(out, "\n  %s\n  %s\n", cliTitleStyle.Render("Configuration saved"), cliValueStyle.Render(o.configPath))
		}
	} else {
		if err := o.writeDefaultConfig(); err != nil {
			return err
		}
		fmt.Fprintf(out, "\n  %s\n", cliValueStyle.Render("No problem! A default config has been created. Run 'devlog config edit' to customize later."))
	}

	fmt.Fprintf(out, "\n  %s\n", cliMutedStyle.Render("Press Enter to continue..."))
	_, err = reader.ReadString('\n')
	if err != nil && err != io.EOF {
		return err
	}
	return nil
}

func (o *onboarder) welcomeMessage() string {
	var b strings.Builder

	b.WriteString(cliTitleStyle.Render("Welcome to devlog"))
	b.WriteByte('\n')
	b.WriteByte('\n')

	b.WriteString("  ")
	b.WriteString(cliValueStyle.Render("Record structured coding session journals right inside your git repo."))
	b.WriteByte('\n')
	b.WriteByte('\n')

	b.WriteString("  ")
	b.WriteString(cliLabelStyle.Render("Quick start:"))
	b.WriteByte('\n')

	b.WriteString("    ")
	b.WriteString(cliValueStyle.Render(`devlog open "Implement auth middleware"`))
	b.WriteByte('\n')

	b.WriteString("    ")
	b.WriteString(cliValueStyle.Render(`devlog note "Refactored JWT package"`))
	b.WriteByte('\n')

	b.WriteString("    ")
	b.WriteString(cliValueStyle.Render(`devlog block "Waiting on design decision"`))
	b.WriteByte('\n')

	b.WriteByte('\n')
	b.WriteString("  ")
	b.WriteString(cliMutedStyle.Render("Run 'devlog config edit' to configure your preferences."))
	b.WriteByte('\n')

	return b.String()
}

func (o *onboarder) writeConfig(cfg config.Config) error {
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("encode config: %w", err)
	}
	dir := filepath.Dir(o.configPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("create config dir: %w", err)
	}
	if err := os.WriteFile(o.configPath, data, 0644); err != nil {
		return fmt.Errorf("write config: %w", err)
	}
	return nil
}

func (o *onboarder) writeDefaultConfig() error {
	return o.writeConfig(config.Default())
}

func parseYesNoDefault(s string) bool {
	switch strings.ToLower(s) {
	case "", "y", "yes":
		return true
	}
	return false
}
