package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
)

type Comment struct {
	File      string `json:"file"`
	Side      string `json:"side"`
	StartLine int    `json:"startLine"`
	EndLine   int    `json:"endLine"`
	Type      string `json:"type"`
	Body      string `json:"body"`
}

type Review struct {
	Summary     string    `json:"summary"`
	Comments    []Comment `json:"comments"`
	SubmittedAt string    `json:"submittedAt"`
}

type Session struct {
	ID        string  `json:"id"`
	Title     string  `json:"title"`
	Repo      string  `json:"repo"`
	Branch    string  `json:"branch"`
	Base      string  `json:"base"`
	Diff      string  `json:"diff"`
	Status    string  `json:"status"`
	Stats     Stats   `json:"stats"`
	CreatedAt string  `json:"createdAt"`
	UpdatedAt string  `json:"updatedAt"`
	Review    *Review `json:"review"`
}

type Store struct {
	mu  sync.Mutex
	dir string
	m   map[string]*Session
}

func safeName(id string) string {
	var b strings.Builder
	for _, r := range id {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z',
			r >= '0' && r <= '9', r == '-', r == '_', r == '.':
			b.WriteRune(r)
		default:
			b.WriteRune('_')
		}
	}
	return b.String()
}

func NewStore(dir string) (*Store, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	s := &Store{dir: dir, m: map[string]*Session{}}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		b, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			return nil, err
		}
		var sess Session
		if json.Unmarshal(b, &sess) == nil && sess.ID != "" {
			s.m[sess.ID] = &sess
		}
	}
	return s, nil
}

func (s *Store) Put(sess *Session) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.m[sess.ID] = sess
	b, err := json.MarshalIndent(sess, "", "  ")
	if err != nil {
		return err
	}
	final := filepath.Join(s.dir, safeName(sess.ID)+".json")
	tmp := final + ".tmp"
	if err := os.WriteFile(tmp, b, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, final)
}

func (s *Store) Get(id string) (*Session, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	sess, ok := s.m[id]
	return sess, ok
}

func (s *Store) List() []*Session {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]*Session, 0, len(s.m))
	for _, v := range s.m {
		out = append(out, v)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].UpdatedAt > out[j].UpdatedAt })
	return out
}

func (s *Store) Delete(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.m, id)
	err := os.Remove(filepath.Join(s.dir, safeName(id)+".json"))
	if os.IsNotExist(err) {
		return nil
	}
	return err
}
