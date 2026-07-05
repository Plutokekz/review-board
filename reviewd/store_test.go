package main

import "testing"

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
