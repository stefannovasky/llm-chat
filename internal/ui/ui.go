package ui

import (
	"context"
	"fmt"
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
	"github.com/stefannovasky/llm-chat/internal/config"
	"github.com/stefannovasky/llm-chat/internal/llm"
	"github.com/stefannovasky/llm-chat/internal/sessions"
)

const (
	compactPrompt = "You are summarizing this conversation so it can be continued with less context. " +
		"Preserve: the user's underlying goal, key facts and decisions, any code or concrete details referenced, " +
		"and any open questions or pending actions. Drop: pleasantries, tangents, and verbose explanations that " +
		"have already been acknowledged. Respond with the summary only — no preamble, no meta-commentary."
)

type modelsLoadedMsg struct {
	all []llmInfo
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
	warnStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("11"))
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
	ch  <-chan llm.StreamEvent
	err error
}

type streamEventMsg struct {
	ev llm.StreamEvent
	ok bool
}

type opKind int

const (
	opNone opKind = iota
	opStream
	opCompact
)

// activeOp holds the state of an in-flight LLM operation. Streaming and
// compacting are mutually exclusive, so they share buffer, channel, cancel and
// cancelled flag — kind discriminates which one is active.
type activeOp struct {
	kind      opKind
	buf       *strings.Builder
	usage     *llm.Usage
	ch        <-chan llm.StreamEvent
	cancel    context.CancelFunc
	cancelled bool
}

type Model struct {
	cfg             *config.Config
	client          *llm.Client
	currentModel    string
	state           State
	modelsCache     []llmInfo
	picker          pickerModel
	pickerActive    bool
	sessionsPicker  sessionsPickerModel
	sessionsActive  bool
	cost            costPanel
	costActive      bool
	width           int
	height          int
	separator       string
	viewport        viewport.Model
	textarea        textarea.Model
	spinner         spinner.Model
	messages        []message
	conversation    sessions.Conversation
	op              activeOp
	initCmd         tea.Cmd
	mdRenderer      *glamour.TermRenderer
	mdRendererWidth int
	currentSession  *sessions.Session
}

func (m *Model) autosave() {
	if m.currentSession == nil {
		now := time.Now().UTC()
		m.currentSession = &sessions.Session{
			ID:             sessions.NewID(),
			CreatedAt:      now,
			LastAccessedAt: now,
		}
	}
	m.currentSession.Title = sessions.DeriveTitle(m.conversation.Messages)
	m.currentSession.Messages = m.conversation.Messages
	_ = sessions.Save(m.currentSession)
}

func New(cfg *config.Config, client *llm.Client, currentModel string, state State) Model {
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
		conversation: newConversation(cfg.SystemPrompt),
	}
}

func newConversation(systemPrompt string) sessions.Conversation {
	return sessions.Conversation{
		Messages: []sessions.Message{
			{Role: sessions.RoleSystem, Content: systemPrompt},
		},
	}
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(m.initCmd, fetchModelsCmd())
}

func formatTokensCompact(n int) string {
	if n >= 1_000_000 {
		v := float64(n) / 1_000_000
		if v == float64(int(v)) {
			return fmt.Sprintf("%dM", int(v))
		}
		return fmt.Sprintf("%.1fM", v)
	}
	if n >= 1_000 {
		v := float64(n) / 1_000
		if v == float64(int(v)) {
			return fmt.Sprintf("%dk", int(v))
		}
		return fmt.Sprintf("%.1fk", v)
	}
	return fmt.Sprintf("%d", n)
}

func counterStyle(used, limit int) lipgloss.Style {
	if limit == 0 {
		return dimStyle
	}
	r := float64(used) / float64(limit)
	switch {
	case r >= 0.95:
		return errorStyle
	case r >= 0.80:
		return warnStyle
	default:
		return dimStyle
	}
}

