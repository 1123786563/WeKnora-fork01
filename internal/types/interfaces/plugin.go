package interfaces

import (
	"context"
	"time"

	"github.com/Tencent/WeKnora/internal/types"
)

// PluginRepository defines the data access for the plugin domain. T02 lands
// the preview slice; installation persistence (T06) extends this interface.
type PluginRepository interface {
	// CreatePreview persists one verified preview row.
	CreatePreview(ctx context.Context, p *types.PluginPreview) error

	// GetPreview retrieves a preview by ID within a tenant.
	// Not found returns (nil, nil) — the MCPServiceRepository convention.
	GetPreview(ctx context.Context, tenantID uint64, id string) (*types.PluginPreview, error)

	// MarkPreviewConsumed atomically flips consumed_at, exactly once and
	// only while unexpired: the UPDATE carries WHERE consumed_at IS NULL
	// AND expires_at > now in one statement, and a zero RowsAffected
	// yields gorm.ErrRecordNotFound — the caller reads that as "already
	// consumed, expired, or absent" (one rejection path, no check-then-act
	// window) and must reject the confirmation.
	MarkPreviewConsumed(ctx context.Context, tenantID uint64, id string) error

	// DeleteExpiredPreviews drops preview rows past their expiry, consumed
	// or not: a preview is a TTL-bound review artifact (expires_at is the TTL
	// already shown to the admin), not an audit record. The service
	// triggers this lazily and best-effort on the preview success path —
	// a cleanup failure must never block an admin's preview.
	DeleteExpiredPreviews(ctx context.Context, before time.Time) error

	// CreateInstallation persists one accepted installation row.
	CreateInstallation(ctx context.Context, inst *types.PluginInstallation) error

	// GetInstallation retrieves an installation by ID within a tenant.
	// Not found (including a foreign tenant's ID) returns (nil, nil) —
	// the MCPServiceRepository convention; existence is not leaked.
	GetInstallation(ctx context.Context, tenantID uint64, id string) (*types.PluginInstallation, error)

	// GetInstallationByTenantPlugin retrieves the (unique) installation of
	// pluginID within a tenant. Not found returns (nil, nil).
	GetInstallationByTenantPlugin(ctx context.Context, tenantID uint64, pluginID string) (*types.PluginInstallation, error)

	// GetByServiceID retrieves the installation bound to a materialized MCP
	// service ID within a tenant (the runtime snapshot guard's lookup).
	// Not found returns (nil, nil) — including manual services, which have
	// no installation row by construction.
	GetByServiceID(ctx context.Context, tenantID uint64, serviceID string) (*types.PluginInstallation, error)

	// ListInstallationsByTenant returns all installations of a tenant.
	ListInstallationsByTenant(ctx context.Context, tenantID uint64) ([]*types.PluginInstallation, error)

	// UpdateInstallationState flips state in place. A zero-row update
	// returns gorm.ErrRecordNotFound.
	UpdateInstallationState(ctx context.Context, tenantID uint64, id, state string) error

	// UpdateInstallationServiceID backfills the materialized service
	// binding after CreateMCPService succeeds.
	UpdateInstallationServiceID(ctx context.Context, tenantID uint64, id, serviceID string) error

	// DeleteInstallation HARD-deletes an installation row — the
	// compensation path of ConfirmInstallation: a failed materialization
	// must free the (tenant_id, plugin_id) unique slot so the admin can
	// retry. Installations are governance state, not audit history.
	DeleteInstallation(ctx context.Context, tenantID uint64, id string) error

	// HardDeleteServiceCascade HARD-deletes a plugin-materialized MCP
	// service row AND its derived per-tool approval rows (OCR round-1 R12
	// F21): the approvals FK only cascades on hard DELETE, so the shared
	// MCPServiceRepository.Delete (soft) would orphan policy rows keyed to
	// a dead serviceID. Used by the confirm compensation and by uninstall.
	HardDeleteServiceCascade(ctx context.Context, tenantID uint64, serviceID string) error
}

