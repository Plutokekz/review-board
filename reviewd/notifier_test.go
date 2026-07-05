package main

import (
	"testing"
	"time"
)

func TestNotifierSignalsSubscriber(t *testing.T) {
	n := newNotifier()
	ch := n.subscribe("a")
	defer n.unsubscribe("a", ch)
	n.publish("a")
	select {
	case <-ch:
	case <-time.After(time.Second):
		t.Fatal("subscriber was not signalled")
	}
}

func TestNotifierIsolatesIdsAndUnsubscribe(t *testing.T) {
	n := newNotifier()
	ch := n.subscribe("a")
	n.publish("b") // different id must not signal "a"
	select {
	case <-ch:
		t.Fatal("signalled by unrelated id")
	case <-time.After(50 * time.Millisecond):
	}
	n.unsubscribe("a", ch)
	n.publish("a") // must not panic and must be dropped (no subscribers)
}
