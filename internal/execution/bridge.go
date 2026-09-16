package execution

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

var ErrBridgeUnavailable = errors.New("execution_bridge_unavailable")
var ErrBridgeInvalidRequest = errors.New("execution_bridge_invalid_request")
var ErrBridgeUnauthorized = errors.New("execution_bridge_unauthorized")
var ErrBridgeProtocol = errors.New("execution_bridge_protocol")

// StartCommand is the versioned, fixed wire contract shared with the TS bridge.
type StartCommand struct {
	CommandID    string `json:"commandID"`
	RunID        string `json:"runID"`
	AttemptID    string `json:"attemptID"`
	TargetID     string `json:"targetID"`
	WorkspaceRef string `json:"workspaceRef"`
	Prompt       string `json:"prompt"`
	Provider     string `json:"provider"`
	Epoch        int64  `json:"epoch"`
	ExpiresAt    int64  `json:"expiresAt"`
}

type BridgeConfig struct {
	BaseURL        string
	ServiceToken   string
	HTTPClient     *http.Client
	Timeout        time.Duration
	VerifyIdentity func(context.Context) error
	VerifyCommand  func(context.Context, StartCommand, AdmissionContext) error
	Authorize      func(context.Context, StartCommand, AdmissionContext) error
	Admission      AdmissionContext
}

type AdmissionContext struct {
	ServiceIdentity      string
	Signature            string
	AuthorizationVersion int
	CommandHash          string
	TargetID             string
	WorkspaceRef         string
	WorkspaceTargetID    string
	Epoch                int64
}

type BridgeClient struct{ config BridgeConfig }

type BridgeResponse struct {
	ID    string `json:"id"`
	State string `json:"state,omitempty"`
}

type bridgeEnvelope struct {
	Version   int             `json:"version"`
	Operation string          `json:"operation"`
	Payload   json.RawMessage `json:"payload"`
}

type bridgeResponseEnvelope struct {
	Version   int             `json:"version"`
	Operation string          `json:"operation"`
	Payload   json.RawMessage `json:"payload"`
}

func decodeBridgeEnvelope(data []byte) (bridgeEnvelope, error) {
	var envelope bridgeEnvelope
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&envelope); err != nil || envelope.Version != 1 || envelope.Operation == "" || len(envelope.Payload) == 0 {
		return bridgeEnvelope{}, ErrBridgeProtocol
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return bridgeEnvelope{}, ErrBridgeProtocol
	}
	return envelope, nil
}

func NewBridgeClient(config BridgeConfig) *BridgeClient {
	config.BaseURL = strings.TrimRight(strings.TrimSpace(config.BaseURL), "/")
	if config.HTTPClient == nil {
		config.HTTPClient = &http.Client{}
	}
	if config.Timeout <= 0 {
		config.Timeout = 30 * time.Second
	}
	return &BridgeClient{config: config}
}

