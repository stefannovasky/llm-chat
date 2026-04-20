package sessions

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/stefannovasky/llm-chat/internal/domain"
)

const (
	currentVersion = 1
	titleMaxRunes  = 60
)

type Session struct {
	Version   int              `json:"version"`
	ID        string           `json:"id"`
	Title     string           `json:"title"`
	CreatedAt time.Time        `json:"created_at"`
	UpdatedAt time.Time        `json:"updated_at"`
	Messages  []domain.Message `json:"messages"`
}

type Summary struct {
	ID        string
	Title     string
	UpdatedAt time.Time
	Cost      float64
	Messages  int
}

func Dir() string {
	if x := os.Getenv("XDG_DATA_HOME"); x != "" {
		return filepath.Join(x, "llm-chat", "sessions")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".local", "share", "llm-chat", "sessions")
}

func NewID() string {
	var b [4]byte
	_, _ = rand.Read(b[:])
	return time.Now().UTC().Format("20060102T150405Z") + "-" + hex.EncodeToString(b[:])
}

// DeriveTitle returns a single-line title derived from the first user message.
// Returns "" when no user message exists yet.
func DeriveTitle(msgs []domain.Message) string {
	for _, m := range msgs {
		if m.Role != domain.RoleUser {
			continue
		}
		t := strings.Join(strings.Fields(m.Content), " ")
		if t == "" {
			continue
		}
		if r := []rune(t); len(r) > titleMaxRunes {
			t = string(r[:titleMaxRunes-1]) + "…"
		}
		return t
	}
	return ""
}

func filePath(id string) (string, error) {
	d := Dir()
	if d == "" {
		return "", errors.New("cannot resolve sessions directory")
	}
	return filepath.Join(d, id+".json"), nil
}

// Save writes the session atomically, stamping UpdatedAt. CreatedAt is preserved
// across saves (caller sets it once on the initial save).
func Save(s *Session) error {
	if s.Version == 0 {
		s.Version = currentVersion
	}
	s.UpdatedAt = time.Now().UTC()
	path, err := filePath(s.ID)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func Load(id string) (*Session, error) {
	path, err := filePath(id)
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var s Session
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, err
	}
	return &s, nil
}

// List returns session summaries sorted by UpdatedAt descending. Unreadable or
// corrupted files are silently skipped — persistence is best-effort.
func List() ([]Summary, error) {
	d := Dir()
	if d == "" {
		return nil, nil
	}
	entries, err := os.ReadDir(d)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	out := make([]Summary, 0, len(entries))
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		id := strings.TrimSuffix(e.Name(), ".json")
		s, err := Load(id)
		if err != nil {
			continue
		}
		sum := Summary{
			ID:        s.ID,
			Title:     s.Title,
			UpdatedAt: s.UpdatedAt,
			Messages:  len(s.Messages),
		}
		for _, m := range s.Messages {
			sum.Cost += m.Cost
		}
		out = append(out, sum)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].UpdatedAt.After(out[j].UpdatedAt)
	})
	return out, nil
}
