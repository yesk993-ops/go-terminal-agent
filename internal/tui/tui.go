package tui

import (
	"context"
	"fmt"
	"strings"
	"time"

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
	id      uint64
	content string
	done    bool
	err     error
}

type errMsg struct {
	id  uint64
	err error
}

type flushStreamMsg struct {
	id uint64
}

// Settings controls rendering and request behavior without requiring callers
// to trade response quality for UI responsiveness.
type Settings struct {
	MaxHistory int
	MaxTokens  int
	Temperature float64
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
	streamID     uint64
	showThinking bool
	maxHistory   int
	maxTokens    int
	temperature  float64

	// Completed messages are rendered once per width/history mutation. The
	// active answer is rendered at a modest frame rate, rather than rebuilding
	// the entire transcript once for every streamed token.
	historyRendered       string
	historyDirty          bool
	streamContent         strings.Builder
	streamFlushScheduled  bool
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
const streamRefreshInterval = 33 * time.Millisecond

func New(agent core.Agent, session core.Session, provider, modelName string, setCore func(provider, model string) error, settings Settings) tea.Model {
	s := spinner.New()
	s.Style = SpinnerStyle
	s.Spinner = spinner.Dot

	ti := textinput.New()
	ti.Placeholder = placeholderTexts[0]
	ti.Prompt = "> "
	ti.Focus()
	ti.CharLimit = 0
	ti.Width = 80

	if settings.MaxHistory <= 0 {
		settings.MaxHistory = 50
	}
	if settings.MaxTokens <= 0 {
		settings.MaxTokens = 8192
	}
	chat := make([]chatMsg, 0, settings.MaxHistory)
	chat = append(chat, chatMsg{
		role:    core.RoleAssistant,
		content: "Hello! How can I help you today?",
	})

	return &model{
		agent:        agent,
		session:      session,
		provider:     provider,
		modelName:    modelName,
		setCore:      setCore,
		input:        ti,
		spinner:      s,
		chat:         chat,
		ctx:          context.Background(),
		maxHistory:   settings.MaxHistory,
		maxTokens:    settings.MaxTokens,
		temperature:  settings.Temperature,
		historyDirty: true,
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
		m.historyDirty = true
		m.refreshChat()
		return m, nil

	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyCtrlC:
			if m.loading {
				if m.cancel != nil {
					m.cancel()
					m.cancel = nil
				}
				m.finishActiveAssistant()
				m.loading = false
				m.appendChat(core.RoleAssistant, "[Session cancelled]")
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

			userMessage := core.Message{Role: core.RoleUser, Content: input}
			if m.session != nil {
				// A session records each user turn exactly once. Agent.Run receives
				// the assembled history but deliberately does not append it again.
				m.session.Append(userMessage)
			}
			m.appendChat(core.RoleUser, input)
			m.streamContent.Reset()
			m.loading = true
			m.refreshChat()

				ctx, cancel := context.WithCancel(m.ctx)
				m.cancel = cancel
				m.streamID++
				streamID := m.streamID

				return m, tea.Batch(m.startStream(ctx, input, streamID), m.spinner.Tick)

		// Scroll keys go to the viewport; everything else (letters,
		// punctuation, backspace, ...) is typed into the input.
		case tea.KeyUp, tea.KeyDown, tea.KeyPgUp, tea.KeyPgDown,
			tea.KeyHome, tea.KeyEnd, tea.KeyCtrlU, tea.KeyCtrlD:
			return m.scrollViewport(msg)
		}

	case streamStartedMsg:
		if msg.id != m.streamID || !m.loading {
			return m, nil
		}
		m.tokenCh = msg.ch
		return m, m.nextToken(msg.id, msg.ch)

	case tokenMsg:
		if msg.id != m.streamID || !m.loading {
			return m, nil
		}
		if msg.err != nil {
			m.finishActiveAssistant()
			m.loading = false
			m.appendChat(core.RoleAssistant, fmt.Sprintf("Error: %v", msg.err))
			m.cancelActiveRequest()
			m.refreshChat()
			return m, nil
		}

		if msg.done {
			m.finishActiveAssistant()
			m.loading = false
			m.tokenCh = nil
			m.cancelActiveRequest()
			m.refreshChat()
			return m, nil
		}

		if msg.content != "" {
			m.streamContent.WriteString(msg.content)
			cmds := []tea.Cmd{m.nextToken(msg.id, m.tokenCh)}
			if !m.streamFlushScheduled {
				m.streamFlushScheduled = true
				cmds = append(cmds, flushStreamAfter(streamRefreshInterval, msg.id))
			}
			return m, tea.Batch(cmds...)
		}
		return m, m.nextToken(msg.id, m.tokenCh)

	case flushStreamMsg:
		if msg.id != m.streamID {
			return m, nil
		}
		m.streamFlushScheduled = false
		if m.loading {
			m.refreshChat()
		}
		return m, nil

	case errMsg:
		if msg.id != m.streamID || !m.loading {
			return m, nil
		}
		m.finishActiveAssistant()
		m.loading = false
		m.err = msg.err
		m.appendChat(core.RoleAssistant, fmt.Sprintf("Error: %v", msg.err))
		m.cancelActiveRequest()
		m.refreshChat()
		return m, nil

	case spinner.TickMsg:
		if !m.loading {
			// Spinner only ticks while a request is in flight.
			return m, nil
		}
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		// refreshChat uses a cached completed transcript, so keeping the spinner
		// lively no longer causes all prior messages to be wrapped again.
		m.refreshChat()
		return m, cmd
	}

	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	return m, cmd
}

func flushStreamAfter(delay time.Duration, streamID uint64) tea.Cmd {
	return tea.Tick(delay, func(time.Time) tea.Msg { return flushStreamMsg{id: streamID} })
}

func (m *model) appendChat(role core.Role, content string) {
	m.chat = append(m.chat, chatMsg{role: role, content: content})
	m.historyDirty = true
}

func (m *model) finishActiveAssistant() {
	if m.streamContent.Len() == 0 {
		return
	}
	m.appendChat(core.RoleAssistant, m.streamContent.String())
	m.streamContent.Reset()
	m.streamFlushScheduled = false
}

func (m *model) cancelActiveRequest() {
	if m.cancel != nil {
		m.cancel()
		m.cancel = nil
	}
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
	b.WriteString(fmt.Sprintf("%s — %s\n", m.provider, m.modelName))
	b.WriteString(m.viewport.View())
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
	return ""
}

// refreshChat rebuilds only the dynamic part of the transcript. Completed
// messages stay cached until a chat mutation or resize changes their layout.
func (m *model) refreshChat() {
	if !m.ready {
		return
	}
	m.viewport.SetContent(m.transcript())
	m.viewport.GotoBottom()
}

func (m *model) completedTranscript() string {
	if !m.historyDirty {
		return m.historyRendered
	}

	var b strings.Builder
	visible := m.chat
	if len(visible) > m.maxHistory {
		visible = visible[len(visible)-m.maxHistory:]
	}

	w := m.width - 2
	if w < 20 {
		w = 20
	}

	for i, msg := range visible {
		if i > 0 {
			// Blank line between turns so "You"/"Assistant" blocks are visually
			// separated rather than stacked flush against each other.
			b.WriteString("\n\n")
		}
		switch msg.role {
		case core.RoleUser:
			b.WriteString(RenderUserMessage(msg.content, w))
		case core.RoleAssistant:
			b.WriteString(RenderAssistantMessage(msg.content, w))
		}
	}

	m.historyRendered = b.String()
	m.historyDirty = false
	return m.historyRendered
}

// transcript renders a cached completed history plus the in-progress answer.
// Each completed message is wrapped once per meaningful UI change, not once
// per provider token.
func (m *model) transcript() string {
	var b strings.Builder
	completed := m.completedTranscript()
	b.WriteString(completed)

	if m.streamContent.Len() > 0 {
		if completed != "" {
			b.WriteString("\n\n")
		}
		w := m.width - 2
		if w < 20 {
			w = 20
		}
		b.WriteString(RenderAssistantMessage(m.streamContent.String(), w))
	}

	if m.loading {
		if completed != "" || m.streamContent.Len() > 0 {
			b.WriteString("\n\n")
		}
		b.WriteString(m.spinner.View() + " Thinking...")
	}

	return b.String()
}

type streamStartedMsg struct {
	id uint64
	ch <-chan core.Token
}

func (m *model) startStream(ctx context.Context, input string, streamID uint64) tea.Cmd {
	return func() tea.Msg {
		prompt := core.SystemPrompt
		if m.showThinking {
			prompt += "\n\nBefore answering, think step by step in a clear chain-of-thought. Show your reasoning inside <thinking>...</thinking> tags, then provide your final answer."
		}

		messages := []core.Message{{
			Role:    core.RoleSystem,
			Content: prompt,
		}}
		if m.session != nil {
			sessionMsgs := m.session.Messages()
			conversation := make([]core.Message, 0, len(sessionMsgs))
			for _, msg := range sessionMsgs {
				// Older releases persisted system prompts. Excluding those from
				// history avoids sending duplicated static instructions forever.
				if msg.Role != core.RoleSystem {
					conversation = append(conversation, msg)
				}
			}
			const maxContextMessages = 200
			if len(conversation) > maxContextMessages {
				conversation = conversation[len(conversation)-maxContextMessages:]
			}
			messages = append(messages, conversation...)
		} else {
			messages = append(messages, core.Message{Role: core.RoleUser, Content: input})
		}

		req := &core.Request{
			Model:     m.modelName,
			Messages:  messages,
			Stream:    true,
			MaxTokens: m.maxTokens,
			Options: map[string]any{
				"temperature": m.temperature,
			},
		}

		tokenCh, err := m.agent.Run(ctx, req)
		if err != nil {
			return errMsg{id: streamID, err: err}
		}

		return streamStartedMsg{id: streamID, ch: tokenCh}
	}
}

func (m *model) nextToken(streamID uint64, tokenCh <-chan core.Token) tea.Cmd {
	return func() tea.Msg {
		token, ok := <-tokenCh
		if !ok {
			return tokenMsg{id: streamID, done: true}
		}
		if token.Error != nil {
			return tokenMsg{id: streamID, err: token.Error}
		}
		if token.Done {
			return tokenMsg{id: streamID, done: true}
		}
		return tokenMsg{id: streamID, content: token.Content}
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
			m.appendChat(core.RoleAssistant, "Usage: /provider <name> (nvidia, groq, openai, anthropic, gemini, openrouter)")
			return nil
		}
		if m.setCore != nil {
			if err := m.setCore(arg, ""); err != nil {
				m.appendChat(core.RoleAssistant, fmt.Sprintf("Error switching provider: %v", err))
				return nil
			}
		}
		m.provider = arg
		m.appendChat(core.RoleAssistant, fmt.Sprintf("Switched provider to: %s", arg))
		return nil

	case "/model":
		if arg == "" {
			m.appendChat(core.RoleAssistant, fmt.Sprintf("Current model: %s", m.modelName))
			return nil
		}
		if m.setCore != nil {
			if err := m.setCore("", arg); err != nil {
				m.appendChat(core.RoleAssistant, fmt.Sprintf("Error switching model: %v", err))
				return nil
			}
		}
		m.modelName = arg
		m.appendChat(core.RoleAssistant, fmt.Sprintf("Switched model to: %s", arg))
		return nil

	case "/help":
		m.appendChat(core.RoleAssistant, "Commands:\n  /provider <name>  - Switch provider (nvidia, groq, openai, etc.)\n  /model <name>     - Switch model\n  /think            - Toggle chain-of-thought reasoning\n  /clear            - Clear chat\n  /status           - Show current provider/model\n  /help             - Show this help\n  /exit, /quit      - Exit the program")
		return nil

	case "/clear":
		m.chat = m.chat[:0]
		m.historyDirty = true
		if m.session != nil {
			m.session.Clear()
		}
		return nil

	case "/status":
		thinking := "off"
		if m.showThinking {
			thinking = "on"
		}
		m.appendChat(core.RoleAssistant, fmt.Sprintf("Provider: %s\nModel: %s\nThinking: %s", m.provider, m.modelName, thinking))
		return nil

	case "/think":
		m.showThinking = !m.showThinking
		status := "enabled"
		if !m.showThinking {
			status = "disabled"
		}
		m.appendChat(core.RoleAssistant, fmt.Sprintf("Chain-of-thought thinking %s.", status))
		return nil

	case "/exit", "/quit":
		// Cancel any in-flight stream so the streaming goroutine doesn't
		// outlive the program and try to push messages into a dead TUI.
		m.cancelActiveRequest()
		return tea.Quit

	default:
		m.appendChat(core.RoleAssistant, fmt.Sprintf("Unknown command: %s\nType /help for available commands.", cmd))
		return nil
	}
}