// PluginService defines the plugin business logic. T02 landed the manifest
// preview; the install slice (T06/T07) adds confirm, state and discovery.
type PluginService interface {
	// PreviewFromManifest fetches and verifies a weknora.plugin/1 manifest,
	// persists the preview (identity fingerprint + TTL) and returns the
	// admin review payload. It never creates an installation. The result is
	// a types-layer struct — this interfaces package must not depend on the
	// handler layer (the handler maps it onto its DTO).
	PreviewFromManifest(ctx context.Context, tenantID uint64, actorID, manifestURL string) (*types.PluginPreviewResult, error)

	// ConfirmInstallation consumes one verified preview and installs the
	// pinned version for the tenant: re-verifies the remote (digest +
	// identity fingerprint), writes the installation row, materializes the
	// MCP service and per-tool approval rows (read tools enabled, WRITE
	// TOOLS DISABLED), and marks the preview consumed. Any failure after
	// the installation row exists is compensated (materialized service +
	// installation row removed) so (tenant, plugin) stays installable.
	ConfirmInstallation(ctx context.Context, tenantID uint64, actorID, previewID string) (*types.PluginInstallationResult, error)

	// SetInstallationState disables/enables an installation and syncs the
	// materialized MCP service's Enabled flag (state ∈ {active, disabled}).
	SetInstallationState(ctx context.Context, tenantID uint64, installationID, state string) (*types.PluginInstallationResult, error)

	// UninstallInstallation removes an installation entirely (OCR round-1
	// R12 F06b): the materialized service and its derived policy rows are
	// hard-cascade-deleted, then the installation row — releasing the
	// (tenant, plugin) unique slot so the plugin is installable again.
	// This is the self-heal entry for compensation residue; normal pause
	// flow remains SetInstallationState(disabled).
	UninstallInstallation(ctx context.Context, tenantID uint64, installationID string) error

	// ListInstallations returns the member-facing summaries of every
	// installation in the tenant.
	ListInstallations(ctx context.Context, tenantID uint64) ([]*types.PluginInstallationSummary, error)

	// GetInstallation returns one installation's full view within the
	// tenant; a foreign tenant's ID is "not found".
	GetInstallation(ctx context.Context, tenantID uint64, installationID string) (*types.PluginInstallationResult, error)

	// GetMyConnectionStatus returns ONE principal's personal connection view
	// of one installation (T11, GAP-4): the three-state verdict
	// (authorized/expired/unauthorized) over the per-principal token stored
	// for the installation's materialized service, plus the legacy MCP OAuth
	// endpoint paths mapped onto that service_id. A foreign tenant's
	// installation ID is "not found". The result carries no token material.
	GetMyConnectionStatus(ctx context.Context, tenantID uint64, installationID string, principal types.Principal) (*types.PluginMyConnection, error)

	// PreviewUpgrade re-fetches the LONG-LIVED manifest source
	// (installation.ManifestURL — the preview row is TTL-bound, consumed and
	// never reusable), verifies the candidate it currently declares, and
	// returns the five-dimension diff against the installation's ACCEPTED
	// snapshot (T14). It is READ-ONLY: no installation field, no materialized
	// service row, no policy row, no preview consumption — the accepted
	// version's availability is guaranteed precisely by writing nothing. A
	// candidate version not above the accepted one still previews (downgrade
	// is an admin decision) but the diff carries IsDowngrade=true. A foreign
	// tenant's installation ID is "not found".
	PreviewUpgrade(ctx context.Context, tenantID uint64, installationID string) (*types.PluginUpgradePreviewResult, error)

	// AcceptUpgrade switches the installation to the candidate the admin
	// previewed (T16): it re-fetches installation.ManifestURL and requires
	// the fresh IdentityFingerprint to EQUAL candidateFingerprint (the value
	// PreviewUpgrade returned — "previewed" is the only authority an accept
	// can cite; a remote that moved on is ErrUpgradeCandidateChanged and the
	// admin must run a new preview). The compensated write order (plan 08
	// Task 16 Step 3): installation row first (accepted_version/endpoint_url/
	// tools_snapshot/tools_digest + drift reset), then the materialized MCP
	// service's URL switch (Name/ID/PluginInstallationID stay put — session
	// server_id stability; UpdatedAt refresh recycles the manager's cached
	// client), then INCREMENTAL per-tool policy rows (a NEW tool lands
	// Enabled=ReadOnly; existing rows keep the admin's verdicts). Any
	// failure after the installation write compensates by writing the
	// memory-held old values back — the old version stays callable. The
	// same candidate accepted twice is idempotent (zero writes). actorID is
	// the accepting admin (audit/log surface; the row keeps its creator).
	AcceptUpgrade(ctx context.Context, tenantID uint64, actorID, installationID, candidateFingerprint string) (*types.PluginInstallationResult, error)

	// CheckDrift re-verifies ONE installation against its accepted endpoint
	// (T17, GAP-6; Admin): a LIVE ListTools against installation.EndpointURL
	// — never via the manifest, which may already declare a newer version —
	// diffed against the accepted tools snapshot, with the verdict persisted
	// (drift_state=detected + a name-only detail document, or drift_state=
	// none + a cleared detail when the endpoint matches the baseline again).
	// An unreachable endpoint rejects with zero writes. A foreign tenant's
	// installation ID is "not found".
	CheckDrift(ctx context.Context, tenantID uint64, installationID string) (*types.PluginDriftReport, error)

	// GetDrift returns ONE installation's persisted drift view (T17; Viewer):
	// the row's current state (never-checked rows carry their install-time
	// default), the persisted detail when one exists, and the accepted
	// snapshot's tool names. A pure read — the refresh lever is CheckDrift.
	// A foreign tenant's installation ID is "not found".
	GetDrift(ctx context.Context, tenantID uint64, installationID string) (*types.PluginDriftReport, error)

	// ResolveDrift closes the drift review loop (T17, GAP-6; Admin): re-verify
	// the accepted endpoint LIVE, then accept the CURRENT remote directory as
	// the new verified snapshot — same version, same endpoint, recomputed
	// digest, drift reset to none; per-tool policy rows land incrementally
	// (a NEW tool cannot self-certify read-only over ListTools and installs
	// disabled; existing rows keep the admin's verdicts). An unreachable
	// endpoint rejects with zero writes — the drift state stays as it was.
	ResolveDrift(ctx context.Context, tenantID uint64, actorID, installationID string) (*types.PluginInstallationResult, error)

	// ListInstallationTools returns the tool-governance view of ONE
	// installation (T18): every tool of the ACCEPTED snapshot with its CURRENT
	// policy verdict as DEFINITE values. The snapshot is the membership
	// authority — a tool REMOVED by an upgrade/drift-resolve never reappears
	// through its residual policy row. A snapshot tool with NO explicit row
	// derives Enabled=ReadOnly (write tools default disabled): the
	// missing-row-default-enabled semantics of types.MCPToolApproval stay
	// reserved for MANUAL services and are untouched here. DisabledReason is
	// the deterministic plugin-domain derivation (read_only=false AND not
	// enabled → PluginWriteToolDisabledReason; otherwise empty). A foreign
	// tenant's installation ID is "not found".
	ListInstallationTools(ctx context.Context, tenantID uint64, installationID string) ([]PluginInstallationToolPolicy, error)

	// SetInstallationToolPolicy patches ONE tool's policy of ONE installation
	// (T18; Admin) and returns the refreshed governance list. toolName MUST be
	// in the accepted snapshot (ErrInstallationToolNotFound otherwise — a
	// removed tool's residual row is not addressable). enabled takes effect
	// now; requireApproval is accepted and passed through to the shared
	// MCPToolApprovalService.SetPolicy unchanged — the two are independent
	// nullable pointers of one patch (the T19 approval slice extends THIS
	// endpoint without a signature change). Both nil is
	// ErrInstallationPolicyInvalid; a foreign tenant's installation ID is
	// "not found".
	SetInstallationToolPolicy(ctx context.Context, tenantID uint64, installationID, toolName string, enabled, requireApproval *bool) ([]PluginInstallationToolPolicy, error)
}

// PluginWriteToolDisabledReason is the deterministic governance copy a
// disabled WRITE tool of a plugin installation carries (T18): the plugin
// domain installs write tools disabled (B5 — a manifest declaration is never
// execution authorization) and the admin enables them per tool. Derived, not
// persisted: read_only=false AND Enabled=false on the accepted snapshot.
const PluginWriteToolDisabledReason = "write tool disabled by default; enable explicit"

// PluginInstallationToolPolicy is one row of an installation's tool-governance
// surface (T18): snapshot metadata plus the CURRENT policy verdict as definite
// values (the detail view's Enabled *bool unknowns resolve here — a snapshot
// tool without an explicit row derives Enabled=ReadOnly). Defined in this
// package (the agent_marketplace ReleaseSubmissionView precedent): the
// interfaces contract must not depend on the handler layer, and the
// types-layer installation view stays owned by its slice. The handler maps it
// onto its DTO.
type PluginInstallationToolPolicy struct {
	Name                 string
	Description          string
	ReadOnly             bool
	RequiresPersonalAuth bool
	Scopes               []string
	Enabled              bool
	RequireApproval      bool
	DisabledReason       string
}
