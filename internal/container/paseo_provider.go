package container

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/google/uuid"

	agentruntime "github.com/Tencent/WeKnora/internal/agent/runtime"
	workbenchservice "github.com/Tencent/WeKnora/internal/application/service/workbench"
	"github.com/Tencent/WeKnora/internal/execution"
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

func (p *paseoRemoteProvider) StopRemoteExecution(ctx context.Context, externalID string, epoch int64, timeout time.Duration) (agentruntime.RemoteStopResult, error) {
	if p == nil || p.client == nil {
		return agentruntime.RemoteStopResult{State: string(execution.StopUnknown), Epoch: epoch}, errors.New("paseo remote provider is unavailable")
	}
	remote := paseoControl{client: p.client, epoch: epoch}
	result, err := execution.StopRemoteExecutionAtEpoch(ctx, remote, strings.TrimSpace(externalID), epoch, timeout)
	return agentruntime.RemoteStopResult{State: string(result.State), ProcessState: result.Observation.ProcessState, Epoch: result.Observation.Epoch}, err
}

type paseoControl struct {
	client *execution.BridgeClient
	epoch  int64
}

func (c paseoControl) Cancel(ctx context.Context, id string) error { return c.client.Cancel(ctx, id) }
func (c paseoControl) Observe(ctx context.Context, id string) (execution.ExecutionObservation, error) {
	response, err := c.client.Observe(ctx, id)
	if err != nil {
		return execution.ExecutionObservation{}, err
	}
	return execution.ExecutionObservation{ProcessState: response.State, Fresh: true, Epoch: c.epoch, ObservedAt: time.Now()}, nil
}

func (p *paseoRemoteProvider) SubmitInteraction(ctx context.Context, tenantID uint64, ownerID, runID, externalPendingID, argsHash, action string, credentialVersion, expectedRevision int64) error {
	if p == nil || p.client == nil {
		return errors.New("paseo remote provider is unavailable")
	}
	if tenantID == 0 || strings.TrimSpace(ownerID) == "" || strings.TrimSpace(runID) == "" || strings.TrimSpace(externalPendingID) == "" || !strings.EqualFold(action, "approve") && !strings.EqualFold(action, "reject") || credentialVersion < 1 || expectedRevision < 1 || len(argsHash) != 64 {
		return errors.New("invalid remote interaction")
	}
	return p.client.Control(ctx, map[string]any{
		"action": "submitInteraction", "actionValue": action, "argsHash": argsHash,
		"commandID": uuid.NewString(), "credentialVersion": credentialVersion,
		"expectedRevision": expectedRevision, "externalPendingID": externalPendingID, "runID": runID,
		"tenantID": tenantID, "ownerID": ownerID,
	})
}

func (p *paseoRemoteProvider) StartCommand(ctx context.Context, request agentruntime.RemoteStartRequest) (string, error) {
	if p == nil || p.client == nil {
		return "", errors.New("paseo remote provider is unavailable")
	}
	if request.CommandID == "" || request.Fence.RunID == "" || request.AttemptID == "" || request.TargetID == "" || request.WorkspaceRef == "" || request.Prompt == "" || request.Provider == "" || request.Fence.Epoch < 1 {
		return "", errors.New("paseo remote provider requires a complete fenced command")
	}
	if request.PayloadHash == "" {
		return "", errors.New("paseo remote provider requires payload hash")
	}
	command := execution.StartCommand{CommandID: request.CommandID, RunID: request.Fence.RunID, AttemptID: request.AttemptID, TargetID: request.TargetID, WorkspaceRef: request.WorkspaceRef, Prompt: request.Prompt, Provider: request.Provider, Epoch: request.Fence.Epoch, PayloadHash: request.PayloadHash, ExpiresAt: time.Now().Add(30 * time.Second).UnixMilli()}
	admission := execution.AdmissionContext{ServiceIdentity: p.identity, Signature: execution.CommandSignature(command, p.signingSecret), AuthorizationVersion: 1, CommandHash: execution.CommandHash(command), TargetID: command.TargetID, WorkspaceRef: command.WorkspaceRef, WorkspaceTargetID: command.TargetID, Epoch: command.Epoch}
	response, err := p.client.StartWithAdmission(ctx, command, admission)
	if err != nil {
		return "", fmt.Errorf("paseo bridge start: %w", err)
	}
	return response.ID, nil
}
