package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/Tencent/WeKnora/internal/logger"
	"github.com/Tencent/WeKnora/internal/modules/agentruntime/agent/tools"
	"github.com/Tencent/WeKnora/internal/modules/plugins"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
	"gorm.io/gorm"
)

// PluginSnapshotLookup builds the production runtime guard provider over
// the plugin repository (T09): a service with no installation row (every
// manual MCP service) resolves to (nil, nil) — the tools layer reads that
// as "not plugin-materialized, legacy behavior"; a found row hands its
// accepted snapshot to the guard. Repository faults propagate as errors so
// the guard fails closed.
func PluginSnapshotLookup(repo interfaces.PluginRepository) tools.PluginSnapshotProvider {
	return func(ctx context.Context, tenantID uint64, serviceID string) (*tools.PluginRuntimeSnapshot, error) {
		inst, err := repo.GetByServiceID(ctx, tenantID, serviceID)
		if err != nil {
			return nil, err
		}
		if inst == nil {
			return nil, nil
		}
		return &tools.PluginRuntimeSnapshot{
			InstallationID: inst.ID,
			Tools:          inst.ToolsSnapshot,
		}, nil
	}
}

// Install-slice sentinels (T06). They are the ONLY texts the handler maps
// onto verdicts; underlying DB faults are logged server-side and surface as
// the 5xx sentinels — driver internals never reach the response body.
var (
	// ErrPluginPreviewNotFound: the preview ID is absent in this tenant —
	// a fresh mistype OR another tenant's ID; one verdict, no existence leak.
	ErrPluginPreviewNotFound = errors.New("plugin preview not found")

	// ErrPreviewAlreadyConsumed: the preview already installed something.
	ErrPreviewAlreadyConsumed = errors.New("plugin preview already consumed")

	// ErrPreviewExpired: the preview's TTL boundary has passed.
	ErrPreviewExpired = errors.New("plugin preview expired")

	// ErrPreviewContentChanged: the re-fetch at confirm time disagrees with
	// the preview's tools digest or identity fingerprint — the admin must
	// review a NEW preview instead of confirming the stale one.
	ErrPreviewContentChanged = errors.New("plugin preview content changed since verification")

	// ErrPluginAlreadyInstalled: this (tenant, plugin) already has an
	// installation — re-install goes through the upgrade path, never a
	// second row.
	ErrPluginAlreadyInstalled = errors.New("plugin already installed in this workspace")

	// ErrInstallationNotFound: the installation ID is absent in this tenant.
	ErrInstallationNotFound = errors.New("plugin installation not found")

	// ErrInstallationStateInvalid: state outside {active, disabled}.
	ErrInstallationStateInvalid = errors.New("invalid plugin installation state")

	// ErrInstallationPersistFailed: 5xx persistence fault on the install path.
	ErrInstallationPersistFailed = errors.New("failed to persist plugin installation")

	// ErrInstallationMaterializeFailed: 5xx fault while materializing the
	// MCP service / tool policies (after compensation).
	ErrInstallationMaterializeFailed = errors.New("failed to materialize plugin installation")
)

