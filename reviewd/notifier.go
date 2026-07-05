package main

import "sync"

// notifier lets HTTP handlers block until a specific session id is signalled.
// The long-poll review endpoint uses it to wake the instant a review is
// submitted. Signals are edge-triggered and coalesced (buffered depth 1), so a
// waiter that is not currently blocked may miss one — callers therefore
// subscribe BEFORE reading state and re-check state after waking.
type notifier struct {
	mu   sync.Mutex
	subs map[string]map[chan struct{}]struct{}
}

func newNotifier() *notifier {
	return &notifier{subs: map[string]map[chan struct{}]struct{}{}}
}

func (n *notifier) subscribe(id string) chan struct{} {
	ch := make(chan struct{}, 1)
	n.mu.Lock()
	defer n.mu.Unlock()
	m := n.subs[id]
	if m == nil {
		m = map[chan struct{}]struct{}{}
		n.subs[id] = m
	}
	m[ch] = struct{}{}
	return ch
}

func (n *notifier) unsubscribe(id string, ch chan struct{}) {
	n.mu.Lock()
	defer n.mu.Unlock()
	if m := n.subs[id]; m != nil {
		delete(m, ch)
		if len(m) == 0 {
			delete(n.subs, id)
		}
	}
}

func (n *notifier) publish(id string) {
	n.mu.Lock()
	defer n.mu.Unlock()
	for ch := range n.subs[id] {
		select {
		case ch <- struct{}{}:
		default: // already has a pending signal; coalesce
		}
	}
}