func (c *BridgeClient) request(ctx context.Context, operation string, payload any) (BridgeResponse, error) {
	if c == nil || c.config.BaseURL == "" || c.config.ServiceToken == "" {
		return BridgeResponse{}, ErrBridgeUnavailable
	}
	if operation != "start" && operation != "observe" && operation != "cancel" {
		return BridgeResponse{}, ErrBridgeInvalidRequest
	}
	body, err := json.Marshal(map[string]any{"version": 1, "operation": operation, "payload": payload})
	if err != nil {
		return BridgeResponse{}, ErrBridgeInvalidRequest
	}
	if len(body) > 1<<20 {
		return BridgeResponse{}, ErrBridgeInvalidRequest
	}
	requestCtx, cancel := context.WithTimeout(ctx, c.config.Timeout)
	defer cancel()
	req, err := http.NewRequestWithContext(requestCtx, http.MethodPost, c.config.BaseURL+"/v1/execution/bridge", bytes.NewReader(body))
	if err != nil {
		return BridgeResponse{}, ErrBridgeUnavailable
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.config.ServiceToken)
	resp, err := c.config.HTTPClient.Do(req)
	if err != nil {
		return BridgeResponse{}, ErrBridgeUnavailable
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		return BridgeResponse{}, ErrBridgeUnauthorized
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return BridgeResponse{}, ErrBridgeUnavailable
	}
	var envelope bridgeResponseEnvelope
	decoder := json.NewDecoder(io.LimitReader(resp.Body, 64<<10))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&envelope); err != nil || envelope.Version != 1 || envelope.Operation != operation {
		return BridgeResponse{}, ErrBridgeProtocol
	}
	var out BridgeResponse
	payloadDecoder := json.NewDecoder(bytes.NewReader(envelope.Payload))
	payloadDecoder.DisallowUnknownFields()
	if err := payloadDecoder.Decode(&out); err != nil {
		return BridgeResponse{}, ErrBridgeProtocol
	}
	if operation == "start" && out.ID == "" {
		return BridgeResponse{}, ErrBridgeProtocol
	}
	if operation == "observe" && out.State == "" {
		return BridgeResponse{}, ErrBridgeProtocol
	}
	if operation == "cancel" {
		return BridgeResponse{}, nil
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return BridgeResponse{}, ErrBridgeUnavailable
	}
	return out, nil
}

func validateStartCommand(c StartCommand) error {
	if c.Epoch < 1 || c.ExpiresAt <= time.Now().UnixMilli() || c.Prompt == "" || c.Provider == "" || c.CommandID == "" || c.RunID == "" || c.AttemptID == "" || c.TargetID == "" || c.WorkspaceRef == "" {
		return ErrBridgeInvalidRequest
	}
	if len(c.Prompt) > 32000 || len(c.Provider) > 256 {
		return ErrBridgeInvalidRequest
	}
	return nil
}

func (c *BridgeClient) Start(ctx context.Context, command StartCommand) (BridgeResponse, error) {
	if err := validateStartCommand(command); err != nil {
		return BridgeResponse{}, err
	}
	if c == nil || c.config.BaseURL == "" || c.config.ServiceToken == "" {
		return BridgeResponse{}, ErrBridgeUnavailable
	}
	if c == nil || c.config.VerifyIdentity == nil || c.config.VerifyCommand == nil || c.config.Authorize == nil {
		return BridgeResponse{}, ErrBridgeUnauthorized
	}
	if err := c.config.VerifyIdentity(ctx); err != nil {
		return BridgeResponse{}, ErrBridgeUnauthorized
	}
	admission := c.config.Admission
	if admission.ServiceIdentity == "" || admission.Signature == "" || admission.AuthorizationVersion != 1 || admission.CommandHash == "" || admission.TargetID != command.TargetID || admission.WorkspaceRef != command.WorkspaceRef || admission.WorkspaceTargetID != command.TargetID || admission.Epoch != command.Epoch {
		return BridgeResponse{}, ErrBridgeUnauthorized
	}
	if err := c.config.VerifyCommand(ctx, command, admission); err != nil {
		return BridgeResponse{}, ErrBridgeUnauthorized
	}
	if err := c.config.Authorize(ctx, command, admission); err != nil {
		return BridgeResponse{}, ErrBridgeUnauthorized
	}
	result, err := c.request(ctx, "start", command)
	if err == nil && result.ID == "" {
		return BridgeResponse{}, fmt.Errorf("%w: missing id", ErrBridgeUnavailable)
	}
	return result, err
}

func (c *BridgeClient) Observe(ctx context.Context, id string) (BridgeResponse, error) {
	if strings.TrimSpace(id) == "" {
		return BridgeResponse{}, ErrBridgeInvalidRequest
	}
	return c.request(ctx, "observe", struct {
		ID string `json:"id"`
	}{id})
}

func (c *BridgeClient) Cancel(ctx context.Context, id string) error {
	if strings.TrimSpace(id) == "" {
		return ErrBridgeInvalidRequest
	}
	_, err := c.request(ctx, "cancel", struct {
		ID string `json:"id"`
	}{id})
	return err
}