// ConfirmInstallation implements the compensated seven-step confirm flow
// (plan 03 Task 6 Step 4 ruling): preview lookup → expiry verdict →
// duplicate-install guard → remote re-verification (digest + identity
// fingerprint) → installation row → materialized MCP service + per-tool
// policies (read tools enabled, WRITE TOOLS DISABLED) → preview consumed
// exactly once. Steps after the installation row compensates on failure:
// the materialized service is soft-deleted and the installation row is
// hard-deleted, so the (tenant_id, plugin_id) unique slot is freed and the
// admin can retry. Compensation failures are logged and never mask the
// original error.
func (s *pluginService) ConfirmInstallation(
	ctx context.Context,
	tenantID uint64,
	actorID, previewID string,
) (*types.PluginInstallationResult, error) {
	if previewID == "" {
		return nil, fmt.Errorf("%w: preview_id is required", ErrPluginPreviewNotFound)
	}

	// Step 1: preview within this tenant (absent/foreign → one verdict).
	preview, err := s.pluginRepo.GetPreview(ctx, tenantID, previewID)
	if err != nil {
		logger.GetLogger(ctx).Errorf("failed to load plugin preview: %v", err)
		return nil, ErrPreviewPersistFailed
	}
	if preview == nil {
		return nil, ErrPluginPreviewNotFound
	}

	// Step 2: friendly expiry verdict (MarkPreviewConsumed re-guards the
	// boundary atomically in step 7 — no check-then-act window either way).
	now := time.Now()
	if preview.ConsumedAt != nil {
		return nil, ErrPreviewAlreadyConsumed
	}
	if !now.Before(preview.ExpiresAt) {
		return nil, ErrPreviewExpired
	}

	// Step 3: (tenant, plugin) uniqueness — re-install is the upgrade path.
	if existing, err := s.pluginRepo.GetInstallationByTenantPlugin(ctx, tenantID, preview.PluginID); err != nil {
		logger.GetLogger(ctx).Errorf("failed to load existing plugin installation: %v", err)
		return nil, ErrInstallationPersistFailed
	} else if existing != nil {
		return nil, fmt.Errorf("%w: %q is already at version %s; use the upgrade path",
			ErrPluginAlreadyInstalled, preview.PluginID, existing.AcceptedVersion)
	}

	// Step 4: re-fetch and re-verify the remote. The admin reviewed THIS
	// fingerprint and digest; anything that moved since is a different
	// document and must go through a fresh preview.
	result, err := plugins.FetchAndVerify(ctx, preview.ManifestURL, s.lister)
	if err != nil {
		return nil, err // deterministic verification fault / fetch fault — handler maps
	}
	if result.ToolsDigest != preview.ToolsDigest || result.IdentityFingerprint != preview.IdentityFingerprint {
		return nil, ErrPreviewContentChanged
	}

	installation := &types.PluginInstallation{
		ID:              uuid.New().String(),
		TenantID:        tenantID,
		PluginID:        preview.PluginID,
		Name:            preview.Name,
		Description:     result.Manifest.Description,
		ManifestURL:     preview.ManifestURL, // long-lived upgrade source, copied HERE
		AcceptedVersion: preview.Version,
		TransportType:   preview.TransportType,
		EndpointURL:     preview.EndpointURL,
		ToolsSnapshot:   types.PluginPreviewTools(result.Snapshot),
		ToolsDigest:     preview.ToolsDigest,
		ServiceID:       "",
		DriftState:      types.PluginDriftNone,
		State:           types.PluginInstallationActive,
		CreatedBy:       actorID,
	}

	// Step 5: installation row.
	if err := s.pluginRepo.CreateInstallation(ctx, installation); err != nil {
		logger.GetLogger(ctx).Errorf("failed to persist plugin installation: %v", err)
		return nil, ErrInstallationPersistFailed
	}

	// Step 6: materialize the MCP service (reuses CreateMCPService's URL
	// validation and default config) and bind it back.
	endpoint := preview.EndpointURL
	materialized := &types.MCPService{
		ID:                   uuid.New().String(),
		TenantID:             tenantID,
		Name:                 "plugin:" + preview.PluginID,
		Description:          preview.Name,
		Enabled:              true,
		TransportType:        types.MCPTransportType(preview.TransportType),
		URL:                  &endpoint,
		PluginInstallationID: &installation.ID,
	}
	if result.Manifest.Auth != nil && result.Manifest.Auth.PersonalOAuth {
		materialized.AuthConfig = &types.MCPAuthConfig{
			AuthType: types.MCPAuthOAuth,
			Scopes:   result.Manifest.Auth.Scopes,
		}
	}
	if err := s.mcpServiceService.CreateMCPService(ctx, materialized); err != nil {
		logger.GetLogger(ctx).Errorf("failed to materialize MCP service for plugin %s: %v", preview.PluginID, err)
		return nil, s.compensateInstallation(ctx, tenantID, installation.ID, "", ErrInstallationMaterializeFailed)
	}
	if err := s.pluginRepo.UpdateInstallationServiceID(ctx, tenantID, installation.ID, materialized.ID); err != nil {
		logger.GetLogger(ctx).Errorf("failed to bind materialized service to plugin installation: %v", err)
		return nil, s.compensateInstallation(ctx, tenantID, installation.ID, materialized.ID, ErrInstallationMaterializeFailed)
	}
	installation.ServiceID = materialized.ID

	// Per-tool explicit policies: read tools exposed, WRITE TOOLS DISABLED
	// (B5 install-time landing — a manifest declaration is never execution
	// authorization).
	for _, tool := range result.Snapshot {
		enabled := tool.ReadOnly
		if err := s.toolApprovalService.SetPolicy(ctx, tenantID, materialized.ID, tool.Name, nil, &enabled); err != nil {
			logger.GetLogger(ctx).Errorf("failed to write tool policy for %s/%s: %v", preview.PluginID, tool.Name, err)
			return nil, s.compensateInstallation(ctx, tenantID, installation.ID, materialized.ID, ErrInstallationMaterializeFailed)
		}
	}

	// Step 7: consume the preview exactly once. A lost race here surfaces
	// as ErrRecordNotFound — the same verdict as the friendly check above.
	if err := s.pluginRepo.MarkPreviewConsumed(ctx, tenantID, previewID); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			err = ErrPreviewAlreadyConsumed
		}
		logger.GetLogger(ctx).Errorf("failed to consume plugin preview: %v", err)
		return nil, s.compensateInstallation(ctx, tenantID, installation.ID, materialized.ID, err)
	}

	return s.installationResult(ctx, tenantID, installation)
}

