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

	"github.com/stefannovasky/llm-chat/internal/storage"
)

const (
	currentVersion = 1
	titleMaxRunes  = 60
)

type Role string

const (
	RoleSystem    Role = "system"
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
)

type Message struct {
	Role             Role       `json:"role"`
	Content          string     `json:"content"`
	Model            string     `json:"model,omitempty"`
	PromptTokens     int        `json:"prompt_tokens,omitempty"`
	CompletionTokens int        `json:"completion_tokens,omitempty"`
	Cost             float64    `json:"cost,omitempty"`
	CompactedAt      *time.Time `json:"compacted_at,omitempty"`
}

type Conversation struct {
	Messages []Message
}

type Session struct {
	Version        int       `json:"version"`
	ID             string    `json:"id"`
	Title          string    `json:"title"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
	LastAccessedAt time.Time `json:"last_accessed_at,omitempty"`
	Messages       []Message `json:"messages"`
}

type Summary struct {
	ID             string
	Title          string
	UpdatedAt      time.Time
	LastAccessedAt time.Time
	Cost           float64
	Messages       int
}

func Dir() string {
	return storage.XDGPath("XDG_DATA_HOME", ".local/share", "llm-chat", "sessions")
}

func NewID() string {
	now := time.Now().UTC()
	var b [4]byte
	if _, err := rand.Read(b[:]); err != nil {
		return now.Format("20060102T150405.000000000Z")
	}
	return now.Format("20060102T150405Z") + "-" + hex.EncodeToString(b[:])
}

// DeriveTitle returns a single-line title derived from the first user message.
// Returns "" when no user message exists yet.
func DeriveTitle(msgs []Message) string {
	for _, m := range msgs {
		if m.Role != RoleUser {
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

// ContextUsed estimates the tokens currently in the active context window.
func ContextUsed(c Conversation) int {
	lastAsstIdx := -1
	for i := len(c.Messages) - 1; i >= 0; i-- {
		m := c.Messages[i]
		if m.CompactedAt != nil || m.Role == RoleSystem {
			continue
		}
		if m.Role == RoleAssistant {
			lastAsstIdx = i
			break
		}
	}
	if lastAsstIdx < 0 {
		return 0
	}

	last := c.Messages[lastAsstIdx]

	// Default to CompletionTokens (fresh compact summary: PromptTokens is
	// inflated). Upgrade to Prompt+Completion if a real user turn precedes it.
	used := last.CompletionTokens
	for i := lastAsstIdx - 1; i >= 0; i-- {
		m := c.Messages[i]
		if m.CompactedAt != nil || m.Role == RoleSystem {
			continue
		}
		if m.Role == RoleUser {
			used = last.PromptTokens + last.CompletionTokens
			break
		}
	}

	if used > 0 {
		return used
	}

	for i := len(c.Messages) - 1; i >= 0; i-- {
		m := c.Messages[i]
		if m.CompactedAt != nil || m.Role != RoleAssistant {
			continue
		}
		if v := m.PromptTokens + m.CompletionTokens; v > 0 {
			return v
		}
	}
	return 0
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
	return storage.WriteJSONAtomic(path, s, true)
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

// Touch stamps LastAccessedAt on s and persists it without modifying UpdatedAt.
// Errors are silently ignored — this is metadata only.
func Touch(s *Session) {
	s.LastAccessedAt = time.Now().UTC()
	path, err := filePath(s.ID)
	if err != nil {
		return
	}
	_ = storage.WriteJSONAtomic(path, s, true)
}

// List returns session summaries sorted by LastAccessedAt descending, falling
// back to UpdatedAt for sessions that pre-date the field. Unreadable or
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
		effectiveTime := s.LastAccessedAt
		if effectiveTime.IsZero() {
			effectiveTime = s.UpdatedAt
		}
		sum := Summary{
			ID:             s.ID,
			Title:          s.Title,
			UpdatedAt:      s.UpdatedAt,
			LastAccessedAt: effectiveTime,
			Messages:       len(s.Messages),
		}
		for _, m := range s.Messages {
			sum.Cost += m.Cost
		}
		out = append(out, sum)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].LastAccessedAt.After(out[j].LastAccessedAt)
	})
	return out, nil
}
