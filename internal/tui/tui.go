package tui

import (
	"context"
	"fmt"
	"strings"

	"github.com/agent/ai-terminal/internal/core"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
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

	chat     []chatMsg
	input    textinput.Model
	viewport viewport.Model
	spinner  spinner.Model
	ready    bool
	loading  bool
	err      error
	cancel   context.CancelFunc
	ctx      context.Context

	width        int
	height       int
	tokenCh      <-chan core.Token
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

// Fixed chrome: one line for the title bar, one for the divider, one for the
// input. Everything between them is the scrollable chat viewport.
const chromeHeight = 3

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

	chat := make([]chatMsg, 0, 100)
	chat = append(chat, chatMsg{
		role:    core.RoleAssistant,
		content: "Hello! How can I help you today?",
	})

	return &model{
		agent:     agent,
		session:   session,
		provider:  provider,
		modelName: modelName,
		setCore:   setCore,
		input:     ti,
		spinner:   s,
		chat:      chat,
		ctx:       context.Background(),
	}
}

func (m *model) Init() tea.Cmd {
	return textinput.Blink
}

func (m *model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		if !m.ready {
			m.viewport = viewport.New(msg.Width, m.viewportHeight())
			m.ready = true
		} else {
			m.viewport.Width = msg.Width
			m.viewport.Height = m.viewportHeight()
		}
		m.input.Width = msg.Width - 2
		if m.input.Width < 20 {
			m.input.Width = 20
		}
		m.refreshChat()
		return m, nil

	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyCtrlC:
			if m.loading {
				if m.cancel != nil {
					m.cancel()
				}
				m.loading = false
				m.chat = append(m.chat, chatMsg{role: core.RoleAssistant, content: "[Session cancelled]"})
				m.refreshChat()
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
				cmd := m.handleCommand(input)
				m.refreshChat()
				return m, cmd
			}

			m.chat = append(m.chat, chatMsg{role: core.RoleUser, content: input})
			m.loading = true
			m.refreshChat()

			ctx, cancel := context.WithCancel(m.ctx)
			m.cancel = cancel

			return m, tea.Batch(m.startStream(ctx, input), m.spinner.Tick)

		// Scroll keys go to the viewport; everything else (letters,
		// punctuation, backspace, ...) is typed into the input.
		case tea.KeyUp, tea.KeyDown, tea.KeyPgUp, tea.KeyPgDown,
			tea.KeyHome, tea.KeyEnd, tea.KeyCtrlU, tea.KeyCtrlD:
			return m.scrollViewport(msg)
		}

	case streamStartedMsg:
		m.tokenCh = msg.ch
		return m, m.nextToken()

	case tokenMsg:
		if msg.err != nil {
			m.loading = false
			m.chat = append(m.chat, chatMsg{role: core.RoleAssistant, content: fmt.Sprintf("Error: %v", msg.err)})
			if m.cancel != nil {
				m.cancel()
			}
			m.refreshChat()
			return m, nil
		}

		if msg.done {
			m.loading = false
			m.tokenCh = nil
			m.refreshChat()
			return m, nil
		}

		if len(m.chat) > 0 && m.chat[len(m.chat)-1].role == core.RoleAssistant {
			m.chat[len(m.chat)-1].content += msg.content
		} else {
			m.chat = append(m.chat, chatMsg{role: core.RoleAssistant, content: msg.content})
		}
		m.refreshChat()
		return m, m.nextToken()

	case errMsg:
		m.loading = false
		m.err = msg.err
		m.chat = append(m.chat, chatMsg{role: core.RoleAssistant, content: fmt.Sprintf("Error: %v", msg.err)})
		m.refreshChat()
		return m, nil

	case spinner.TickMsg:
		if !m.loading {
			// Spinner only ticks while a request is in flight.
			return m, nil
		}
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		m.refreshChat()
		return m, cmd
	}

	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	return m, cmd
}

// scrollViewport forwards a scroll key to the chat viewport so the user can
// read back through history without leaving the keyboard.
func (m *model) scrollViewport(msg tea.Msg) (tea.Model, tea.Cmd) {
	if !m.ready {
		return m, nil
	}
	var cmd tea.Cmd
	m.viewport, cmd = m.viewport.Update(msg)
	return m, cmd
}

func (m *model) View() string {
	if !m.ready {
		// First render happens before the initial window size arrives.
		return "\n  Loading..."
	}

	var b strings.Builder
	b.WriteString(TitleStyle.Render(fmt.Sprintf(" %s — %s ", m.provider, m.modelName)))
	b.WriteByte('\n')
	b.WriteString(m.viewport.View())
	b.WriteByte('\n')
	b.WriteString(m.divider())
	b.WriteByte('\n')
	b.WriteString(m.input.View())
	return b.String()
}