func (m *Model) renderHeader() string {
	const brand = "llm-chat"
	brandRendered := dimStyle.Render(brand)
	brandWidth := lipgloss.Width(brandRendered)

	used := sessions.ContextUsed(m.conversation)

	var limit int
	for _, info := range m.modelsCache {
		if info.ID == m.currentModel {
			limit = info.ContextLength
			break
		}
	}

	var counter string
	if used > 0 || limit > 0 {
		usedStr := formatTokensCompact(used)
		if limit > 0 {
			counter = counterStyle(used, limit).Render(usedStr + " / " + formatTokensCompact(limit) + " tokens")
		} else {
			counter = dimStyle.Render(usedStr + " tokens")
		}
	}

	modelRendered := dimStyle.Render(m.currentModel)
	var rightContent string
	if counter != "" {
		rightContent = modelRendered + dimStyle.Render(" · ") + counter
	} else {
		rightContent = modelRendered
	}

	rightWidth := m.width - brandWidth
	if rightWidth < 1 {
		return brandRendered
	}

	const ellipsis = "…"
	if lipgloss.Width(rightContent) > rightWidth {
		rightContent = modelRendered
	}
	if lipgloss.Width(rightContent) > rightWidth {
		available := rightWidth - lipgloss.Width(ellipsis)
		runes := []rune(m.currentModel)
		if available > 0 && available < len(runes) {
			runes = runes[:available]
		}
		rightContent = dimStyle.Render(string(runes) + ellipsis)
	}
	if lipgloss.Width(rightContent) > rightWidth {
		return brandRendered
	}

	right := lipgloss.NewStyle().Width(rightWidth).Align(lipgloss.Right).Render(rightContent)
	return brandRendered + right
}

