package commands

import "strings"

type Command struct {
	Name string
}

func Parse(input string) (Command, bool) {
	if !strings.HasPrefix(input, "/") {
		return Command{}, false
	}
	rest := input[1:]
	if rest == "" || rest[0] == ' ' {
		return Command{}, false
	}
	return Command{Name: strings.Fields(rest)[0]}, true
}
