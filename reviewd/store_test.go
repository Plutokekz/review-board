package main

import (
	"sync"
	"testing"
)

func TestStorePutGetPersist(t *testing.T) {
	dir := t.TempDir()
	s, err := NewStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	sess := &Session{ID: "s1", Title: "T", Status: "pending",
		Stats: Stats{Files: 1, Additions: 2}, UpdatedAt: "2026-07-05T10:00:00Z"}
	if err := s.Put(sess); err != nil {
		t.Fatal(err)
	}
	if got, ok := s.Get("s1"); !ok || got.Title != "T" {
		t.Fatalf("Get = %+v, ok=%v", got, ok)
	}

	// A fresh store over the same dir must reload the session from disk.
	s2, err := NewStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	got, ok := s2.Get("s1")
	if !ok || got.Stats.Additions != 2 {
		t.Fatalf("reload Get = %+v, ok=%v", got, ok)
	}
}

func TestStoreListOrderAndDelete(t *testing.T) {
	s, _ := NewStore(t.TempDir())
	s.Put(&Session{ID: "a", UpdatedAt: "2026-07-05T09:00:00Z"})
	s.Put(&Session{ID: "b", UpdatedAt: "2026-07-05T11:00:00Z"})
	list := s.List()
	if len(list) != 2 || list[0].ID != "b" {
		t.Fatalf("List order wrong: %+v", list)
	}
	if err := s.Delete("a"); err != nil {
		t.Fatal(err)
	}
	if _, ok := s.Get("a"); ok {
		t.Fatal("expected a deleted")
	}
}

// TestStoreConcurrentPutGetList guards against pointer aliasing between the
// store's internal map and values returned to (or passed in by) callers. It
// hammers Put/Get/List concurrently on the same session id from many
// goroutines; run with -race to confirm no data race and no panic.
func TestStoreConcurrentPutGetList(t *testing.T) {
	s, err := NewStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}

	const id = "concurrent"
	const iterations = 50

	var wg sync.WaitGroup
	for i := 0; i < iterations; i++ {
		wg.Add(3)

		go func(n int) {
			defer wg.Done()
			sess := &Session{
				ID:        id,
				Title:     "T",
				UpdatedAt: "2026-07-05T10:00:00Z",
				Review:    &Review{Summary: "s"},
			}
			if err := s.Put(sess); err != nil {
				t.Errorf("Put: %v", err)
			}
			// Mutate the caller's copy after handing it to Put; this must
			// never be visible to (or race with) the store's own state.
			sess.Title = "mutated"
		}(i)

		go func() {
			defer wg.Done()
			if got, ok := s.Get(id); ok {
				// Mutate the returned session; must not race with Put's
				// internal marshal/copy or other Get/List calls.
				got.Title = "also mutated"
				got.Status = "changed"
			}
		}()

		go func() {
			defer wg.Done()
			for _, sess := range s.List() {
				sess.Title = "list mutated"
			}
		}()
	}

	wg.Wait()

	if _, ok := s.Get(id); !ok {
		t.Fatal("expected session to still exist after concurrent access")
	}
}
