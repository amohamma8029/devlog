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
)

type onboarder struct {
	configPath string
	repoRoot   string
}

func newOnboarder() *onboarder {
	path, err := config.Path()
	if err != nil {
		return &onboarder{configPath: ""}
	}

	root, err := internalgit.RepoRoot()
	if err != nil {
		root = ""
	}

	return &onboarder{configPath: path, repoRoot: root}
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
	if _, err := fmt.Fprint(out, "\n  "+cliMutedStyle.Render("Press Enter to continue...")+"\n"); err != nil {
		return err
	}
	_, err := bufio.NewReader(in).ReadString('\n')
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
