package service

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/Tencent/WeKnora/internal/application/repository"
	"github.com/Tencent/WeKnora/internal/logger"
	"github.com/Tencent/WeKnora/internal/modules/agentruntime/agent/tools"
	"github.com/Tencent/WeKnora/internal/modules/airesource/mcp"
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

// PluginSnapshotLookupWithDriftMarking wraps the T09 guard provider with the
// T17 best-effort drift marker (plan 09 Architecture: "运行时目录加载检测到
// 差异时 best-effort 置位……由 provider 闭包调用，失败仅日志"). Whenever the
// guard resolves an installation whose persisted state is not yet "detected",
// the provider LIVE-lists the accepted endpoint (the drift truth source —
// never the manifest) and persists drift_state=detected + the name-only
// detail when the directory deviates from the accepted snapshot. The whole
// detection is BEST-EFFORT: any fault (endpoint unreachable, JSON encode,
// persistence) is logged and swallowed — the provider's return contract is
// exactly PluginSnapshotLookup's (snapshot or fail-closed repository error);
// a detection failure must never block or fail a member's directory load.
// Manual services resolve to (nil, nil) untouched — they have no drift
// concept. A nil lister degrades to the plain lookup.
//
// Cost shape: the extra ListTools runs once per directory load while the row
// is not yet marked detected (the T09 load path re-lists the endpoint live on
// every load anyway, so this at most doubles a cost already being paid), and
// never again once the state sticks — CheckDrift/ResolveDrift are the
// authoritative transitions. For rows that STAY healthy the marker is
// additionally throttled per installation (OCR 终局 F11): a healthy row would
// otherwise re-run the nonce-exclusive handshake + live ListTools on EVERY
// member directory load, serially ahead of the load path's own listing — a
// standing 2× connection/ ListTools amplification against the third-party
// endpoint and a worst-case +10s on session assembly. One probe per window is
// plenty for a best-effort marker; the authoritative CheckDrift is untouched.
func PluginSnapshotLookupWithDriftMarking(
	repo interfaces.PluginRepository, lister plugins.EndpointLister,
) tools.PluginSnapshotProvider {
	if lister == nil {
		return PluginSnapshotLookup(repo)
	}
	return func(ctx context.Context, tenantID uint64, serviceID string) (*tools.PluginRuntimeSnapshot, error) {
		inst, err := repo.GetByServiceID(ctx, tenantID, serviceID)
		if err != nil {
			return nil, err
		}
		if inst == nil {
			return nil, nil
		}
		if inst.DriftState != types.PluginDriftDetected && driftMarkDue(inst.ID) {
			markDriftBestEffort(ctx, repo, lister, tenantID, inst)
		}
		return &tools.PluginRuntimeSnapshot{
			InstallationID: inst.ID,
			Tools:          inst.ToolsSnapshot,
		}, nil
	}
}

// driftMarkCheckInterval is the per-installation throttle window for the
// best-effort health probe above (var so tests can compress it). Detected
// rows skip the probe entirely and are never throttled from anything.
var driftMarkCheckInterval = 5 * time.Minute

// driftMarkLastChecked records the last probe time per installation
// (installationID → time.Time). Entries are never evicted — one timestamp per
// installation that ever loaded a directory, negligible (the
// upgradeAcceptMutexes precedent). In-process state like the mutex family:
// multi-replica deployments throttle per replica, which a best-effort marker
// accommodates.
var driftMarkLastChecked sync.Map

// driftMarkDue reports whether the healthy-row probe for installationID is
// outside its throttle window, recording this probe attempt when it is. The
// snapshot supply itself is NEVER throttled — only the extra live listing is.
func driftMarkDue(installationID string) bool {
	now := time.Now()
	if v, ok := driftMarkLastChecked.Load(installationID); ok {
		if last, ok := v.(time.Time); ok && now.Sub(last) < driftMarkCheckInterval {
			return false
		}
	}
	driftMarkLastChecked.Store(installationID, now)
	return true
}

// SetDriftMarkCheckIntervalForTest overrides the marker's throttle window and
// returns the restore func (test-only seam, the SnapshotSSRFWhitelistForTest
// convention; production callers never touch it).
func SetDriftMarkCheckIntervalForTest(d time.Duration) func() {
	prev := driftMarkCheckInterval
	driftMarkCheckInterval = d
	return func() { driftMarkCheckInterval = prev }
}

// markDriftBestEffort runs the T17 runtime-side detection for one
// installation: live-list the accepted endpoint, diff against the accepted
// snapshot, and persist the detected verdict. Every failure is logged and
// swallowed (best-effort by contract — see PluginSnapshotLookupWithDriftMarking).
func markDriftBestEffort(
	ctx context.Context,
	repo interfaces.PluginRepository,
	lister plugins.EndpointLister,
	tenantID uint64,
	inst *types.PluginInstallation,
) {
	listCtx, cancel := context.WithTimeout(ctx, driftMarkListTimeout)
	defer cancel()
	live, err := lister(listCtx, inst.TransportType, inst.EndpointURL)
	if err != nil {
		// Unreachable/deferred endpoints surface through CheckDrift (the
		// admin lever) and the load path's own ListTools — the marker stays
		// silent.
		return
	}
	detail, err := plugins.DiffLiveAgainstSnapshot(live, inst.ToolsSnapshot)
	if err != nil {
		// OCR R1 F15: an unvetted live directory cannot yield a drift
		// verdict — the best-effort marker stays silent (same posture as the
		// load-path failure above), never persists unvetted names.
		logger.GetLogger(ctx).Errorf("plugin drift marker: cannot vet live directory for installation %s: %v", inst.ID, err)
		return
	}
	if !detail.HasDrift() {
		return
	}
	raw, err := json.Marshal(detail)
	if err != nil {
		logger.GetLogger(ctx).Errorf("plugin drift marker: failed to encode detail for installation %s: %v", inst.ID, err)
		return
	}
	// Baseline-guarded write (T17-OCR1-F2): the marker runs unserialized, so
	// the verdict lands only while the row still carries the baseline it was
	// computed against — a concurrently rebased row (accept/resolve) makes
	// the precondition miss and the stale verdict is dropped, never written.
	writer, ok := repo.(installationDriftMarker)
	if !ok {
		logger.GetLogger(ctx).Errorf(
			"plugin drift marker: repository %T does not implement installationDriftMarker", repo)
		return
	}
	applied, err := writer.UpdateDriftIfToolsDigest(ctx, tenantID, inst.ID, inst.ToolsDigest, types.PluginDriftDetected, raw)
	if err != nil {
		logger.GetLogger(ctx).Errorf(
			"plugin drift marker: failed to persist detected state for installation %s: %v", inst.ID, err)
		return
	}
	if !applied {
		logger.GetLogger(ctx).Infof(
			"plugin drift marker: baseline for installation %s moved before the verdict landed — dropping the stale detection",
			inst.ID)
		return
	}
	logger.GetLogger(ctx).Infof(
		"plugin drift detected at runtime: installation %s (tenant %d) added=%v removed=%v schema_changed=%v description_changed=%v",
		inst.ID, tenantID, detail.Added, detail.Removed, detail.SchemaChanged, detail.DescriptionChanged)
}

// driftMarkListTimeout bounds the best-effort marker's LIVE ListTools: the
// marker runs inside the directory-load path, and a slow endpoint must not
// add unbounded latency to a member's session assembly (the authoritative
// CheckDrift keeps its own handling).
const driftMarkListTimeout = 10 * time.Second

