package commands

import "strings"

type Command struct {
	Name string
	Args []string
}

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
