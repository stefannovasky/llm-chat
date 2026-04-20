package ui

import (
	"charm.land/bubbles/v2/list"
	"charm.land/lipgloss/v2"
)

func newListDelegate() list.DefaultDelegate {
	d := list.NewDefaultDelegate()
	d.SetSpacing(1)
	zero := lipgloss.NewStyle()
	d.Styles.NormalTitle = zero.Foreground(lipgloss.Color("252"))
	d.Styles.NormalDesc = zero.Foreground(lipgloss.Color("240"))
	d.Styles.SelectedTitle = zero.Foreground(lipgloss.Color("12"))
	d.Styles.SelectedDesc = zero.Foreground(lipgloss.Color("12"))
	d.Styles.DimmedTitle = zero.Foreground(lipgloss.Color("240"))
	d.Styles.DimmedDesc = zero.Foreground(lipgloss.Color("238"))
	// Default underlines matched filter chars; we want a plain highlight instead.
	d.Styles.FilterMatch = zero.Foreground(lipgloss.Color("11"))
	return d
}

func configureListChrome(l *list.Model, title string) {
	l.Title = title
	l.KeyMap.Filter.SetHelp("/", "search")
	l.SetShowStatusBar(true)
	l.SetShowPagination(false)
	l.SetFilteringEnabled(true)
	l.SetShowHelp(true)
	l.DisableQuitKeybindings()
	l.Styles.Title = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
}

func centerMessage(w, h int, body string) string {
	return lipgloss.Place(w, h, lipgloss.Center, lipgloss.Center, body)
}