// NewManagerEndpointLister adapts the airesource MCPManager onto the plugins
// EndpointLister seam for the runtime drift marker (T17): each call verifies
// against a NONCE-EXCLUSIVE throwaway service identity (globally unique ID,
// disconnected and evicted on return) so the marker's listing can never alias
// or evict a live session's cached client. This is the service-package twin
// of container.NewPluginMCPEndpointLister — internal/modules must not import
// the container assembly, and agent_service (this package) needs the lister
// at the provider wiring point without a constructor-signature change.
func NewManagerEndpointLister(manager *mcp.MCPManager) plugins.EndpointLister {
	return func(ctx context.Context, transportType, endpointURL string) ([]*types.MCPTool, error) {
		if manager == nil {
			return nil, fmt.Errorf("mcp manager is required")
		}
		var nonce [4]byte
		if _, err := rand.Read(nonce[:]); err != nil {
			return nil, fmt.Errorf("generate verification nonce: %w", err)
		}
		sum := sha256.Sum256([]byte(endpointURL))
		serviceID := "plugin-drift-" + hex.EncodeToString(sum[:8]) + "-" + hex.EncodeToString(nonce[:])
		verify := &types.MCPService{
			ID:            serviceID,
			Name:          "plugin-drift-verify",
			Enabled:       true,
			TransportType: types.MCPTransportType(transportType),
			URL:           &endpointURL,
		}
		client, err := manager.GetOrCreateClient(ctx, verify)
		if err != nil {
			// Retire the nonce key on the error path too: a client that
			// finishes connecting after this call returned would otherwise
			// sit mounted under a globally unique key nothing references
			// again (idle cleanup only removes !IsConnected entries).
			_ = manager.CloseClient(verify.ID)
			return nil, err
		}
		defer func() {
			_ = client.Disconnect()
			_ = manager.CloseClient(verify.ID)
		}()
		return client.ListTools(ctx)
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

	// ErrUpgradeCandidateChanged (T16): the candidate at the installation's
	// long-lived manifest source no longer matches the fingerprint the admin
	// previewed — the remote moved on between preview and accept. The accept
	// is rejected with ZERO writes; the admin's path is a fresh preview.
	ErrUpgradeCandidateChanged = errors.New("candidate changed since preview")

	// ErrUpgradeWriterNotWired (T16): the injected plugin repository does not
	// implement the installationUpgradeWriter capability — a wiring fault
	// (production always injects the gorm repository). Fail loudly rather
	// than reporting a misleading success or silently skipping persistence.
	ErrUpgradeWriterNotWired = errors.New("plugin upgrade persistence is not wired")

	// ErrDriftEndpointUnreachable (T17): the LIVE ListTools against the
	// installation's ACCEPTED endpoint failed while checking or resolving
	// drift — the remote truth source the drift verdict is defined over is
	// not observable right now. Both CheckDrift and ResolveDrift surface it
	// with ZERO writes: a verdict (or a rebase onto a "current" directory)
	// must never be minted from an endpoint that cannot be read.
	ErrDriftEndpointUnreachable = errors.New("plugin drift check: accepted endpoint unreachable")

	// ErrDriftPersistFailed (T17): 5xx persistence fault on the drift path —
	// the UpdateDrift capability write failed, or the injected repository
	// does not implement the capability at all (a wiring fault; production
	// always injects the gorm repository). Logged with the cause server-side;
	// the caller sees the sentinel semantics only.
	ErrDriftPersistFailed = errors.New("failed to persist plugin drift state")

	// ErrInstallationToolNotFound (T18): the tool name is not in the
	// installation's ACCEPTED snapshot — either never declared or removed by
	// an upgrade/drift-resolve (whose residual policy row stays
	// unaddressable; the snapshot is the membership authority). Deterministic
	// 4xx rejection of the addressed tool.
	ErrInstallationToolNotFound = errors.New("plugin installation tool not found")

	// ErrInstallationPolicyInvalid (T18): a policy patch that updates nothing
	// (enabled and requireApproval both nil). Deterministic 4xx rejection —
	// mirrors the manual MCP endpoint's "require_approval or enabled is
	// required".
	ErrInstallationPolicyInvalid = errors.New("plugin tool policy requires enabled or require_approval")

	// ErrInstallationServiceMissing (T18-OCR1-F4): the installation carries no
	// resolvable materialized service row — the interrupted-uninstall dangling
	// anchor (the cascade took the service row AND its policy rows; the
	// installation row stayed) after the empty-service_id self-heal found
	// nothing. There is no policy store to write against: a deterministic
	// state rejection (the operator's path is uninstall/re-install), never a
	// 5xx persistence fault.
	ErrInstallationServiceMissing = errors.New("plugin installation has no materialized service for tool policies")
)

// installationDriftWriter is the narrow persistence capability the drift
// slice needs (T17): persisting a drift verdict (state + detail document,
// nil clears) onto the installation row. The gorm pluginRepository implements
// it. Consumed via a type assertion on the injected PluginRepository rather
// than by extending that interface — the same additive-seam ruling as T16's
// installationUpgradeWriter: the T06-era contract and its in-tree test fakes
// predate the drift slice; repositories without the method fail LOUDLY here
// (ErrDriftPersistFailed) instead of at compile time.
type installationDriftWriter interface {
	// UpdateDrift persists one installation's drift state and detail in one
	// parameter-bound update; detail == nil clears the column.
	UpdateDrift(ctx context.Context, tenantID uint64, id, state string, detail json.RawMessage) error
}

// installationDriftMarker is the BASELINE-GUARDED persistence capability the
// best-effort runtime marker needs (T17-OCR1-F2): the marker runs OUTSIDE the
// accept/resolve serialization (it sits on the member directory-load path and
// must not hold the lock across a live ListTools), so its write carries the
// tools_digest it computed the verdict against as a precondition — one
// parameter-bound UPDATE ... WHERE tools_digest = ?. When the baseline moved
// (a concurrent CheckDrift/ResolveDrift/accept rebased the snapshot between
// the marker's read and its write), ZERO rows match and applied=false: the
// stale verdict (a detail naming tools outside the new baseline) never lands
// on the rebased row — the marker gives up silently, and the next load
// re-evaluates against the new baseline.
type installationDriftMarker interface {
	// UpdateDriftIfToolsDigest persists the drift verdict only while the
	// installation row's tools_digest still equals expectToolsDigest.
	// applied reports whether the row was written; a moved baseline is NOT
	// an error.
	UpdateDriftIfToolsDigest(
		ctx context.Context, tenantID uint64, id, expectToolsDigest, state string, detail json.RawMessage,
	) (applied bool, err error)
}

// upgradeAcceptMutexes serializes AcceptUpgrade per installation
// (T16-OCR2-F1): the WHOLE accept — from the installation read through the
// compensated writes — runs under one in-process mutex keyed by installation
// ID (the processCraftWorkspaceLock precedent, craft_snapshot.go). Without
// serialization, two concurrent accepts of the same installation interleave
// their ListByService snapshots and SetPolicy writes: a failing accept's
// compensation delete could remove a policy row ANOTHER accept had already
// landed, and with the missing-row default enabled=true (the runtime gate
// reads IsEnabled at call time, mcp_tool.go) plus require_approval default
// false until T18 lands, the deleted row re-exposes a WRITE tool of the
// accepted snapshot — a fail-open outcome. A keyed in-process mutex rather
// than a DB row lock: the accept spans a remote manifest fetch plus
// compensated writes across three stores, and holding a transaction/row lock
// across network I/O is the wrong shape. Scope: one process — the deployment
// model is one API replica per workspace; multi-replica deployments would
// need DB-level advisory locks (out of scope, noted). Entries are never
// evicted: one small mutex per installation that ever accepted, negligible
// for an admin-gated operation.
var upgradeAcceptMutexes sync.Map // installationID → *sync.Mutex

// lockUpgradeAccept acquires the per-installation accept mutex and returns
// the release func (defer at the accept's entry).
func lockUpgradeAccept(installationID string) func() {
	v, _ := upgradeAcceptMutexes.LoadOrStore(installationID, &sync.Mutex{})
	mu := v.(*sync.Mutex)
	mu.Lock()
	return mu.Unlock
}

// installationUpgradeWriter is the narrow persistence capability AcceptUpgrade
// needs (T16): persisting an accepted upgrade (or its compensation write-back)
// onto the installation row, and deleting the per-tool policy rows this
// accept created when a later step fails (T16-OCR1-F2 — the plugin domain
// owns its derived policy rows, HardDeleteServiceCascade discipline; the
// shared MCPToolApprovalRepository has no delete API). The gorm
// pluginRepository implements both. It is consumed via a type assertion on
// the injected PluginRepository rather than by extending that interface: the
// T06-era contract and its in-tree test fakes predate the upgrade slice, and
// the upgrade writes are additive methods — repositories without them fail
// LOUDLY here (ErrUpgradeWriterNotWired) instead of at compile time.
type installationUpgradeWriter interface {
	UpdateInstallationAccepted(ctx context.Context, tenantID uint64, id, acceptedVersion, endpointURL string, toolsSnapshot types.PluginPreviewTools, toolsDigest string) error

	// DeleteInstallationToolPolicies hard-deletes the policy rows this accept
	// wrote for toolNames (the compensation of the incremental 7c loop).
	DeleteInstallationToolPolicies(ctx context.Context, tenantID uint64, serviceID string, toolNames []string) error
}

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
	// B+A 裁决 #1 同族（ConfirmInstallation 同理）：确认写序为 安装行 →
	// 物化+绑定 → 策略行 → 消费预览；策略行写完前被硬杀（无补偿运行）的
	// 落库形态 = 安装行已在预览内容上（digest/版本/端点全等）+ 策略行缺失
	// + 预览未消费。对这一形态的同内容重试必须落入增量补齐（行完备时保持
	// 既有 already-installed 判定——完整安装上的重复确认不是崩溃窗口）。
	if existing, err := s.pluginRepo.GetInstallationByTenantPlugin(ctx, tenantID, preview.PluginID); err != nil {
		logger.GetLogger(ctx).Errorf("failed to load existing plugin installation: %v", err)
		return nil, ErrInstallationPersistFailed
	} else if existing != nil {
		if existing.ToolsDigest != preview.ToolsDigest ||
			existing.AcceptedVersion != preview.Version ||
			existing.EndpointURL != preview.EndpointURL {
			return nil, fmt.Errorf("%w: %q is already at version %s; use the upgrade path",
				ErrPluginAlreadyInstalled, preview.PluginID, existing.AcceptedVersion)
		}
		// 内容全等的既有行：崩溃窗口重试（或同内容新预览确认，落库状态
		// 不可区分）→ 尝试补齐；行已完备时维持 409 判定。
		return s.completeCrashedConfirm(ctx, tenantID, preview, existing)
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

	// OCR 终局 F10: serialize every post-create write against
	// uninstall/accept/state flips of the SAME installation. The row is
	// visible to UninstallInstallation the moment CreateInstallation
	// commits; without the per-installation mutex a concurrent uninstall's
	// cascade (materialized service + policy rows + the installation row)
	// interleaves with this flow's materialize/bind/policy/consume writes —
	// on PG an FK violation forces the compensation path (the admin sees a
	// spurious 500), on production SQLite (foreign_keys off) the rows land
	// as permanent orphans, and a mid-policy uninstall lets the confirm
	// consume the preview and return success against a deleted row. Same
	// lock family as UninstallInstallation/AcceptUpgrade/ResolveDrift/
	// CheckDrift/SetInstallationState; it does NOT span the Step-4 remote
	// re-verification (keep the critical section to the write sequence).
	unlockConfirm := lockUpgradeAccept(installation.ID)
	defer unlockConfirm()

	// In-lock recheck: the window between CreateInstallation's commit and
	// this lock is enough for a concurrent uninstall to have deleted the row
	// — nothing left to materialize onto. Fail closed instead of writing an
	// orphan service + policy rows against a dead installation.
	if current, err := s.pluginRepo.GetInstallation(ctx, tenantID, installation.ID); err != nil {
		logger.GetLogger(ctx).Errorf("failed to recheck plugin installation %s after locking: %v", installation.ID, err)
		return nil, ErrInstallationPersistFailed
	} else if current == nil {
		logger.GetLogger(ctx).Infof(
			"plugin installation %s was uninstalled concurrently during confirm; aborting before materialization", installation.ID)
		return nil, ErrInstallationNotFound
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

// completeCrashedConfirm heals the crash-window shape of a prior confirm of
// the SAME preview content (B+A 裁决 #1, ConfirmInstallation 同族): the prior
// attempt died between CreateInstallation and the policy loop (a hard kill —
// no compensation ran), leaving the row half-installed while the preview
// stayed unconsumed. The heal:
//   - rebinds an empty service_id anchor through the plugin_installation_id
//     reverse lookup (same anchor discipline as UninstallInstallation's
//     self-heal); an unresolvable anchor keeps the already-installed verdict —
//     no materialized row means nothing is serving (fail-closed), and
//     uninstall + re-confirm is the documented path;
//   - INCREMENTALLY fills the missing per-tool rows from the preview's
//     reviewed declarations (read exposed, WRITE TOOLS DISABLED — B5) —
//     existing verdicts are never touched; when the rows are already
//     complete the established duplicate-content 409 verdict stands;
//   - consumes the preview so the retry is terminal ("already consumed" from
//     a racing retry of the same preview reads as success — everything the
//     admin asked for has landed).
func (s *pluginService) completeCrashedConfirm(
	ctx context.Context,
	tenantID uint64,
	preview *types.PluginPreview,
	inst *types.PluginInstallation,
) (*types.PluginInstallationResult, error) {
	alreadyInstalled := func() error {
		return fmt.Errorf("%w: %q is already at version %s; use the upgrade path",
			ErrPluginAlreadyInstalled, preview.PluginID, inst.AcceptedVersion)
	}

	// OCR 终局 F10: the heal's writes (rebind / incremental policy rows /
	// preview consume) race the same uninstall cascade the confirm path
	// serializes against — same per-installation lock. The in-lock recheck
	// closes the gap between the caller's Step-3 read and this lock: a row
	// deleted by a concurrent uninstall in that window must fail closed, not
	// write orphan policy rows against the cascade-deleted service.
	defer lockUpgradeAccept(inst.ID)()
	current, err := s.pluginRepo.GetInstallation(ctx, tenantID, inst.ID)
	if err != nil {
		logger.GetLogger(ctx).Errorf(
			"confirm heal: failed to recheck installation %s after locking: %v", inst.ID, err)
		return nil, ErrInstallationPersistFailed
	}
	if current == nil {
		logger.GetLogger(ctx).Infof(
			"confirm heal: installation %s was uninstalled concurrently; failing closed", inst.ID)
		return nil, ErrInstallationNotFound
	}
	inst = current

	// Anchor heal: the crash between CreateMCPService and the service_id bind
	// leaves the materialized row orphaned-but-findable.
	if inst.ServiceID == "" {
		resolved, err := s.serviceIDByInstallation(ctx, tenantID, inst.ID)
		if err != nil {
			logger.GetLogger(ctx).Errorf(
				"confirm heal: failed to resolve orphan materialized service for installation %s: %v", inst.ID, err)
			return nil, ErrInstallationPersistFailed
		}
		if resolved == "" {
			// No materialized row at all: nothing serves, nothing to key
			// policy rows to — fail-closed shape, not the fail-open window.
			return nil, alreadyInstalled()
		}
		if err := s.pluginRepo.UpdateInstallationServiceID(ctx, tenantID, inst.ID, resolved); err != nil {
			logger.GetLogger(ctx).Errorf(
				"confirm heal: failed to rebind materialized service %s to installation %s: %v", resolved, inst.ID, err)
			return nil, ErrInstallationPersistFailed
		}
		inst.ServiceID = resolved
		logger.GetLogger(ctx).Infof(
			"confirm heal: rebound installation %s to orphan materialized service %s", inst.ID, resolved)
	}

	// Incremental rows — only tools WITHOUT an existing row land the
	// install-time verdict; a complete row set means this is NOT the crash
	// window and the duplicate-content verdict stands.
	rows, err := s.toolApprovalService.ListByService(ctx, tenantID, inst.ServiceID)
	if err != nil {
		logger.GetLogger(ctx).Errorf(
			"confirm heal: failed to load tool policies for installation %s: %v", inst.ID, err)
		return nil, ErrInstallationMaterializeFailed
	}
	existing := map[string]bool{}
	for _, row := range rows {
		existing[row.ToolName] = true
	}
	wrote := false
	for _, tool := range preview.ToolsSnapshot {
		if existing[tool.Name] {
			continue
		}
		enabled := tool.ReadOnly
		if err := s.toolApprovalService.SetPolicy(ctx, tenantID, inst.ServiceID, tool.Name, nil, &enabled); err != nil {
			logger.GetLogger(ctx).Errorf(
				"confirm heal: failed to write tool policy for %s/%s: %v", inst.PluginID, tool.Name, err)
			return nil, ErrInstallationMaterializeFailed
		}
		wrote = true
	}
	if !wrote {
		return nil, alreadyInstalled()
	}

	// Consume the preview: terminal for retries of this preview. A racing
	// retry that already consumed it means everything has landed — success.
	if err := s.pluginRepo.MarkPreviewConsumed(ctx, tenantID, preview.ID); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			if rerr := s.reclassifyPreviewMiss(ctx, tenantID, preview.ID); rerr != nil && !errors.Is(rerr, ErrPreviewAlreadyConsumed) {
				// 过期中途（4xx 判决如实上抛——行已补齐，重试将收敛到
				// already-installed 的既有判定）；已消费=并发重试已收敛=成功。
				return nil, rerr
			}
		} else {
			logger.GetLogger(ctx).Errorf("failed to consume plugin preview in confirm heal: %v", err)
			return nil, ErrInstallationPersistFailed
		}
	}
	logger.GetLogger(ctx).Infof(
		"plugin installation heal: completed crashed confirm of %s (installation %s, actor-reviewed preview content)",
		preview.PluginID, inst.ID)
	return s.installationResult(ctx, tenantID, inst)
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
	// B+A 裁决 #2：状态切换与升级接受/漂移重定基/卸载是同族读-改-写——
	// syncService 先 GetByID 载入物化服务再整行 Update（含 URL/AuthConfig）。
	// 不持 per-installation 锁时，与 accept 的 7b（整行切换 URL/OAuth 基线）
	// 交错会让旧内存副本把 accept 刚切上的端点/基线整行回滚，或在 7a 与
	// 7b 之间落一个「安装行 active+服务行停用」再被 7b 覆盖复活。与
	// AcceptUpgrade/ResolveDrift/CheckDrift/UninstallInstallation 同款串行化。
	defer lockUpgradeAccept(installationID)()

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
			// OCR R2 F21: the repository contract returns gorm.ErrRecordNotFound
			// for a 0-row update. The entry lookup above already confirmed the
			// row, so a 0-row flip can only mean a concurrent uninstall removed
			// it — surface the 404 verdict, not a 500 persistence fault.
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrInstallationNotFound
			}
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
	// OCR R2 F20: uninstall rewrites the same rows the upgrade/drift writers
	// touch (the cascade hard-deletes the materialized service row AND its
	// approval rows). Without the per-installation lock a concurrent
	// AcceptUpgrade can pass its re-verification, have the cascade delete the
	// service row under it, and then write policy rows against the deleted
	// serviceID — on PG an FK violation forces the compensation path and the
	// admin sees a spurious 500; on production SQLite (foreign_keys off) the
	// rows land as permanent orphans. Same serialization as
	// AcceptUpgrade/ResolveDrift/CheckDrift.
	defer lockUpgradeAccept(installationID)()

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
	// 装配故障（生产经 dig 必注入）：响亮失败而非按「无孤儿」静默跳过
	// ——静默跳过会让卸载删掉锚行后物化服务成永久孤儿（唯一索引阻断
	// 重装）、连接视图退回死端视图，与 oauthRepo == nil 的 fail-loudly
	// 惯例一致（T07-OCR3-F2）。调用方已按各自域哨兵 fail-closed；
	// 测试夹具注入 stub MCPServiceRepository（fakeInstallMCPServiceRepo
	// 先例）而非依赖 nil 妥协。
	if s.mcpServiceRepo == nil {
		return "", fmt.Errorf("plugin service store is not wired")
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

// PreviewUpgrade re-fetches the installation's LONG-LIVED manifest source and
// returns the five-dimension diff between the ACCEPTED snapshot and the
// candidate the manifest currently declares (T14). The re-fetch source is
// installation.ManifestURL by design (总索引「安装后远端真相的统一口径」):
// the upgrade's object is "the candidate version the manifest currently
// declares", and the consumed, TTL-bound preview row is never a reusable
// source. The whole flow is READ-ONLY — no installation field, no materialized
// service row, no policy row and no preview row is written on ANY path,
// including failures: the old version's availability is guaranteed precisely
// by writing nothing (plan 07 Global Constraints). Downgrade candidates still
// preview (accepting an older version is an admin decision); the diff flags
// them via IsDowngrade (semver three-segment numeric comparison).
func (s *pluginService) PreviewUpgrade(
	ctx context.Context,
	tenantID uint64,
	installationID string,
) (*types.PluginUpgradePreviewResult, error) {
	// Step 1: installation within this tenant (absent/foreign → one verdict).
	inst, err := s.pluginRepo.GetInstallation(ctx, tenantID, installationID)
	if err != nil {
		logger.GetLogger(ctx).Errorf("failed to load plugin installation: %v", err)
		return nil, ErrInstallationPersistFailed
	}
	if inst == nil {
		return nil, ErrInstallationNotFound
	}

	// Step 2: re-fetch and verify the candidate from the long-lived manifest
	// source. Error faces mirror ConfirmInstallation's classification: fetch
	// faults keep the 503 sentinel chain (the wrapper text names the
	// candidate as unreachable for the admin surface), OAuth-protected keeps
	// its own marker, and every other verification failure (invalid manifest,
	// declaration/live disagreement) is a deterministic 4xx rejection via
	// ErrPluginVerifyFailed — the T01 verification path is reused, never
	// bypassed (plan 07 Review Focus).
	result, err := plugins.FetchAndVerify(ctx, inst.ManifestURL, s.lister)
	if err != nil {
		if errors.Is(err, plugins.ErrManifestFetchFailed) {
			return nil, fmt.Errorf("candidate plugin unreachable: %w", err)
		}
		if plugins.IsOAuthProtected(err) {
			return nil, err
		}
		return nil, fmt.Errorf("%w: %v", ErrPluginVerifyFailed, err)
	}

	// Step 3: identity guard (T14-OCR1-F1) — the manifest at the long-lived
	// URL may have been swapped wholesale to a DIFFERENT plugin (a
	// self-consistent manifest of another plugin_id passes FetchAndVerify
	// untouched). Previewing across identities would return a
	// self-contradictory diff (PluginID names the installed plugin while the
	// candidate fingerprint/tools belong to another), and a later accept
	// would write ANOTHER plugin's snapshot into this installation row —
	// silently bypassing the (tenant_id, plugin_id) uniqueness governance.
	// Deterministic 4xx rejection, mirroring ConfirmInstallation's
	// fingerprint/digest guard; the admin's path is uninstall + re-install.
	if result.Manifest.PluginID != inst.PluginID {
		return nil, fmt.Errorf("%w: manifest now declares plugin %q, not the installed %q; uninstall and re-install instead",
			ErrPluginVerifyFailed, result.Manifest.PluginID, inst.PluginID)
	}

	// OCR 终局 F13: a transport TYPE change is not upgrade-carriable. The
	// identity fingerprint and this diff carry no transport dimension and
	// neither accept write (7a/7b) persists one, so accepting an sse ↔
	// http-streamable switch would leave the installation row (and every
	// dialer keyed off it — the runtime lister, CheckDrift) on the OLD
	// protocol against the NEW endpoint: connection failures / a permanent
	// endpoint-unreachable drift whose cause the admin cannot see in the
	// diff. Deterministic 4xx at the preview face; uninstall + re-install
	// is the path. (The value is the manifest-validated short enum, not
	// free text — safe to echo.)
	if result.Manifest.Transport.Type != inst.TransportType {
		return nil, fmt.Errorf("%w: candidate transport %q differs from the installed %q; uninstall and re-install instead",
			ErrPluginVerifyFailed, result.Manifest.Transport.Type, inst.TransportType)
	}

	// OCR R1 F45: the candidate endpoint enters the diff served to the admin
	// (and, on the accept side, endpoint_url varchar(512) + the materialized
	// service URL). ValidateManifest checks scheme/host but not length —
	// bound it here so an oversized declaration is the mapped 4xx verdict
	// instead of a PG value-too-long misreported as 500 on accept.
	if err := validatePluginURLLength("plugin transport endpoint", result.Manifest.Transport.Endpoint); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrPluginVerifyFailed, err)
	}

	// Step 4: diff the ACCEPTED snapshot against the freshly verified
	// candidate snapshot, then stamp the identity/version pair the pure
	// function cannot know. No write happens anywhere — steps 1-4 are reads.
	diff := plugins.DiffSnapshots(
		inst.ToolsSnapshot, result.Snapshot,
		inst.EndpointURL, result.Manifest.Transport.Endpoint,
	)
	diff.PluginID = inst.PluginID
	diff.CurrentVersion = inst.AcceptedVersion
	diff.CandidateVersion = result.Manifest.Version
	diff.IsDowngrade = plugins.ComparePluginVersions(result.Manifest.Version, inst.AcceptedVersion) < 0

	return &types.PluginUpgradePreviewResult{
		Diff:                 *diff,
		CandidateFingerprint: result.IdentityFingerprint,
		CandidateToolsDigest: result.ToolsDigest,
	}, nil
}

