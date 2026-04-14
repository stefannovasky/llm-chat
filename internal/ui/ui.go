package ui

import (
	"strings"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/textarea"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/stefannovasky/llm-chat/internal/config"
)

const (
	maxInputLines = 6
	dot           = "●"
)

var (
	dimStyle       = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	userDotStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("4"))
	assistDotStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("2"))
)

type role int

const (
	roleUser role = iota
	roleAssistant
)

type message struct {
	role    role
	content string
}

type Model struct {
	cfg       *config.Config
	width     int
	height    int
	separator string
	viewport  viewport.Model
	textarea  textarea.Model
	messages  []message
	initCmd   tea.Cmd
}

func New(cfg *config.Config) Model {
	ta := textarea.New()
	ta.Prompt = ""
	ta.ShowLineNumbers = false
	ta.Placeholder = ""
	ta.DynamicHeight = true
	ta.MinHeight = 1
	ta.MaxHeight = maxInputLines
	// MaxContentHeight must be set to a value higher than MaxHeight so that
	// atContentLimit() uses the visual-line check instead of the legacy
	// logical-line check, which would block InsertNewline at MaxHeight rows.
	ta.MaxContentHeight = 200

	styles := ta.Styles()
	styles.Focused.CursorLine = lipgloss.NewStyle()
	ta.SetStyles(styles)

	// Remap InsertNewline to alt+enter so plain enter can submit the message.
	// Terminals that don't support the Kitty keyboard protocol send ESC+CR for
	// shift+enter, which bubbletea decodes as "alt+enter".
	km := ta.KeyMap
	km.InsertNewline = key.NewBinding(key.WithKeys("alt+enter"))
	ta.KeyMap = km

	// Focus must be set before the model is stored so the textarea
	// accepts key events from the first Update tick.
	focusCmd := ta.Focus()

	vp := viewport.New()

	return Model{
		cfg:      cfg,
		viewport: vp,
		textarea: ta,
		initCmd:  focusCmd,
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

	// header (1 line) + top separator (1 line) + bottom separator (1 line)
	vpHeight := m.height - 3 - inputLines
	if vpHeight < 0 {
		vpHeight = 0
	}

	m.viewport.SetWidth(m.width)
	m.viewport.SetHeight(vpHeight)
}

func (m *Model) refreshViewport() {
	if len(m.messages) == 0 {
		m.viewport.SetContent("")
		return
	}

	contentWidth := m.viewport.Width() - 2 // "- 2" for "● " prefix
	if contentWidth < 1 {
		contentWidth = 1
	}

	var sb strings.Builder
	for i, msg := range m.messages {
		if i > 0 {
			sb.WriteString("\n\n")
		}

		var d string
		if msg.role == roleUser {
			d = userDotStyle.Render(dot)
		} else {
			d = assistDotStyle.Render(dot)
		}

		wrapped := lipgloss.Wrap(msg.content, contentWidth, " ")
		sb.WriteString(prefixLines(wrapped, d+" ", "  "))
	}

	m.viewport.SetContent(sb.String())
}

// prefixLines prepends first to the first line of s and rest to every
// subsequent line.
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

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.separator = dimStyle.Render(strings.Repeat("─", m.width))
		m.recalcLayout()
		m.refreshViewport()
		return m, nil

	case tea.KeyPressMsg:
		switch msg.String() {
		case "ctrl+c":
			return m, tea.Quit

		case "enter":
			content := strings.TrimSpace(m.textarea.Value())
			if content != "" {
				m.messages = append(m.messages, message{role: roleUser, content: content})
				m.textarea.Reset()
				m.recalcLayout()
				m.refreshViewport()
				m.viewport.GotoBottom()
			}
			return m, nil

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
