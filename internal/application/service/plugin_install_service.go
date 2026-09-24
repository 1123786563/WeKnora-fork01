package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/Tencent/WeKnora/internal/application/repository"
	"github.com/Tencent/WeKnora/internal/logger"
	"github.com/Tencent/WeKnora/internal/modules/agentruntime/agent/tools"
	"github.com/Tencent/WeKnora/internal/modules/plugins"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
	"gorm.io/gorm"
)

// MCPClientCloser closes the MCPManager's cached client connection(s) for
// one service (T06-OCR1-F7/F13). The manager caches SSE/long-lived clients
// keyed by serviceID (plus per-principal variants for OAuth), and cleanup
// only evicts dead ones — a hard-deleted or disabled service would keep its
// live connection, holding member tokens in memory, until process restart.
// The composition root injects the real closer over the airesource
// MCPManager (same seam pattern as EndpointLister); tests inject counters.
type MCPClientCloser func(serviceID string)

// closeServiceClient is the nil-safe wrapper: a closer-less wiring (preview
// tests) simply skips; the close itself is best-effort — the manager's own
// CloseClient already swallows "not connected".
func (s *pluginService) closeServiceClient(ctx context.Context, serviceID string) {
	if s.clientCloser == nil || serviceID == "" {
		return
	}
	s.clientCloser(serviceID)
	logger.GetLogger(ctx).Infof("plugin service client closed: %s", serviceID)
}

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

	// ErrPluginVerifyFailed: the confirm-time re-verification rejected the
	// remote document (invalid manifest, declaration mismatch) — a
	// deterministic 4xx rejection of the confirmed input, mirroring the
	// preview path. Fetch faults keep their own 503 sentinel.
	ErrPluginVerifyFailed = errors.New("plugin verification failed")

	// ErrConnectionQueryFailed (T11): 5xx fault while reading the member's
	// OAuth token row for the connection view. Token details never travel
	// with the error — only the server log sees the driver text.
	ErrConnectionQueryFailed = errors.New("failed to query plugin connection status")

	// ErrConnectionPrincipalRequired (T11): the caller's principal context
	// is missing/invalid — the connection view is inherently per-identity.
	ErrConnectionPrincipalRequired = errors.New("principal context is required to query plugin connection status")
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
	// document and must go through a fresh preview. Verification faults are
	// wrapped in a 4xx-class sentinel (fetch faults keep ErrManifestFetchFailed
	// → 503; OAuth-protected keeps its own marker) so the handler's default
	// branch can stay conservatively 5xx (OCR round-1 R12 F01).
	result, err := plugins.FetchAndVerify(ctx, preview.ManifestURL, s.lister)
	if err != nil {
		if errors.Is(err, plugins.ErrManifestFetchFailed) || plugins.IsOAuthProtected(err) {
			return nil, err
		}
		return nil, fmt.Errorf("%w: %v", ErrPluginVerifyFailed, err)
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

	// Step 5: installation row. A unique-index conflict means a concurrent
	// confirm of the same plugin won the race — surface the 409 semantics,
	// never a blanket 500 (OCR round-1 R12 F15).
	if err := s.pluginRepo.CreateInstallation(ctx, installation); err != nil {
		if errors.Is(err, repository.ErrInstallationDuplicateKey) {
			return nil, fmt.Errorf("%w: %q was installed concurrently", ErrPluginAlreadyInstalled, preview.PluginID)
		}
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
	// The materialized AuthConfig is derived from the VERIFIED baseline
	// (the preview's tool-level declarations), never from the fresh
	// manifest's auth block (跨任务转交 T01-OCR1-F3): manifest-level
	// auth.personal_oauth / auth.scopes enter neither ToolsDigest nor
	// IdentityFingerprint, so the remote can flip or escalate them inside
	// the preview TTL window with an unchanged tool directory — the
	// confirm guard would still pass. Trusting the fresh value would let a
	// plugin author silently widen members' personal OAuth scope after the
	// admin's review; spec line 49: a manifest's auth declaration alone is
	// never execution authorization. The tool-level declarations ARE
	// digest-covered, so the baseline below is exactly what the admin
	// reviewed.
	materialized.AuthConfig = oauthConfigFromVerifiedBaseline(preview.ToolsSnapshot)
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
	// as ErrRecordNotFound — re-read the preview to distinguish "already
	// consumed" from "crossed the TTL boundary mid-confirm" (OCR round-1
	// R12 F17: a fresh 15-minute TTL can expire between step 2 and here,
	// across the remote re-verification); any other repo fault is wrapped
	// in the 5xx sentinel so the handler never sees raw driver text
	// (R12 F01).
	if err := s.pluginRepo.MarkPreviewConsumed(ctx, tenantID, previewID); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			err = s.reclassifyPreviewMiss(ctx, tenantID, previewID)
		} else {
			logger.GetLogger(ctx).Errorf("failed to consume plugin preview: %v", err)
			err = ErrInstallationPersistFailed
		}
		return nil, s.compensateInstallation(ctx, tenantID, installation.ID, materialized.ID, err)
	}

	return s.installationResult(ctx, tenantID, installation)
}