// AcceptUpgrade switches the installation to the previewed candidate (T16,
// plan 08 Task 16 Step 3). The flow:
//
//  1. Installation lookup (absent/foreign → one 404 verdict).
//  2. Capability check: the repository must implement installationUpgradeWriter
//     (a wiring fault fails loudly).
//  3. Re-fetch + re-verify from installation.ManifestURL — the SAME source and
//     error classification PreviewUpgrade uses (fetch faults keep the 503
//     sentinel chain, OAuth-protected its marker, verification failures the
//     4xx ErrPluginVerifyFailed).
//  4. Identity guard (mirror of T14-OCR1-F1): the manifest must still declare
//     the INSTALLED plugin — a wholesale swap to another plugin_id would write
//     another plugin's snapshot into this row.
//  5. Fingerprint guard: the fresh IdentityFingerprint must equal the
//     candidateFingerprint the admin reviewed — anything else is
//     ErrUpgradeCandidateChanged with zero writes (a new preview is the path).
//  6. Idempotency: when the installation already carries the candidate
//     (version + digest + endpoint equal AND the materialized service URL is
//     in sync AND no drift is pending), return the current installation with
//     ZERO writes — the sync clause keeps a half-compensated state retriable.
//  7. Compensated write order: installation row (new snapshot/endpoint/
//     version/digest + drift reset) → materialized service URL switch
//     (UpdatedAt refresh recycles the manager's cached client) → INCREMENTAL
//     policy rows (only tools WITHOUT an existing row; a NEW tool lands
//     Enabled=ReadOnly — read exposed, write disabled; existing rows keep
//     the admin's verdicts). Any failure after the installation write
//     compensates by writing the memory-held OLD values back (service row
//     first — the runtime-visible surface — then the installation row), so
//     the old version stays callable from member conversations.
func (s *pluginService) AcceptUpgrade(
	ctx context.Context,
	tenantID uint64,
	actorID, installationID, candidateFingerprint string,
) (*types.PluginInstallationResult, error) {
	// Serialize accepts of the SAME installation end to end (T16-OCR2-F1) —
	// see upgradeAcceptMutexes. Foreign tenants' IDs never share a key with
	// this tenant's (UUIDs are globally unique), so the lock never
	// cross-tenant blocks.
	defer lockUpgradeAccept(installationID)()

	// Step 1: installation within this tenant (absent/foreign → one verdict).
	inst, err := s.pluginRepo.GetInstallation(ctx, tenantID, installationID)
	if err != nil {
		logger.GetLogger(ctx).Errorf("failed to load plugin installation: %v", err)
		return nil, ErrInstallationPersistFailed
	}
	if inst == nil {
		return nil, ErrInstallationNotFound
	}

	// Step 2: re-fetch and re-verify the candidate from the long-lived
	// manifest source — identical classification to PreviewUpgrade.
	result, err := plugins.FetchAndVerify(ctx, inst.ManifestURL, s.lister)
	if err != nil {
		if errors.Is(err, plugins.ErrManifestFetchFailed) {
			return nil, fmt.Errorf("candidate plugin unreachable: %w", err)
		}
		if plugins.IsOAuthProtected(err) {
			return nil, err
		}
		return nil, fmt.Errorf("%w: %v", ErrPluginVerifyFailed, err)
	}

	// Step 3: identity guard (T14-OCR1-F1 mirror) — accepting another
	// plugin's snapshot into this installation row would silently bypass the
	// (tenant_id, plugin_id) uniqueness governance.
	if result.Manifest.PluginID != inst.PluginID {
		return nil, fmt.Errorf("%w: manifest now declares plugin %q, not the installed %q; uninstall and re-install instead",
			ErrPluginVerifyFailed, result.Manifest.PluginID, inst.PluginID)
	}

	// OCR 终局 F13: transport type changes are not upgrade-carriable (the
	// fingerprint has no transport dimension and no accept write persists
	// one — see PreviewUpgrade's guard for the full rationale). Checked
	// BEFORE the fingerprint guard: when both mismatch, the transport
	// verdict is the actionable one (uninstall + re-install), while
	// candidate-changed would misleadingly suggest a fresh preview suffices.
	if result.Manifest.Transport.Type != inst.TransportType {
		return nil, fmt.Errorf("%w: candidate transport %q differs from the installed %q; uninstall and re-install instead",
			ErrPluginVerifyFailed, result.Manifest.Transport.Type, inst.TransportType)
	}

	// Step 5: fingerprint guard — the admin accepted THIS fingerprint; a
	// candidate that moved on is a different document. Zero writes on any
	// rejection path above and here.
	if result.IdentityFingerprint != candidateFingerprint {
		return nil, fmt.Errorf("%w: remote candidate is %q now; run a new upgrade preview",
			ErrUpgradeCandidateChanged, result.Manifest.Version)
	}

	// OCR R1 F45: bound the candidate endpoint BEFORE any write — it lands in
	// endpoint_url (varchar(512)) and the materialized service URL. Without
	// this a >512-rune declaration passes the fingerprint guard and PG fails
	// the first write with value-too-long, misreporting deterministic input
	// as ErrInstallationPersistFailed (500); SQLite would silently store the
	// oversized value.
	if err := validatePluginURLLength("plugin transport endpoint", result.Manifest.Transport.Endpoint); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrPluginVerifyFailed, err)
	}

	// Capability seam (production always injects the gorm repository). Runs
	// AFTER the read-only guards (identity/transport/fingerprint/length):
	// those verdicts need no write capability, and a rejected candidate on an
	// unwired stack should surface the content verdict, not the wiring fault.
	writer, ok := s.pluginRepo.(installationUpgradeWriter)
	if !ok {
		logger.GetLogger(ctx).Errorf(
			"plugin upgrade accept: repository %T does not implement installationUpgradeWriter", s.pluginRepo)
		return nil, ErrUpgradeWriterNotWired
	}

	candidateVersion := result.Manifest.Version
	candidateEndpoint := result.Manifest.Transport.Endpoint
	candidateSnapshot := types.PluginPreviewTools(result.Snapshot)

	// The materialized service row (URL switch target); a missing row (svc ==
	// nil, including an empty ServiceID) leaves nothing to sync — the
	// installation row stays authoritative and the plugin is simply not
	// serving until the row is healed (same reading as SetInstallationState).
	var svc *types.MCPService
	if inst.ServiceID != "" {
		svc, err = s.mcpServiceRepo.GetByID(ctx, tenantID, inst.ServiceID)
		if err != nil {
			logger.GetLogger(ctx).Errorf("failed to load materialized service for upgrade: %v", err)
			return nil, ErrInstallationPersistFailed
		}
	}

	// Step 6: idempotency — the installation already carries exactly this
	// candidate (version/digest/endpoint) with the service row in sync and no
	// pending drift → return it with zero writes. The sync clause matters on
	// the failure path: a half-compensated state (installation restored but
	// service URL not) must NOT early-return — a retried accept walks the
	// write path again and converges.
	//
	// B+A 裁决 #1：短路前必须校验候选快照每工具都有显式策略行。快照落库
	// （7a）后策略行写完（7c）前被硬杀的窗口里，重试会命中本短路零写入，
	// 缺策略行的写工具按运行时门默认启用（MCPToolApproval 缺行=启用），
	// 触碰「新增写工具默认关闭」边界——行缺失时落入写路径：7a 重写同值
	// 无害，7c 只补缺行（既有行保留管理员裁决）。svc == nil（服务行不在，
	// 无行可键）维持原零写入读法。
	serviceInSync := svc == nil || (svc.URL != nil && *svc.URL == inst.EndpointURL)
	if candidateVersion == inst.AcceptedVersion &&
		result.ToolsDigest == inst.ToolsDigest &&
		candidateEndpoint == inst.EndpointURL &&
		inst.DriftState == types.PluginDriftNone &&
		serviceInSync {
		policiesComplete := true
		if svc != nil {
			var err error
			policiesComplete, err = s.installationPolicyRowsComplete(ctx, tenantID, inst.ServiceID, candidateSnapshot)
			if err != nil {
				logger.GetLogger(ctx).Errorf(
					"failed to load tool policies for upgrade idempotency check on %s: %v", installationID, err)
				return nil, ErrInstallationMaterializeFailed
			}
		}
		if policiesComplete {
			logger.GetLogger(ctx).Infof(
				"plugin upgrade accept: installation %s already at candidate %s (fingerprint match) — idempotent no-op (actor %s)",
				installationID, candidateVersion, actorID)
			return s.installationResult(ctx, tenantID, inst)
		}
		logger.GetLogger(ctx).Infof(
			"plugin upgrade accept: installation %s carries candidate %s with incomplete tool policy rows (crashed accept window) — completing (actor %s)",
			installationID, candidateVersion, actorID)
	}

	// Memory-held old values — the compensation write-back source.
	oldVersion := inst.AcceptedVersion
	oldEndpoint := inst.EndpointURL
	oldSnapshot := append(types.PluginPreviewTools(nil), inst.ToolsSnapshot...)
	oldDigest := inst.ToolsDigest
	var oldAuth *types.MCPAuthConfig // captured before 7b overwrites it
	if svc != nil {
		oldAuth = svc.AuthConfig
	}
	// writtenPolicies accumulates the per-tool rows THIS accept creates in the
	// incremental 7c loop — the compensation delete set (T16-OCR1-F2): a row
	// appended only after its SetPolicy SUCCEEDED, so a mid-loop failure
	// deletes exactly the rows this call wrote (UpsertPolicy is atomic).
	var writtenPolicies []string

	// Step 7a: installation row first (the snapshot is the runtime authority;
	// writing it before the service row means a later failure leaves the OLD
	// runtime surface after compensation, never a new snapshot with no policy).
	if err := writer.UpdateInstallationAccepted(ctx, tenantID, installationID,
		candidateVersion, candidateEndpoint, candidateSnapshot, result.ToolsDigest); err != nil {
		logger.GetLogger(ctx).Errorf("failed to persist accepted upgrade for installation %s: %v", installationID, err)
		return nil, ErrInstallationPersistFailed
	}

	// compensate writes the OLD values back on a FRESH context (the request
	// ctx may already be cancelled — R12 F14 discipline), unwinding the write
	// order in REVERSE: this accept's NEW policy rows first (T16-OCR1-F2 — a
	// residual row keyed to a tool outside the restored snapshot would
	// short-circuit the install-time Enabled=ReadOnly rule on a later accept
	// of a candidate that reclassifies the same name), then the service row
	// (the runtime-visible endpoint AND the OAuth baseline, T16-OCR1-F3),
	// then the installation row (the snapshot authority last, so a failing
	// compensation leaves the retriable "installation ahead, service behind"
	// shape rather than the reverse).
	compensate := func(cause error) error {
		compCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 15*time.Second)
		defer cancel()
		if len(writtenPolicies) > 0 {
			if err := writer.DeleteInstallationToolPolicies(compCtx, tenantID, inst.ServiceID, writtenPolicies); err != nil {
				logger.GetLogger(ctx).Errorf(
					"plugin upgrade accept compensation: failed to remove this accept's new policy rows for %s (%v): %v",
					inst.ServiceID, writtenPolicies, err)
			}
		}
		if svc != nil {
			old := oldEndpoint
			svc.URL = &old
			if oldAuth != nil {
				svc.AuthConfig = oldAuth
			} else {
				// A nil old config cannot be expressed through the shared
				// Update (nil skips the auth_config column) — restore the
				// functional equivalent: explicit none.
				svc.AuthConfig = &types.MCPAuthConfig{AuthType: types.MCPAuthNone}
			}
			svc.UpdatedAt = time.Now()
			if err := s.mcpServiceRepo.Update(compCtx, svc); err != nil {
				logger.GetLogger(ctx).Errorf(
					"plugin upgrade accept compensation: failed to restore materialized service URL for %s: %v", inst.ServiceID, err)
			} else {
				// Restored row moved — recycle any client cached at the
				// candidate endpoint during the forward window.
				s.closeServiceClient(compCtx, inst.ServiceID)
			}
		}
		if err := writer.UpdateInstallationAccepted(compCtx, tenantID, installationID,
			oldVersion, oldEndpoint, oldSnapshot, oldDigest); err != nil {
			logger.GetLogger(ctx).Errorf(
				"plugin upgrade accept compensation: failed to restore installation %s to %s: %v",
				installationID, oldVersion, err)
		}
		return cause
	}

	// Step 7b: materialized service URL switch + OAuth baseline re-derivation.
	// Name/ID/PluginInstallationID stay put (session server_id stability —
	// Global Constraints); the UpdatedAt refresh invalidates the manager's
	// version-keyed client cache and the explicit close evicts any live
	// connection at the old endpoint. The AuthConfig is re-derived from the
	// ACCEPTED candidate baseline (T16-OCR1-F3) — the mirror of
	// ConfirmInstallation's materialization: the connection view derives
	// RequiresPersonalAuth from the accepted SNAPSHOT while the authorize
	// flow reads svc.AuthConfig.IsOAuth(); leaving the install-time config
	// would dead-end a newly personal-auth tool at IsOAuth() (permanent 400)
	// and keep authorization requests at the OLD scope union.
	if svc != nil {
		newEndpoint := candidateEndpoint
		svc.URL = &newEndpoint
		newAuth := oauthConfigFromVerifiedBaseline(candidateSnapshot)
		if newAuth == nil {
			// The shared repo Update SKIPS a nil AuthConfig (it only writes
			// the column when non-nil) — a candidate baseline with no
			// personal-auth tools must CLEAR the column explicitly, not
			// silently keep the install-time OAuth config.
			newAuth = &types.MCPAuthConfig{AuthType: types.MCPAuthNone}
		}
		svc.AuthConfig = newAuth
		svc.UpdatedAt = time.Now()
		if err := s.mcpServiceRepo.Update(ctx, svc); err != nil {
			logger.GetLogger(ctx).Errorf(
				"failed to switch materialized service URL for installation %s: %v", installationID, err)
			return nil, compensate(ErrInstallationMaterializeFailed)
		}
		s.closeServiceClient(ctx, inst.ServiceID)
	} else {
		logger.GetLogger(ctx).Infof(
			"plugin upgrade accept: installation %s has no materialized service row to sync (service_id empty or row gone)", installationID)
	}

	// Step 7c: INCREMENTAL per-tool policy rows. Tools already carrying a row
	// (from install or a previous accept) keep the admin's verdicts — an
	// upgrade never resets governance. A NEW tool lands the install-time
	// rule: read tools exposed, WRITE TOOLS DISABLED (B5; full write-tool
	// governance acceptance is T18's slice — this lands the rows).
	//
	// Guarded on svc != nil, NOT ServiceID != "" (T16-OCR1-F1): with the
	// service row GONE (a dangling service_id anchor — an interrupted
	// uninstall) ListByService/SetPolicy fail inside on the missing service
	// row and would deterministically abort EVERY accept after 7a wrote the
	// installation, contradicting 7b's "nothing to sync" reading. Policy
	// rows are keyed to the service row; with the row gone there is nothing
	// to read and nothing to write.
	if svc != nil {
		existing := map[string]bool{}
		rows, err := s.toolApprovalService.ListByService(ctx, tenantID, inst.ServiceID)
		if err != nil {
			logger.GetLogger(ctx).Errorf("failed to load tool policies for upgrade: %v", err)
			return nil, compensate(ErrInstallationMaterializeFailed)
		}
		for _, row := range rows {
			existing[row.ToolName] = true
		}
		for _, tool := range candidateSnapshot {
			if existing[tool.Name] {
				continue
			}
			enabled := tool.ReadOnly
			if err := s.toolApprovalService.SetPolicy(ctx, tenantID, inst.ServiceID, tool.Name, nil, &enabled); err != nil {
				logger.GetLogger(ctx).Errorf(
					"failed to write tool policy for %s/%s on upgrade: %v", inst.PluginID, tool.Name, err)
				return nil, compensate(ErrInstallationMaterializeFailed)
			}
			// Recorded only after the row landed — the compensation delete
			// set is exactly what this call wrote (T16-OCR1-F2).
			writtenPolicies = append(writtenPolicies, tool.Name)
		}
	}

	logger.GetLogger(ctx).Infof(
		"plugin upgrade accepted: installation %s (%s) %s → %s (actor %s)",
		installationID, inst.PluginID, oldVersion, candidateVersion, actorID)

	inst.AcceptedVersion = candidateVersion
	inst.EndpointURL = candidateEndpoint
	inst.ToolsSnapshot = candidateSnapshot
	inst.ToolsDigest = result.ToolsDigest
	inst.DriftState = types.PluginDriftNone
	inst.DriftDetail = nil
	return s.installationResult(ctx, tenantID, inst)
}

