package container

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	agentruntime "github.com/Tencent/WeKnora/internal/agent/runtime"
	workbenchservice "github.com/Tencent/WeKnora/internal/application/service/workbench"
)

// paseoRemoteProvider is the server-side Paseo bridge adapter. It deliberately
// speaks the versioned bridge envelope instead of pretending a local worker is
// a remote provider. The bridge remains the authority for admission checks.
type paseoRemoteProvider struct {
	baseURL, token, target, workspace, provider, prompt string
	client                                              *http.Client
}

func newPaseoRemoteProvider() (workbenchservice.RemoteProvider, error) {
	baseURL := strings.TrimRight(strings.TrimSpace(os.Getenv("PASEO_BRIDGE_URL")), "/")
	if baseURL == "" {
		return nil, nil
	}
	p := &paseoRemoteProvider{
		baseURL: baseURL, token: strings.TrimSpace(os.Getenv("PASEO_SERVICE_TOKEN")),
		target:    strings.TrimSpace(os.Getenv("PASEO_TARGET_ID")),
		workspace: strings.TrimSpace(os.Getenv("PASEO_WORKSPACE_REF")),
		provider:  strings.TrimSpace(os.Getenv("PASEO_PROVIDER")),
		prompt:    strings.TrimSpace(os.Getenv("PASEO_DEFAULT_PROMPT")),
		client:    &http.Client{Timeout: 30 * time.Second},
	}
	if p.token == "" || p.target == "" || p.workspace == "" || p.provider == "" || p.prompt == "" {
		return nil, errors.New("PASEO_BRIDGE_URL requires PASEO_SERVICE_TOKEN, PASEO_TARGET_ID, PASEO_WORKSPACE_REF, PASEO_PROVIDER, and PASEO_DEFAULT_PROMPT")
	}
	return p, nil
}

func (p *paseoRemoteProvider) Start(ctx context.Context, key agentruntime.RunKey, commandID string) (string, error) {
	return p.StartCommand(ctx, agentruntime.RemoteStartRequest{Fence: agentruntime.Fence{RunKey: key}, CommandID: commandID, AttemptID: commandID, TargetID: p.target, WorkspaceRef: p.workspace, Prompt: p.prompt, Provider: p.provider})
}

func (p *paseoRemoteProvider) StartCommand(ctx context.Context, request agentruntime.RemoteStartRequest) (string, error) {
	if p == nil || p.client == nil {
		return "", errors.New("paseo remote provider is unavailable")
	}
	if request.CommandID == "" || request.Fence.RunID == "" || request.AttemptID == "" || request.TargetID == "" || request.WorkspaceRef == "" || request.Prompt == "" || request.Provider == "" || request.Fence.Epoch < 1 {
		return "", errors.New("paseo remote provider requires a complete fenced command")
	}
	payload := map[string]any{
		"commandID": request.CommandID, "runID": request.Fence.RunID, "attemptID": request.AttemptID,
		"targetID": request.TargetID, "workspaceRef": request.WorkspaceRef, "prompt": request.Prompt,
		"provider": request.Provider, "epoch": request.Fence.Epoch, "expiresAt": time.Now().Add(30 * time.Second).UnixMilli(),
		"payloadHash": request.PayloadHash,
	}
	body, err := json.Marshal(map[string]any{"version": 1, "operation": "start", "payload": payload})
	if err != nil {
		return "", err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.baseURL+"/v1/execution/bridge", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+p.token)
	resp, err := p.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("paseo bridge start: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("paseo bridge start: status %s", resp.Status)
	}
	var envelope struct {
		Version   int    `json:"version"`
		Operation string `json:"operation"`
		Payload   struct {
			ID string `json:"id"`
		} `json:"payload"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 64<<10)).Decode(&envelope); err != nil {
		return "", err
	}
	if envelope.Version != 1 || envelope.Operation != "start" || envelope.Payload.ID == "" {
		return "", errors.New("invalid Paseo bridge response")
	}
	return envelope.Payload.ID, nil
}
