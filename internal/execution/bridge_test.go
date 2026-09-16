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
	cmd := validStartCommand()
	admission := AdmissionContext{ServiceIdentity: "weknora-execution", Signature: commandSignature(cmd, "secret"), AuthorizationVersion: 1, CommandHash: commandHash(cmd), TargetID: cmd.TargetID, WorkspaceRef: cmd.WorkspaceRef, WorkspaceTargetID: cmd.TargetID, Epoch: cmd.Epoch}
	got, err := NewBridgeClient(BridgeConfig{BaseURL: server.URL, ServiceToken: "secret", Admission: admission, AdmissionVerifier: AdmissionVerifier{ServiceIdentity: "weknora-execution", SigningSecret: "secret", AuthorizationVersion: 1}, VerifyIdentity: func(context.Context) error { return nil }, VerifyCommand: func(context.Context, StartCommand, AdmissionContext) error { return nil }, Authorize: func(context.Context, StartCommand, AdmissionContext) error { return nil }}).Start(context.Background(), cmd)
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

func TestBridgeRejectsTamperedAdmissionBeforeNetwork(t *testing.T) {
	called := false
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { called = true }))
	defer server.Close()
	cmd := validStartCommand()
	base := BridgeConfig{BaseURL: server.URL, ServiceToken: "secret", Admission: AdmissionContext{ServiceIdentity: "weknora-execution", Signature: commandSignature(cmd, "secret"), AuthorizationVersion: 1, CommandHash: commandHash(cmd), TargetID: cmd.TargetID, WorkspaceRef: cmd.WorkspaceRef, WorkspaceTargetID: cmd.TargetID, Epoch: cmd.Epoch}, AdmissionVerifier: AdmissionVerifier{ServiceIdentity: "weknora-execution", SigningSecret: "secret", AuthorizationVersion: 1}, VerifyIdentity: func(context.Context) error { return nil }, VerifyCommand: func(context.Context, StartCommand, AdmissionContext) error { return nil }, Authorize: func(context.Context, StartCommand, AdmissionContext) error { return nil }}
	for name, mutate := range map[string]func(*AdmissionContext){"identity": func(a *AdmissionContext) { a.ServiceIdentity = "" }, "signature": func(a *AdmissionContext) { a.Signature = "" }, "authz": func(a *AdmissionContext) { a.AuthorizationVersion = 2 }, "target": func(a *AdmissionContext) { a.TargetID = "other" }, "workspace": func(a *AdmissionContext) { a.WorkspaceTargetID = "other" }} {
		t.Run(name, func(t *testing.T) {
			cfg := base
			mutate(&cfg.Admission)
			if _, err := NewBridgeClient(cfg).Start(context.Background(), cmd); !errors.Is(err, ErrBridgeUnauthorized) {
				t.Fatalf("error=%v", err)
			}
		})
	}
	if called {
		t.Fatal("tampered admission reached bridge")
	}
}

func TestCanonicalFixtureUsesCamelCasePayload(t *testing.T) {
	b, err := os.ReadFile("testdata/start-command.json")
	if err != nil {
		t.Fatal(err)
	}
	envelope, err := decodeBridgeEnvelope(b)
	if err != nil {
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
	encoded, err := encodeBridgeEnvelope("start", cmd)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := decodeBridgeEnvelope(encoded); err != nil {
		t.Fatalf("encoded round-trip: %v", err)
	}
}

func TestBridgeRejectsUnknownEnvelopeFields(t *testing.T) {
	if _, err := decodeBridgeEnvelope([]byte(`{"version":1,"operation":"start","extra":true,"payload":{}}`)); !errors.Is(err, ErrBridgeProtocol) {
		t.Fatalf("error=%v", err)
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

func TestBridgeControlAndObservationEpochFixture(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var envelope struct {
			Operation string `json:"operation"`
		}
		if err := json.NewDecoder(r.Body).Decode(&envelope); err != nil {
			t.Fatal(err)
		}
		switch envelope.Operation {
		case "control":
			_ = json.NewEncoder(w).Encode(map[string]any{"version": 1, "operation": "control", "payload": map[string]any{"accepted": true}})
		case "observe":
			_ = json.NewEncoder(w).Encode(map[string]any{"version": 1, "operation": "observe", "payload": map[string]any{"state": "exited", "epoch": 3}})
		default:
			t.Fatalf("unexpected operation %q", envelope.Operation)
		}
	}))
	defer server.Close()
	client := NewBridgeClient(BridgeConfig{BaseURL: server.URL, ServiceToken: "secret"})
	accepted := true
	_ = accepted
	if err := client.Control(context.Background(), map[string]any{"action": "cancel"}); err != nil {
		t.Fatalf("control: %v", err)
	}
	observation, err := client.Observe(context.Background(), "external-1")
	if err != nil || observation.State != "exited" || observation.Epoch != 3 {
		t.Fatalf("observation=%+v err=%v", observation, err)
	}
}