// liveDirectoryFromAcceptedEndpoint fetches the LIVE tool directory from the
// installation's ACCEPTED endpoint (T17 drift truth source — 总索引「安装后
// 远端真相的统一口径」): the manifest may already point at a newer version,
// and drift is DEFINED as "the accepted endpoint deviates from the accepted
// snapshot", so the check must never re-fetch the manifest. Failures wrap
// ErrDriftEndpointUnreachable: the drift verdict is minted from a live
// observation or not at all.
func (s *pluginService) liveDirectoryFromAcceptedEndpoint(
	ctx context.Context, inst *types.PluginInstallation,
) ([]*types.MCPTool, error) {
	live, err := s.lister(ctx, inst.TransportType, inst.EndpointURL)
	if err != nil {
		logger.GetLogger(ctx).Errorf(
			"plugin drift check: live ListTools against accepted endpoint %s failed: %v", inst.EndpointURL, err)
		return nil, fmt.Errorf("%w: %v", ErrDriftEndpointUnreachable, err)
	}
	return live, nil
}

// snapshotToolNamesOf extracts the accepted snapshot's tool names (the
// baseline the drift report shows next to the deviation).
func snapshotToolNamesOf(snapshot []types.PluginToolSnapshot) []string {
	names := make([]string, 0, len(snapshot))
	for _, tool := range snapshot {
		names = append(names, tool.Name)
	}
	return names
}

