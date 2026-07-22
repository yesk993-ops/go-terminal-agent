package tui

import "strings"

// RenderUserMessage renders a user turn as a bold label above the wrapped
// body. width is the column budget for the body text; wrapping happens on the
// plain content before any styling so ANSI escapes are never split.
func RenderUserMessage(content string, width int) string {
	body := WrapText(strings.Trim(content, "\n"), width)
	return UserLabelStyle.Render("❯ You") + "\n" + body
}

// RenderAssistantMessage renders an assistant turn the same way, additionally
// applying lightweight markdown styling (headers, bold, inline code) to each
// already-wrapped line.
func RenderAssistantMessage(content string, width int) string {
	body := WrapText(strings.Trim(content, "\n"), width)
	lines := strings.Split(body, "\n")
	for i, line := range lines {
		lines[i] = RenderMarkdownLine(line)
	}
	return AssistantLabelStyle.Render("❯ Assistant") + "\n" + strings.Join(lines, "\n")
}
