package main

import (
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func newTestServer(t *testing.T) *Server {
	t.Helper()
	store, err := NewStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	return NewServer(store)
}

func do(t *testing.T, srv *Server, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	srv.Handler().ServeHTTP(rec, req)
	return rec
}

func TestPushGetListDelete(t *testing.T) {
	srv := newTestServer(t)
	push := `{"id":"s1","title":"T","repo":"/r","branch":"main","base":"HEAD",` +
		`"diff":"diff --git a/x b/x\n--- a/x\n+++ b/x\n@@ -0,0 +1 @@\n+hello\n","createdAt":"t0","updatedAt":"t0"}`
	if rec := do(t, srv, "POST", "/api/sessions", push); rec.Code != 200 {
		t.Fatalf("push code=%d body=%s", rec.Code, rec.Body)
	}

	rec := do(t, srv, "GET", "/api/sessions/s1", "")
	if rec.Code != 200 {
		t.Fatalf("get code=%d", rec.Code)
	}
	var sess Session
	json.Unmarshal(rec.Body.Bytes(), &sess)
	if sess.Status != "pending" || sess.Stats.Additions != 1 {
		t.Fatalf("session = %+v", sess)
	}

	if rec := do(t, srv, "GET", "/api/sessions", ""); rec.Code != 200 ||
		!strings.Contains(rec.Body.String(), `"id":"s1"`) {
		t.Fatalf("list code=%d body=%s", rec.Code, rec.Body)
	}

	if rec := do(t, srv, "DELETE", "/api/sessions/s1", ""); rec.Code != 200 {
		t.Fatalf("delete code=%d", rec.Code)
	}
	if rec := do(t, srv, "GET", "/api/sessions/s1", ""); rec.Code != 404 {
		t.Fatalf("expected 404 after delete, got %d", rec.Code)
	}
}

func TestPushBadBodyAndUnknownId(t *testing.T) {
	srv := newTestServer(t)
	if rec := do(t, srv, "POST", "/api/sessions", `{"title":"no id"}`); rec.Code != 400 {
		t.Fatalf("expected 400 for missing id, got %d", rec.Code)
	}
	if rec := do(t, srv, "GET", "/api/sessions/nope", ""); rec.Code != 404 {
		t.Fatalf("expected 404 unknown id, got %d", rec.Code)
	}
}

func TestSubmitAndPollReview(t *testing.T) {
	srv := newTestServer(t)
	do(t, srv, "POST", "/api/sessions",
		`{"id":"s2","title":"T","diff":"","updatedAt":"t0"}`)

	// Before submission: pending, null review.
	rec := do(t, srv, "GET", "/api/sessions/s2/review", "")
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), `"status":"pending"`) {
		t.Fatalf("pre-poll = %d %s", rec.Code, rec.Body)
	}

	review := `{"summary":"looks ok","submittedAt":"t1","comments":[` +
		`{"file":"x","side":"new","startLine":3,"endLine":5,"type":"request_change","body":"rename"}]}`
	if rec := do(t, srv, "POST", "/api/sessions/s2/review", review); rec.Code != 200 {
		t.Fatalf("submit code=%d body=%s", rec.Code, rec.Body)
	}

	rec = do(t, srv, "GET", "/api/sessions/s2/review", "")
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), `"status":"submitted"`) ||
		!strings.Contains(rec.Body.String(), `"body":"rename"`) {
		t.Fatalf("post-poll = %d %s", rec.Code, rec.Body)
	}

	if rec := do(t, srv, "POST", "/api/sessions/nope/review", review); rec.Code != 404 {
		t.Fatalf("expected 404 review on unknown id, got %d", rec.Code)
	}
}

