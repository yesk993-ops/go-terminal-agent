package tui

import "strings"

func stripMarkdown(s string) string {
	var b strings.Builder
	inCode := false
	for i := 0; i < len(s); i++ {
		if s[i] == '`' {
			inCode = !inCode
			b.WriteByte(s[i])
			continue
		}
		if inCode {
			b.WriteByte(s[i])
			continue
		}
		if i+1 < len(s) && s[i] == '*' && s[i+1] == '*' {
			if i > 0 && s[i-1] >= '0' && s[i-1] <= '9' {
				b.WriteByte(s[i])
				continue
			}
			if i+2 < len(s) && s[i+2] >= '0' && s[i+2] <= '9' {
				b.WriteByte(s[i])
				continue
			}
			j := i + 2
			matched := false
			for j+1 < len(s) && !(s[j] == '*' && s[j+1] == '*') {
				b.WriteByte(s[j])
				j++
			}
			if j+1 < len(s) {
				i = j + 1
				matched = true
			}
			if matched {
				continue
			}
		}
		if s[i] == '*' && (i == 0 || s[i-1] == ' ' || s[i-1] == '\n') {
			if i+1 < len(s) && s[i+1] == ' ' {
				b.WriteByte(s[i])
				continue
			}
			j := i + 1
			matched := false
			for j < len(s) && s[j] != '*' && s[j] != '\n' {
				b.WriteByte(s[j])
				j++
			}
			if j < len(s) && s[j] == '*' {
				i = j
				matched = true
			}
			if matched {
				continue
			}
		}
		b.WriteByte(s[i])
	}
	return b.String()
}

func RenderAssistantMessage(content string) string {
	lines := strings.Split(content, "\n")
	var result strings.Builder
	inCodeBlock := false
	for _, l := range lines {
		trimmed := strings.TrimSpace(l)
		if strings.HasPrefix(trimmed, "### ") {
			result.WriteString("\033[35m" + strings.TrimPrefix(trimmed, "### ") + "\033[39m\n")
		} else if strings.HasPrefix(trimmed, "## ") {
			result.WriteString("\033[35m" + strings.TrimPrefix(trimmed, "## ") + "\033[39m\n")
		} else if strings.HasPrefix(trimmed, "# ") {
			result.WriteString("\033[35m" + strings.TrimPrefix(trimmed, "# ") + "\033[39m\n")
		} else if strings.HasPrefix(trimmed, "```") {
			inCodeBlock = !inCodeBlock
			result.WriteString(l + "\n")
		} else if inCodeBlock {
			result.WriteString(l + "\n")
		} else {
			result.WriteString(stripMarkdown(l) + "\n")
		}
	}
	return AssistantBubbleStyle.Render(strings.TrimRight(result.String(), "\n"))
}

func RenderUserMessage(content string) string {
	return UserBubbleStyle.Render(" " + content + " ")
}
