package tui

import (
	"context"
	"fmt"
	"strings"

	"github.com/agent/ai-terminal/internal/core"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

type chatMsg struct {
	role    core.Role
	content string
}

type tokenMsg struct {
	content string
	done    bool
	err     error
}

type errMsg struct {
	err error
}

type model struct {
	agent     core.Agent
	session   core.Session
	provider  string
	modelName string
	setCore   func(provider, model string) error

	chat    []chatMsg
	input   textinput.Model
	spinner spinner.Model
	loading bool
	err     error
	cancel  context.CancelFunc
	ctx     context.Context

	width       int
	height      int
	tokenCh     <-chan core.Token
	showThinking bool
}

var placeholderTexts = []string{
	"Ask anything...",
	"Pregunta lo que sea...",
	"Posez votre question...",
	"Fragen Sie alles...",
	"Fai qualsiasi domanda...",
	"何でも質問してください...",
	"질문하세요...",
	"询问任何问题...",
	"Задайте вопрос...",
	"Bir şey sor...",
}

func New(agent core.Agent, session core.Session, provider, modelName string, setCore func(provider, model string) error) tea.Model {
	s := spinner.New()
	s.Style = SpinnerStyle
	s.Spinner = spinner.Dot

	ti := textinput.New()
	ti.Placeholder = placeholderTexts[0]
	ti.Prompt = "> "
	ti.Focus()
	ti.CharLimit = 0
	ti.Width = 80

	return &model{
		agent:     agent,
		session:   session,
		provider:  provider,
		modelName: modelName,
		setCore:   setCore,
		input:     ti,
		spinner:   s,
		chat:      make([]chatMsg, 0, 100),
		ctx:       context.Background(),
	}
}

func (m *model) Init() tea.Cmd {
	m.chat = append(m.chat, chatMsg{
		role:    core.RoleAssistant,
		content: "Hello! How can I help you today?",
	})
	return tea.Batch(
		textinput.Blink,
		m.spinner.Tick,
	)
}

func (m *model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.input.Width = msg.Width - 4
		return m, nil

	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyCtrlC:
			if m.loading {
				if m.cancel != nil {
					m.cancel()
				}
				m.loading = false
				m.chat = append(m.chat, chatMsg{role: core.RoleAssistant, content: "\n\n[Session cancelled]"})
				return m, nil
			}
			return m, tea.Quit

		case tea.KeyEnter:
			input := strings.TrimSpace(m.input.Value())
			if input == "" || m.loading {
				return m, nil
			}

			m.input.SetValue("")

			if strings.HasPrefix(input, "/") {
				return m.handleCommand(input)
			}

			m.chat = append(m.chat, chatMsg{role: core.RoleUser, content: input})
			m.loading = true

			ctx, cancel := context.WithCancel(m.ctx)
			m.cancel = cancel

			return m, m.startStream(ctx, input)
		}

	case streamStartedMsg:
		m.tokenCh = msg.ch
		return m, m.nextToken()

	case tokenMsg:
		if msg.err != nil {
			m.loading = false
			m.chat = append(m.chat, chatMsg{role: core.RoleAssistant, content: fmt.Sprintf("\n\nError: %v", msg.err)})
			if m.cancel != nil {
				m.cancel()
			}
			return m, nil
		}

		if msg.done {
			m.loading = false
			m.tokenCh = nil
			return m, nil
		}

		if len(m.chat) > 0 && m.chat[len(m.chat)-1].role == core.RoleAssistant {
			m.chat[len(m.chat)-1].content += msg.content
		} else {
			m.chat = append(m.chat, chatMsg{role: core.RoleAssistant, content: msg.content})
		}
		return m, m.nextToken()

	case errMsg:
		m.loading = false
		m.err = msg.err
		m.chat = append(m.chat, chatMsg{role: core.RoleAssistant, content: fmt.Sprintf("\n\nError: %v", msg.err)})
		return m, nil

	case spinner.TickMsg:
		var cmd tea.Cmd
		if m.loading {
			m.spinner, cmd = m.spinner.Update(msg)
		}
		return m, cmd
	}

	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	return m, cmd
}

func (m *model) View() string {
	if m.width == 0 {
		m.width = 80
	}

	var b strings.Builder

	b.WriteString(TitleStyle.Render(fmt.Sprintf(" %s — %s ", m.provider, m.modelName)))
	b.WriteString("\n\n")

	visibleChat := m.chat
	if len(visibleChat) > 100 {
		visibleChat = visibleChat[len(visibleChat)-100:]
	}

	for _, msg := range visibleChat {
		switch msg.role {
		case core.RoleUser:
			b.WriteString(RenderUserMessage(msg.content))
			b.WriteString("\n")
		case core.RoleAssistant:
			b.WriteString(RenderAssistantMessage(msg.content))
			b.WriteString("\n")
		}
	}

	if m.loading {
		b.WriteString(SpinnerStyle.Render(m.spinner.View() + " Thinking..."))
		b.WriteString("\n")
	}

	b.WriteString(DividerStyle.String())
	b.WriteString("\n")
	b.WriteString(m.input.View())

	out := AppStyle.Render(b.String())
	// Apply word wrapping to fit terminal width
	width := m.width
	if width > 4 {
		width -= 4
	}
	if width > 0 {
		out = wrapText(out, width)
	}
	return out
}