// compensateInstallation rolls a failed confirm back to zero: soft-delete
// the materialized service (plugin-derived rows carry no history value)
// and hard-delete the installation row so the (tenant, plugin) unique slot
// is free. Compensation is best-effort — its own failures are logged, and
// the ORIGINAL cause is what the caller sees.
func (s *pluginService) compensateInstallation(
	ctx context.Context,
	tenantID uint64,
	installationID, serviceID string,
	cause error,
) error {
	if serviceID != "" {
		if err := s.mcpServiceRepo.Delete(ctx, tenantID, serviceID); err != nil {
			logger.GetLogger(ctx).Errorf(
				"plugin installation compensation: failed to remove materialized service %s: %v", serviceID, err)
		}
	}
	if err := s.pluginRepo.DeleteInstallation(ctx, tenantID, installationID); err != nil {
		logger.GetLogger(ctx).Errorf(
			"plugin installation compensation: failed to remove installation %s: %v", installationID, err)
	}
	return cause
}

// SetInstallationState flips an installation between active and disabled
// and syncs the materialized MCP service's Enabled flag. The service row's
// UpdatedAt refresh matters: the manager caches clients keyed by config,
// and the refreshed timestamp invalidates them so a disabled installation
// stops serving on the next registration pass, not "eventually".
func (s *pluginService) SetInstallationState(
	ctx context.Context,
	tenantID uint64,
	installationID, state string,
) (*types.PluginInstallationResult, error) {
	if state != types.PluginInstallationActive && state != types.PluginInstallationDisabled {
		return nil, fmt.Errorf("%w: %q (want %q or %q)",
			ErrInstallationStateInvalid, state, types.PluginInstallationActive, types.PluginInstallationDisabled)
	}
	inst, err := s.pluginRepo.GetInstallation(ctx, tenantID, installationID)
	if err != nil {
		logger.GetLogger(ctx).Errorf("failed to load plugin installation: %v", err)
		return nil, ErrInstallationPersistFailed
	}
	if inst == nil {
		return nil, ErrInstallationNotFound
	}
	if err := s.pluginRepo.UpdateInstallationState(ctx, tenantID, installationID, state); err != nil {
		logger.GetLogger(ctx).Errorf("failed to update plugin installation state: %v", err)
		return nil, ErrInstallationPersistFailed
	}
	inst.State = state

	if inst.ServiceID != "" {
		svc, err := s.mcpServiceRepo.GetByID(ctx, tenantID, inst.ServiceID)
		if err != nil {
			logger.GetLogger(ctx).Errorf("failed to load materialized service for state sync: %v", err)
			return nil, ErrInstallationPersistFailed
		}
		if svc != nil {
			svc.Enabled = state == types.PluginInstallationActive
			svc.UpdatedAt = time.Now()
			if err := s.mcpServiceRepo.Update(ctx, svc); err != nil {
				logger.GetLogger(ctx).Errorf("failed to sync materialized service state: %v", err)
				return nil, ErrInstallationPersistFailed
			}
		}
		// svc == nil: materialized row already gone — the installation
		// state itself is authoritative; nothing to sync.
	}

	return s.installationResult(ctx, tenantID, inst)
}

