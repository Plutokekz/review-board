package main

import (
	"encoding/json"
	"net/http"
)

type Server struct {
	store *Store
}

func NewServer(store *Store) *Server { return &Server{store: store} }

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/sessions", s.handlePush)
	mux.HandleFunc("GET /api/sessions", s.handleList)
	mux.HandleFunc("GET /api/sessions/{id}", s.handleGet)
	mux.HandleFunc("DELETE /api/sessions/{id}", s.handleDelete)
	return mux
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

type pushReq struct {
	ID        string `json:"id"`
	Title     string `json:"title"`
	Repo      string `json:"repo"`
	Branch    string `json:"branch"`
	Base      string `json:"base"`
	Diff      string `json:"diff"`
	CreatedAt string `json:"createdAt"`
	UpdatedAt string `json:"updatedAt"`
}

func (s *Server) handlePush(w http.ResponseWriter, r *http.Request) {
	var req pushReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.ID == "" {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	sess := &Session{
		ID: req.ID, Title: req.Title, Repo: req.Repo, Branch: req.Branch,
		Base: req.Base, Diff: req.Diff, Status: "pending",
		Stats: DiffStats(req.Diff), CreatedAt: req.CreatedAt, UpdatedAt: req.UpdatedAt,
		Review: nil,
	}
	if err := s.store.Put(sess); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"id": sess.ID, "url": "/s/" + sess.ID})
}

func (s *Server) handleList(w http.ResponseWriter, r *http.Request) {
	type item struct {
		ID        string `json:"id"`
		Title     string `json:"title"`
		Repo      string `json:"repo"`
		Branch    string `json:"branch"`
		Status    string `json:"status"`
		Stats     Stats  `json:"stats"`
		UpdatedAt string `json:"updatedAt"`
	}
	out := []item{}
	for _, se := range s.store.List() {
		out = append(out, item{se.ID, se.Title, se.Repo, se.Branch, se.Status, se.Stats, se.UpdatedAt})
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) handleGet(w http.ResponseWriter, r *http.Request) {
	sess, ok := s.store.Get(r.PathValue("id"))
	if !ok {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	writeJSON(w, http.StatusOK, sess)
}

func (s *Server) handleDelete(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.store.Get(r.PathValue("id")); !ok {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	if err := s.store.Delete(r.PathValue("id")); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}
