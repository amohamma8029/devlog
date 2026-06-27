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

var SearchMatchStyle = lipgloss.NewStyle().
	Bold(true).
	Background(lipgloss.Color("#FFD166")).
	Foreground(lipgloss.Color("#111111"))

var ActiveSearchMatchStyle = lipgloss.NewStyle().
	Bold(true).
	Background(lipgloss.Color("#7DD3FC")).
	Foreground(lipgloss.Color("#0B1020"))

var WarningPromptStyle = lipgloss.NewStyle().
	Border(lipgloss.RoundedBorder()).
	BorderForeground(lipgloss.Color("#FFAA00"))

var SelectedEventStyle = lipgloss.NewStyle().
	Border(lipgloss.RoundedBorder()).
	BorderForeground(lipgloss.Color("#7DD3FC")).
	Padding(0, 1)

var TodoPanelStyle = lipgloss.NewStyle().
	Border(lipgloss.RoundedBorder()).
	BorderForeground(lipgloss.Color("#4B5563")).
	Padding(0, 2)

var TodoHeaderStyle = lipgloss.NewStyle().
	Bold(true).
	Foreground(lipgloss.Color("#FFFFFF"))

var TodoAccentStyle = lipgloss.NewStyle().
	Foreground(lipgloss.Color("#7DD3FC"))

var TodoMutedStyle = lipgloss.NewStyle().
	Foreground(lipgloss.Color("#6B7280"))

var TodoActionStyle = lipgloss.NewStyle().
	Foreground(lipgloss.Color("#A7F3D0"))

var TodoSelectedRowStyle = lipgloss.NewStyle().
	Background(lipgloss.Color("#1F2937")).
	Foreground(lipgloss.Color("#FFFFFF"))

var TodoNumberStyle = lipgloss.NewStyle().
	Foreground(lipgloss.Color("#93C5FD"))

var TodoOpenCheckboxStyle = lipgloss.NewStyle().
	Foreground(lipgloss.Color("#9CA3AF"))

var TodoDoneCheckboxStyle = lipgloss.NewStyle().
	Foreground(lipgloss.Color("#22C55E")).
	Bold(true)

var TodoCompletedTextStyle = lipgloss.NewStyle().
	Foreground(lipgloss.Color("#848B9A"))

var TodoListHeadingStyle = lipgloss.NewStyle().
	Bold(true).
	Foreground(lipgloss.Color("#23FF6C"))

var TodoListSubheadingStyle = lipgloss.NewStyle().
	Bold(true).
	Foreground(lipgloss.Color("#E5E7EB"))

var ChangesHeadingStyle = lipgloss.NewStyle().
	Bold(true).
	Foreground(lipgloss.Color("#FF2DA1"))

var TodoEmptyTitleStyle = lipgloss.NewStyle().
	Bold(true).
	Foreground(lipgloss.Color("#FFFFFF"))

var TodoPromptStyle = lipgloss.NewStyle().
	Border(lipgloss.RoundedBorder()).
	BorderForeground(lipgloss.Color("#7DD3FC"))
