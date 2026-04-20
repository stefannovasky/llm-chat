package ui

import (
	"context"
	"math"
	"strings"
	"time"

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
	"github.com/stefannovasky/llm-chat/internal/models"
)

type modelsLoadedMsg struct {
	all []models.Model
	err error
}

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
	roleInfo
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
	cfg              *config.Config
	client           *llm.Client
	currentModel     string
	state            models.State
	modelsCache      []models.Model
	picker           pickerModel
	pickerActive     bool
	cost             costPanel
	costActive       bool
	width            int
	height           int
	separator        string
	viewport         viewport.Model
	textarea         textarea.Model
	spinner          spinner.Model
	messages         []message
	conversation     domain.Conversation
	streaming        bool
	streamBuf        *strings.Builder
	streamCh         <-chan domain.StreamEvent
	streamUsage      *domain.Usage
	cancel           context.CancelFunc
	compacting       bool
	compactBuf       *strings.Builder
	compactUsage     *domain.Usage
	compactCancelled bool
	initCmd          tea.Cmd
	mdRenderer       *glamour.TermRenderer
	mdRendererWidth  int
}

func New(cfg *config.Config, client *llm.Client, currentModel string, state models.State) Model {
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
		cfg:          cfg,
		client:       client,
		currentModel: currentModel,
		state:        state,
		viewport:     vp,
		textarea:     ta,
		spinner:      s,
		initCmd:      focusCmd,
		streamBuf:    &strings.Builder{},
		compactBuf:   &strings.Builder{},
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
		} else if msg.role == roleInfo {
			prefix = dimStyle.Render(dot) + " "
			wrapped := dimStyle.Render(lipgloss.Wrap(msg.content, contentWidth, " "))
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
	} else if m.compacting {
		if len(m.messages) > 0 {
			sb.WriteString("\n\n")
		}
		prefix := dimStyle.Render(dot) + " "
		sb.WriteString(prefix)
		sb.WriteString(dimStyle.Render("Compacting conversation..."))
		sb.WriteString(" ")
		sb.WriteString(m.spinner.View())
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

func startStreamCmd(ctx context.Context, client *llm.Client, model string, conv domain.Conversation) tea.Cmd {
	return func() tea.Msg {
		ch, err := client.Stream(ctx, model, conv)
		if err != nil {
			return streamStartMsg{err: err}
		}
		return streamStartMsg{ch: ch}
	}
}

func fetchModelsCmd() tea.Cmd {
	return func() tea.Msg {
		all, err := models.Fetch(context.Background())
		return modelsLoadedMsg{all: all, err: err}
	}
}

func (m *Model) openPicker() tea.Cmd {
	p := newPicker(m.width, m.height, m.currentModel)
	if m.modelsCache != nil {
		p.setModels(m.modelsCache, m.state.Recent)
	}
	m.picker = p
	m.pickerActive = true

	if m.modelsCache != nil {
		return nil
	}
	return tea.Batch(fetchModelsCmd(), p.spinner.Tick)
}

func (m *Model) selectModel(id string) {
	m.currentModel = id
	m.state.Touch(id)
	_ = models.SaveState(m.state)
}

func waitForEvent(ch <-chan domain.StreamEvent) tea.Cmd {
	return func() tea.Msg {
		ev, ok := <-ch
		return streamEventMsg{ev: ev, ok: ok}
	}
}

func isCompactable(msg domain.Message) bool {
	return msg.Role != domain.RoleSystem && msg.CompactedAt == nil
}

func (m *Model) startCompact() tea.Cmd {
	hasNewUser := false
	for _, msg := range m.conversation.Messages {
		if isCompactable(msg) && msg.Role == domain.RoleUser {
			hasNewUser = true
			break
		}
	}
	if !hasNewUser {
		m.addError("nothing new to compact")
		return nil
	}
	m.compacting = true
	m.compactCancelled = false
	m.compactBuf.Reset()
	m.compactUsage = nil
	ctx, cancel := context.WithCancel(context.Background())
	m.cancel = cancel

	msgs := make([]domain.Message, 0, len(m.conversation.Messages)+1)
	msgs = append(msgs, m.conversation.Messages...)
	msgs = append(msgs, domain.Message{Role: domain.RoleUser, Content: domain.CompactPrompt})
	compactConv := domain.Conversation{Messages: msgs}

	m.recalcLayout()
	m.refreshViewport()
	m.viewport.GotoBottom()
	return tea.Batch(
		startStreamCmd(ctx, m.client, m.currentModel, compactConv),
		m.spinner.Tick,
	)
}

func (m *Model) resetCompactState() {
	m.compactBuf.Reset()
	m.compactUsage = nil
	m.compacting = false
	m.compactCancelled = false
	m.streamCh = nil
	m.cancel = nil
}

func (m *Model) addError(text string) {
	m.messages = append(m.messages, message{role: roleError, content: text})
	m.refreshViewport()
	m.viewport.GotoBottom()
}

func (m *Model) finalizeCompact() {
	defer m.resetCompactState()

	if m.compactCancelled {
		m.messages = append(m.messages, message{role: roleInfo, content: "Compact cancelled."})
		return
	}
	if m.compactBuf.Len() == 0 {
		m.messages = append(m.messages, message{role: roleError, content: "compact produced no summary"})
		return
	}

	now := time.Now().UTC()
	for i, msg := range m.conversation.Messages {
		if !isCompactable(msg) {
			continue
		}
		t := now
		m.conversation.Messages[i].CompactedAt = &t
	}
	summary := domain.Message{
		Role: domain.RoleAssistant,
		Content: "[Conversation summary — condensed history of earlier turns]\n" +
			m.compactBuf.String() +
			"\n[End of summary]",
		Model: m.currentModel,
	}
	if m.compactUsage != nil {
		summary.PromptTokens = m.compactUsage.PromptTokens
		summary.CompletionTokens = m.compactUsage.CompletionTokens
		summary.Cost = m.compactUsage.Cost
	}
	m.conversation.Messages = append(m.conversation.Messages, summary)
	m.messages = append(m.messages, message{role: roleInfo, content: "Conversation compacted."})
}

func (m *Model) finalizeStream() {
	if m.streamBuf.Len() > 0 {
		content := m.streamBuf.String()
		m.messages = append(m.messages, message{role: roleAssistant, content: content})
		dm := domain.Message{Role: domain.RoleAssistant, Content: content, Model: m.currentModel}
		if m.streamUsage != nil {
			dm.PromptTokens = m.streamUsage.PromptTokens
			dm.CompletionTokens = m.streamUsage.CompletionTokens
			dm.Cost = m.streamUsage.Cost
		}
		m.conversation.Messages = append(m.conversation.Messages, dm)
	}
	m.streamBuf.Reset()
	m.streamUsage = nil
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
		if m.pickerActive {
			m.picker.setSize(m.width, m.height)
		}
		return m, nil

	case modelsLoadedMsg:
		if msg.err == nil {
			m.modelsCache = msg.all
		}
		if m.pickerActive {
			if msg.err != nil {
				m.picker.setError("failed to fetch models: " + msg.err.Error())
			} else {
				m.picker.setModels(msg.all, m.state.Recent)
			}
		}
		return m, nil

	case streamStartMsg:
		if msg.err != nil {
			cancelled := m.compacting && m.compactCancelled
			m.streaming = false
			if m.cancel != nil {
				m.cancel()
			}
			m.resetCompactState()
			if cancelled {
				m.messages = append(m.messages, message{role: roleInfo, content: "Compact cancelled."})
				m.refreshViewport()
				m.viewport.GotoBottom()
			} else {
				m.addError(msg.err.Error())
			}
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
			if m.compacting {
				m.finalizeCompact()
			} else {
				m.finalizeStream()
			}
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
			cancelled := m.compacting && m.compactCancelled
			if m.compacting {
				m.resetCompactState()
			} else {
				m.finalizeStream()
			}
			if cancelled {
				m.messages = append(m.messages, message{role: roleInfo, content: "Compact cancelled."})
			} else {
				m.messages = append(m.messages, message{role: roleError, content: msg.ev.Err.Error()})
			}
			m.refreshViewport()
			if wasAtBottom {
				m.viewport.GotoBottom()
			}
			return m, nil
		}

		if m.compacting {
			if msg.ev.Usage != nil {
				m.compactUsage = msg.ev.Usage
			}
			if msg.ev.Delta != "" {
				m.compactBuf.WriteString(msg.ev.Delta)
			}
			return m, waitForEvent(m.streamCh)
		}

		if msg.ev.Usage != nil {
			m.streamUsage = msg.ev.Usage
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
		if m.pickerActive && m.picker.loading {
			var cmd tea.Cmd
			m.picker, cmd = m.picker.Update(msg)
			return m, cmd
		}
		if m.streaming || m.compacting {
			var cmd tea.Cmd
			m.spinner, cmd = m.spinner.Update(msg)
			m.refreshViewport()
			return m, cmd
		}
		return m, nil

	case tea.KeyPressMsg:
		if m.costActive {
			var cmd tea.Cmd
			m.cost, cmd = m.cost.Update(msg)
			if m.cost.done {
				m.costActive = false
			}
			return m, cmd
		}
		if m.pickerActive {
			var cmd tea.Cmd
			m.picker, cmd = m.picker.Update(msg)
			if m.picker.done {
				m.pickerActive = false
				if m.picker.selected != "" {
					m.selectModel(m.picker.selected)
				}
				return m, nil
			}
			return m, cmd
		}
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
			if m.compacting {
				m.compactCancelled = true
				if m.cancel != nil {
					m.cancel()
				}
				return m, nil
			}
			return m, tea.Quit

		case "enter":
			if m.streaming || m.compacting {
				return m, nil
			}
			content := strings.TrimSpace(m.textarea.Value())
			if content == "" {
				return m, nil
			}
			if cmd, ok := commands.Parse(content); ok {
				m.textarea.Reset()
				switch cmd.Name {
				case "model":
					return m, m.openPicker()
				case "cost":
					m.cost = newCostPanel(m.width, m.height, m.conversation)
					m.costActive = true
				case "compact":
					return m, m.startCompact()
				case "help":
					m.messages = append(m.messages, message{role: roleInfo, content: commands.Help()})
					m.refreshViewport()
					m.viewport.GotoBottom()
				default:
					m.addError("unknown command: /" + cmd.Name)
				}
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
				startStreamCmd(ctx, m.client, m.currentModel, m.conversation),
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

	if m.pickerActive {
		var cmd tea.Cmd
		m.picker, cmd = m.picker.Update(msg)
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

	if m.costActive {
		v.SetContent(m.cost.View())
		return v
	}

	if m.pickerActive {
		v.SetContent(m.picker.View())
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
