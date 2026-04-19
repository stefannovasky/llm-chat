package ui

import (
	"context"
	"math"
	"strings"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/spinner"
	"charm.land/bubbles/v2/textarea"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	glamour "charm.land/glamour/v2"
	glamourstyles "charm.land/glamour/v2/styles"
	"charm.land/lipgloss/v2"
	"github.com/stefannovasky/llm-chat/internal/commands"
	"github.com/stefannovasky/llm-chat/internal/config"
	"github.com/stefannovasky/llm-chat/internal/domain"
	"github.com/stefannovasky/llm-chat/internal/llm"
)

const (
	maxInputLines = 6
	dot           = "●"
	errorMark     = "!"
)

var (
	dimStyle       = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	userDotStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("12"))
	assistDotStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("10"))
	errorStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("1"))
)

type role int

const (
	roleUser role = iota
	roleAssistant
	roleError
)

type message struct {
	role    role
	content string
}

type streamStartMsg struct {
	ch  <-chan domain.StreamEvent
	err error
}

type streamEventMsg struct {
	ev domain.StreamEvent
	ok bool
}

type Model struct {
	cfg            *config.Config
	client         *llm.Client
	width          int
	height         int
	separator      string
	viewport       viewport.Model
	textarea       textarea.Model
	spinner        spinner.Model
	messages       []message
	conversation   domain.Conversation
	streaming      bool
	streamBuf      *strings.Builder
	streamCh       <-chan domain.StreamEvent
	cancel         context.CancelFunc
	initCmd        tea.Cmd
	mdRenderer     *glamour.TermRenderer
	mdRendererWidth int
}

func New(cfg *config.Config, client *llm.Client) Model {
	ta := textarea.New()
	ta.Prompt = ""
	ta.ShowLineNumbers = false
	ta.Placeholder = ""
	ta.DynamicHeight = true
	ta.MinHeight = 1
	ta.MaxHeight = maxInputLines
	// Without this, atContentLimit() uses the legacy logical-line check and
	// blocks InsertNewline once MaxHeight rows are reached.
	ta.MaxContentHeight = math.MaxInt

	styles := ta.Styles()
	styles.Focused.CursorLine = lipgloss.NewStyle()
	ta.SetStyles(styles)

	// alt+enter = newline; plain enter = submit.
	// Terminals without Kitty protocol send ESC+CR for shift+enter, which
	// bubbletea decodes as "alt+enter".
	km := ta.KeyMap
	km.InsertNewline = key.NewBinding(key.WithKeys("alt+enter"))
	ta.KeyMap = km

	// Must focus before storing so keystrokes work from the first tick.
	focusCmd := ta.Focus()

	vp := viewport.New()

	s := spinner.New(
		spinner.WithSpinner(spinner.Dot),
		spinner.WithStyle(assistDotStyle),
	)

	return Model{
		cfg:       cfg,
		client:    client,
		viewport:  vp,
		textarea:  ta,
		spinner:   s,
		initCmd:   focusCmd,
		streamBuf: &strings.Builder{},
		conversation: domain.Conversation{
			Messages: []domain.Message{
				{Role: domain.RoleSystem, Content: domain.DefaultSystemPrompt},
			},
		},
	}
}

func (m Model) Init() tea.Cmd {
	return m.initCmd
}

func (m *Model) recalcLayout() {
	if m.width == 0 || m.height == 0 {
		return
	}

	m.textarea.SetWidth(m.width - 2) // "- 2" for "> " prefix
	inputLines := m.textarea.Height()

	vpHeight := m.height - 3 - inputLines // 3 = header + 2 separators
	if vpHeight < 0 {
		vpHeight = 0
	}

	m.viewport.SetWidth(m.width)
	m.viewport.SetHeight(vpHeight)

	contentWidth := m.width - 2 // "- 2" for "● " prefix
	if contentWidth < 1 {
		contentWidth = 1
	}
	if m.mdRenderer == nil || m.mdRendererWidth != contentWidth {
		style := glamourstyles.DarkStyleConfig
		zero := uint(0)
		style.Document.Margin = &zero
		r, err := glamour.NewTermRenderer(
			glamour.WithStyles(style),
			glamour.WithWordWrap(contentWidth),
		)
		if err == nil {
			m.mdRenderer = r
			m.mdRendererWidth = contentWidth
		}
	}
}

