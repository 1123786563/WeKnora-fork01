package container

import (
	"context"
	"errors"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// fakeSweeper counts sweep passes and can hold one pass mid-flight.
type fakeSweeper struct {
	mu       sync.Mutex
	passes   int
	err      error
	holdOpen bool
	held     chan struct{}
	release  chan struct{}
}

func (f *fakeSweeper) Sweep(_ context.Context, _ int) error {
	return f.run()
}

func (f *fakeSweeper) run() error {
	f.mu.Lock()
	f.passes++
	hold := f.holdOpen
	f.mu.Unlock()
	if hold && f.held != nil {
		f.held <- struct{}{}
		<-f.release
	}
	return f.err
}

func (f *fakeSweeper) count() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.passes
}

// The periodic sweep runs one pass per interval, propagates pass errors
// without dying, and stops cleanly at shutdown.
func TestStartPeriodicSweepRunsStopsAndSurvivesErrors(t *testing.T) {
	cleaner := NewResourceCleaner()
	fake := &fakeSweeper{err: errors.New("provider hiccup")}

	StartPeriodicSweep(cleaner, "TestSweep", 5*time.Millisecond, func(context.Context) error { return fake.run() })
	require.Eventually(t, func() bool { return fake.count() >= 2 },
		2*time.Second, 2*time.Millisecond, "sweep never ran periodically")

	errs := cleaner.Cleanup(context.Background())
	require.Empty(t, errs)
	count := fake.count()
	time.Sleep(30 * time.Millisecond)
	require.Equal(t, count, fake.count(), "sweep kept running after shutdown")
}

// A slow pass never overlaps itself: ticks that arrive while a pass holds the
// loop are skipped, and the pass count only moves while one runs.
func TestStartPeriodicSweepNeverOverlapsPasses(t *testing.T) {
	cleaner := NewResourceCleaner()
	fake := &fakeSweeper{holdOpen: true, held: make(chan struct{}, 1), release: make(chan struct{})}

	StartPeriodicSweep(cleaner, "HeldSweep", 5*time.Millisecond, func(context.Context) error { return fake.run() })

	select {
	case <-fake.held:
	case <-time.After(2 * time.Second):
		t.Fatal("first sweep never started")
	}
	// The first pass is still holding the loop: at most one pass may run.
	require.Equal(t, 1, fake.count())

	close(fake.release)
	require.Empty(t, cleaner.Cleanup(context.Background()))
}

// The craft sweep wiring honors its environment switches: disabled starts
// nothing, defaults land on the suggested values, and a malformed value
// falls back instead of failing the boot.
func TestStartCraftLifecycleSweepEnvSwitches(t *testing.T) {
	t.Run("disabled", func(t *testing.T) {
		t.Setenv("CRAFT_LIFECYCLE_SWEEP_DISABLED", "true")
		cleaner := NewResourceCleaner()
		fake := &fakeSweeper{}
		StartCraftLifecycleSweep(fake, cleaner)
		time.Sleep(20 * time.Millisecond)
		require.Equal(t, 0, fake.count(), "sweep started although disabled")
		require.Empty(t, cleaner.Cleanup(context.Background()))
	})

	t.Run("nil service starts nothing", func(t *testing.T) {
		require.NotPanics(t, func() { StartCraftLifecycleSweep(nil, NewResourceCleaner()) })
	})

	t.Run("interval and batch parsing", func(t *testing.T) {
		require.Equal(t, craftLifecycleDefaultInterval, craftLifecycleSweepInterval())
		t.Setenv("CRAFT_LIFECYCLE_SWEEP_INTERVAL", "250ms")
		require.Equal(t, 250*time.Millisecond, craftLifecycleSweepInterval())
		t.Setenv("CRAFT_LIFECYCLE_SWEEP_INTERVAL", "not-a-duration")
		require.Equal(t, craftLifecycleDefaultInterval, craftLifecycleSweepInterval())

		require.Equal(t, 100, craftLifecycleSweepBatch())
		t.Setenv("CRAFT_LIFECYCLE_SWEEP_BATCH", "7")
		require.Equal(t, 7, craftLifecycleSweepBatch())
		t.Setenv("CRAFT_LIFECYCLE_SWEEP_BATCH", "-3")
		require.Equal(t, 100, craftLifecycleSweepBatch())
	})

	t.Run("env off by default", func(t *testing.T) {
		require.False(t, parseCraftLifecycleBoolEnv("CRAFT_LIFECYCLE_SWEEP_DISABLED"))
		_ = os.Unsetenv("CRAFT_LIFECYCLE_SWEEP_DISABLED")
	})
}
