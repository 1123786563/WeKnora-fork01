package container

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	agentruntime "github.com/Tencent/WeKnora/internal/agent/runtime"
	workbenchservice "github.com/Tencent/WeKnora/internal/application/service/workbench"
	"github.com/Tencent/WeKnora/internal/modules/execution"
)

type paseoRemoteProvider struct {
	baseURL, token, target, workspace, provider string
	identity, signingSecret                     string
	client                                      *execution.BridgeClient
}

func newPaseoRemoteProvider() (workbenchservice.RemoteProvider, error) {
	baseURL := strings.TrimRight(strings.TrimSpace(os.Getenv("PASEO_BRIDGE_URL")), "/")
	if baseURL == "" {
		return nil, nil
	}
	p := &paseoRemoteProvider{baseURL: baseURL, token: strings.TrimSpace(os.Getenv("PASEO_SERVICE_TOKEN")), target: strings.TrimSpace(os.Getenv("PASEO_TARGET_ID")), workspace: strings.TrimSpace(os.Getenv("PASEO_WORKSPACE_REF")), provider: strings.TrimSpace(os.Getenv("PASEO_PROVIDER")), identity: strings.TrimSpace(os.Getenv("PASEO_SERVICE_IDENTITY")), signingSecret: os.Getenv("PASEO_SIGNING_SECRET")}
	if p.token == "" || p.target == "" || p.workspace == "" || p.provider == "" || p.identity == "" || p.signingSecret == "" {
		return nil, errors.New("PASEO_BRIDGE_URL requires service token, target, workspace, provider, service identity, and signing secret")
	}
	p.client = execution.NewBridgeClient(execution.BridgeConfig{BaseURL: p.baseURL, ServiceToken: p.token, AdmissionVerifier: execution.AdmissionVerifier{ServiceIdentity: p.identity, SigningSecret: p.signingSecret, AuthorizationVersion: 1}, VerifyIdentity: func(context.Context) error { return nil }, VerifyCommand: func(context.Context, execution.StartCommand, execution.AdmissionContext) error { return nil }, Authorize: func(context.Context, execution.StartCommand, execution.AdmissionContext) error { return nil }})
	return p, nil
}

func (p *paseoRemoteProvider) Start(context.Context, agentruntime.RunKey, string) (string, error) {
	return "", errors.New("legacy unfenced Paseo start is rejected")
}

func (p *paseoRemoteProvider) StartCommand(ctx context.Context, request agentruntime.RemoteStartRequest) (string, error) {
	result, err := p.startCommandWithUsage(ctx, request)
	return result.ExternalID, err
}

func (p *paseoRemoteProvider) StartCommandWithUsage(ctx context.Context, request agentruntime.RemoteStartRequest) (agentruntime.RemoteStartResult, error) {
	return p.startCommandWithUsage(ctx, request)
}

func (p *paseoRemoteProvider) startCommandWithUsage(ctx context.Context, request agentruntime.RemoteStartRequest) (agentruntime.RemoteStartResult, error) {
	if p == nil || p.client == nil {
		return agentruntime.RemoteStartResult{}, errors.New("paseo remote provider is unavailable")
	}
	if request.CommandID == "" || request.Fence.RunID == "" || request.AttemptID == "" || request.TargetID == "" || request.WorkspaceRef == "" || request.Prompt == "" || request.Provider == "" || request.Fence.Epoch < 1 {
		return agentruntime.RemoteStartResult{}, errors.New("paseo remote provider requires a complete fenced command")
	}
	if request.PayloadHash == "" {
		return agentruntime.RemoteStartResult{}, errors.New("paseo remote provider requires payload hash")
	}
	command := execution.StartCommand{CommandID: request.CommandID, RunID: request.Fence.RunID, AttemptID: request.AttemptID, TargetID: request.TargetID, WorkspaceRef: request.WorkspaceRef, Prompt: request.Prompt, Provider: request.Provider, Epoch: request.Fence.Epoch, PayloadHash: request.PayloadHash, ExpiresAt: time.Now().Add(30 * time.Second).UnixMilli()}
	admission := execution.AdmissionContext{ServiceIdentity: p.identity, Signature: execution.CommandSignature(command, p.signingSecret), AuthorizationVersion: 1, CommandHash: execution.CommandHash(command), TargetID: command.TargetID, WorkspaceRef: command.WorkspaceRef, WorkspaceTargetID: command.TargetID, Epoch: command.Epoch}
	response, err := p.client.StartWithAdmission(ctx, command, admission)
	if err != nil {
		return agentruntime.RemoteStartResult{}, fmt.Errorf("paseo bridge start: %w", err)
	}
	result := agentruntime.RemoteStartResult{ExternalID: response.ID}
	if response.Usage != nil {
		result.Usage = &agentruntime.RemoteUsageObservation{Service: response.Usage.Service, Status: response.Usage.Status, PriceVersion: response.Usage.PriceVersion, Revision: response.Usage.Revision, OccurredAt: response.Usage.OccurredAt, Dimensions: response.Usage.Dimensions}
	}
	return result, nil
}
