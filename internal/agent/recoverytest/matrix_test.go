package recoverytest

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"sync"
	"testing"
)

// The SIGKILL matrix below runs against the real production recovery path:
// the provider binary owns the migrated database, the durable worker, the SDK
// graph, the repository checkpoint saver and tool journal, the durable
// decision parking and the finalize transaction. Only the chat model and the
// external side-effect endpoint are deterministic doubles.

type counterServer struct {
	mu    sync.Mutex
	count int
	seen  map[string]bool
}

func newCounterServer() *counterServer { return &counterServer{seen: map[string]bool{}} }

func (c *counterServer) handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/incr", func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Key string `json:"key"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		c.mu.Lock()
		first := false
		if body.Key != "" && !c.seen[body.Key] {
			c.seen[body.Key] = true
			c.count++
			first = true
		} else if body.Key == "" {
			c.count++
			first = true
		}
		count := c.count
		c.mu.Unlock()
		_ = json.NewEncoder(w).Encode(map[string]any{"count": count, "first": first})
	})
	mux.HandleFunc("/count", func(w http.ResponseWriter, _ *http.Request) {
		c.mu.Lock()
		count := c.count
		c.mu.Unlock()
		_ = json.NewEncoder(w).Encode(map[string]any{"count": count})
	})
	return mux
}

var (
	providerBuildOnce sync.Once
	providerBinary    string
	providerBuildErr  error
)

func buildRecoveryProvider(t *testing.T) string {
	t.Helper()
	providerBuildOnce.Do(func() {
		bin, err := os.CreateTemp("", "trpc-recovery-provider-*")
		if err != nil {
			providerBuildErr = err
			return
		}
		_ = bin.Close()
		providerBinary = bin.Name()
		cmd := exec.Command("go", "build", "-o", providerBinary, "./provider")
		cmd.Env = append(os.Environ(), "GOWORK=off")
		out, err := cmd.CombinedOutput()
		if err != nil {
			providerBuildErr = err
			t.Logf("provider build output: %s", out)
		}
	})
	if providerBuildErr != nil {
		t.Fatalf("recovery provider build failed: %v", providerBuildErr)
	}
	return providerBinary
}

func TestCrashMatrixSQLite(t *testing.T) {
	bin := buildRecoveryProvider(t)
	t.Setenv("TRPC_RECOVERY_GRAPH_PROVIDER", bin)

	matrix := []struct {
		name string
		want struct {
			externalCalls int
			finalStatus   string
			assistantRows int
			lostEvents    int
			parked        bool
		}
	}{
		{name: "after_admission"},
		{name: "after_plan_before_dispatch"},
		{name: "after_result_before_checkpoint"},
		{name: "after_finalize"},
		{name: "after_side_effect_before_result"},
		{name: "waiting_user"},
		{name: "unknown_result_user_retry"},
		{name: "idempotent_redelivery"},
		{name: "oauth_park"},
		{name: "mcp_set_drift"},
	}
	for _, tc := range matrix {
		t.Run(tc.name, func(t *testing.T) {
			counter := newCounterServer()
			server := httptest.NewServer(counter.handler())
			defer server.Close()
			t.Setenv("TRPC_RECOVERY_COUNTER_URL", server.URL)

			report := runCrashCase(t, tc.name)
			want := tc.want
			switch tc.name {
			case "after_admission", "after_plan_before_dispatch", "after_result_before_checkpoint",
				"after_finalize", "idempotent_redelivery", "oauth_park":
				want.externalCalls, want.finalStatus = 1, "succeeded"
				want.assistantRows, want.lostEvents = 1, 0
			case "after_side_effect_before_result", "waiting_user":
				want.externalCalls, want.finalStatus = 1, "waiting_user"
				want.assistantRows, want.lostEvents, want.parked = 0, 0, true
			case "unknown_result_user_retry":
				want.externalCalls, want.finalStatus = 2, "succeeded"
				want.assistantRows, want.lostEvents = 1, 0
			case "mcp_set_drift":
				want.externalCalls, want.finalStatus = 0, "failed"
				want.assistantRows, want.lostEvents = 0, 0
			}
			if report.ExternalCalls != want.externalCalls ||
				report.FinalStatus != want.finalStatus ||
				report.AssistantRows != want.assistantRows ||
				report.LostEvents != want.lostEvents {
				t.Fatalf("case %s report = %#v, want calls=%d status=%s rows=%d lost=%d",
					tc.name, report, want.externalCalls, want.finalStatus, want.assistantRows, want.lostEvents)
			}
		})
	}
}
