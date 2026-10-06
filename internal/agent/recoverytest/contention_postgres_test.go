package recoverytest

import (
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

// TestTwoWorkerContentionPostgreSQL runs the same stale-worker takeover row
// against PostgreSQL: per-case schema, real lease expiry in database time,
// fenced rejection of the superseded worker, exactly-one side effect.
func TestTwoWorkerContentionPostgreSQL(t *testing.T) {
	pgDSN := os.Getenv("TRPC_RECOVERY_PG_DSN")
	if pgDSN == "" {
		t.Skip("TRPC_RECOVERY_PG_DSN unset: PostgreSQL contention row NOT VERIFIED")
	}
	freshDSN := bootstrapMatrixDatabase(t, pgDSN)
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
	t.Setenv("TRPC_RECOVERY_TEST_NAMESPACE", "contention-pg")
	t.Setenv("TRPC_RECOVERY_COUNTER_URL", server.URL)
	t.Setenv("TRPC_RECOVERY_USE_PG", "1")
	t.Setenv("TRPC_RECOVERY_PG_DSN", freshDSN)

	stale := exec.Command(bin, append([]string{
		"--recovery-case", "two_worker_contention",
		"--recovery-report", staleReport,
	}, base...)...)
	stale.Env = os.Environ()
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
	takeover.Env = os.Environ()
	takeover.Stderr = os.Stderr
	out, err := takeover.Output()
	if err != nil {
		t.Fatalf("takeover worker: %v (output %s)", err, out)
	}
	counter.mu.Lock()
	got := counter.count
	counter.mu.Unlock()
	if got != 1 {
		t.Fatalf("external calls after contention = %d, want 1", got)
	}
	_ = stale.Process.Kill()
	_ = stale.Wait()
}
