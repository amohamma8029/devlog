package main

import (
	"errors"
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/mattn/go-runewidth"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

const cliPreviewMaxWidth = 80

type cliField struct {
	Label string
	Value string
}

var (
	cliTitleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#FFFFFF"))

	cliLabelStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#888888"))

	cliMutedStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#888888"))

	cliHelpGroupStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("#81A1C1"))

	cliValueStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#DDDDDD"))

	cliActiveStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#00FF00"))

	cliClosedStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FF3333"))

	cliBlockerStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#FF3333"))

	cliBlockerTextStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#FF3333"))

	cliTodoListHeadingStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("#23FF6C"))

	cliTodoListSubheadingStyle = lipgloss.NewStyle().
					Bold(true).
					Foreground(lipgloss.Color("#E5E7EB"))

	cliTodoOpenCheckboxStyle = lipgloss.NewStyle().
					Foreground(lipgloss.Color("#9CA3AF"))

	cliTodoDoneCheckboxStyle = lipgloss.NewStyle().
					Foreground(lipgloss.Color("#22C55E")).
					Bold(true)

	cliTodoCompletedTextStyle = lipgloss.NewStyle().
					Foreground(lipgloss.Color("#848B9A"))

	cliErrorStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#FF3333"))

	cliErrorHintStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#888888"))

	cliWarningStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#FFFF00"))
)

var rootHelpGroups = []struct {
	Title    string
	Commands []string
}{
	{"Session workflow", []string{"open", "note", "block", "status", "close"}},
	{"Review", []string{"list", "handoff"}},
	{"Tasks", []string{"todo"}},
	{"Agent integration", []string{"skill"}},
}

func renderCLIConfirmation(title string, fields ...cliField) string {
	return renderCLIConfirmationWithTitle(cliTitleStyle.Render(title), fields...)
}

func renderCLIBlockerConfirmation(title string, fields ...cliField) string {
	return renderCLIConfirmationWithTitle(cliBlockerStyle.Render(title), fields...)
}

func renderCLISessionConfirmation(action, body string, blocker bool, title, id, branch string, closed bool) string {
	line := cliActionLine(action, body, blocker)
	var b strings.Builder
	b.WriteString(line)
	b.WriteByte('\n')
	writeCLISessionMetadata(&b, title, id, branch, closed)
	return b.String()
}

func renderCLIHandoffConfirmation(path, title, id, branch string, closed bool) string {
	var b strings.Builder
	b.WriteString(cliTitleStyle.Render("Handoff written"))
	b.WriteByte('\n')
	writeCLISessionMetadata(&b, title, id, branch, closed)
	writeCLIField(&b, "path", path)
	return b.String()
}

func renderCLIConfirmationWithTitle(title string, fields ...cliField) string {
	var b strings.Builder
	b.WriteString(title)
	b.WriteByte('\n')
	writeCLIFields(&b, fields...)
	return b.String()
}

type cliStructuredError struct {
	title  string
	fields []cliField
	cause  string
	hint   string
}

func (e *cliStructuredError) Error() string {
	var b strings.Builder
	b.WriteString(e.title)
	if e.cause != "" {
		b.WriteString(": ")
		b.WriteString(e.cause)
	}
	for _, f := range e.fields {
		b.WriteString("; ")
		b.WriteString(f.Label)
		b.WriteString(": ")
		b.WriteString(f.Value)
	}
	if e.hint != "" {
		b.WriteString("; hint: ")
		b.WriteString(e.hint)
	}
	return b.String()
}

func renderCLIError(err error) string {
	if err == nil {
		return ""
	}

	var se *cliStructuredError
	if errors.As(err, &se) {
		var b strings.Builder
		b.WriteString(cliErrorStyle.Render(se.title))
		b.WriteByte('\n')
		for _, f := range se.fields {
			writeCLIField(&b, f.Label, f.Value)
		}
		if se.cause != "" {
			writeCLIField(&b, "error", se.cause)
		}
		if se.hint != "" {
			b.WriteString("  ")
			b.WriteString(cliErrorHintStyle.Render(se.hint))
			b.WriteByte('\n')
		}
		return b.String()
	}

	return cliErrorStyle.Render("Error") + "\n  " + cliValueStyle.Render(err.Error()) + "\n"
}

