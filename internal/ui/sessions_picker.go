package ui

import (
	"fmt"

	"charm.land/bubbles/v2/list"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/stefannovasky/llm-chat/internal/sessions"
)

const sessionSearchHint = "press / to search"

type sessionItem struct {
	sum sessions.Summary
}

func (i sessionItem) FilterValue() string { return i.sum.Title }

func (i sessionItem) Title() string {
	t := i.sum.Title
	if t == "" {
		t = "(untitled)"
	}
	return "  " + t
}

func (i sessionItem) Description() string {
	return fmt.Sprintf("  %s · %d msgs · $%.4f",
		i.sum.UpdatedAt.Local().Format("2006-01-02 15:04"),
		i.sum.Messages,
		i.sum.Cost,
	)
}

type sessionsPickerModel struct {
	list   list.Model
	err    string
	width  int
	height int

	done     bool
	selected string
}

func newSessionsPicker(width, height int, summaries []sessions.Summary, loadErr error) sessionsPickerModel {
	delegate := list.NewDefaultDelegate()
	delegate.SetSpacing(1)
	zero := lipgloss.NewStyle()
	delegate.Styles.NormalTitle = zero.Foreground(lipgloss.Color("252"))
	delegate.Styles.NormalDesc = zero.Foreground(lipgloss.Color("240"))
	delegate.Styles.SelectedTitle = zero.Foreground(lipgloss.Color("12"))
	delegate.Styles.SelectedDesc = zero.Foreground(lipgloss.Color("12"))
	delegate.Styles.DimmedTitle = zero.Foreground(lipgloss.Color("240"))
	delegate.Styles.DimmedDesc = zero.Foreground(lipgloss.Color("238"))
	delegate.Styles.FilterMatch = zero.Foreground(lipgloss.Color("11"))

	items := make([]list.Item, len(summaries))
	for i, s := range summaries {
		items[i] = sessionItem{sum: s}
	}

	l := list.New(items, delegate, width, height)
	l.Title = "Resume session · " + sessionSearchHint
	l.KeyMap.Filter.SetHelp("/", "search")
	l.SetShowStatusBar(true)
	l.SetShowPagination(false)
	l.SetFilteringEnabled(true)
	l.SetShowHelp(true)
	l.DisableQuitKeybindings()
	l.Styles.Title = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))

	p := sessionsPickerModel{list: l, width: width, height: height}
	switch {
	case loadErr != nil:
		p.err = "failed to list sessions: " + loadErr.Error()
	case len(summaries) == 0:
		p.err = "no saved sessions"
	}
	return p
}

func (p *sessionsPickerModel) setSize(w, h int) {
	p.width = w
	p.height = h
	p.list.SetSize(w, h)
}

func (p sessionsPickerModel) Update(msg tea.Msg) (sessionsPickerModel, tea.Cmd) {
	if m, ok := msg.(tea.KeyPressMsg); ok {
		key := m.String()
		switch key {
		case "ctrl+c", "esc":
			if key == "esc" && p.list.FilterState() != list.Unfiltered {
				break
			}
			p.done = true
			p.selected = ""
			return p, nil
		case "enter":
			if p.err != "" {
				return p, nil
			}
			if it, ok := p.list.SelectedItem().(sessionItem); ok {
				p.done = true
				p.selected = it.sum.ID
				return p, nil
			}
			return p, nil
		}
	}
	var cmd tea.Cmd
	p.list, cmd = p.list.Update(msg)
	return p, cmd
}

func (p sessionsPickerModel) View() string {
	if p.err != "" {
		return lipgloss.Place(p.width, p.height,
			lipgloss.Center, lipgloss.Center,
			dimStyle.Render(p.err)+"\n"+dimStyle.Render("press esc to close"))
	}
	return p.list.View()
}
