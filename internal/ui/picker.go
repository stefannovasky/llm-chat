package ui

import (
	"fmt"
	"strconv"

	"charm.land/bubbles/v2/list"
	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"
)

const modelSearchHint = "press / to search"

type modelItem struct {
	model     llmInfo
	isCurrent bool
}

func (i modelItem) FilterValue() string { return i.model.ID }

func (i modelItem) Title() string {
	name := i.model.Name
	if name == "" {
		name = i.model.ID
	}
	if i.isCurrent {
		return assistDotStyle.Render("● ") + name
	}
	return "  " + name
}

func (i modelItem) Description() string {
	return fmt.Sprintf("  %s · %s · %s",
		i.model.ID,
		formatContext(i.model.ContextLength),
		formatPricing(i.model.PromptPrice, i.model.CompletionPrice),
	)
}

type pickerModel struct {
	list         list.Model
	spinner      spinner.Model
	loading      bool
	err          string
	width        int
	height       int
	currentModel string

	done     bool
	selected string
}

func newPicker(width, height int, currentModel string) pickerModel {
	sp := spinner.New(
		spinner.WithSpinner(spinner.Dot),
		spinner.WithStyle(assistDotStyle),
	)

	l := list.New(nil, newListDelegate(), width, height)
	configureListChrome(&l, "Select model · current: "+currentModel+" · "+modelSearchHint)

	return pickerModel{
		list:         l,
		spinner:      sp,
		loading:      true,
		width:        width,
		height:       height,
		currentModel: currentModel,
	}
}

func (p *pickerModel) setSize(w, h int) {
	p.width = w
	p.height = h
	p.list.SetSize(w, h)
}

func (p *pickerModel) setModels(all []llmInfo, recent []string) {
	ordered := orderModels(all, recent)
	items := make([]list.Item, len(ordered))
	for i, m := range ordered {
		items[i] = modelItem{model: m, isCurrent: m.ID == p.currentModel}
	}
	p.list.SetItems(items)
	p.loading = false
	p.err = ""
}

func (p *pickerModel) setError(msg string) {
	p.loading = false
	p.err = msg
}

func (p pickerModel) Update(msg tea.Msg) (pickerModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		key := msg.String()
		switch key {
		case "ctrl+c", "esc":
			// esc only closes when filter is inactive; otherwise the list eats it.
			if key == "esc" && p.list.FilterState() != list.Unfiltered {
				break
			}
			p.done = true
			p.selected = ""
			return p, nil
		case "enter":
			if p.loading || p.err != "" {
				return p, nil
			}
			if it, ok := p.list.SelectedItem().(modelItem); ok {
				p.done = true
				p.selected = it.model.ID
				return p, nil
			}
			return p, nil
		}
	case spinner.TickMsg:
		if p.loading {
			var cmd tea.Cmd
			p.spinner, cmd = p.spinner.Update(msg)
			return p, cmd
		}
		return p, nil
	}

	var cmd tea.Cmd
	p.list, cmd = p.list.Update(msg)
	return p, cmd
}

func (p pickerModel) View() string {
	switch {
	case p.loading:
		return centerMessage(p.width, p.height, p.spinner.View()+" Loading models...")
	case p.err != "":
		return centerMessage(p.width, p.height,
			errorStyle.Render(p.err)+"\n"+dimStyle.Render("press esc to close"))
	default:
		return p.list.View()
	}
}

func formatContext(n int) string {
	switch {
	case n >= 1_000_000:
		return fmt.Sprintf("%dM ctx", n/1_000_000)
	case n >= 1000:
		return fmt.Sprintf("%dK ctx", n/1000)
	case n > 0:
		return fmt.Sprintf("%d ctx", n)
	default:
		return "? ctx"
	}
}

func formatPricing(promptStr, completionStr string) string {
	p, errP := strconv.ParseFloat(promptStr, 64)
	c, errC := strconv.ParseFloat(completionStr, 64)
	if errP != nil || errC != nil {
		return "pricing n/a"
	}
	return fmt.Sprintf("$%.2f/$%.2f per 1M", p*1e6, c*1e6)
}