func writeCLIFields(b *strings.Builder, fields ...cliField) {
	for _, field := range fields {
		writeCLIField(b, field.Label, field.Value)
	}
}

func writeCLIField(b *strings.Builder, label, value string) {
	b.WriteString("  ")
	b.WriteString(cliLabelStyle.Render(label + ":"))
	b.WriteByte(' ')
	b.WriteString(cliValueStyle.Render(value))
	b.WriteByte('\n')
}

func writeCLISessionMetadata(b *strings.Builder, title, id, branch string, closed bool) {
	writeCLISessionField(b, "session", title, id)
	writeCLIBranchField(b, branch, closed)
}

func writeCLISessionField(b *strings.Builder, label, title, id string) {
	b.WriteString("  ")
	b.WriteString(cliLabelStyle.Render(label + ":"))
	b.WriteByte(' ')
	b.WriteString(cliSessionRef(title, id))
	b.WriteByte('\n')
}

func writeCLIBranchField(b *strings.Builder, branch string, closed bool) {
	b.WriteString("  ")
	b.WriteString(cliLabelStyle.Render("branch:"))
	b.WriteByte(' ')
	b.WriteString(cliValueStyle.Render(branch))
	b.WriteByte(' ')
	b.WriteString(cliStateText(closed))
	b.WriteByte('\n')
}

func cliStatusText(status string, closed bool) string {
	if closed {
		return cliClosedStyle.Render(status)
	}
	return cliActiveStyle.Render(status)
}

func cliStateText(closed bool) string {
	if closed {
		return cliStatusText("(closed)", true)
	}
	return cliStatusText("(active)", false)
}

func cliEventText(eventType, line string) string {
	if eventType == "Blocker" {
		return cliBlockerStyle.Render(line)
	}
	return cliValueStyle.Render(line)
}

func cliBlockerTitle(title string) string {
	return cliBlockerStyle.Render(title)
}

func cliSessionTitleWithID(title, id string) string {
	return cliTitleStyle.Render(title) + " " + cliMutedStyle.Render("("+id+")")
}

func cliSessionRef(title, id string) string {
	return cliValueStyle.Render(title) + " " + cliMutedStyle.Render("("+id+")")
}

func cliActionLine(action, body string, blocker bool) string {
	preview := cliPreview(body)
	if blocker {
		line := cliBlockerStyle.Render(action)
		if preview != "" {
			line += " " + cliMutedStyle.Render("→") + " " + cliBlockerTextStyle.Render(preview)
		}
		return line
	}

	line := cliTitleStyle.Render(action)
	if preview != "" {
		line += " " + cliMutedStyle.Render("→") + " " + cliValueStyle.Render(preview)
	}
	return line
}

func cliPreview(body string) string {
	preview := strings.Join(strings.Fields(body), " ")
	if preview == "" {
		return ""
	}
	if runewidth.StringWidth(preview) <= cliPreviewMaxWidth {
		return preview
	}
	return runewidth.Truncate(preview, cliPreviewMaxWidth, "…")
}

func cliBulletLine(text string) string {
	return "  • " + text
}

func configureCLIHelp(cmd *cobra.Command) {
	cmd.SetHelpFunc(func(cmd *cobra.Command, args []string) {
		_, _ = fmt.Fprint(cmd.OutOrStdout(), renderCLIHelp(cmd))
	})
}

