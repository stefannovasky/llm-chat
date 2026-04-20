package commands

import (
	"strings"
)

type Info struct {
	Name        string
	Description string
}

var All = []Info{
	{"model", "switch active model"},
	{"cost", "show session cost and token usage"},
	{"compact", "compact conversation history"},
	{"resume", "list and reopen a previous conversation"},
	{"help", "list available commands"},
}

func Help() string {
	maxName := 0
	for _, c := range All {
		if len(c.Name) > maxName {
			maxName = len(c.Name)
		}
	}
	var b strings.Builder
	b.WriteString("Available commands:\n")
	for _, c := range All {
		b.WriteString("  /")
		b.WriteString(c.Name)
		b.WriteString(strings.Repeat(" ", maxName-len(c.Name)+3))
		b.WriteString(c.Description)
		b.WriteByte('\n')
	}
	return strings.TrimRight(b.String(), "\n")
}