// installationPolicyRowsComplete reports whether every tool of the snapshot
// carries an explicit per-tool policy row keyed to the materialized service
// (B+A 裁决 #1). MCPToolApproval treats a MISSING row as enabled for
// backwards compatibility, so the idempotent short-circuits of the upgrade/
// drift accept paths must not zero-write over a snapshot whose rows were
// killed mid-write (hard crash between the snapshot write and the policy
// loop): a write tool without a row would ride the runtime gate's
// default-enabled verdict — the exact inversion of B5's
// "new write tools land disabled".
func (s *pluginService) installationPolicyRowsComplete(
	ctx context.Context,
	tenantID uint64,
	serviceID string,
	snapshot types.PluginPreviewTools,
) (bool, error) {
	rows, err := s.toolApprovalService.ListByService(ctx, tenantID, serviceID)
	if err != nil {
		return false, err
	}
	present := make(map[string]struct{}, len(rows))
	for _, row := range rows {
		present[row.ToolName] = struct{}{}
	}
	for _, tool := range snapshot {
		if _, ok := present[tool.Name]; !ok {
			return false, nil
		}
	}
	return true, nil
}

// parseDriftDetail decodes a persisted drift_detail document. A nil/empty
// column decodes to nil (never detected/never checked both read as "no
// detail"); a corrupt document logs and reads as nil too — the drift STATE on
// the row stays authoritative, the detail is its human-readable annex.
func parseDriftDetail(ctx context.Context, raw json.RawMessage) *types.PluginDriftDetail {
	if len(raw) == 0 {
		return nil
	}
	var detail types.PluginDriftDetail
	if err := json.Unmarshal(raw, &detail); err != nil {
		logger.GetLogger(ctx).Errorf("plugin drift detail is unparsable (%d bytes): %v", len(raw), err)
		return nil
	}
	return &detail
}

