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

func (s *Store) path(id string) string {
	return filepath.Join(s.dir, safeName(id)+".json")
}

// cloneSession returns a copy of s that shares no mutable state with it: the
// Session value itself is copied, and if Review is set it (and its Comments
// slice) are deep-copied too. Comment is a pure value struct, so copying the
// slice fully isolates it. This is used by Put/Get/List so that callers can
// never mutate a Review reachable from the store's internal map, or from a
// Session returned by a previous call.
func cloneSession(s *Session) *Session {
	cp := *s
	if s.Review != nil {
		r := *s.Review
		r.Comments = append([]Comment(nil), s.Review.Comments...)
		cp.Review = &r
	}
	return &cp
}

// Put persists sess to disk and then stores a copy of it in memory. The
// file write (via a temp file + rename) happens before the in-memory map
// is touched, so a failure leaves the store's visible state unchanged.
func (s *Store) Put(sess *Session) error {
	b, err := json.MarshalIndent(sess, "", "  ")
	if err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	final := s.path(sess.ID)
	tmp := final + ".tmp"
	if err := os.WriteFile(tmp, b, 0o644); err != nil {
		return err
	}
	if err := os.Rename(tmp, final); err != nil {
		os.Remove(tmp) // best-effort cleanup of the orphaned temp file
		return err
	}

	s.m[sess.ID] = cloneSession(sess)
	return nil
}

func (s *Store) Get(id string) (*Session, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	sess, ok := s.m[id]
	if !ok {
		return nil, false
	}
	return cloneSession(sess), true
}

func (s *Store) List() []*Session {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]*Session, 0, len(s.m))
	for _, v := range s.m {
		out = append(out, cloneSession(v))
	}
	sort.Slice(out, func(i, j int) bool { return out[i].UpdatedAt > out[j].UpdatedAt })
	return out
}

// Delete removes the persisted file before mutating memory. A missing file
// is treated as success; a real removal error leaves the in-memory map
// unchanged.
func (s *Store) Delete(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := os.Remove(s.path(id)); err != nil && !os.IsNotExist(err) {
		return err
	}
	delete(s.m, id)
	return nil
}
