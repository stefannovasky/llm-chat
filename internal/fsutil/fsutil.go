package fsutil

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// XDGPath returns an XDG-compliant path. If envVar is set, uses $envVar joined
// with parts. Otherwise falls back to $HOME/defaultSubdir joined with parts.
// Returns "" when HOME cannot be resolved.
func XDGPath(envVar, defaultSubdir string, parts ...string) string {
	if x := os.Getenv(envVar); x != "" {
		return filepath.Join(append([]string{x}, parts...)...)
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(append([]string{home, defaultSubdir}, parts...)...)
}

// WriteJSONAtomic marshals v to path via tmp + rename. Parent directories are
// created with 0755. indent=true pretty-prints with 2-space indentation.
func WriteJSONAtomic(path string, v any, indent bool) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	var data []byte
	var err error
	if indent {
		data, err = json.MarshalIndent(v, "", "  ")
	} else {
		data, err = json.Marshal(v)
	}
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}
