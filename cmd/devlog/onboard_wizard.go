package main

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"strings"
	"time"
	_ "time/tzdata"

	"github.com/amohamma8029/devlog/internal/config"
)

var errWizardAborted = errors.New("wizard aborted")

func (o *onboarder) runWizard(out io.Writer, reader *bufio.Reader) error {
	gitName, gitEmail, _ := o.gitProfile()

	displayName, err := promptString(out, reader, "Display name", gitName, func(s string) error {
		if strings.TrimSpace(s) == "" {
			return fmt.Errorf("display name is required")
		}
		return nil
	})
	if err != nil {
		return err
	}

	email, err := promptString(out, reader, "Email", gitEmail, func(s string) error {
		s = strings.TrimSpace(s)
		if s == "" {
			return nil
		}
		if !strings.Contains(s, "@") || !strings.ContainsRune(s, '.') {
			return fmt.Errorf("email must contain @ and .")
		}
		return nil
	})
	if err != nil {
		return err
	}

	editorCmd, err := promptString(out, reader, "Editor command (e.g., code, vim, nano — Enter to skip)", "", func(s string) error {
		return nil
	})
	if err != nil {
		return err
	}

	if _, err := exec.LookPath(editorCmd); err != nil {
		if editorCmd != "" {
			fmt.Fprintf(out, "  %s %s\n", cliWarningStyle.Render("Warning:"), cliMutedStyle.Render(fmt.Sprintf("%q was not found on your PATH", editorCmd)))
		}
	}

	if editorCmd == "" {
		fmt.Fprintf(out, "  %s\n", cliTitleStyle.Render("Using $EDITOR (not configured)"))
	}

	tz, err := promptString(out, reader, "Timezone", "UTC", func(s string) error {
		s = strings.TrimSpace(s)
		if s == "" || s == "UTC" || s == "local" {
			return nil
		}
		if _, err := time.LoadLocation(s); err != nil {
			return fmt.Errorf("%q is not a valid timezone", s)
		}
		return nil
	})
	if err != nil {
		return err
	}

	if tz == "UTC" {
		fmt.Fprintf(out, "  %s\n", cliTitleStyle.Render("Using UTC (default)"))
	} else if tz == "local" {
		fmt.Fprintf(out, "  %s\n", cliTitleStyle.Render("Using local timezone"))
	} else {
		fmt.Fprintf(out, "  %s\n", cliTitleStyle.Render(fmt.Sprintf("Using %s", tz)))
	}

	clockFormat, err := promptString(out, reader, "Clock format (12h or 24h)", "24h", func(s string) error {
		s = strings.TrimSpace(s)
		if s != "12h" && s != "24h" {
			return fmt.Errorf("clock format must be 12h or 24h")
		}
		return nil
	})
	if err != nil {
		return err
	}

	fmt.Fprintf(out, "  %s\n", cliTitleStyle.Render(fmt.Sprintf("Using %s", clockFormat)))

	cfg := config.Default()

	if displayName != "" || email != "" {
		if displayName == gitName && email == gitEmail {
			cfg.Author.DefaultProfile = config.BuiltInGitProfile
		} else {
			cfg.Author.DefaultProfile = "default"
			cfg.Author.Profiles = map[string]config.AuthorProfile{
				"default": {Display: displayName, Email: email},
			}
		}
	}

	if editorCmd != "" {
		cfg.Editor.Command = editorCmd
	}

	if tz != "" {
		cfg.Display.Timezone = tz
	}

	if clockFormat != "" {
		cfg.Display.ClockFormat = clockFormat
	}

	return o.writeConfig(cfg)
}

func promptString(out io.Writer, reader *bufio.Reader, label, defaultVal string, validate func(string) error) (string, error) {
	for {
		if defaultVal != "" {
			fmt.Fprintf(out, "\n  %s [%s]: ", label, cliMutedStyle.Render(defaultVal))
		} else {
			fmt.Fprintf(out, "\n  %s: ", label)
		}

		input, err := reader.ReadString('\n')
		if err != nil {
			if errors.Is(err, io.EOF) {
				return "", errWizardAborted
			}
			return "", err
		}

		input = strings.TrimSpace(input)
		if input == "" && defaultVal != "" {
			input = defaultVal
		}

		if err := validate(input); err != nil {
			fmt.Fprintf(out, "  %s\n", cliErrorStyle.Render(err.Error()))
			continue
		}

		return input, nil
	}
}
