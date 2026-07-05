package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
)

func defaultStateDir() string {
	if x := os.Getenv("XDG_STATE_HOME"); x != "" {
		return filepath.Join(x, "review-board", "sessions")
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".local", "state", "review-board", "sessions")
}

func main() {
	port := flag.Int("port", 7654, "port to listen on")
	stateDir := flag.String("state-dir", defaultStateDir(), "session state directory")
	flag.Parse()

	store, err := NewStore(*stateDir)
	if err != nil {
		log.Fatalf("store: %v", err)
	}
	addr := fmt.Sprintf("127.0.0.1:%d", *port)
	log.Printf("review-board listening on http://%s (state: %s)", addr, *stateDir)
	log.Fatal(http.ListenAndServe(addr, NewServer(store).Handler()))
}
