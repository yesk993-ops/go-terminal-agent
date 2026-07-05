package tui

func RenderAssistantMessage(content string) string {
	return AssistantBubbleStyle.Render(content)
}

func RenderUserMessage(content string) string {
	return UserBubbleStyle.Render(" " + content + " ")
}
