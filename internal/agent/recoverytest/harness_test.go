// Package recoverytest contains opt-in black-box acceptance tests for durable
// tRPC runs. The provider process is deliberately external: this package must
// not replace a real graph with an in-process mock and call that recovery.
package recoverytest

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

type CrashReport struct {
	ExternalCalls int    `json:"external_calls"`
	FinalStatus   string `json:"final_status"`
	AssistantRows int    `json:"assistant_rows"`
	LostEvents    int    `json:"lost_events"`
}

// runCrashCase starts the configured real graph provider, kills it at the
// provider-owned barrier, then starts the same provider against the same
// durable database. It is skipped unless an executable provider is supplied.
func runCrashCase(t *testing.T, point string) CrashReport {
	t.Helper()
	provider := strings.TrimSpace(os.Getenv("TRPC_RECOVERY_GRAPH_PROVIDER"))
	if provider == "" {
		t.Skip("TRPC_RECOVERY_GRAPH_PROVIDER unset: SIGKILL recovery requires a real graph provider")
	}
	if _, err := exec.LookPath(provider); err != nil {
		t.Skipf("TRPC_RECOVERY_GRAPH_PROVIDER %q unavailable: %v", provider, err)
	}
	if point == "" {
		t.Fatal("crash point is required")
	}

	dir := t.TempDir()
	dbPath := filepath.Join(dir, "recovery.db")
	barrierPath := filepath.Join(dir, "barrier")
	reportPath := filepath.Join(dir, "report.json")
	ns := "recoverytest-" + filepath.Base(dir)
	args := []string{"--recovery-case", point, "--recovery-db", dbPath, "--recovery-barrier", barrierPath, "--recovery-report", reportPath}
	env := append(os.Environ(), "TRPC_RECOVERY_TEST_NAMESPACE="+ns)

	first := exec.Command(provider, args...)
	first.Env = env
	first.Stderr = os.Stderr
	if err := first.Start(); err != nil {
		t.Fatalf("start recovery provider: %v", err)
	}
	if err := waitForFile(barrierPath, 30*time.Second); err != nil {
		_ = first.Process.Kill()
		_ = first.Wait()
		t.Fatalf("provider did not reach barrier %q: %v", point, err)
	}
	if err := first.Process.Kill(); err != nil && !errors.Is(err, os.ErrProcessDone) {
		t.Fatalf("kill provider at %q: %v", point, err)
	}
	_ = first.Wait()

	resumeArgs := append([]string{"--recovery-resume", point}, args[2:]...)
	resume := exec.Command(provider, resumeArgs...)
	resume.Env = env
	resume.Stderr = os.Stderr
	output, err := resume.Output()
	if err != nil {
		t.Fatalf("resume provider after %q: %v", point, err)
	}
	var report CrashReport
	if err := json.Unmarshal(output, &report); err != nil {
		// Providers may write the report to the durable path to avoid log noise.
		raw, readErr := os.ReadFile(reportPath)
		if readErr != nil {
			t.Fatalf("decode provider report: %v (stdout %q; report read: %v)", err, output, readErr)
		}
		if decodeErr := json.Unmarshal(raw, &report); decodeErr != nil {
			t.Fatalf("decode provider report: stdout %v; report %v", err, decodeErr)
		}
	}
	return report
}

func waitForFile(path string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if info, err := os.Stat(path); err == nil && !info.IsDir() {
			return nil
		}
		time.Sleep(25 * time.Millisecond)
	}
	return fmt.Errorf("timed out waiting for %s", path)
}

// readBarrier is used by provider adapters that expose a line-oriented
// barrier stream while retaining the same kill protocol.
func readBarrier(t *testing.T, r *os.File, want string) {
	t.Helper()
	s := bufio.NewScanner(r)
	for s.Scan() {
		if strings.TrimSpace(s.Text()) == want {
			return
		}
	}
	if err := s.Err(); err != nil {
		t.Fatal(err)
	}
	t.Fatalf("barrier %q not observed", want)
}
