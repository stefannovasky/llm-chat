package ui

import (
	"encoding/json"
	"os"

	"github.com/stefannovasky/llm-chat/internal/storage"
)

const recentCap = 10

// State tracks the current and recently used models.
type State struct {
	Current string   `json:"current"`
	Recent  []string `json:"recent"`
}

func statePath() string {
	return storage.XDGPath("XDG_STATE_HOME", ".local/state", "llm-chat", "recent_models.json")
}

// LoadState reads the recent models file. Missing or corrupted files return an
// empty state with no error — callers always get a usable value.
func LoadState() State {
	path := statePath()
	if path == "" {
		return State{}
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return State{}
	}
	var s State
	if err := json.Unmarshal(data, &s); err != nil {
		return State{}
	}
	return s
}

// saveState writes the state atomically. Persistence is best-effort.
func saveState(s State) error {
	path := statePath()
	if path == "" {
		return nil
	}
	return storage.WriteJSONAtomic(path, s, false)
}

func (s *State) touch(id string) {
	s.Current = id
	out := make([]string, 0, len(s.Recent)+1)
	out = append(out, id)
	for _, m := range s.Recent {
		if m != id {
			out = append(out, m)
		}
	}
	if len(out) > recentCap {
		out = out[:recentCap]
	}
	s.Recent = out
}
