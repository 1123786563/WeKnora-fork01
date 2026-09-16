package execution

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func validStartCommand() StartCommand {
	return StartCommand{Version: 1, CommandID: "c", RunID: "r", AttemptID: "a", TargetID: "t", WorkspaceRef: "w", Prompt: "hi", Provider: "p", Epoch: 1, ExpiresAt: time.Now().Add(time.Minute).UnixMilli()}
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
		_ = json.NewEncoder(w).Encode(BridgeResponse{ID: "paseo-1"})
	}))
	defer server.Close()
	got, err := NewBridgeClient(BridgeConfig{BaseURL: server.URL, ServiceToken: "secret"}).Start(context.Background(), validStartCommand())
	if err != nil || got.ID != "paseo-1" {
		t.Fatalf("got=%+v err=%v", got, err)
	}
}

func TestBridgeRejectsUnknownOrExpiredCommandsBeforeNetwork(t *testing.T) {
	called := false
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { called = true }))
	defer server.Close()
	cmd := validStartCommand()
	cmd.Version = 2
	if _, err := NewBridgeClient(BridgeConfig{BaseURL: server.URL, ServiceToken: "secret"}).Start(context.Background(), cmd); !errors.Is(err, ErrBridgeInvalidRequest) {
		t.Fatalf("version error=%v", err)
	}
	cmd = validStartCommand()
	cmd.ExpiresAt = time.Now().Add(-time.Second).UnixMilli()
	if _, err := NewBridgeClient(BridgeConfig{BaseURL: server.URL, ServiceToken: "secret"}).Start(context.Background(), cmd); !errors.Is(err, ErrBridgeInvalidRequest) {
		t.Fatalf("expiry error=%v", err)
	}
	if called {
		t.Fatal("invalid commands reached bridge")
	}
}
