package main

import (
	"embed"
	"encoding/json"
	"mime"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

//go:embed all:web
var webFS embed.FS

type Server struct {
	store *Store
	notif *notifier
}

func NewServer(store *Store) *Server { return &Server{store: store, notif: newNotifier()} }

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/sessions", s.handlePush)
	mux.HandleFunc("GET /api/sessions", s.handleList)
	mux.HandleFunc("GET /api/sessions/{id}", s.handleGet)
	mux.HandleFunc("DELETE /api/sessions/{id}", s.handleDelete)
	mux.HandleFunc("POST /api/sessions/{id}/review", s.handleSubmitReview)
	mux.HandleFunc("GET /api/sessions/{id}/review", s.handleGetReview)
	mux.HandleFunc("GET /s/{id}", s.handleStatic)
	mux.HandleFunc("GET /", s.handleStatic)
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

func (s *Server) handleSubmitReview(w http.ResponseWriter, r *http.Request) {
	sess, ok := s.store.Get(r.PathValue("id"))
	if !ok {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	var rev Review
	if err := json.NewDecoder(r.Body).Decode(&rev); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	sess.Review = &rev
	sess.Status = "submitted"
	if err := s.store.Put(sess); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	s.notif.publish(sess.ID)
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) handleGetReview(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if secs := parseWait(r.URL.Query().Get("wait")); secs > 0 {
		ch := s.notif.subscribe(id)
		defer s.notif.unsubscribe(id, ch)
		// Subscribe-before-read closes the submit-during-setup race.
		if sess, ok := s.store.Get(id); ok && sess.Status != "submitted" {
			select {
			case <-ch:
			case <-time.After(time.Duration(secs) * time.Second):
			case <-r.Context().Done():
				return
			}
		}
	}
	sess, ok := s.store.Get(id)
	if !ok {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"status": sess.Status, "review": sess.Review})
}

// parseWait clamps the ?wait= seconds to [0,600]; 0 (or invalid) means do not block.
func parseWait(s string) int {
	n, err := strconv.Atoi(s)
	if err != nil || n <= 0 {
		return 0
	}
	if n > 600 {
		n = 600
	}
	return n
}

func (s *Server) handleStatic(w http.ResponseWriter, r *http.Request) {
	p := r.URL.Path
	if p == "/" || strings.HasPrefix(p, "/s/") {
		p = "/index.html"
	}
	data, err := webFS.ReadFile("web" + p)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	ct := mime.TypeByExtension(filepath.Ext(p))
	if strings.HasSuffix(p, ".mjs") || strings.HasSuffix(p, ".js") {
		ct = "text/javascript; charset=utf-8"
	}
	if ct != "" {
		w.Header().Set("Content-Type", ct)
	}
	_, _ = w.Write(data)
}