// reclassifyPreviewMiss disambiguates a MarkPreviewConsumed zero-row update
// (OCR round-1 R12 F17): consumed-at-set → already consumed; anything else
// (expiry crossed mid-confirm, or the row vanished) reads as expired. The
// verdict never misreports an unconsumed-but-expired preview as consumed.
func (s *pluginService) reclassifyPreviewMiss(ctx context.Context, tenantID uint64, previewID string) error {
	preview, err := s.pluginRepo.GetPreview(ctx, tenantID, previewID)
	if err != nil {
		// 重读故障是服务端瞬时故障——按 R12 F01 原则以 5xx 哨兵呈现，
		// 不得误读为确定性 400「已消费」判决（T07-OCR1 low）。
		logger.GetLogger(ctx).Errorf("failed to re-read plugin preview: %v", err)
		return ErrInstallationPersistFailed
	}
	// 行消失只可能来自 DeleteExpiredPreviews 的惰性清理（只删
	// expires_at <= now 的行，即必然已过期）——「已过期」才是准确判决，
	// 不得误报为已消费（T07-OCR1 low：对齐 R12 F17 契约 "row vanished
	// reads as expired"）。
	if preview == nil || preview.ConsumedAt == nil {
		return ErrPreviewExpired
	}
	return ErrPreviewAlreadyConsumed
}

// oauthConfigFromVerifiedBaseline derives the materialized service's OAuth
// config from the preview's verified tool-level declarations (跨任务转交
// T01-OCR1-F3): oauth is configured only when at least one reviewed tool
// requires personal auth, and the scopes are the union (first-seen order) of
// the scopes declared by tools that REQUIRE personal auth. Scopes carried by
// tools with requires_personal_auth=false (T06-OCR1-F8) are deliberately
// excluded: an unauthenticated tool's declared scopes must never widen the
// member's personal OAuth grant — a read-only tool smuggling write:xxx into
// the authorization request would violate the spec's least-privilege
// posture. With no tool-level requirement the result is nil — a fresh
// manifest auth flip is not a reviewed requirement.
func oauthConfigFromVerifiedBaseline(snapshot []types.PluginToolSnapshot) *types.MCPAuthConfig {
	requiresOAuth := false
	var scopes []string
	seen := map[string]bool{}
	for _, tool := range snapshot {
		if !tool.RequiresPersonalAuth {
			continue
		}
		requiresOAuth = true
		for _, s := range tool.Scopes {
			if !seen[s] {
				seen[s] = true
				scopes = append(scopes, s)
			}
		}
	}
	if !requiresOAuth {
		return nil
	}
	return &types.MCPAuthConfig{
		AuthType: types.MCPAuthOAuth,
		Scopes:   scopes,
	}
}

