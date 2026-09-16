package execution

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"
)

func validStartCommand() StartCommand {
	return StartCommand{CommandID: "c", RunID: "r", AttemptID: "a", TargetID: "t", WorkspaceRef: "w", Prompt: "hi", Provider: "p", Epoch: 1, ExpiresAt: time.Now().Add(time.Minute).UnixMilli()}
}

func TestBridgeUnavailableWithoutConfiguration(t *testing.T) {
	_, err := NewBridgeClient(BridgeConfig{}).Start(context.Background(), validStartCommand())
	if !errors.Is(err, ErrBridgeUnavailable) {
		t.Fatalf("error = %v", err)
	}
}

func TestBridgeFixedProtocolAndAuthorization(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/execution/bridge" || r.Method != http.MethodPost {
			t.Errorf("unexpected endpoint: %s %s", r.Method, r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer secret" {
			t.Errorf("missing service identity")
		}
		var envelope struct {
			Version   int          `json:"version"`
			Operation string       `json:"operation"`
			Payload   StartCommand `json:"payload"`
		}
		if err := json.NewDecoder(r.Body).Decode(&envelope); err != nil {
			t.Fatal(err)
		}
		if envelope.Version != 1 || envelope.Operation != "start" || envelope.Payload.WorkspaceRef != "w" {
			t.Errorf("bad envelope: %+v", envelope)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"version": 1, "operation": "start", "payload": BridgeResponse{ID: "paseo-1"}})
	}))
	defer server.Close()
	got, err := NewBridgeClient(BridgeConfig{BaseURL: server.URL, ServiceToken: "secret", VerifyIdentity: func(context.Context) error { return nil }, VerifyCommand: func(context.Context, StartCommand) error { return nil }, Authorize: func(context.Context, StartCommand) error { return nil }}).Start(context.Background(), validStartCommand())
	if err != nil || got.ID != "paseo-1" {
		t.Fatalf("got=%+v err=%v", got, err)
	}
}

func TestBridgeRejectsWithoutTrustedAdmission(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { t.Fatal("unauthorized request reached bridge") }))
	defer server.Close()
	_, err := NewBridgeClient(BridgeConfig{BaseURL: server.URL, ServiceToken: "secret"}).Start(context.Background(), validStartCommand())
	if !errors.Is(err, ErrBridgeUnauthorized) {
		t.Fatalf("error = %v", err)
	}
}

func TestCanonicalFixtureUsesCamelCasePayload(t *testing.T) {
	b, err := os.ReadFile("testdata/start-command.json")
	if err != nil {
		t.Fatal(err)
	}
	var envelope bridgeEnvelope
	if err := json.Unmarshal(b, &envelope); err != nil {
		t.Fatal(err)
	}
	if envelope.Version != 1 || envelope.Operation != "start" {
		t.Fatalf("envelope=%+v", envelope)
	}
	var cmd StartCommand
	if err := json.Unmarshal(envelope.Payload, &cmd); err != nil {
		t.Fatal(err)
	}
	if cmd.CommandID != "cmd-1" || cmd.WorkspaceRef != "workspace-1" || cmd.ExpiresAt != 4102444800000 {
		t.Fatalf("command=%+v", cmd)
	}
}

func TestBridgeRejectsUnknownOrExpiredCommandsBeforeNetwork(t *testing.T) {
	called := false
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { called = true }))
	defer server.Close()
	cmd := validStartCommand()
	cmd.ExpiresAt = time.Now().Add(-time.Second).UnixMilli()
	if _, err := NewBridgeClient(BridgeConfig{BaseURL: server.URL, ServiceToken: "secret"}).Start(context.Background(), cmd); !errors.Is(err, ErrBridgeInvalidRequest) {
		t.Fatalf("expiry error=%v", err)
	}
	if called {
		t.Fatal("invalid commands reached bridge")
	}
}