// ListInstallations returns the member-facing summaries of the tenant's
// installations. No snapshot payload and no endpoint echo — members see
// WHAT is installed and its state, not the verified directory detail.
func (s *pluginService) ListInstallations(
	ctx context.Context,
	tenantID uint64,
) ([]*types.PluginInstallationSummary, error) {
	installations, err := s.pluginRepo.ListInstallationsByTenant(ctx, tenantID)
	if err != nil {
		logger.GetLogger(ctx).Errorf("failed to list plugin installations: %v", err)
		return nil, ErrInstallationPersistFailed
	}
	summaries := make([]*types.PluginInstallationSummary, 0, len(installations))
	for _, inst := range installations {
		summaries = append(summaries, installationSummary(inst))
	}
	return summaries, nil
}

// GetInstallation returns one installation's full view within the tenant;
// a foreign tenant's ID reads as not found (no existence leak).
func (s *pluginService) GetInstallation(
	ctx context.Context,
	tenantID uint64,
	installationID string,
) (*types.PluginInstallationResult, error) {
	inst, err := s.pluginRepo.GetInstallation(ctx, tenantID, installationID)
	if err != nil {
		logger.GetLogger(ctx).Errorf("failed to load plugin installation: %v", err)
		return nil, ErrInstallationPersistFailed
	}
	if inst == nil {
		return nil, ErrInstallationNotFound
	}
	return s.installationResult(ctx, tenantID, inst)
}

// installationResult assembles the full view: snapshot metadata per tool,
// with the CURRENT policy verdict merged in when an explicit approval row
// exists. Reading policies is best-effort — a policy-store fault degrades
// to Enabled=nil (unknown), never fails the whole view.
func (s *pluginService) installationResult(
	ctx context.Context,
	tenantID uint64,
	inst *types.PluginInstallation,
) (*types.PluginInstallationResult, error) {
	enabledByTool := map[string]*bool{}
	if inst.ServiceID != "" {
		if svc, err := s.mcpServiceRepo.GetByID(ctx, tenantID, inst.ServiceID); err == nil && svc != nil {
			if rows, err := s.toolApprovalService.ListByService(ctx, tenantID, inst.ServiceID); err == nil {
				for _, row := range rows {
					enabled := row.Enabled
					enabledByTool[row.ToolName] = &enabled
				}
			}
		}
	}

	tools := make([]types.PluginInstallationToolView, 0, len(inst.ToolsSnapshot))
	for _, tool := range inst.ToolsSnapshot {
		scopes := make([]string, len(tool.Scopes))
		copy(scopes, tool.Scopes)
		view := types.PluginInstallationToolView{
			Name:                 tool.Name,
			Description:          tool.Description,
			ReadOnly:             tool.ReadOnly,
			RequiresPersonalAuth: tool.RequiresPersonalAuth,
			Scopes:               scopes,
			Enabled:              enabledByTool[tool.Name],
		}
		tools = append(tools, view)
	}

	return &types.PluginInstallationResult{
		InstallationID: inst.ID,
		PluginID:       inst.PluginID,
		Name:           inst.Name,
		Description:    inst.Description,
		Version:        inst.AcceptedVersion,
		State:          inst.State,
		DriftState:     inst.DriftState,
		TransportType:  inst.TransportType,
		EndpointURL:    inst.EndpointURL,
		ServiceID:      inst.ServiceID,
		Tools:          tools,
	}, nil
}

// installationSummary maps one installation onto its member-facing row.
func installationSummary(inst *types.PluginInstallation) *types.PluginInstallationSummary {
	requiresPersonalAuth := false
	for _, tool := range inst.ToolsSnapshot {
		if tool.RequiresPersonalAuth {
			requiresPersonalAuth = true
			break
		}
	}
	return &types.PluginInstallationSummary{
		InstallationID:       inst.ID,
		PluginID:             inst.PluginID,
		Name:                 inst.Name,
		Version:              inst.AcceptedVersion,
		State:                inst.State,
		DriftState:           inst.DriftState,
		RequiresPersonalAuth: requiresPersonalAuth,
		ToolCount:            len(inst.ToolsSnapshot),
	}
}