func (m *Model) recalcLayout() {
	if m.width == 0 || m.height == 0 {
		return
	}

	m.textarea.SetWidth(m.width - 2) // "- 2" for "> " prefix
	inputLines := m.textarea.Height()

	vpHeight := max(m.height-3-inputLines, 0) // 3 = header + 2 separators

	m.viewport.SetWidth(m.width)
	m.viewport.SetHeight(vpHeight)

	contentWidth := max(m.width-2, 1) // "- 2" for "● " prefix
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
	contentWidth := max(m.viewport.Width()-2, 1) // "- 2" for "● " prefix

	var sb strings.Builder
	for i, msg := range m.messages {
		if i > 0 {
			sb.WriteString("\n\n")
		}

		switch msg.role {
		case roleError:
			prefix := errorStyle.Render(errorMark) + " "
			wrapped := errorStyle.Render(lipgloss.Wrap(msg.content, contentWidth, " "))
			sb.WriteString(prefixLines(wrapped, prefix, "  "))
		case roleInfo:
			prefix := dimStyle.Render(dot) + " "
			wrapped := dimStyle.Render(lipgloss.Wrap(msg.content, contentWidth, " "))
			sb.WriteString(prefixLines(wrapped, prefix, "  "))
		case roleUser:
			prefix := userDotStyle.Render(dot) + " "
			wrapped := lipgloss.Wrap(msg.content, contentWidth, " ")
			sb.WriteString(prefixLines(wrapped, prefix, "  "))
		default:
			prefix := assistDotStyle.Render(dot) + " "
			rendered := m.renderMarkdown(msg.content, contentWidth)
			sb.WriteString(prefixLines(rendered, prefix, "  "))
		}
	}

	switch m.op.kind {
	case opStream:
		if len(m.messages) > 0 {
			sb.WriteString("\n\n")
		}
		prefix := assistDotStyle.Render(dot) + " "
		if m.op.buf.Len() == 0 {
			sb.WriteString(prefix)
			sb.WriteString(m.spinner.View())
		} else {
			wrapped := lipgloss.Wrap(m.op.buf.String(), contentWidth, " ")
			sb.WriteString(prefixLines(wrapped, prefix, "  "))
			sb.WriteString(" ")
			sb.WriteString(m.spinner.View())
		}
	case opCompact:
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

func startStreamCmd(ctx context.Context, client *llm.Client, model string, conv sessions.Conversation) tea.Cmd {
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
		all, err := fetchModels(context.Background())
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

func (m *Model) openSessionsPicker() {
	summaries, err := sessions.List()
	m.sessionsPicker = newSessionsPicker(m.width, m.height, summaries, err)
	m.sessionsActive = true
}

func (m *Model) applySession(s *sessions.Session) {
	if len(s.Messages) == 0 || s.Messages[0].Role != sessions.RoleSystem {
		m.addError("session file is corrupt: missing system prompt")
		return
	}
	m.currentSession = s
	m.currentSession.LastAccessedAt = time.Now().UTC()
	m.conversation.Messages = append([]sessions.Message(nil), s.Messages...)
	m.conversation.Messages[0].Content = m.cfg.SystemPrompt

	m.messages = m.messages[:0]
	for _, dm := range s.Messages {
		switch dm.Role {
		case sessions.RoleUser:
			m.messages = append(m.messages, message{role: roleUser, content: dm.Content})
		case sessions.RoleAssistant:
			m.messages = append(m.messages, message{role: roleAssistant, content: dm.Content})
		}
	}

	for i := len(s.Messages) - 1; i >= 0; i-- {
		if s.Messages[i].Role == sessions.RoleAssistant && s.Messages[i].Model != "" {
			m.selectModel(s.Messages[i].Model)
			break
		}
	}

	m.refreshViewport()
	m.viewport.GotoBottom()
}

func (m *Model) resetSession() {
	if len(m.conversation.Messages) > 1 {
		m.autosave()
	}
	m.messages = m.messages[:0]
	m.conversation = newConversation(m.cfg.SystemPrompt)
	m.currentSession = nil
	m.refreshViewport()
	m.viewport.GotoTop()
}

func (m *Model) selectModel(id string) {
	m.currentModel = id
	m.state.touch(id)
	_ = saveState(m.state)
}

func waitForEvent(ch <-chan llm.StreamEvent) tea.Cmd {
	return func() tea.Msg {
		ev, ok := <-ch
		return streamEventMsg{ev: ev, ok: ok}
	}
}

func isCompactable(msg sessions.Message) bool {
	return msg.Role != sessions.RoleSystem && msg.CompactedAt == nil
}

func (m *Model) startCompact() tea.Cmd {
	hasNewUser := false
	for _, msg := range m.conversation.Messages {
		if isCompactable(msg) && msg.Role == sessions.RoleUser {
			hasNewUser = true
			break
		}
	}
	if !hasNewUser {
		m.addError("nothing new to compact")
		return nil
	}
	ctx, cancel := context.WithCancel(context.Background())
	m.op = activeOp{kind: opCompact, buf: &strings.Builder{}, cancel: cancel}

	msgs := make([]sessions.Message, 0, len(m.conversation.Messages)+1)
	msgs = append(msgs, m.conversation.Messages...)
	msgs = append(msgs, sessions.Message{Role: sessions.RoleUser, Content: compactPrompt})
	compactConv := sessions.Conversation{Messages: msgs}

	m.recalcLayout()
	m.refreshViewport()
	m.viewport.GotoBottom()
	return tea.Batch(
		startStreamCmd(ctx, m.client, m.currentModel, compactConv),
		m.spinner.Tick,
	)
}

func (m *Model) resetOp() {
	m.op = activeOp{}
}

func (m *Model) addError(text string) {
	m.messages = append(m.messages, message{role: roleError, content: text})
	m.refreshViewport()
	m.viewport.GotoBottom()
}

func (m *Model) finalizeCompact() {
	defer m.resetOp()

	if m.op.cancelled {
		m.messages = append(m.messages, message{role: roleInfo, content: "Compact cancelled."})
		return
	}
	if m.op.buf.Len() == 0 {
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
	summary := sessions.Message{
		Role: sessions.RoleAssistant,
		Content: "[Conversation summary — condensed history of earlier turns]\n" +
			m.op.buf.String() +
			"\n[End of summary]",
		Model: m.currentModel,
	}
	if m.op.usage != nil {
		summary.PromptTokens = m.op.usage.PromptTokens
		summary.CompletionTokens = m.op.usage.CompletionTokens
		summary.Cost = m.op.usage.Cost
	}
	m.conversation.Messages = append(m.conversation.Messages, summary)
	m.autosave()
	m.messages = append(m.messages, message{role: roleInfo, content: "Conversation compacted."})
}

func (m *Model) finalizeStream() {
	if m.op.buf.Len() > 0 {
		content := m.op.buf.String()
		m.messages = append(m.messages, message{role: roleAssistant, content: content})
		dm := sessions.Message{Role: sessions.RoleAssistant, Content: content, Model: m.currentModel}
		if m.op.usage != nil {
			dm.PromptTokens = m.op.usage.PromptTokens
			dm.CompletionTokens = m.op.usage.CompletionTokens
			dm.Cost = m.op.usage.Cost
		}
		m.conversation.Messages = append(m.conversation.Messages, dm)
		m.autosave()
	}
	m.resetOp()
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
		if m.sessionsActive {
			m.sessionsPicker.setSize(m.width, m.height)
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
			cancelled := m.op.kind == opCompact && m.op.cancelled
			if m.op.cancel != nil {
				m.op.cancel()
			}
			m.resetOp()
			if cancelled {
				m.messages = append(m.messages, message{role: roleInfo, content: "Compact cancelled."})
				m.refreshViewport()
				m.viewport.GotoBottom()
			} else {
				m.addError(msg.err.Error())
			}
			return m, nil
		}
		m.op.ch = msg.ch
		return m, waitForEvent(msg.ch)

	case streamEventMsg:
		wasAtBottom := m.viewport.AtBottom()

		if !msg.ok {
			if m.op.cancel != nil {
				m.op.cancel()
			}
			if m.op.kind == opCompact {
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
			if m.op.cancel != nil {
				m.op.cancel()
			}
			cancelled := m.op.kind == opCompact && m.op.cancelled
			if m.op.kind == opCompact {
				m.resetOp()
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

		if m.op.kind == opCompact {
			if msg.ev.Usage != nil {
				m.op.usage = msg.ev.Usage
			}
			if msg.ev.Delta != "" {
				m.op.buf.WriteString(msg.ev.Delta)
			}
			return m, waitForEvent(m.op.ch)
		}

		if msg.ev.Usage != nil {
			m.op.usage = msg.ev.Usage
		}
		if msg.ev.Delta != "" {
			m.op.buf.WriteString(msg.ev.Delta)
			m.refreshViewport()
			if wasAtBottom {
				m.viewport.GotoBottom()
			}
		}
		return m, waitForEvent(m.op.ch)

	case spinner.TickMsg:
		if m.pickerActive && m.picker.loading {
			var cmd tea.Cmd
			m.picker, cmd = m.picker.Update(msg)
			return m, cmd
		}
		if m.op.kind != opNone {
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
		if m.sessionsActive {
			var cmd tea.Cmd
			m.sessionsPicker, cmd = m.sessionsPicker.Update(msg)
			if m.sessionsPicker.done {
				m.sessionsActive = false
				if id := m.sessionsPicker.selected; id != "" {
					if s, err := sessions.Load(id); err != nil {
						m.addError("failed to load session: " + err.Error())
					} else {
						m.applySession(s)
						sessions.Touch(s)
					}
				}
				return m, nil
			}
			return m, cmd
		}
		switch msg.String() {
		case "ctrl+c":
			switch m.op.kind {
			case opStream:
				if m.op.cancel != nil {
					m.op.cancel()
				}
				// Do not finalize here; wait for channel close so any pending
				// events are drained through the normal path.
				return m, nil
			case opCompact:
				m.op.cancelled = true
				if m.op.cancel != nil {
					m.op.cancel()
				}
				return m, nil
			}
			return m, tea.Quit

		case "enter":
			if m.op.kind != opNone {
				return m, nil
			}
			content := strings.TrimSpace(m.textarea.Value())
			if content == "" {
				return m, nil
			}
			if cmd, ok := parseCommand(content); ok {
				m.textarea.Reset()
				switch cmd.Name {
				case "model":
					return m, m.openPicker()
				case "cost":
					m.cost = newCostPanel(m.width, m.height, m.conversation)
					m.costActive = true
				case "compact":
					return m, m.startCompact()
				case "new":
					m.resetSession()
				case "resume":
					m.openSessionsPicker()
					return m, nil
				case "help":
					m.messages = append(m.messages, message{role: roleInfo, content: helpText()})
					m.refreshViewport()
					m.viewport.GotoBottom()
				default:
					m.addError("unknown command: /" + cmd.Name)
				}
				return m, nil
			}
			m.messages = append(m.messages, message{role: roleUser, content: content})
			m.conversation.Messages = append(m.conversation.Messages, sessions.Message{
				Role:    sessions.RoleUser,
				Content: content,
			})
			m.textarea.Reset()
			ctx, cancel := context.WithCancel(context.Background())
			m.op = activeOp{kind: opStream, buf: &strings.Builder{}, cancel: cancel}
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

	if m.sessionsActive {
		v.SetContent(m.sessionsPicker.View())
		return v
	}

	header := m.renderHeader()
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
