package models

import (
	"encoding/json"
	"os"
	"path/filepath"
)

const recentCap = 10

type State struct {
	Current string   `json:"current"`
	Recent  []string `json:"recent"`
}

func StatePath() string {
	if x := os.Getenv("XDG_STATE_HOME"); x != "" {
		return filepath.Join(x, "llm-chat", "recent_models.json")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".local", "state", "llm-chat", "recent_models.json")
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
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	data, err := json.Marshal(s)
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
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