// CheckDrift re-verifies ONE installation against its accepted endpoint and
// persists the verdict (T17, GAP-6; Admin): a LIVE ListTools against
// installation.EndpointURL (never via the manifest), DiffLiveAgainstSnapshot
// against the accepted snapshot, then one UpdateDrift write —
// drift_state=detected + the name-only detail document when any evidence
// exists, drift_state=none + a cleared detail when the endpoint matches the
// baseline again (drift is not a one-way ratchet; a healed endpoint heals the
// row). The fetch happens BEFORE any write, so an unreachable endpoint
// rejects with zero writes. A foreign tenant's installation ID is "not found".
//
// Serialized by the SAME per-installation mutex as AcceptUpgrade/ResolveDrift
// (T17-OCR1-F2): without it, a check that read the OLD snapshot could land a
// detected verdict (and a detail naming the old baseline) onto a row a
// concurrent resolve already rebased — a stale verdict the runtime marker
// then never self-heals (state==detected skips re-evaluation). Holding the
// mutex across the live ListTools matches the accept/resolve shape (the lock
// spans remote I/O by design, upgradeAcceptMutexes).
func (s *pluginService) CheckDrift(
	ctx context.Context,
	tenantID uint64,
	installationID string,
) (*types.PluginDriftReport, error) {
	defer lockUpgradeAccept(installationID)()

	inst, err := s.pluginRepo.GetInstallation(ctx, tenantID, installationID)
	if err != nil {
		logger.GetLogger(ctx).Errorf("failed to load plugin installation: %v", err)
		return nil, ErrInstallationPersistFailed
	}
	if inst == nil {
		return nil, ErrInstallationNotFound
	}

	live, err := s.liveDirectoryFromAcceptedEndpoint(ctx, inst)
	if err != nil {
		return nil, err
	}

	detail, err := plugins.DiffLiveAgainstSnapshot(live, inst.ToolsSnapshot)
	if err != nil {
		// OCR R1 F15: the live directory failed the vetting gates (size cap/
		// duplicates/hygiene) — a drift verdict cannot be minted from it.
		// Deterministic 4xx (the endpoint's own declaration is the problem),
		// zero writes.
		logger.GetLogger(ctx).Errorf("plugin drift check: cannot vet live directory for installation %s: %v", inst.ID, err)
		return nil, fmt.Errorf("%w: %v", ErrPluginVerifyFailed, err)
	}
	state := types.PluginDriftNone
	var raw json.RawMessage
	if detail.HasDrift() {
		state = types.PluginDriftDetected
		raw, err = json.Marshal(detail)
		if err != nil {
			logger.GetLogger(ctx).Errorf("failed to encode plugin drift detail: %v", err)
			return nil, ErrDriftPersistFailed
		}
	}

	writer, ok := s.pluginRepo.(installationDriftWriter)
	if !ok {
		logger.GetLogger(ctx).Errorf(
			"plugin drift check: repository %T does not implement installationDriftWriter", s.pluginRepo)
		return nil, ErrDriftPersistFailed
	}
	if err := writer.UpdateDrift(ctx, tenantID, installationID, state, raw); err != nil {
		logger.GetLogger(ctx).Errorf("failed to persist plugin drift state: %v", err)
		return nil, ErrDriftPersistFailed
	}

	report := &types.PluginDriftReport{
		InstallationID:    inst.ID,
		DriftState:        state,
		SnapshotToolNames: snapshotToolNamesOf(inst.ToolsSnapshot),
	}
	if state == types.PluginDriftDetected {
		report.Detail = detail
	}
	return report, nil
}

// GetDrift returns ONE installation's persisted drift view (T17; Viewer) —
// a pure read: the row's current state (never-checked rows carry their
// install-time default), its detail document when one exists, and the
// accepted snapshot's tool names. It never re-fetches the endpoint — the
// admin's refresh lever is CheckDrift. A foreign tenant's installation ID is
// "not found".
func (s *pluginService) GetDrift(
	ctx context.Context,
	tenantID uint64,
	installationID string,
) (*types.PluginDriftReport, error) {
	inst, err := s.pluginRepo.GetInstallation(ctx, tenantID, installationID)
	if err != nil {
		logger.GetLogger(ctx).Errorf("failed to load plugin installation: %v", err)
		return nil, ErrInstallationPersistFailed
	}
	if inst == nil {
		return nil, ErrInstallationNotFound
	}
	return &types.PluginDriftReport{
		InstallationID:    inst.ID,
		DriftState:        inst.DriftState,
		Detail:            parseDriftDetail(ctx, inst.DriftDetail),
		SnapshotToolNames: snapshotToolNamesOf(inst.ToolsSnapshot),
	}, nil
}

// rebaseSnapshotOnLive rebuilds the accepted snapshot FROM the live directory
// (ResolveDrift's core): names, descriptions and schema digests come from the
// LIVE endpoint (the values the runtime guard compares against, so the
// rebased baseline matches what members will actually be served); the
// governance classification (read-only / personal-auth / scopes) is a
// manifest-declaration-domain field the live MCP directory does not carry, so
// a SAME-NAME tool inherits its accepted classification while a NEW tool
// lands the conservative default: ReadOnly=false, no personal auth, no scopes
// — unable to self-certify read-only over ListTools, a new tool installs the
// way a new WRITE tool does (Enabled=false policy row), and the admin can
// widen it from the tool governance surface afterwards.
func rebaseSnapshotOnLive(live []*types.MCPTool, accepted []types.PluginToolSnapshot) types.PluginPreviewTools {
	classification := make(map[string]types.PluginToolSnapshot, len(accepted))
	for _, tool := range accepted {
		classification[tool.Name] = tool
	}
	rebased := make(types.PluginPreviewTools, 0, len(live))
	for _, tool := range live {
		if tool == nil {
			continue
		}
		snap := types.PluginToolSnapshot{
			Name:              tool.Name,
			Description:       tool.Description,
			InputSchemaDigest: plugins.ToolSchemaDigest(tool.InputSchema),
			ReadOnly:          false,
			Scopes:            []string{},
		}
		if prev, ok := classification[tool.Name]; ok {
			snap.ReadOnly = prev.ReadOnly
			snap.RequiresPersonalAuth = prev.RequiresPersonalAuth
			scopes := make([]string, len(prev.Scopes))
			copy(scopes, prev.Scopes)
			snap.Scopes = scopes
		}
		rebased = append(rebased, snap)
	}
	return rebased
}

