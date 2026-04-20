package models

import (
	"encoding/json"
	"os"

	"github.com/stefannovasky/llm-chat/internal/fsutil"
)

const recentCap = 10

type State struct {
	Current string   `json:"current"`
	Recent  []string `json:"recent"`
}

func StatePath() string {
	return fsutil.XDGPath("XDG_STATE_HOME", ".local/state", "llm-chat", "recent_models.json")
}

// LoadState reads the recent models file. Missing or corrupted files return an
// empty state with no error — callers always get a usable value.
func LoadState() State {
	path := StatePath()
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

// SaveState writes the state atomically. Returns nil on success, error on IO
// failure (caller may ignore — persistence is best-effort).
func SaveState(s State) error {
	path := StatePath()
	if path == "" {
		return nil
	}
	return fsutil.WriteJSONAtomic(path, s, false)
}

func (s *State) Touch(id string) {
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

func Order(all []Model, recent []string) []Model {
	byID := make(map[string]Model, len(all))
	for _, m := range all {
		byID[m.ID] = m
	}
	out := make([]Model, 0, len(all))
	seen := make(map[string]bool)
	for _, id := range recent {
		if m, ok := byID[id]; ok && !seen[id] {
			out = append(out, m)
			seen[id] = true
		}
	}
	for _, m := range all {
		if !seen[m.ID] {
			out = append(out, m)
		}
	}
	return out
}
