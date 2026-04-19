package commands

import "strings"

type Command struct {
	Name string
	Args []string
}

// Parse parses a slash command from input. Returns (Command, true) if input
// starts with '/' followed immediately by a non-empty command name.
func Parse(input string) (Command, bool) {
	if !strings.HasPrefix(input, "/") {
		return Command{}, false
	}
	rest := input[1:]
	if rest == "" || rest[0] == ' ' {
		return Command{}, false
	}
	parts := strings.Fields(rest)
	return Command{Name: parts[0], Args: parts[1:]}, true
}