// compensateInstallation rolls a failed confirm back to zero: hard-cascade
// delete the materialized service WITH its derived policy rows (R12 F21 —
// the shared soft delete would orphan approvals keyed to a dead serviceID)
// and hard-delete the installation row so the (tenant, plugin) unique slot
// is free. The compensation runs on a FRESH context derived via
// context.WithoutCancel with its own timeout (R12 F14): the original
// request ctx may already be cancelled (client disconnect mid-confirm) —
// reusing it would fail BOTH compensation writes and strand the unique
// slot with no self-heal. If the cascade delete itself fails, the
// installation row is KEPT as the self-heal anchor (T06-OCR2-F1): the
// sweep rolled back atomically, and dropping the row would strand the
// service row with no reachable cleanup — the uninstall endpoint is the
// operator's recovery path instead. Compensation failures are logged, the
// ORIGINAL cause is what the caller sees.
func (s *pluginService) compensateInstallation(
	ctx context.Context,
	tenantID uint64,
	installationID, serviceID string,
	cause error,
) error {
	compCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 15*time.Second)
	defer cancel()
	if serviceID != "" {
		if err := s.pluginRepo.HardDeleteServiceCascade(compCtx, tenantID, serviceID); err != nil {
			// Cascade failed → the sweep rolled back atomically (T06-OCR2-F1)
			// and the installation row is the ONLY self-heal anchor left:
			// the uninstall endpoint needs it (GetInstallation nil → not
			// found). Deleting it here would strand the materialized
			// service row forever with no reachable cleanup — keep the row,
			// surface the original cause; the operator uninstalls once the
			// sweep fault clears.
			logger.GetLogger(ctx).Errorf(
				"plugin installation compensation: cascade-delete of materialized service %s failed (%v); KEEPING installation %s as the self-heal anchor",
				serviceID, err, installationID)
			return cause
		}
		// The rows are gone — drop the manager's cached client too
		// (T06-OCR1-F7): a live SSE connection holding member tokens
		// must not outlive the service it pointed at.
		s.closeServiceClient(compCtx, serviceID)
	}
	if err := s.pluginRepo.DeleteInstallation(compCtx, tenantID, installationID); err != nil {
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
//
// Directional write order (OCR round-1 R12 F16): every failure face
// converges FAIL-CLOSED instead of relying on rollback. disable flips the
// SERVICE first — if the installation update then fails, tools are already
// invisible (the runtime only reads service.Enabled) and a retry heals.
// enable flips the INSTALLATION first — if the service sync then fails,
// the service stays disabled (fail-closed) while the installation reads
// active, and a retry completes it. The unreachable state is exactly the
// dangerous one: installation=disabled with service Enabled=true.
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

	syncService := func(enabled bool) error {
		if inst.ServiceID == "" {
			return nil
		}
		svc, err := s.mcpServiceRepo.GetByID(ctx, tenantID, inst.ServiceID)
		if err != nil {
			logger.GetLogger(ctx).Errorf("failed to load materialized service for state sync: %v", err)
			return ErrInstallationPersistFailed
		}
		if svc == nil {
			// Materialized row already gone — the installation state itself
			// is authoritative; nothing to sync.
			return nil
		}
		svc.Enabled = enabled
		svc.UpdatedAt = time.Now()
		if err := s.mcpServiceRepo.Update(ctx, svc); err != nil {
			logger.GetLogger(ctx).Errorf("failed to sync materialized service state: %v", err)
			return ErrInstallationPersistFailed
		}
		// The DB row moved — recycle the cached client (T06-OCR1-F13): the
		// manager only re-reads config when a caller asks for a client, and
		// a disabled service's live SSE connection (per-principal OAuth
		// variants included) would otherwise linger until process restart.
		// Manual UpdateMCPService closes on BOTH transitions for the same
		// reason (enable gets a clean reconnect) — mirror it.
		s.closeServiceClient(ctx, inst.ServiceID)
		return nil
	}
	flipInstallation := func() error {
		if err := s.pluginRepo.UpdateInstallationState(ctx, tenantID, installationID, state); err != nil {
			logger.GetLogger(ctx).Errorf("failed to update plugin installation state: %v", err)
			return ErrInstallationPersistFailed
		}
		inst.State = state
		return nil
	}

	if state == types.PluginInstallationDisabled {
		// disable: service first, installation second.
		if err := syncService(false); err != nil {
			return nil, err
		}
		if err := flipInstallation(); err != nil {
			return nil, err
		}
	} else {
		// enable: installation first, service second.
		if err := flipInstallation(); err != nil {
			return nil, err
		}
		if err := syncService(true); err != nil {
			return nil, err
		}
	}

	return s.installationResult(ctx, tenantID, inst)
}

