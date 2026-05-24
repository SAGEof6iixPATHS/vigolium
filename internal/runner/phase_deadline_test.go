package runner

import (
	"context"
	"errors"
	"testing"
	"time"
)

// These tests guard the per-phase max_duration enforcement. The original bug:
// the known-issue-scan phase computed the right budget but only applied it to
// one leg (Nuclei) while the Kingfisher leg ran unbounded, so the phase could
// overrun max_duration. The fix routes every phase deadline through
// phaseDeadline; these tests pin its behavior so the regression can't return.
//
// Durations are kept small but upper-bound tolerances are generous so the tests
// stay deterministic on slow/loaded CI without false failures.

// TestPhaseDeadline_AppliesBudget: a positive budget produces a real deadline
// roughly maxDuration out, and the context expires with DeadlineExceeded.
func TestPhaseDeadline_AppliesBudget(t *testing.T) {
	const budget = 60 * time.Millisecond
	start := time.Now()

	ctx, cancel := phaseDeadline(context.Background(), budget)
	defer cancel()

	dl, ok := ctx.Deadline()
	if !ok {
		t.Fatal("phaseDeadline(budget>0) must set a deadline")
	}
	if got := dl.Sub(start); got < budget/2 || got > budget+500*time.Millisecond {
		t.Fatalf("deadline %v is not ~%v from start", got, budget)
	}

	select {
	case <-ctx.Done():
		if !errors.Is(ctx.Err(), context.DeadlineExceeded) {
			t.Fatalf("ctx.Err() = %v, want DeadlineExceeded", ctx.Err())
		}
	case <-time.After(2 * time.Second):
		t.Fatal("phase ctx never expired; the budget was not enforced")
	}
}

// TestPhaseDeadline_NoBudget: maxDuration <= 0 means "unbounded phase" — the
// parent ctx is returned unchanged (no deadline) and the cancel is a safe no-op.
func TestPhaseDeadline_NoBudget(t *testing.T) {
	for _, d := range []time.Duration{0, -1 * time.Second} {
		parent := context.Background()
		ctx, cancel := phaseDeadline(parent, d)

		if _, ok := ctx.Deadline(); ok {
			t.Fatalf("d=%v: expected no deadline on an unbounded phase", d)
		}
		if ctx != parent {
			t.Fatalf("d=%v: expected the parent ctx returned unchanged", d)
		}
		cancel() // must not panic, and must not cancel the parent
		if parent.Err() != nil {
			t.Fatalf("d=%v: no-op cancel must not affect the parent ctx", d)
		}
	}
}

// TestPhaseDeadline_CapsToParentDeadline: a phase budget larger than the time
// remaining on the parent (e.g. an overall scan deadline) must NOT extend it —
// a phase can never run past the scan. This is the property that lets each phase
// resolve its budget independently without overrunning a tighter outer bound.
func TestPhaseDeadline_CapsToParentDeadline(t *testing.T) {
	const parentBudget = 40 * time.Millisecond

	parent, cancelParent := context.WithTimeout(context.Background(), parentBudget)
	defer cancelParent()

	// Phase asks for far more than the parent has left.
	ctx, cancel := phaseDeadline(parent, 10*time.Second)
	defer cancel()

	dl, ok := ctx.Deadline()
	if !ok {
		t.Fatal("expected a deadline inherited from the parent")
	}
	if remaining := time.Until(dl); remaining > time.Second {
		t.Fatalf("phase deadline %v exceeded the parent's ~%v budget", remaining, parentBudget)
	}

	select {
	case <-ctx.Done():
		// Expired on the parent's schedule, as required.
	case <-time.After(2 * time.Second):
		t.Fatal("phase ctx outlived the parent deadline")
	}
}

// TestPhaseDeadline_BoundsSequentialLegs directly encodes the known-issue-scan
// regression: a phase runs two legs sequentially (Nuclei then Kingfisher) under
// ONE shared deadline. The bug was that the second leg ran unbounded. Here the
// first leg consumes the whole budget; a second leg that respects ctx must then
// observe the deadline and return immediately instead of running unbounded.
func TestPhaseDeadline_BoundsSequentialLegs(t *testing.T) {
	const budget = 60 * time.Millisecond

	ctx, cancel := phaseDeadline(context.Background(), budget)
	defer cancel()

	// Leg 1 (e.g. the Nuclei scan) runs until the phase budget is exhausted.
	leg1 := runUntilCtxDone(ctx)
	if !errors.Is(leg1, context.DeadlineExceeded) {
		t.Fatalf("leg 1 ended with %v, want DeadlineExceeded", leg1)
	}

	// Leg 2 (e.g. the Kingfisher batch) shares the same ctx. A ctx-respecting
	// leg must return effectively instantly rather than starting fresh work.
	start := time.Now()
	leg2 := runUntilCtxDone(ctx)
	elapsed := time.Since(start)

	if leg2 == nil {
		t.Fatal("leg 2 ran without observing the shared phase deadline (the original bug)")
	}
	if elapsed > 25*time.Millisecond {
		t.Fatalf("leg 2 ran %v past the exhausted phase deadline; expected immediate return", elapsed)
	}
}

// runUntilCtxDone models a phase leg that does work while honoring ctx: it
// returns ctx.Err() when the phase deadline fires, or nil if it would have
// finished its (here, long) work first.
func runUntilCtxDone(ctx context.Context) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(10 * time.Second):
		return nil
	}
}
