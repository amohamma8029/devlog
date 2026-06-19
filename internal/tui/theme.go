package tui

import "github.com/charmbracelet/lipgloss"

var TitleStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#FFFFFF"))

var ActiveStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#00FF00"))

var InactiveStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#808080"))

var BorderStyle = lipgloss.NewStyle().
	Border(lipgloss.RoundedBorder()).
	Padding(0, 1)

var BlockerStyle = lipgloss.NewStyle().
	Border(lipgloss.RoundedBorder()).
	BorderForeground(lipgloss.Color("#FF6600")).
	Padding(0, 1)

var BlockerLabelStyle = lipgloss.NewStyle().
	Bold(true).
	Foreground(lipgloss.Color("#FF6600"))

var TimelineStyle = lipgloss.NewStyle().
	Foreground(lipgloss.Color("#555555"))

var MenuStyle = lipgloss.NewStyle().
	Padding(0, 1)

var MenuSelectedStyle = lipgloss.NewStyle().
	Padding(0, 1).
	Background(lipgloss.Color("#444444")).
	Foreground(lipgloss.Color("#FFFFFF"))

var ComposerBlockerTokenStyle = MenuSelectedStyle.
	Background(lipgloss.Color("#AA0000")).
	Foreground(lipgloss.Color("#FFFFFF"))

var ComposerBodyStyle = lipgloss.NewStyle().
	Foreground(lipgloss.Color("#FFFFFF"))

var MenuHoverStyle = lipgloss.NewStyle().
	Padding(0, 1).
	Background(lipgloss.Color("#333333")).
	Foreground(lipgloss.Color("#FFFFFF"))

var MenuBoxStyle = lipgloss.NewStyle().
	Border(lipgloss.RoundedBorder()).
	BorderForeground(lipgloss.Color("#666666"))

var HelpOverlayStyle = lipgloss.NewStyle().
	Border(lipgloss.RoundedBorder()).
	Padding(0, 2).
	BorderForeground(lipgloss.Color("#666666"))

var HelpBackdropStyle = lipgloss.NewStyle().
	Faint(true).
	Foreground(lipgloss.Color("#555555"))

var HelpTitleStyle = lipgloss.NewStyle().
	Bold(true).
	Foreground(lipgloss.Color("#FFFFFF"))

var HelpKeyStyle = lipgloss.NewStyle().
	Foreground(lipgloss.Color("#81A1C1"))

var HelpDescStyle = lipgloss.NewStyle().
	Foreground(lipgloss.Color("#AAAAAA"))

var HelpSectionStyle = lipgloss.NewStyle().
	Bold(true).
	Foreground(lipgloss.Color("#FFFFFF"))

var HelpDividerStyle = lipgloss.NewStyle().
	Foreground(lipgloss.Color("#555555"))

var HelpFooterStyle = lipgloss.NewStyle().
	Foreground(lipgloss.Color("#555555"))

var ErrorBannerStyle = lipgloss.NewStyle().
	Background(lipgloss.Color("#AA0000")).
	Foreground(lipgloss.Color("#FFFFFF")).
	Padding(0, 1).
	Width(80)

var HintStyle = lipgloss.NewStyle().
	Foreground(lipgloss.Color("#808080"))

var SectionStyle = lipgloss.NewStyle().
	Foreground(lipgloss.Color("#888888")).
	Bold(true)

var PanelStyle = lipgloss.NewStyle().
	Border(lipgloss.RoundedBorder()).
	Padding(0, 1)

var EventStyle = lipgloss.NewStyle().
	Foreground(lipgloss.Color("#DDDDDD"))

var MetadataStyle = lipgloss.NewStyle().
	Foreground(lipgloss.Color("#AAAAAA"))

var NoSessionStyle = lipgloss.NewStyle().
	Foreground(lipgloss.Color("#808080"))

var ConnectorStyle = lipgloss.NewStyle().
	Foreground(lipgloss.Color("#555555"))

var ComposerSelectionStyle = lipgloss.NewStyle().
	Reverse(true)

var CursorStyle = lipgloss.NewStyle().
	Reverse(true)

var CursorSelectionStyle = lipgloss.NewStyle().
	Reverse(true).
	Bold(true)

var IDParenStyle = lipgloss.NewStyle().
	Foreground(lipgloss.Color("#888888"))

var PaletteInputStyle = lipgloss.NewStyle().
	Border(lipgloss.RoundedBorder()).
	BorderForeground(lipgloss.Color("#666666"))

var HandoffHeaderStyle = lipgloss.NewStyle().
	Bold(true).
	Foreground(lipgloss.Color("#FFFFFF"))

var HandoffButtonStyle = lipgloss.NewStyle().
	Foreground(lipgloss.Color("#AAAAAA"))

var SavePromptStyle = lipgloss.NewStyle().
	Border(lipgloss.RoundedBorder()).
	BorderForeground(lipgloss.Color("#666666"))

var SearchPromptStyle = lipgloss.NewStyle().
	Border(lipgloss.RoundedBorder()).
	BorderForeground(lipgloss.Color("#666666"))

var WarningPromptStyle = lipgloss.NewStyle().
	Border(lipgloss.RoundedBorder()).
	BorderForeground(lipgloss.Color("#FFAA00"))