func renderCLIHelp(cmd *cobra.Command) string {
	var b strings.Builder
	b.WriteString(cliTitleStyle.Render(cmd.CommandPath()))
	b.WriteByte('\n')

	description := strings.TrimSpace(cmd.Long)
	if description == "" {
		description = strings.TrimSpace(cmd.Short)
	}
	if description != "" {
		for _, line := range strings.Split(description, "\n") {
			line = strings.TrimSpace(line)
			if line != "" {
				b.WriteString("  ")
				b.WriteString(cliValueStyle.Render(line))
				b.WriteByte('\n')
			}
		}
	}

	b.WriteByte('\n')
	writeCLIField(&b, "usage", cmd.UseLine())

	commands := visibleCommands(cmd)
	if len(commands) > 0 {
		b.WriteByte('\n')
		b.WriteString(cliTitleStyle.Render("Commands"))
		b.WriteByte('\n')
		if cmd.Parent() == nil {
			writeGroupedCLICommands(&b, cmd, commands)
		} else {
			writeCLICommands(&b, commands, "  ")
		}
	}

	flags := renderCLIFlags(cmd.NonInheritedFlags())
	if flags != "" {
		b.WriteByte('\n')
		b.WriteString(cliTitleStyle.Render("Flags"))
		b.WriteByte('\n')
		b.WriteString(flags)
	}

	inheritedFlags := renderCLIFlags(cmd.InheritedFlags())
	if inheritedFlags != "" {
		b.WriteByte('\n')
		b.WriteString(cliTitleStyle.Render("Global flags"))
		b.WriteByte('\n')
		b.WriteString(inheritedFlags)
	}

	return b.String()
}

func writeGroupedCLICommands(b *strings.Builder, cmd *cobra.Command, commands []*cobra.Command) {
	used := make(map[string]bool)
	for _, group := range rootHelpGroups {
		var grouped []*cobra.Command
		for _, name := range group.Commands {
			child, _, err := cmd.Find([]string{name})
			if err != nil || child == nil || !child.IsAvailableCommand() || child.Parent() != cmd {
				continue
			}
			grouped = append(grouped, child)
			used[child.Name()] = true
		}
		if len(grouped) == 0 {
			continue
		}
		b.WriteString("  ")
		b.WriteString(cliHelpGroupStyle.Render(group.Title))
		b.WriteByte('\n')
		writeCLICommands(b, grouped, "    ")
	}

	var remaining []*cobra.Command
	for _, child := range commands {
		if !used[child.Name()] {
			remaining = append(remaining, child)
		}
	}
	if len(remaining) > 0 {
		b.WriteString("  ")
		b.WriteString(cliLabelStyle.Render("Other"))
		b.WriteByte('\n')
		writeCLICommands(b, remaining, "    ")
	}
}

func writeCLICommands(b *strings.Builder, commands []*cobra.Command, indent string) {
	for _, child := range commands {
		b.WriteString(indent)
		b.WriteString(cliValueStyle.Render(child.Name()))
		if child.Short != "" {
			b.WriteString("  ")
			b.WriteString(cliLabelStyle.Render(child.Short))
		}
		b.WriteByte('\n')
	}
}

func visibleCommands(cmd *cobra.Command) []*cobra.Command {
	var commands []*cobra.Command
	for _, child := range cmd.Commands() {
		if child.IsAvailableCommand() {
			commands = append(commands, child)
		}
	}
	return commands
}

func renderCLIFlags(flags *pflag.FlagSet) string {
	if flags == nil || !flags.HasAvailableFlags() {
		return ""
	}

	var b strings.Builder
	flags.VisitAll(func(flag *pflag.Flag) {
		if flag.Hidden {
			return
		}
		b.WriteString("  ")
		b.WriteString(cliValueStyle.Render(cliFlagName(flag)))
		if flag.Usage != "" {
			b.WriteString("  ")
			b.WriteString(cliLabelStyle.Render(flag.Usage))
		}
		if flag.DefValue != "" && flag.DefValue != "false" {
			b.WriteString(" ")
			b.WriteString(cliLabelStyle.Render("(default " + flag.DefValue + ")"))
		}
		b.WriteByte('\n')
	})
	return b.String()
}

func cliFlagName(flag *pflag.Flag) string {
	name := "--" + flag.Name
	if flag.Shorthand != "" {
		return "-" + flag.Shorthand + ", " + name
	}
	return name
}
