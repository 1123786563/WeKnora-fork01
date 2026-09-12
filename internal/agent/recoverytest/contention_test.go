package recoverytest

import (
	"encoding/json"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
)

// TestTwoWorkerContentionSQLite drives the two-worker matrix row on SQLite:
// the stale worker claims the run with a short lease and keeps attempting
// fenced writes; the takeover process waits for lease expiry, claims with a
// higher epoch, runs the graph to completion exactly once (journal reuse),
// and every stale-worker write after the takeover must be rejected.
func TestTwoWorkerContentionSQLite(t *testing.T) {
	bin := buildRecoveryProvider(t)
	dir := t.TempDir()
	barrier := filepath.Join(dir, "barrier")
	takeoverReport := filepath.Join(dir, "takeover.json")
	staleReport := filepath.Join(dir, "stale.json")
	dbPath := filepath.Join(dir, "recovery.db")
	base := []string{
		"--recovery-db", dbPath,
		"--recovery-barrier", barrier,
	}
	counter := newCounterServer()
	server := httptest.NewServer(counter.handler())
	defer server.Close()
	env := append(os.Environ(),
		"TRPC_RECOVERY_TEST_NAMESPACE=contention",
		"TRPC_RECOVERY_COUNTER_URL="+server.URL,
	)

	stale := exec.Command(bin, append([]string{
		"--recovery-case", "two_worker_contention",
		"--recovery-report", staleReport,
	}, base...)...)
	stale.Env = env
	stale.Stderr = os.Stderr
	if err := stale.Start(); err != nil {
		t.Fatalf("start stale worker: %v", err)
	}
	defer func() { _ = stale.Process.Kill() }()

	if err := waitForFile(barrier, 120*time.Second); err != nil {
		t.Fatalf("stale worker never claimed: %v", err)
	}

	takeover := exec.Command(bin, append([]string{
		"--recovery-resume", "two_worker_contention",
		"--recovery-report", takeoverReport,
	}, base...)...)
	takeover.Env = env
	takeover.Stderr = os.Stderr
	out, err := takeover.Output()
	if err != nil {
		t.Fatalf("takeover worker: %v (output %s)", err, out)
	}
	var report CrashReport
	decodeErr := json.Unmarshal(out, &report)
	if decodeErr != nil {
		if raw, rerr := os.ReadFile(takeoverReport); rerr == nil {
			decodeErr = json.Unmarshal(raw, &report)
		}
	}
	if decodeErr != nil {
		t.Fatalf("decode takeover report: %v", decodeErr)
	}
	if report.ExternalCalls != 1 || report.FinalStatus != "succeeded" ||
		report.AssistantRows != 1 || report.LostEvents != 0 {
		t.Fatalf("takeover report = %#v, want calls=1 status=succeeded rows=1 lost=0", report)
	}

	// The stale worker must report that its writes were fenced off.
	staleDone := make(chan error, 1)
	go func() { staleDone <- stale.Wait() }()
	select {
	case <-staleDone:
	case <-time.After(60 * time.Second):
		_ = stale.Process.Signal(syscall.SIGKILL)
		t.Fatal("stale worker did not finish after takeover")
	}
	raw, err := os.ReadFile(staleReport)
	if err != nil {
		t.Fatalf("stale report missing: %v", err)
	}
	if !strings.Contains(string(raw), "stale-rejected") {
		t.Fatalf("stale report = %s, want stale-rejected", raw)
	}
	// The takeover completed the side effect exactly once even though the
	// stale worker held the claim first.
	counter.mu.Lock()
	got := counter.count
	counter.mu.Unlock()
	if got != 1 {
		t.Fatalf("external calls after contention = %d, want 1", got)
	}
}
