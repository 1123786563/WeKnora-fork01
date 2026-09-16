package container

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	agentruntime "github.com/Tencent/WeKnora/internal/agent/runtime"
)

func TestPaseoRemoteProviderAssemblyAndStart(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer token" {
			t.Fatalf("authorization not forwarded")
		}
		var envelope struct {
			Version   int            `json:"version"`
			Operation string         `json:"operation"`
			Payload   map[string]any `json:"payload"`
		}
		if err := json.NewDecoder(r.Body).Decode(&envelope); err != nil {
			t.Fatal(err)
		}
		if envelope.Version != 1 || envelope.Operation != "start" || envelope.Payload["runID"] != "run-1" {
			t.Fatalf("envelope=%v", envelope)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"version": 1, "operation": "start", "payload": map[string]string{"id": "paseo-1"}})
	}))
	defer server.Close()
	for key, value := range map[string]string{"PASEO_BRIDGE_URL": server.URL, "PASEO_SERVICE_TOKEN": "token", "PASEO_TARGET_ID": "target", "PASEO_WORKSPACE_REF": "workspace", "PASEO_PROVIDER": "provider", "PASEO_DEFAULT_PROMPT": "hello"} {
		t.Setenv(key, value)
	}
	p, err := newPaseoRemoteProvider()
	if err != nil || p == nil {
		t.Fatalf("provider=%T err=%v", p, err)
	}
	id, err := p.Start(context.Background(), agentruntime.RunKey{TenantID: 7, RunID: "run-1"}, "cmd-1")
	if err != nil || id != "paseo-1" {
		t.Fatalf("id=%q err=%v", id, err)
	}
}

func TestPaseoRemoteProviderAbsentWithoutURL(t *testing.T) {
	_ = os.Unsetenv("PASEO_BRIDGE_URL")
	p, err := newPaseoRemoteProvider()
	if err != nil || p != nil {
		t.Fatalf("provider=%T err=%v", p, err)
	}
}
