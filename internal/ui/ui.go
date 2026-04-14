package ui

import (
	"strings"

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
	dimStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	userDotStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("4"))
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
	cfg      *config.Config
	width    int
	height   int
	viewport viewport.Model
	textarea textarea.Model
	messages []message
	initCmd  tea.Cmd
}

func New(cfg *config.Config) Model {
	ta := textarea.New()
	ta.Prompt = ""
	ta.ShowLineNumbers = false
	ta.Placeholder = ""

	styles := ta.Styles()
	styles.Focused.CursorLine = lipgloss.NewStyle()
	ta.SetStyles(styles)

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

	// header (1 line) + top separator (1 line) = 2 fixed at top
	// bottom separator (1 line) = 1 fixed
	// input: 1–6 lines, dynamic
	inputLines := m.textarea.LineCount()
	if inputLines < 1 {
		inputLines = 1
	}
	if inputLines > maxInputLines {
		inputLines = maxInputLines
	}

	vpHeight := m.height - 2 - 1 - inputLines
	if vpHeight < 0 {
		vpHeight = 0
	}

	m.viewport.SetWidth(m.width)
	m.viewport.SetHeight(vpHeight)
	m.textarea.SetWidth(m.width - 2) // "- 2" for "> " prefix
	m.textarea.SetHeight(inputLines)
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

		wrapped := wordWrap(msg.content, contentWidth)
		lines := strings.Split(wrapped, "\n")
		sb.WriteString(d + " " + lines[0])
		for _, line := range lines[1:] {
			sb.WriteString("\n  " + line)
		}
	}

	m.viewport.SetContent(sb.String())
}

func wordWrap(text string, width int) string {
	if width <= 0 {
		return text
	}
	var result []string
	for _, para := range strings.Split(text, "\n") {
		result = append(result, wrapLine(para, width))
	}
	return strings.Join(result, "\n")
}

func wrapLine(line string, width int) string {
	if len(line) <= width {
		return line
	}
	words := strings.Fields(line)
	if len(words) == 0 {
		return ""
	}
	var lines []string
	current := ""
	for _, word := range words {
		if current == "" {
			current = word
		} else if len(current)+1+len(word) <= width {
			current += " " + word
		} else {
			lines = append(lines, current)
			current = word
		}
	}
	if current != "" {
		lines = append(lines, current)
	}
	return strings.Join(lines, "\n")
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.recalcLayout()
		m.refreshViewport()
		return m, nil

	case tea.KeyPressMsg:
		// shift+enter: insert newline
		if msg.Code == tea.KeyEnter && msg.Mod&tea.ModShift != 0 {
			m.textarea.InsertRune('\n')
			m.recalcLayout()
			return m, nil
		}

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

		var cmd tea.Cmd
		m.textarea, cmd = m.textarea.Update(msg)
		m.recalcLayout()
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

	separator := dimStyle.Render(strings.Repeat("─", m.width))
	header := dimStyle.Render("llm-chat")

	taLines := strings.Split(m.textarea.View(), "\n")
	var inputParts []string
	for i, line := range taLines {
		if i == 0 {
			inputParts = append(inputParts, dimStyle.Render(">")+" "+line)
		} else {
			inputParts = append(inputParts, "  "+line)
		}
	}
	input := strings.Join(inputParts, "\n")

	v.SetContent(strings.Join([]string{
		header,
		separator,
		m.viewport.View(),
		separator,
		input,
	}, "\n"))

	return v
}