func TestReviewWaitReturnsOnSubmit(t *testing.T) {
	srv := newTestServer(t)
	do(t, srv, "POST", "/api/sessions", `{"id":"w1","diff":"","updatedAt":"t0"}`)

	type res struct {
		code int
		body string
	}
	done := make(chan res, 1)
	go func() {
		rec := do(t, srv, "GET", "/api/sessions/w1/review?wait=5", "")
		done <- res{rec.Code, rec.Body.String()}
	}()

	time.Sleep(50 * time.Millisecond) // let the waiter block
	if rec := do(t, srv, "POST", "/api/sessions/w1/review",
		`{"summary":"ok","decision":"approve","comments":[],"submittedAt":"t1"}`); rec.Code != 200 {
		t.Fatalf("submit code=%d", rec.Code)
	}

	select {
	case r := <-done:
		if r.code != 200 || !strings.Contains(r.body, `"status":"submitted"`) {
			t.Fatalf("wait result = %d %s", r.code, r.body)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("wait did not return after submit")
	}
}

func TestReviewWaitReturnsImmediatelyWhenAlreadySubmitted(t *testing.T) {
	srv := newTestServer(t)
	do(t, srv, "POST", "/api/sessions", `{"id":"w2","diff":"","updatedAt":"t0"}`)
	do(t, srv, "POST", "/api/sessions/w2/review",
		`{"summary":"ok","decision":"approve","comments":[],"submittedAt":"t1"}`)

	start := time.Now()
	rec := do(t, srv, "GET", "/api/sessions/w2/review?wait=5", "")
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), `"status":"submitted"`) {
		t.Fatalf("result = %d %s", rec.Code, rec.Body)
	}
	if time.Since(start) > time.Second {
		t.Fatal("already-submitted wait should return immediately")
	}
}

func TestReviewWaitTimesOutToPending(t *testing.T) {
	srv := newTestServer(t)
	do(t, srv, "POST", "/api/sessions", `{"id":"w3","diff":"","updatedAt":"t0"}`)
	rec := do(t, srv, "GET", "/api/sessions/w3/review?wait=1", "")
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), `"status":"pending"`) {
		t.Fatalf("timeout result = %d %s", rec.Code, rec.Body)
	}
}

func TestStaticServing(t *testing.T) {
	srv := newTestServer(t)

	rec := do(t, srv, "GET", "/", "")
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), "review-board") {
		t.Fatalf("index: code=%d", rec.Code)
	}
	// SPA route also serves index.
	if rec := do(t, srv, "GET", "/s/anything", ""); rec.Code != 200 {
		t.Fatalf("spa route code=%d", rec.Code)
	}
	// JS modules must be served as JavaScript so the browser executes them.
	rec = do(t, srv, "GET", "/lib/anchor.mjs", "")
	if rec.Code != 200 || !strings.Contains(rec.Header().Get("Content-Type"), "javascript") {
		t.Fatalf("mjs: code=%d ct=%q", rec.Code, rec.Header().Get("Content-Type"))
	}
}

func TestSetStatus(t *testing.T) {
	srv := newTestServer(t)
	do(t, srv, "POST", "/api/sessions", `{"id":"st1","diff":"","updatedAt":"t0"}`)

	if rec := do(t, srv, "POST", "/api/sessions/st1/status", `{"status":"applying"}`); rec.Code != 200 {
		t.Fatalf("set applying code=%d body=%s", rec.Code, rec.Body)
	}
	rec := do(t, srv, "GET", "/api/sessions/st1", "")
	if !strings.Contains(rec.Body.String(), `"status":"applying"`) {
		t.Fatalf("status not applied: %s", rec.Body)
	}

	if rec := do(t, srv, "POST", "/api/sessions/st1/status", `{"status":"applied"}`); rec.Code != 200 {
		t.Fatalf("set applied code=%d", rec.Code)
	}

	// invalid status value is rejected
	if rec := do(t, srv, "POST", "/api/sessions/st1/status", `{"status":"bogus"}`); rec.Code != 400 {
		t.Fatalf("expected 400 for bogus status, got %d", rec.Code)
	}
	// unknown id is 404
	if rec := do(t, srv, "POST", "/api/sessions/nope/status", `{"status":"applying"}`); rec.Code != 404 {
		t.Fatalf("expected 404 for unknown id, got %d", rec.Code)
	}
}
