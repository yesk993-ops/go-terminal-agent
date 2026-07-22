package tui

import "github.com/charmbracelet/lipgloss"

var (
	// FrameStyle removed; standard chat mode uses plain text.
	FrameStyle = lipgloss.NewStyle()

	// InlineCodeStyle and BoldStyle style lightweight markdown spans in CLI
	// output. HeaderStyle styles leading "#"/"##" lines.
	InlineCodeStyle = lipgloss.NewStyle()
	BoldStyle       = lipgloss.NewStyle().Bold(true)
	HeaderStyle     = lipgloss.NewStyle().Bold(true)

	TitleStyle = lipgloss.NewStyle().Bold(true)

	// Message labels in the chat transcript. Distinct accent colors give each
	// speaker a clear identity; AdaptiveColor keeps them legible on both light
	// and dark terminals.
	UserLabelStyle = lipgloss.NewStyle().Bold(true).
			Foreground(lipgloss.AdaptiveColor{Light: "#0F766E", Dark: "#2DD4BF"}) // teal
	AssistantLabelStyle = lipgloss.NewStyle().Bold(true).
				Foreground(lipgloss.AdaptiveColor{Light: "#B45309", Dark: "#FBBF24"}) // amber

	SpinnerStyle = lipgloss.NewStyle()

	DividerStyle = lipgloss.NewStyle()
)
