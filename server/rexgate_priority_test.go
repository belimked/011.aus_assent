package main

import (
	"testing"
	"time"
)

// Interactive search must not queue behind background slots, and must still
// consume one interval so the account-wide rate stays at one call per interval.
func TestRexGatePriority(t *testing.T) {
	g := &rexGate{Interval: time.Second}
	for range 5 {
		g.reserve() // poller/worker queue ~5s ahead
	}
	before := g.next
	c := &rexClient{Gate: g, Priority: true}
	start := time.Now()
	if err := c.wait(t.Context()); err != nil {
		t.Fatal(err)
	}
	if time.Since(start) > 100*time.Millisecond {
		t.Fatalf("priority call waited %v", time.Since(start))
	}
	if got := g.next.Sub(before); got != time.Second {
		t.Fatalf("schedule shifted by %v, want 1s", got)
	}
	if d, _ := g.reserve(); d < 5*time.Second {
		t.Fatalf("background slot moved earlier: %v", d)
	}
}