func (m *Model) refreshViewport() {
	contentWidth := m.viewport.Width() - 2 // "- 2" for "● " prefix
	if contentWidth < 1 {
		contentWidth = 1
	}

	var sb strings.Builder
	for i, msg := range m.messages {
		if i > 0 {
			sb.WriteString("\n\n")
		}

		var prefix string
		if msg.role == roleError {
			prefix = errorStyle.Render(errorMark) + " "
			wrapped := errorStyle.Render(lipgloss.Wrap(msg.content, contentWidth, " "))
			sb.WriteString(prefixLines(wrapped, prefix, "  "))
		} else {
			if msg.role == roleUser {
				prefix = userDotStyle.Render(dot) + " "
				wrapped := lipgloss.Wrap(msg.content, contentWidth, " ")
				sb.WriteString(prefixLines(wrapped, prefix, "  "))
			} else {
				prefix = assistDotStyle.Render(dot) + " "
				rendered := m.renderMarkdown(msg.content, contentWidth)
				sb.WriteString(prefixLines(rendered, prefix, "  "))
			}
		}
	}

	if m.streaming {
		if len(m.messages) > 0 {
			sb.WriteString("\n\n")
		}
		prefix := assistDotStyle.Render(dot) + " "
		if m.streamBuf.Len() == 0 {
			sb.WriteString(prefix)
			sb.WriteString(m.spinner.View())
		} else {
			wrapped := lipgloss.Wrap(m.streamBuf.String(), contentWidth, " ")
			sb.WriteString(prefixLines(wrapped, prefix, "  "))
			sb.WriteString(" ")
			sb.WriteString(m.spinner.View())
		}
	}

	m.viewport.SetContent(sb.String())
}

func (m *Model) renderMarkdown(content string, width int) string {
	if m.mdRenderer != nil {
		if out, err := m.mdRenderer.Render(content); err == nil {
			return strings.TrimSpace(out)
		}
	}
	return lipgloss.Wrap(content, width, " ")
}

func prefixLines(s, first, rest string) string {
	lines := strings.Split(s, "\n")
	var sb strings.Builder
	for i, line := range lines {
		if i > 0 {
			sb.WriteByte('\n')
			sb.WriteString(rest)
		} else {
			sb.WriteString(first)
		}
		sb.WriteString(line)
	}
	return sb.String()
}

func startStreamCmd(ctx context.Context, client *llm.Client, conv domain.Conversation) tea.Cmd {
	return func() tea.Msg {
		ch, err := client.Stream(ctx, conv)
		if err != nil {
			return streamStartMsg{err: err}
		}
		return streamStartMsg{ch: ch}
	}
}

func waitForEvent(ch <-chan domain.StreamEvent) tea.Cmd {
	return func() tea.Msg {
		ev, ok := <-ch
		return streamEventMsg{ev: ev, ok: ok}
	}
}

