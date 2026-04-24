package ui

import "strings"

type command struct {
	Name string
}

type commandInfo struct {
	Name        string
	Description string
}

var allCommands = []commandInfo{
	{"help", "list available commands"},
	{"new", "start a fresh conversation"},
	{"model", "switch active model"},
	{"cost", "show session cost and token usage"},
	{"compact", "compact conversation history"},
	{"resume", "list and reopen a previous conversation"},
}

func parseCommand(input string) (command, bool) {
	if !strings.HasPrefix(input, "/") {
		return command{}, false
	}
	rest := input[1:]
	if rest == "" || rest[0] == ' ' {
		return command{}, false
	}
	return command{Name: strings.Fields(rest)[0]}, true
}

func helpText() string {
	maxName := 0
	for _, c := range allCommands {
		if len(c.Name) > maxName {
			maxName = len(c.Name)
		}
	}
	var b strings.Builder
	b.WriteString("Available commands:\n")
	for _, c := range allCommands {
		b.WriteString("  /")
		b.WriteString(c.Name)
		b.WriteString(strings.Repeat(" ", maxName-len(c.Name)+3))
		b.WriteString(c.Description)
		b.WriteByte('\n')
	}
	return strings.TrimRight(b.String(), "\n")
}