// UninstallInstallation removes an installation entirely (OCR round-1 R12
// F06b): the materialized service and its derived policy rows are
// hard-cascade-deleted first, then the installation row — releasing the
// (tenant, plugin) unique slot. Service-first ordering keeps every failure
// face safe: if the installation delete fails after the service is gone,
// the row remains (retryable) and the orphan-less guard (T09 F20)
// fail-closes the missing-snapshot case.
func (s *pluginService) UninstallInstallation(
	ctx context.Context,
	tenantID uint64,
	installationID string,
) error {
	inst, err := s.pluginRepo.GetInstallation(ctx, tenantID, installationID)
	if err != nil {
		logger.GetLogger(ctx).Errorf("failed to load plugin installation: %v", err)
		return ErrInstallationPersistFailed
	}
	if inst == nil {
		return ErrInstallationNotFound
	}
	if inst.ServiceID == "" {
		// T07-OCR1-F5 自愈锚兜底：confirm 在 CreateMCPService 之后、
		// UpdateInstallationServiceID 持久化绑定之前中断（进程崩溃，或
		// 绑定更新失败且补偿级联同样失败而保留安装行）时，安装行
		// service_id 为空而物化服务行已在。按
		// mcp_services.plugin_installation_id 反查孤儿并入级联——否则卸载
		// 会删掉锚行、留下 Enabled=true 的孤儿服务（运行时守卫查不到
		// 安装行会按非插件服务处理，未接受工具目录继续暴露，且孤儿占用
		// (tenant_id, name='plugin:<plugin_id>') 唯一索引阻断重装）。
		// 反查故障 fail-closed：保留锚行供重试自愈。
		resolved, resolveErr := s.serviceIDByInstallation(ctx, tenantID, installationID)
		if resolveErr != nil {
			logger.GetLogger(ctx).Errorf("failed to resolve orphan materialized service for installation %s: %v", installationID, resolveErr)
			return ErrInstallationPersistFailed
		}
		if resolved != "" {
			logger.GetLogger(ctx).Infof("uninstall healing empty service_id anchor: installation %s resolves to orphan service %s", installationID, resolved)
			inst.ServiceID = resolved
		}
	}
	if inst.ServiceID != "" {
		if err := s.pluginRepo.HardDeleteServiceCascade(ctx, tenantID, inst.ServiceID); err != nil {
			logger.GetLogger(ctx).Errorf("failed to cascade-delete materialized service: %v", err)
			return ErrInstallationPersistFailed
		}
		// Rows gone → cached client goes too (T06-OCR1-F7): same
		// close-before-return discipline as the manual DeleteMCPService.
		s.closeServiceClient(ctx, inst.ServiceID)
	}
	if err := s.pluginRepo.DeleteInstallation(ctx, tenantID, installationID); err != nil {
		logger.GetLogger(ctx).Errorf("failed to delete plugin installation: %v", err)
		return ErrInstallationPersistFailed
	}
	return nil
}

