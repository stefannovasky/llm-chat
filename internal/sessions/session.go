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
	"github.com/stefannovasky/llm-chat/internal/fsutil"
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
	return fsutil.XDGPath("XDG_DATA_HOME", ".local/share", "llm-chat", "sessions")
}

func NewID() string {
	now := time.Now().UTC()
	var b [4]byte
	if _, err := rand.Read(b[:]); err != nil {
		// crypto/rand virtually never fails, but fall back to a time-based
		// suffix so IDs remain unique within a second.
		return now.Format("20060102T150405.000000000Z")
	}
	return now.Format("20060102T150405Z") + "-" + hex.EncodeToString(b[:])
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

// Save marshals s to disk atomically and stamps UpdatedAt. Caller owns
// CreatedAt — Save does not read the existing file.
func Save(s *Session) error {
	if s.Version == 0 {
		s.Version = currentVersion
	}
	s.UpdatedAt = time.Now().UTC()
	path, err := filePath(s.ID)
	if err != nil {
		return err
	}
	return fsutil.WriteJSONAtomic(path, s, true)
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
