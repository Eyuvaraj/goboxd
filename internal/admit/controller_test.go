package admit_test

import (
	"context"
	"errors"
	"runtime"
	"testing"

	"github.com/thesouldev/goboxd/internal/admit"
)

// waitFor spins until cond holds, yielding the scheduler. Deterministic and
// sleep-free: every state change the loop makes is published to an atomic, so
// the condition becomes true as soon as the loop has processed the event.
func waitFor(t *testing.T, cond func() bool) {
	t.Helper()
	for range 1_000_000 {
		if cond() {
			return
		}
		runtime.Gosched()
	}
	t.Fatal("condition not met")
}

func TestAcquire_GrantsWithinBudget(t *testing.T) {
	c := admit.New(admit.Config{CPU: 2, MemKB: 1000})
	defer c.Close()

	rel1, err := c.Acquire(context.Background(), admit.Resources{CPU: 1, MemKB: 400})
	if err != nil {
		t.Fatalf("first acquire: %v", err)
	}
	rel2, err := c.Acquire(context.Background(), admit.Resources{CPU: 1, MemKB: 400})
	if err != nil {
		t.Fatalf("second acquire: %v", err)
	}
	if got := c.InFlight(); got != 2 {
		t.Fatalf("InFlight: want 2, got %d", got)
	}
	rel1()
	rel2()
	waitFor(t, func() bool { return c.InFlight() == 0 })
}

func TestAcquire_BlocksOnCPUThenGrantsOnRelease(t *testing.T) {
	c := admit.New(admit.Config{CPU: 1, MemKB: 0}) // memory dimension disabled
	defer c.Close()

	rel1, err := c.Acquire(context.Background(), admit.Resources{CPU: 1})
	if err != nil {
		t.Fatalf("first acquire: %v", err)
	}

	granted := make(chan struct{})
	go func() {
		rel2, _ := c.Acquire(context.Background(), admit.Resources{CPU: 1})
		close(granted)
		rel2()
	}()

	waitFor(t, func() bool { return c.Queued() == 1 }) // second is parked
	rel1()                                             // frees the only CPU permit
	<-granted                                          // second now proceeds
}

func TestAcquire_BlocksOnMemoryEvenWhenCPUFree(t *testing.T) {
	c := admit.New(admit.Config{CPU: 4, MemKB: 1000})
	defer c.Close()

	rel1, err := c.Acquire(context.Background(), admit.Resources{CPU: 1, MemKB: 800})
	if err != nil {
		t.Fatalf("first acquire: %v", err)
	}

	// CPU is free (3 permits left) but only 200 KiB remain; this must park.
	parked := make(chan func(), 1)
	go func() {
		rel2, _ := c.Acquire(context.Background(), admit.Resources{CPU: 1, MemKB: 500})
		parked <- rel2
	}()

	waitFor(t, func() bool { return c.Queued() == 1 })
	rel1() // releases 800 KiB
	rel2 := <-parked
	rel2()
}

func TestAcquire_ShedsWhenQueueFull(t *testing.T) {
	c := admit.New(admit.Config{CPU: 0, MaxQueueDepth: 1}) // CPU 0 => nothing ever runs
	defer c.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() { _, _ = c.Acquire(ctx, admit.Resources{CPU: 1}) }() // becomes the one allowed waiter

	waitFor(t, func() bool { return c.Queued() == 1 })

	_, err := c.Acquire(context.Background(), admit.Resources{CPU: 1})
	if !errors.Is(err, admit.ErrOverloaded) {
		t.Fatalf("want ErrOverloaded, got %v", err)
	}
}

func TestAcquire_UnboundedQueueDoesNotShed(t *testing.T) {
	c := admit.New(admit.Config{CPU: 0, MaxQueueDepth: 0}) // unbounded
	defer c.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // already cancelled
	_, err := c.Acquire(ctx, admit.Resources{CPU: 1})
	if errors.Is(err, admit.ErrOverloaded) {
		t.Fatal("unbounded queue must not shed")
	}
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("want context.Canceled, got %v", err)
	}
}

func TestAcquire_CancelWhileQueuedReleasesSlot(t *testing.T) {
	c := admit.New(admit.Config{CPU: 1, MemKB: 0})
	defer c.Close()

	rel1, err := c.Acquire(context.Background(), admit.Resources{CPU: 1})
	if err != nil {
		t.Fatalf("first acquire: %v", err)
	}

	ctx, cancel := context.WithCancel(t.Context())
	done := make(chan error, 1)
	go func() {
		_, err := c.Acquire(ctx, admit.Resources{CPU: 1})
		done <- err
	}()

	waitFor(t, func() bool { return c.Queued() == 1 })
	cancel()
	if err := <-done; !errors.Is(err, context.Canceled) {
		t.Fatalf("want context.Canceled, got %v", err)
	}
	waitFor(t, func() bool { return c.Queued() == 0 })

	rel1() // the cancelled waiter must not have consumed the freed slot
	// a fresh acquire should now succeed immediately
	rel2, err := c.Acquire(context.Background(), admit.Resources{CPU: 1})
	if err != nil {
		t.Fatalf("acquire after cancel: %v", err)
	}
	rel2()
}

func TestAcquire_OversizedReservationRunsAlone(t *testing.T) {
	c := admit.New(admit.Config{CPU: 2, MemKB: 1000})
	defer c.Close()

	// Needs more memory than the whole budget; clamped to the budget so it can
	// still run, but only when the system is otherwise idle.
	rel, err := c.Acquire(context.Background(), admit.Resources{CPU: 1, MemKB: 5000})
	if err != nil {
		t.Fatalf("oversized acquire: %v", err)
	}
	rel()
}