func (m *Model) finalizeStream() {
	if m.streamBuf.Len() > 0 {
		content := m.streamBuf.String()
		m.messages = append(m.messages, message{role: roleAssistant, content: content})
		m.conversation.Messages = append(m.conversation.Messages, domain.Message{
			Role:    domain.RoleAssistant,
			Content: content,
		})
	}
	m.streamBuf.Reset()
	m.streaming = false
	m.streamCh = nil
	m.cancel = nil
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.separator = dimStyle.Render(strings.Repeat("─", m.width))
		m.recalcLayout()
		m.refreshViewport()
		return m, nil

	case streamStartMsg:
		if msg.err != nil {
			m.streaming = false
			if m.cancel != nil {
				m.cancel()
				m.cancel = nil
			}
			m.messages = append(m.messages, message{role: roleError, content: msg.err.Error()})
			m.recalcLayout()
			m.refreshViewport()
			m.viewport.GotoBottom()
			return m, nil
		}
		m.streamCh = msg.ch
		return m, waitForEvent(msg.ch)

	case streamEventMsg:
		wasAtBottom := m.viewport.AtBottom()

		if !msg.ok {
			if m.cancel != nil {
				m.cancel()
			}
			m.finalizeStream()
			m.refreshViewport()
			if wasAtBottom {
				m.viewport.GotoBottom()
			}
			return m, nil
		}

		if msg.ev.Err != nil {
			if m.cancel != nil {
				m.cancel()
			}
			m.finalizeStream()
			m.messages = append(m.messages, message{role: roleError, content: msg.ev.Err.Error()})
			m.refreshViewport()
			if wasAtBottom {
				m.viewport.GotoBottom()
			}
			return m, nil
		}

		if msg.ev.Delta != "" {
			m.streamBuf.WriteString(msg.ev.Delta)
			m.refreshViewport()
			if wasAtBottom {
				m.viewport.GotoBottom()
			}
		}
		return m, waitForEvent(m.streamCh)

	case spinner.TickMsg:
		if m.streaming {
			var cmd tea.Cmd
			m.spinner, cmd = m.spinner.Update(msg)
			m.refreshViewport()
			return m, cmd
		}
		return m, nil

	case tea.KeyPressMsg:
		switch msg.String() {
		case "ctrl+c":
			if m.streaming {
				if m.cancel != nil {
					m.cancel()
				}
				// Do not finalize here; wait for channel close so any pending
				// events are drained through the normal path.
				return m, nil
			}
			return m, tea.Quit

		case "enter":
			if m.streaming {
				return m, nil
			}
			content := strings.TrimSpace(m.textarea.Value())
			if content == "" {
				return m, nil
			}
			if cmd, ok := commands.Parse(content); ok {
				m.textarea.Reset()
				m.recalcLayout()
				_ = cmd
				m.messages = append(m.messages, message{role: roleError, content: "unknown command: " + content})
				m.refreshViewport()
				m.viewport.GotoBottom()
				return m, nil
			}
			m.messages = append(m.messages, message{role: roleUser, content: content})
			m.conversation.Messages = append(m.conversation.Messages, domain.Message{
				Role:    domain.RoleUser,
				Content: content,
			})
			m.textarea.Reset()
			m.streaming = true
			m.streamBuf.Reset()
			ctx, cancel := context.WithCancel(context.Background())
			m.cancel = cancel
			m.recalcLayout()
			m.refreshViewport()
			m.viewport.GotoBottom()
			return m, tea.Batch(
				startStreamCmd(ctx, m.client, m.conversation),
				m.spinner.Tick,
			)

		case "pgup":
			m.viewport.PageUp()
			return m, nil

		case "pgdown":
			m.viewport.PageDown()
			return m, nil
		}

		prevHeight := m.textarea.Height()
		var cmd tea.Cmd
		m.textarea, cmd = m.textarea.Update(msg)
		if m.textarea.Height() != prevHeight {
			m.recalcLayout()
		}
		return m, cmd

	case tea.MouseWheelMsg:
		var cmd tea.Cmd
		m.viewport, cmd = m.viewport.Update(msg)
		return m, cmd
	}

	var cmd tea.Cmd
	m.textarea, cmd = m.textarea.Update(msg)
	return m, cmd
}

func (m Model) View() tea.View {
	var v tea.View
	v.AltScreen = true
	v.MouseMode = tea.MouseModeCellMotion

	if m.width == 0 {
		return v
	}

	header := dimStyle.Render("llm-chat")
	input := prefixLines(m.textarea.View(), dimStyle.Render(">")+" ", "  ")

	v.SetContent(strings.Join([]string{
		header,
		m.separator,
		m.viewport.View(),
		m.separator,
		input,
	}, "\n"))

	return v
}
