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

// StartCommand is the versioned, fixed wire contract shared with the TS bridge.
type StartCommand struct {
	Version      int    `json:"version"`
	CommandID    string `json:"command_id"`
	RunID        string `json:"run_id"`
	AttemptID    string `json:"attempt_id"`
	TargetID     string `json:"target_id"`
	WorkspaceRef string `json:"workspace_ref"`
	Prompt       string `json:"prompt"`
	Provider     string `json:"provider"`
	Epoch        int64  `json:"epoch"`
	ExpiresAt    int64  `json:"expires_at"`
}

type BridgeConfig struct {
	BaseURL      string
	ServiceToken string
	HTTPClient   *http.Client
	Timeout      time.Duration
}

type BridgeClient struct{ config BridgeConfig }

type BridgeResponse struct {
	ID    string `json:"id"`
	State string `json:"state,omitempty"`
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
	var out BridgeResponse
	if err := json.NewDecoder(io.LimitReader(resp.Body, 64<<10)).Decode(&out); err != nil {
		return BridgeResponse{}, ErrBridgeUnavailable
	}
	return out, nil
}

func validateStartCommand(c StartCommand) error {
	if c.Version != 1 || c.Epoch < 1 || c.ExpiresAt <= time.Now().UnixMilli() || c.Prompt == "" || c.Provider == "" || c.CommandID == "" || c.RunID == "" || c.AttemptID == "" || c.TargetID == "" || c.WorkspaceRef == "" {
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
