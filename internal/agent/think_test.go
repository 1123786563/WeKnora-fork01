package agent

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// TestThinkStreamStallWatchdogPanicIsRecovered pins #3837: the stall watchdog
// runs on a goroutine detached from the streaming call, beyond the reach of any
// caller's recover, so a panic inside it used to kill the whole process. A
// zero stall timeout makes time.NewTicker panic immediately, which is the
// deterministic way to fire that path; the watchdog must degrade to losing
// stall detection for this stream instead.
func TestThinkStreamStallWatchdogPanicIsRecovered(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	lastChunk := &atomic.Int64{}
	start := time.Now()
	stalled, stop := watchStreamStall(ctx, cancel, 0, lastChunk)
	defer stop()
	// Give the detached goroutine a beat so the panic fires while this test is
	// the visible stack; without the guard the test binary would die here.
	time.Sleep(100 * time.Millisecond)
	require.Less(t, time.Since(start), 5*time.Second, "watchStreamStall must return promptly")
	require.False(t, stalled.Load(), "a broken watchdog must not report a stall")
}