func (m *model) viewportHeight() int {
	h := m.height - chromeHeight
	if h < 1 {
		h = 1
	}
	return h
}

func (m *model) divider() string {
	w := m.width
	if w < 1 {
		w = 1
	}
	return DividerStyle.Render(strings.Repeat("─", w))
}

// refreshChat rebuilds the transcript and keeps the viewport pinned to the
// newest content, the way a normal chat app behaves.
func (m *model) refreshChat() {
	if !m.ready {
		return
	}
	m.viewport.SetContent(m.transcript())
	m.viewport.GotoBottom()
}

// transcript renders the chat history as one plain-ish string for the
// viewport. Each message is wrapped exactly once, before any ANSI styling is
// applied, so colors never leak across wrapped lines.
func (m *model) transcript() string {
	var b strings.Builder

	visible := m.chat
	const maxHistory = 100
	if len(visible) > maxHistory {
		visible = visible[len(visible)-maxHistory:]
	}

	w := m.width - 2
	if w < 20 {
		w = 20
	}

	for i, msg := range visible {
		if i > 0 {
			b.WriteByte('\n')
		}
		switch msg.role {
		case core.RoleUser:
			b.WriteString(RenderUserMessage(msg.content, w))
		case core.RoleAssistant:
			b.WriteString(RenderAssistantMessage(msg.content, w))
		}
		b.WriteByte('\n')
	}

	if m.loading {
		b.WriteByte('\n')
		b.WriteString(m.spinner.View() + " Thinking...")
		b.WriteByte('\n')
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
			// Preallocate and build the message list in one pass.
			msgs := make([]core.Message, 0, len(sessionMsgs)-start+2)
			msgs = append(msgs, req.Messages[0])
			msgs = append(msgs, sessionMsgs[start:]...)
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

func (m *model) handleCommand(input string) tea.Cmd {
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
			return nil
		}
		if m.setCore != nil {
			if err := m.setCore(arg, ""); err != nil {
				m.chat = append(m.chat, chatMsg{role: core.RoleAssistant, content: fmt.Sprintf("Error switching provider: %v", err)})
				return nil
			}
		}
		m.provider = arg
		m.chat = append(m.chat, chatMsg{role: core.RoleAssistant, content: fmt.Sprintf("Switched provider to: %s", arg)})
		return nil

	case "/model":
		if arg == "" {
			m.chat = append(m.chat, chatMsg{role: core.RoleAssistant, content: fmt.Sprintf("Current model: %s", m.modelName)})
			return nil
		}
		if m.setCore != nil {
			if err := m.setCore("", arg); err != nil {
				m.chat = append(m.chat, chatMsg{role: core.RoleAssistant, content: fmt.Sprintf("Error switching model: %v", err)})
				return nil
			}
		}
		m.modelName = arg
		m.chat = append(m.chat, chatMsg{role: core.RoleAssistant, content: fmt.Sprintf("Switched model to: %s", arg)})
		return nil

	case "/help":
		m.chat = append(m.chat, chatMsg{role: core.RoleAssistant, content: "Commands:\n  /provider <name>  - Switch provider (nvidia, groq, openai, etc.)\n  /model <name>     - Switch model\n  /think            - Toggle chain-of-thought reasoning\n  /clear            - Clear chat\n  /status           - Show current provider/model\n  /help             - Show this help\n  /exit, /quit      - Exit the program"})
		return nil

	case "/clear":
		m.chat = m.chat[:0]
		return nil

	case "/status":
		thinking := "off"
		if m.showThinking {
			thinking = "on"
		}
		m.chat = append(m.chat, chatMsg{role: core.RoleAssistant, content: fmt.Sprintf("Provider: %s\nModel: %s\nThinking: %s", m.provider, m.modelName, thinking)})
		return nil

	case "/think":
		m.showThinking = !m.showThinking
		status := "enabled"
		if !m.showThinking {
			status = "disabled"
		}
		m.chat = append(m.chat, chatMsg{role: core.RoleAssistant, content: fmt.Sprintf("Chain-of-thought thinking %s.", status)})
		return nil

	case "/exit", "/quit":
		return tea.Quit

	default:
		m.chat = append(m.chat, chatMsg{role: core.RoleAssistant, content: fmt.Sprintf("Unknown command: %s\nType /help for available commands.", cmd)})
		return nil
	}
}
