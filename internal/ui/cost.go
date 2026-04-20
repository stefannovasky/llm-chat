package ui

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/stefannovasky/llm-chat/internal/domain"
)

type costPanel struct {
	content string
	width   int
	height  int
	done    bool
}

type modelStats struct {
	promptTokens     int
	completionTokens int
	cost             float64
}

func newCostPanel(width, height int, conv domain.Conversation) costPanel {
	return costPanel{
		content: buildCostContent(conv),
		width:   width,
		height:  height,
	}
}

func buildCostContent(conv domain.Conversation) string {
	stats := map[string]*modelStats{}
	order := []string{}

	for _, m := range conv.Messages {
		if m.Role != domain.RoleAssistant || m.Model == "" {
			continue
		}
		if _, ok := stats[m.Model]; !ok {
			stats[m.Model] = &modelStats{}
			order = append(order, m.Model)
		}
		s := stats[m.Model]
		s.promptTokens += m.PromptTokens
		s.completionTokens += m.CompletionTokens
		s.cost += m.Cost
	}

	if len(order) == 0 {
		return dimStyle.Render("no usage yet")
	}

	// Find max model name length for alignment.
	maxLen := 0
	for _, id := range order {
		if len(id) > maxLen {
			maxLen = len(id)
		}
	}

	var sb strings.Builder
	var totalCost float64
	var totalPrompt, totalCompletion int

	for _, id := range order {
		s := stats[id]
		totalCost += s.cost
		totalPrompt += s.promptTokens
		totalCompletion += s.completionTokens
		padding := strings.Repeat(" ", maxLen-len(id))
		sb.WriteString(fmt.Sprintf("%s%s  $%.6f  %s in + %s out\n",
			id, padding,
			s.cost,
			formatTokens(s.promptTokens),
			formatTokens(s.completionTokens),
		))
	}

	if len(order) > 1 {
		sep := strings.Repeat("─", maxLen+32)
		sb.WriteString(sep + "\n")
		padding := strings.Repeat(" ", maxLen-5) // len("total") = 5
		sb.WriteString(fmt.Sprintf("total%s  $%.6f  %s in + %s out",
			padding,
			totalCost,
			formatTokens(totalPrompt),
			formatTokens(totalCompletion),
		))
	} else {
		// Trim trailing newline when there's only one row.
		result := strings.TrimRight(sb.String(), "\n")
		return dimStyle.Render(result)
	}

	return dimStyle.Render(sb.String())
}

func formatTokens(n int) string {
	if n < 1000 {
		return fmt.Sprintf("%d", n)
	}
	return fmt.Sprintf("%d,%03d", n/1000, n%1000)
}

func (p costPanel) Update(msg tea.Msg) (costPanel, tea.Cmd) {
	if kp, ok := msg.(tea.KeyPressMsg); ok {
		switch kp.String() {
		case "esc", "ctrl+c", "q":
			p.done = true
		}
	}
	return p, nil
}

func (p costPanel) View() string {
	header := dimStyle.Render("session cost") + "\n" +
		dimStyle.Render(strings.Repeat("─", p.width)) + "\n\n"
	footer := "\n\n" + dimStyle.Render(strings.Repeat("─", p.width)) + "\n" +
		dimStyle.Render("esc to close")

	body := lipgloss.NewStyle().
		Width(p.width).
		Render(p.content)

	full := header + body + footer
	return lipgloss.Place(p.width, p.height, lipgloss.Left, lipgloss.Top, full)
}