// serviceIDByInstallation resolves the materialized service bound to an
// installation through the mcp_services.plugin_installation_id back-reference
// — the self-heal lookup for the interrupted-confirm window where the
// installation row's service_id never landed (T07-OCR1-F5). Scopes to the
// tenant via the repository's tenant-scoped List.
func (s *pluginService) serviceIDByInstallation(
	ctx context.Context,
	tenantID uint64,
	installationID string,
) (string, error) {
	// nil 装配防御（最小测试夹具形态，如 connection_status_test 的
	// mcpServiceRepo=nil 装配）：无可反查的存储时按「无孤儿」处理，
	// 调用方保持既有空 service_id 行为，绝不 panic。
	if s.mcpServiceRepo == nil {
		return "", nil
	}
	services, err := s.mcpServiceRepo.List(ctx, tenantID)
	if err != nil {
		return "", err
	}
	for _, svc := range services {
		if svc.PluginInstallationID != nil && *svc.PluginInstallationID == installationID {
			return svc.ID, nil
		}
	}
	return "", nil
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

// GetMyConnectionStatus returns ONE principal's personal connection view of
// one installation (T11, GAP-4): the tenant-scoped installation lookup, the
// personal-auth requirement derived from the ACCEPTED tool snapshot (never
// the live remote), and the three-state verdict over the per-principal token
// stored for the materialized service — the plugin domain reuses the MCP
// OAuth storage keyed by (tenant, principal, service_id), it owns no token
// table of its own. AuthorizeURLPath/RevokePath map onto the materialized
// service_id's LEGACY MCP OAuth endpoints (fixed concatenation); the result
// carries no token material.
func (s *pluginService) GetMyConnectionStatus(
	ctx context.Context,
	tenantID uint64,
	installationID string,
	principal types.Principal,
) (*types.PluginMyConnection, error) {
	principal = principal.Normalize()
	if !principal.Valid() {
		return nil, ErrConnectionPrincipalRequired
	}

	inst, err := s.pluginRepo.GetInstallation(ctx, tenantID, installationID)
	if err != nil {
		// 读路径故障用连接域读语义哨兵（mapPluginConnectionError 与
		// ErrInstallationPersistFailed 同落 5xx，仅日志归类更准确）。
		logger.GetLogger(ctx).Errorf("failed to load plugin installation: %v", err)
		return nil, ErrConnectionQueryFailed
	}
	if inst == nil {
		return nil, ErrInstallationNotFound
	}
	if inst.ServiceID == "" {
		// T07-OCR2-F3 中断窗口自愈（与 UninstallInstallation 的
		// T07-OCR1-F5 同款）：confirm 在 CreateMCPService 之后、
		// UpdateInstallationServiceID 持久化之前中断时，安装行
		// service_id 为空而物化服务行（及可能已存的成员令牌）已在——
		// 直接以空 service_id 查询必然无命中，成员会看到 unauthorized
		// 且授权/撤销路径皆空的死端视图。按
		// mcp_services.plugin_installation_id 反查孤儿后照常给出三态与
		// 端点路径；反查故障 fail-closed（连接域 5xx 哨兵）。
		resolved, resolveErr := s.serviceIDByInstallation(ctx, tenantID, installationID)
		if resolveErr != nil {
			logger.GetLogger(ctx).Errorf("failed to resolve orphan materialized service for installation %s: %v", installationID, resolveErr)
			return nil, ErrConnectionQueryFailed
		}
		if resolved != "" {
			logger.GetLogger(ctx).Infof("connection status healing empty service_id anchor: installation %s resolves to orphan service %s", installationID, resolved)
			inst.ServiceID = resolved
		}
	}

	conn := &types.PluginMyConnection{
		InstallationID:    inst.ID,
		PluginID:          inst.PluginID,
		Name:              inst.Name,
		ServiceID:         inst.ServiceID,
		Authorized:        true,
		State:             types.PluginConnectionAuthorized,
		RequiresAuthTools: []string{},
	}
	for _, tool := range inst.ToolsSnapshot {
		if tool.RequiresPersonalAuth {
			conn.RequiresPersonalAuth = true
			conn.RequiresAuthTools = append(conn.RequiresAuthTools, tool.Name)
		}
	}
	if !conn.RequiresPersonalAuth {
		// No personal OAuth in the accepted snapshot — nothing to authorize,
		// the connection is trivially usable; no endpoint paths to offer.
		return conn, nil
	}
	if inst.ServiceID != "" {
		conn.AuthorizeURLPath = "/api/v1/mcp-services/" + inst.ServiceID + "/oauth/authorize-url"
		conn.RevokePath = "/api/v1/mcp-services/" + inst.ServiceID + "/oauth/token"
	}

	if s.oauthRepo == nil {
		// Wiring fault (production always injects via dig): fail loudly
		// rather than reporting a misleading unauthorized.
		logger.GetLogger(ctx).Errorf("plugin connection status: oauth repository is not wired")
		return nil, ErrConnectionQueryFailed
	}
	token, err := s.oauthRepo.GetTokenForPrincipal(ctx, tenantID, principal, inst.ServiceID)
	if err != nil {
		logger.GetLogger(ctx).Errorf("failed to load member oauth token for plugin connection: %v", err)
		return nil, ErrConnectionQueryFailed
	}
	switch {
	case token == nil || token.AccessToken == "":
		conn.State = types.PluginConnectionUnauthorized
		conn.Authorized = false
	case token.ExpiresAt.IsZero() || token.ExpiresAt.After(time.Now()):
		// usable now
	default:
		if token.RefreshToken == "" {
			// Expired with no refresh token: only a NEW member consent
			// recovers — the UI guides re-authorization.
			conn.State = types.PluginConnectionExpired
			conn.Authorized = false
		}
		// Expired WITH a refresh token stays authorized: the runtime
		// renews under the member's existing consent (oauthRuntime.
		// ensureFresh), so the member's connection is live.
	}
	return conn, nil
}
