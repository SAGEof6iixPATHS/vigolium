package knownissuescan

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"
)

// runScanWithDeadline is the wall-clock-cutoff seam used by Run to bound nuclei,
// whose cancellation is cooperative (it blocks draining in-flight work even after
// the context is cancelled). These tests exercise the seam directly — no nuclei,
// no DB, no templates.

// TestRunScanWithDeadline_ReturnsAtDeadline_WhenScanBlocks is the core regression
// guard: even when scan() blocks forever, runScanWithDeadline must return at the
// context deadline rather than waiting for the scan to drain.
func TestRunScanWithDeadline_ReturnsAtDeadline_WhenScanBlocks(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	var completeCalled, abandonCalled atomic.Bool

	start := time.Now()
	err := runScanWithDeadline(ctx,
		func() error { select {} }, // blocks forever
		func(error) { completeCalled.Store(true) },
		func() { abandonCalled.Store(true) },
	)
	elapsed := time.Since(start)

	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected context.DeadlineExceeded, got %v", err)
	}
	if elapsed > 250*time.Millisecond {
		t.Fatalf("runScanWithDeadline blocked %s past deadline; expected prompt return", elapsed)
	}
	if completeCalled.Load() {
		t.Fatal("onComplete must not run on the deadline path")
	}
	// The scan still blocks, so the detached cleanup has not run yet.
	if abandonCalled.Load() {
		t.Fatal("onAbandon ran before the scan drained")
	}
}

// TestRunScanWithDeadline_CleanupRunsAfterDrain verifies the detached cleanup runs
// exactly once, and only after the abandoned scan goroutine finally returns.
func TestRunScanWithDeadline_CleanupRunsAfterDrain(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	release := make(chan struct{})
	abandoned := make(chan struct{})
	var completeCalled atomic.Bool
	var abandonCount atomic.Int64

	err := runScanWithDeadline(ctx,
		func() error { <-release; return nil }, // blocks until released by the test
		func(error) { completeCalled.Store(true) },
		func() {
			abandonCount.Add(1)
			close(abandoned)
		},
	)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected context.DeadlineExceeded, got %v", err)
	}

	// Cleanup must not have run while the scan is still blocked.
	select {
	case <-abandoned:
		t.Fatal("onAbandon ran before the scan drained")
	case <-time.After(50 * time.Millisecond):
	}

	// Release the scan; cleanup should now fire.
	close(release)
	select {
	case <-abandoned:
	case <-time.After(2 * time.Second):
		t.Fatal("onAbandon did not run after the scan drained")
	}
	if got := abandonCount.Load(); got != 1 {
		t.Fatalf("onAbandon ran %d times, want 1", got)
	}
	if completeCalled.Load() {
		t.Fatal("onComplete must not run on the deadline path")
	}
}

// TestRunScanWithDeadline_CompletesWithinBudget verifies the normal path: the scan
// finishes before the deadline, onComplete runs inline with the scan's error, and
// onAbandon never runs.
func TestRunScanWithDeadline_CompletesWithinBudget(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	sentinel := errors.New("scan boom")
	var gotErr error
	var completeCount, abandonCount atomic.Int64

	err := runScanWithDeadline(ctx,
		func() error { return sentinel },
		func(e error) { gotErr = e; completeCount.Add(1) },
		func() { abandonCount.Add(1) },
	)
	if err != nil {
		t.Fatalf("completion path must return nil, got %v", err)
	}
	if !errors.Is(gotErr, sentinel) {
		t.Fatalf("onComplete got %v, want sentinel", gotErr)
	}
	if got := completeCount.Load(); got != 1 {
		t.Fatalf("onComplete ran %d times, want 1", got)
	}
	// Give any erroneous detached cleanup a chance to run before asserting.
	time.Sleep(20 * time.Millisecond)
	if got := abandonCount.Load(); got != 0 {
		t.Fatalf("onAbandon ran %d times, want 0", got)
	}
}

// TestRunScanWithDeadline_ParentCancel verifies a parent cancel (not just a
// timeout) returns promptly via the same deadline path.
func TestRunScanWithDeadline_ParentCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	var completeCalled atomic.Bool
	done := make(chan error, 1)
	go func() {
		done <- runScanWithDeadline(ctx,
			func() error { select {} }, // blocks forever
			func(error) { completeCalled.Store(true) },
			func() {},
		)
	}()

	cancel()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("expected context.Canceled, got %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("runScanWithDeadline did not return promptly after parent cancel")
	}
	if completeCalled.Load() {
		t.Fatal("onComplete must not run on the parent-cancel path")
	}
}