func wrapText(s string, width int) string {
	var b strings.Builder
	lines := strings.Split(s, "\n")
	for i, line := range lines {
		if i > 0 {
			b.WriteByte('\n')
		}
		if len(line) <= width {
			b.WriteString(line)
			continue
		}
		for len(line) > 0 {
			if len(line) <= width {
				b.WriteString(line)
				break
			}
			idx := strings.LastIndex(line[:width], " ")
			if idx < 1 {
				idx = width
			}
			b.WriteString(line[:idx])
			b.WriteByte('\n')
			line = line[idx:]
		}
	}
	return b.String()
}

type streamStartedMsg struct {
	ch <-chan core.Token
}

func (m *model) startStream(ctx context.Context, input string) tea.Cmd {
	return func() tea.Msg {
		prompt := core.SystemPrompt
		if m.showThinking {
			prompt += "\n\nBefore answering, think step by step in a clear chain-of-thought. Show your reasoning inside <thinking>...</thinking> tags, then provide your final answer."
		}

		req := &core.Request{
			Messages: []core.Message{
				{
					Role:    core.RoleSystem,
					Content: prompt,
				},
				{
					Role:    core.RoleUser,
					Content: input,
				},
			},
			Stream:    true,
			MaxTokens: 8192,
		}

		sessionMsgs := m.session.Messages()
		if len(sessionMsgs) > 0 {
			start := 0
			if len(sessionMsgs) > 50 {
				start = len(sessionMsgs) - 50
			}
			prev := make([]core.Message, 0, len(sessionMsgs[start:]))
			for _, sm := range sessionMsgs[start:] {
				prev = append(prev, sm)
			}
			msgs := make([]core.Message, 0, len(prev)+2)
			msgs = append(msgs, req.Messages[0])
			msgs = append(msgs, prev...)
			msgs = append(msgs, core.Message{Role: core.RoleUser, Content: input})
			req.Messages = msgs
		}

		tokenCh, err := m.agent.Run(ctx, req)
		if err != nil {
			return errMsg{err: err}
		}

		return streamStartedMsg{ch: tokenCh}
	}
}

func (m *model) nextToken() tea.Cmd {
	return func() tea.Msg {
		token, ok := <-m.tokenCh
		if !ok {
			return tokenMsg{done: true}
		}
		if token.Error != nil {
			return errMsg{err: token.Error}
		}
		if token.Done {
			return tokenMsg{done: true}
		}
		return tokenMsg{content: token.Content}
	}
}

func (m *model) handleCommand(input string) (tea.Model, tea.Cmd) {
	parts := strings.SplitN(input, " ", 2)
	cmd := strings.ToLower(parts[0])
	arg := ""
	if len(parts) > 1 {
		arg = strings.TrimSpace(parts[1])
	}

	switch cmd {
	case "/provider":
		if arg == "" {
			m.chat = append(m.chat, chatMsg{role: core.RoleAssistant, content: "Usage: /provider <name> (nvidia, groq, openai, anthropic, gemini, openrouter)"})
			return m, nil
		}
		if m.setCore != nil {
			if err := m.setCore(arg, ""); err != nil {
				m.chat = append(m.chat, chatMsg{role: core.RoleAssistant, content: fmt.Sprintf("Error switching provider: %v", err)})
				return m, nil
			}
		}
		m.provider = arg
		m.chat = append(m.chat, chatMsg{role: core.RoleAssistant, content: fmt.Sprintf("Switched provider to: %s", arg)})
		return m, nil

	case "/model":
		if arg == "" {
			m.chat = append(m.chat, chatMsg{role: core.RoleAssistant, content: fmt.Sprintf("Current model: %s", m.modelName)})
			return m, nil
		}
		if m.setCore != nil {
			if err := m.setCore("", arg); err != nil {
				m.chat = append(m.chat, chatMsg{role: core.RoleAssistant, content: fmt.Sprintf("Error switching model: %v", err)})
				return m, nil
			}
		}
		m.modelName = arg
		m.chat = append(m.chat, chatMsg{role: core.RoleAssistant, content: fmt.Sprintf("Switched model to: %s", arg)})
		return m, nil

	case "/help":
		m.chat = append(m.chat, chatMsg{role: core.RoleAssistant, content: "Commands:\n  /provider <name>  - Switch provider (nvidia, groq, openai, etc.)\n  /model <name>     - Switch model\n  /think            - Toggle chain-of-thought reasoning\n  /clear            - Clear chat\n  /status           - Show current provider/model\n  /help             - Show this help\n  /exit, /quit      - Exit the program"})
		return m, nil

	case "/clear":
		m.chat = m.chat[:0]
		return m, nil

	case "/status":
		thinking := "off"
		if m.showThinking {
			thinking = "on"
		}
		m.chat = append(m.chat, chatMsg{role: core.RoleAssistant, content: fmt.Sprintf("Provider: %s\nModel: %s\nThinking: %s", m.provider, m.modelName, thinking)})
		return m, nil

	case "/think":
		m.showThinking = !m.showThinking
		status := "enabled"
		if !m.showThinking {
			status = "disabled"
		}
		m.chat = append(m.chat, chatMsg{role: core.RoleAssistant, content: fmt.Sprintf("Chain-of-thought thinking %s.", status)})
		return m, nil

	case "/exit", "/quit":
		return m, tea.Quit

	default:
		m.chat = append(m.chat, chatMsg{role: core.RoleAssistant, content: fmt.Sprintf("Unknown command: %s\nType /help for available commands.", cmd)})
		return m, nil
	}
}
