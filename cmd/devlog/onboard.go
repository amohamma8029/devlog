package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/amo/devlog/internal/config"
)

type onboarder struct {
	configPath string
}

func newOnboarder() *onboarder {
	path, err := config.Path()
	if err != nil {
		return &onboarder{configPath: ""}
	}
	return &onboarder{configPath: path}
}

func (o *onboarder) shouldRun() bool {
	if o.configPath == "" {
		return false
	}
	_, err := os.Stat(o.configPath)
	return os.IsNotExist(err)
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
