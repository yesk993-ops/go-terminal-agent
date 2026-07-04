package tui

import "github.com/charmbracelet/lipgloss"

var (
	subtle    = lipgloss.AdaptiveColor{Light: "#D9DCCF", Dark: "#383838"}
	highlight = lipgloss.AdaptiveColor{Light: "#874BFD", Dark: "#7B59F0"}

	AppStyle = lipgloss.NewStyle().
			Padding(1, 2).
			Border(lipgloss.RoundedBorder()).
			BorderForeground(highlight)

	TitleStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FFFFFF")).
			Background(lipgloss.Color("#7B59F0")).
			Padding(0, 1).
			Bold(true)

	UserBubbleStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FFFFFF")).
			Background(lipgloss.Color("#3B82F6")).
			Padding(0, 1).
			MarginLeft(2)

	AssistantBubbleStyle = lipgloss.NewStyle().
				Background(lipgloss.Color("#374151")).
				Padding(0, 1).
				MarginRight(2)

	SpinnerStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#7B59F0"))

	DividerStyle = lipgloss.NewStyle().
			Foreground(subtle).
			SetString("────────────────────────────────")
)