// ResolveDrift closes the drift review loop (T17, GAP-6; Admin): re-verify
// the accepted endpoint LIVE, then accept the CURRENT remote directory as the
// new verified snapshot — same version, same endpoint, new snapshot/digest,
// drift reset to none. The flow:
//
//  1. Installation lookup (absent/foreign → one 404 verdict).
//  2. Capability check: the repository must implement installationUpgradeWriter
//     (the rebase persists through UpdateInstallationAccepted — which also
//     resets drift, making the accepting write the drift-healing write; and
//     compensates through DeleteInstallationToolPolicies).
//  3. LIVE ListTools against installation.EndpointURL (never via the
//     manifest). Unreachable → ErrDriftEndpointUnreachable with ZERO writes:
//     an admin must not be able to "resolve" onto a directory that cannot be
//     read; the drift state stays exactly as it was.
//  4. Idempotency: when the row already carries this snapshot (digest equal)
//     and no drift is pending, return the current installation with zero
//     writes.
//  5. Compensated write order (T16 pattern): installation row first
//     (rebased snapshot + recomputed digest; version/endpoint pass through
//     unchanged), then INCREMENTAL per-tool policy rows — only tools WITHOUT
//     an existing row, each landing the install-time rule Enabled=ReadOnly
//     (with rebaseSnapshotOnLive's conservative classification, a NEW tool is
//     ReadOnly=false → disabled); existing rows keep the admin's verdicts.
//     A failure after the installation write compensates by writing the
//     memory-held old values back and deleting this call's new policy rows.
//
// The materialized MCP service row is NOT touched: the endpoint is unchanged
// and same-name tools inherit their classification, so neither the URL nor
// the OAuth baseline can move. Serialized against AcceptUpgrade by the shared
// per-installation mutex — both rewrite the same row's snapshot.
func (s *pluginService) ResolveDrift(
	ctx context.Context,
	tenantID uint64,
	actorID, installationID string,
) (*types.PluginInstallationResult, error) {
	// Same per-installation serialization as AcceptUpgrade (T16-OCR2-F1):
	// resolve and accept both rewrite the installation's snapshot/digest —
	// interleaving them against a shared drift state would let one
	// compensate the other's writes.
	defer lockUpgradeAccept(installationID)()

	// Step 1: installation within this tenant.
	inst, err := s.pluginRepo.GetInstallation(ctx, tenantID, installationID)
	if err != nil {
		logger.GetLogger(ctx).Errorf("failed to load plugin installation: %v", err)
		return nil, ErrInstallationPersistFailed
	}
	if inst == nil {
		return nil, ErrInstallationNotFound
	}

	// Step 2: capability seams — the rebase persists (and compensates) through
	// the T16 upgrade writer, and the compensation restores the drift verdict
	// through the drift writer (T17-OCR1-F1).
	writer, ok := s.pluginRepo.(installationUpgradeWriter)
	if !ok {
		logger.GetLogger(ctx).Errorf(
			"plugin drift resolve: repository %T does not implement installationUpgradeWriter", s.pluginRepo)
		return nil, ErrUpgradeWriterNotWired
	}
	driftWriter, ok := s.pluginRepo.(installationDriftWriter)
	if !ok {
		logger.GetLogger(ctx).Errorf(
			"plugin drift resolve: repository %T does not implement installationDriftWriter", s.pluginRepo)
		return nil, ErrDriftPersistFailed
	}

	// Step 3: the remote truth the admin is accepting — the accepted
	// endpoint's CURRENT directory. Unreachable → zero writes, state stays.
	live, err := s.liveDirectoryFromAcceptedEndpoint(ctx, inst)
	if err != nil {
		return nil, err
	}
	// Hygiene gate (T17-OCR1-F6): drift's premise is that the endpoint may
	// have turned hostile, and this directory is about to be minted into the
	// tenant's accepted snapshot (DB rows, per-tool policies, the
	// member-visible directory) — the same gates BuildVerifiedSnapshot runs
	// the install path's directory through: size cap, duplicate-name
	// rejection, identifier hygiene. A rejection is deterministic 4xx with
	// ZERO writes.
	if err := plugins.ValidateLiveDirectoryForRebase(live); err != nil {
		logger.GetLogger(ctx).Errorf(
			"plugin drift resolve: live directory rejected for installation %s: %v", installationID, err)
		return nil, fmt.Errorf("%w: %v", ErrPluginVerifyFailed, err)
	}

	rebased := rebaseSnapshotOnLive(live, inst.ToolsSnapshot)
	rebasedDigest := plugins.SnapshotDigest(rebased)

	// The materialized service row (policy-key target); a missing row leaves
	// nothing to write policies against — the installation row stays
	// authoritative (same reading as AcceptUpgrade's 7c guard). Loaded before
	// the step-4 short-circuit: the completeness check below needs it.
	var svc *types.MCPService
	if inst.ServiceID != "" {
		svc, err = s.mcpServiceRepo.GetByID(ctx, tenantID, inst.ServiceID)
		if err != nil {
			logger.GetLogger(ctx).Errorf("failed to load materialized service for drift resolve: %v", err)
			return nil, ErrInstallationPersistFailed
		}
	}

	// Step 4: idempotency — already rebased and clean → zero writes. B+A 裁决
	// #1 同族：短路前校验 rebased 快照每工具都有显式策略行——5a 落库、5b
	// 写行中途被硬杀的窗口里，重试不得零写入放行缺行写工具（缺行=运行时
	// 门默认启用）；行缺失时落入写路径增量补齐（5a 重写同值无害）。svc ==
	// nil（无行可键）维持原零写入读法。
	if inst.DriftState == types.PluginDriftNone && rebasedDigest == inst.ToolsDigest {
		policiesComplete := true
		if svc != nil {
			var checkErr error
			policiesComplete, checkErr = s.installationPolicyRowsComplete(ctx, tenantID, inst.ServiceID, rebased)
			if checkErr != nil {
				logger.GetLogger(ctx).Errorf(
					"failed to load tool policies for drift resolve idempotency check on %s: %v", installationID, checkErr)
				return nil, ErrInstallationMaterializeFailed
			}
		}
		if policiesComplete {
			logger.GetLogger(ctx).Infof(
				"plugin drift resolve: installation %s already carries the live directory as its snapshot — idempotent no-op (actor %s)",
				installationID, actorID)
			return s.installationResult(ctx, tenantID, inst)
		}
		logger.GetLogger(ctx).Infof(
			"plugin drift resolve: installation %s already rebased but tool policy rows are incomplete (crashed resolve window) — completing (actor %s)",
			installationID, actorID)
	}

	// Memory-held old values — the compensation write-back source. The drift
	// verdict is captured alongside the snapshot fields: UpdateInstallation-
	// Accepted resets drift as part of the accepting write, and the
	// compensation must restore BOTH (T17-OCR1-F1). Local copies, never
	// fields read back off inst at compensation time — the in-memory row can
	// be aliased by the forward write (the fake repositories mutate the same
	// pointer; a gorm row is a fresh scan either way, but the compensation
	// must not depend on that).
	oldSnapshot := append(types.PluginPreviewTools(nil), inst.ToolsSnapshot...)
	oldDigest := inst.ToolsDigest
	oldDriftState := inst.DriftState
	oldDriftDetail := append(json.RawMessage(nil), inst.DriftDetail...)

	// Step 5a: installation row first — the snapshot is the runtime
	// authority; UpdateInstallationAccepted resets drift as part of the
	// accepting write (version/endpoint pass through unchanged).
	if err := writer.UpdateInstallationAccepted(ctx, tenantID, installationID,
		inst.AcceptedVersion, inst.EndpointURL, rebased, rebasedDigest); err != nil {
		logger.GetLogger(ctx).Errorf("failed to persist rebased snapshot for installation %s: %v", installationID, err)
		return nil, ErrInstallationPersistFailed
	}

	// writtenPolicies accumulates the rows THIS resolve creates — the
	// compensation delete set (T16-OCR1-F2 discipline).
	var writtenPolicies []string
	compensate := func(cause error) error {
		compCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 15*time.Second)
		defer cancel()
		if len(writtenPolicies) > 0 {
			if err := writer.DeleteInstallationToolPolicies(compCtx, tenantID, inst.ServiceID, writtenPolicies); err != nil {
				logger.GetLogger(ctx).Errorf(
					"plugin drift resolve compensation: failed to remove this resolve's new policy rows for %s (%v): %v",
					inst.ServiceID, writtenPolicies, err)
			}
		}
		if err := writer.UpdateInstallationAccepted(compCtx, tenantID, installationID,
			inst.AcceptedVersion, inst.EndpointURL, oldSnapshot, oldDigest); err != nil {
			logger.GetLogger(ctx).Errorf(
				"plugin drift resolve compensation: failed to restore installation %s snapshot: %v", installationID, err)
		} else {
			// UpdateInstallationAccepted unconditionally resets drift to
			// none — the compensation must ALSO restore the drift verdict it
			// cleared (T17-OCR1-F1): the endpoint still deviates from the
			// restored snapshot, and the persisted detail is audit material.
			// Without this, member directories fail-closed on ErrPluginDrift
			// while GetDrift reports "none" — a user-visible contradiction.
			if err := driftWriter.UpdateDrift(compCtx, tenantID, installationID,
				oldDriftState, oldDriftDetail); err != nil {
				logger.GetLogger(ctx).Errorf(
					"plugin drift resolve compensation: failed to restore drift verdict %q for installation %s: %v",
					oldDriftState, installationID, err)
			}
		}
		return cause
	}

	// Step 5b: INCREMENTAL per-tool policy rows (guarded on svc != nil — with
	// the service row gone there is nothing to key policies to, the same
	// reading as AcceptUpgrade's 7c).
	if svc != nil {
		existing := map[string]bool{}
		rows, err := s.toolApprovalService.ListByService(ctx, tenantID, inst.ServiceID)
		if err != nil {
			logger.GetLogger(ctx).Errorf("failed to load tool policies for drift resolve: %v", err)
			return nil, compensate(ErrInstallationMaterializeFailed)
		}
		for _, row := range rows {
			existing[row.ToolName] = true
		}
		for _, tool := range rebased {
			if existing[tool.Name] {
				continue
			}
			enabled := tool.ReadOnly
			if err := s.toolApprovalService.SetPolicy(ctx, tenantID, inst.ServiceID, tool.Name, nil, &enabled); err != nil {
				logger.GetLogger(ctx).Errorf(
					"failed to write tool policy for %s/%s on drift resolve: %v", inst.PluginID, tool.Name, err)
				return nil, compensate(ErrInstallationMaterializeFailed)
			}
			writtenPolicies = append(writtenPolicies, tool.Name)
		}
	}

	logger.GetLogger(ctx).Infof(
		"plugin drift resolved: installation %s (%s) rebased onto the live directory at version %s (actor %s)",
		installationID, inst.PluginID, inst.AcceptedVersion, actorID)

	inst.ToolsSnapshot = rebased
	inst.ToolsDigest = rebasedDigest
	inst.DriftState = types.PluginDriftNone
	inst.DriftDetail = nil
	return s.installationResult(ctx, tenantID, inst)
}

