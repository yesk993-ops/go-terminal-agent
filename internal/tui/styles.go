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

	// Message labels in the chat transcript.
	UserLabelStyle      = lipgloss.NewStyle().Bold(true)
	AssistantLabelStyle = lipgloss.NewStyle().Bold(true)

	SpinnerStyle = lipgloss.NewStyle()

	DividerStyle = lipgloss.NewStyle()
)