// ListInstallationTools returns the tool-governance view of ONE installation
// (T18): every tool of the ACCEPTED snapshot with its CURRENT policy verdict
// as definite values. Unlike the detail view's best-effort Enabled *bool, a
// policy-store fault FAILS the request — the admin acts on this surface, and
// a governance list read under a silent store fault would show every write
// tool under its default reason while the runtime gate enforces stale rows.
// A foreign tenant's installation ID is "not found".
func (s *pluginService) ListInstallationTools(
	ctx context.Context,
	tenantID uint64,
	installationID string,
) ([]interfaces.PluginInstallationToolPolicy, error) {
	inst, err := s.pluginRepo.GetInstallation(ctx, tenantID, installationID)
	if err != nil {
		logger.GetLogger(ctx).Errorf("failed to load plugin installation: %v", err)
		return nil, ErrInstallationPersistFailed
	}
	if inst == nil {
		return nil, ErrInstallationNotFound
	}
	return s.installationToolPolicies(ctx, tenantID, inst)
}

// SetInstallationToolPolicy patches ONE tool's policy of ONE installation
// (T18) and returns the refreshed governance list. The tool must be in the
// accepted snapshot — the snapshot is the membership authority, so a removed
// tool's residual Enabled row is never addressable (and can never resurrect
// the tool). enabled and requireApproval are independent nullable pointers of
// one patch passed through to the shared MCPToolApprovalService.SetPolicy
// (requireApproval takes effect from T19's endpoint extension — the signature
// is final here), with ONE plugin-domain guard on top (T18-OCR1-F1): the
// shared repository's FIRST INSERT materializes any omitted field with its
// default, and its enabled default is true — so a requireApproval-only patch
// against a tool with NO row would silently arm a write tool's dispatch
// eligibility. When enabled is nil and no row exists, the plugin-domain
// default (enabled=ReadOnly) is materialized explicitly first; an existing
// row keeps its verdict under a requireApproval-only patch.
func (s *pluginService) SetInstallationToolPolicy(
	ctx context.Context,
	tenantID uint64,
	installationID, toolName string,
	enabled, requireApproval *bool,
) ([]interfaces.PluginInstallationToolPolicy, error) {
	// OCR 终局第 2 轮 f14: the last unserialized policy write path. The
	// snapshot-membership check reads the installation row and the SetPolicy
	// write lands against its service — both race AcceptUpgrade/ResolveDrift/
	// Uninstall's row rewrites and cascades (a mid-patch accept can flip the
	// snapshot under the membership check, or a cascade can delete the service
	// between check and write → spurious 5xx / a row the verdict was checked
	// against no longer exists). Same per-installation lock as the other seven
	// write paths; see upgradeAcceptMutexes.
	defer lockUpgradeAccept(installationID)()

	inst, err := s.pluginRepo.GetInstallation(ctx, tenantID, installationID)
	if err != nil {
		logger.GetLogger(ctx).Errorf("failed to load plugin installation: %v", err)
		return nil, ErrInstallationPersistFailed
	}
	if inst == nil {
		return nil, ErrInstallationNotFound
	}

	var snapshotTool *types.PluginToolSnapshot
	for i := range inst.ToolsSnapshot {
		if inst.ToolsSnapshot[i].Name == toolName {
			snapshotTool = &inst.ToolsSnapshot[i]
			break
		}
	}
	if snapshotTool == nil {
		return nil, fmt.Errorf("%w: %q is not in the accepted snapshot", ErrInstallationToolNotFound, toolName)
	}
	if enabled == nil && requireApproval == nil {
		return nil, ErrInstallationPolicyInvalid
	}

	// Resolve the policy store through the degraded-anchor guards
	// (T18-OCR1-F4): the empty-service_id interrupt window self-heals via the
	// mcp_services back-reference; a dangling anchor (service row gone) has
	// no store to write against — a deterministic state rejection.
	svc, rowsByTool, err := s.resolveInstallationPolicyStore(ctx, tenantID, inst)
	if err != nil {
		return nil, err
	}
	if svc == nil {
		return nil, fmt.Errorf("%w: %q (uninstall and re-install to recover)", ErrInstallationServiceMissing, inst.PluginID)
	}

	// T18-OCR1-F1: a requireApproval-only patch on a MISSING row must not ride
	// the shared insert default (enabled=true) — materialize the plugin-domain
	// default explicitly.
	enabledPatch := enabled
	if enabledPatch == nil {
		if _, exists := rowsByTool[toolName]; !exists {
			materialized := snapshotTool.ReadOnly
			enabledPatch = &materialized
		}
	}
	if err := s.toolApprovalService.SetPolicy(ctx, tenantID, svc.ID, toolName, requireApproval, enabledPatch); err != nil {
		logger.GetLogger(ctx).Errorf(
			"failed to write tool policy for %s/%s: %v", inst.PluginID, toolName, err)
		return nil, ErrInstallationPersistFailed
	}
	return s.installationToolPolicies(ctx, tenantID, inst)
}

// resolveInstallationPolicyStore resolves the policy-store face of ONE
// installation for the governance surface (T18-OCR1-F4) — the two degraded
// anchor shapes this file already knows (the T16-OCR1-F1 / T07-OCR1-F5
// precedents) must not read as store faults here:
//
//   - Empty ServiceID (the confirm interrupt window, after
//     CreateMCPService and before UpdateInstallationServiceID): self-heal
//     through the mcp_services.plugin_installation_id back-reference — the
//     orphan row and its install-time policy rows are reachable.
//   - Dangling service_id (the interrupted uninstall: the cascade took the
//     service row AND its derived policy rows; the installation row stayed):
//     reads as "no explicit rows" — every snapshot tool falls back to its
//     plugin-domain default, which is the TRUE fail-closed state of the
//     cascade-gone rows, matching the detail view's svc==nil degradation.
//
// Returns the effective service row (nil = no policy store) and the explicit
// rows keyed by tool name. Only real store faults surface as errors.
func (s *pluginService) resolveInstallationPolicyStore(
	ctx context.Context,
	tenantID uint64,
	inst *types.PluginInstallation,
) (*types.MCPService, map[string]*types.MCPToolApproval, error) {
	rowsByTool := map[string]*types.MCPToolApproval{}
	serviceID := inst.ServiceID
	if serviceID == "" {
		resolved, err := s.serviceIDByInstallation(ctx, tenantID, inst.ID)
		if err != nil {
			logger.GetLogger(ctx).Errorf(
				"failed to resolve orphan materialized service for installation %s: %v", inst.ID, err)
			return nil, nil, ErrInstallationPersistFailed
		}
		if resolved != "" {
			logger.GetLogger(ctx).Infof(
				"tool policy store healing empty service_id anchor: installation %s resolves to orphan service %s", inst.ID, resolved)
			serviceID = resolved
		}
	}
	if serviceID == "" {
		return nil, rowsByTool, nil
	}
	svc, err := s.mcpServiceRepo.GetByID(ctx, tenantID, serviceID)
	if err != nil {
		logger.GetLogger(ctx).Errorf(
			"failed to load materialized service for installation %s: %v", inst.ID, err)
		return nil, nil, ErrInstallationPersistFailed
	}
	if svc == nil {
		// Dangling anchor: the service row and its policy rows are gone
		// together — "no explicit rows" is the accurate reading, not a fault.
		return nil, rowsByTool, nil
	}
	rows, err := s.toolApprovalService.ListByService(ctx, tenantID, svc.ID)
	if err != nil {
		logger.GetLogger(ctx).Errorf(
			"failed to load tool policies for installation %s: %v", inst.ID, err)
		return nil, nil, ErrInstallationPersistFailed
	}
	for _, row := range rows {
		rowsByTool[row.ToolName] = row
	}
	return svc, rowsByTool, nil
}

// installationToolPolicies derives the governance rows of ONE installation:
// the accepted snapshot drives membership (removed tools drop out even with
// residual rows), and each tool's verdict comes from its explicit
// MCPToolApproval row when one exists — otherwise the plugin-domain default
// Enabled=ReadOnly. DisabledReason is the deterministic copy from the
// architecture formula: read_only=false AND not enabled.
func (s *pluginService) installationToolPolicies(
	ctx context.Context,
	tenantID uint64,
	inst *types.PluginInstallation,
) ([]interfaces.PluginInstallationToolPolicy, error) {
	_, rowsByTool, err := s.resolveInstallationPolicyStore(ctx, tenantID, inst)
	if err != nil {
		return nil, err
	}
	out := make([]interfaces.PluginInstallationToolPolicy, 0, len(inst.ToolsSnapshot))
	for _, tool := range inst.ToolsSnapshot {
		scopes := make([]string, len(tool.Scopes))
		copy(scopes, tool.Scopes)
		policy := interfaces.PluginInstallationToolPolicy{
			Name:                 tool.Name,
			Description:          tool.Description,
			ReadOnly:             tool.ReadOnly,
			RequiresPersonalAuth: tool.RequiresPersonalAuth,
			Scopes:               scopes,
			// Missing row → plugin-domain default: read tools enabled, WRITE
			// tools disabled (the shared missing-row-default-enabled semantics
			// stay reserved for manual services — types/mcp.go:148-152).
			Enabled: tool.ReadOnly,
		}
		if row, ok := rowsByTool[tool.Name]; ok {
			policy.Enabled = row.Enabled
			policy.RequireApproval = row.RequireApproval
		}
		if !policy.Enabled && !tool.ReadOnly {
			policy.DisabledReason = interfaces.PluginWriteToolDisabledReason
		}
		out = append(out, policy)
	}
	return out, nil
}
