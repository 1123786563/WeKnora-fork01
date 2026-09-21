package service

// O03: Craft sandbox residency, snapshots and resource reclamation.
//
// CraftLifecycle owns the reclamation program of Craft's physical resources.
// The policy core (craft.ResourceState / craft.CanDeleteResource) is absolute:
// active, unknown, decision-pending and referenced resources are never
// reclaimed. The service derives every state flag from the REAL run
// relationships under the session's existing sandbox lifecycle lock — never
// from a caller-supplied boolean:
//
//   - Sweep batches over the durable sweep ledger (craft_lifecycle_states):
//     it holds the lifecycle lock FIRST, re-queries active runs, unfinished
//     (outcome-unknown) delegations, pending decisions and session
//     references, compare-and-swats the deleting mark that blocks new
//     dispatch and restore, then deletes through the provider. A failed
//     provider delete keeps its retry record for the next pass.
//   - The binding clear after a successful cleanup is generation-matched: a
//     workspace rebound to a NEW sandbox generation keeps its new binding —
//     the sweep of the old instance can never take it down.
//   - TombstoneSession writes the tombstone BEFORE anything else, so a
//     session being deleted cannot be taken over by a new dispatch or a
//     restore mid-teardown; run cancellation and the unknown-remote-task risk
//     record come after it.
//   - Dormancy (pausing an idle sandbox with a verified complete snapshot) is
//     a storage decision strictly separate from deletion: versions and
//     snapshots follow the session retention policy and are never deleted
//     because of a sandbox TTL. Orphan objects wait out a recorded candidate
//     window before their references are re-checked.

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	agentruntime "github.com/Tencent/WeKnora/internal/agent/runtime"
	"github.com/Tencent/WeKnora/internal/craft"
	"github.com/Tencent/WeKnora/internal/logger"
	"github.com/Tencent/WeKnora/internal/metrics"
	"github.com/Tencent/WeKnora/internal/modules/execution/sandbox"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// craftLifecycleDeleteTimeout bounds one provider delete call so a wedged
// provider cannot pin the lifecycle lock forever.
const craftLifecycleDeleteTimeout = 30 * time.Second

// ErrCraftQuotaExceeded reports a NEW sandbox refused because the tenant is
// over its sandbox or storage quota. Only new resources are gated — existing
// authorized downloads and cleanup stay available.
var ErrCraftQuotaExceeded = errors.New("craft quota exceeded")

// CraftSandboxDeleter is the provider half of sandbox reclamation: it deletes
// one remote sandbox. The sandbox package's lifecycle clients (Cube/E2B/
// Docker) satisfy this with their Delete call; tests drive fakes.
type CraftSandboxDeleter interface {
	Delete(ctx context.Context, tenantID uint64, sandboxID string) error
}

// CraftSessionRunCanceler cancels the live main runs of a session being
// deleted. The production adapter lists the session's non-terminal agent_runs
// and cancels each through the existing run controller.
type CraftSessionRunCanceler interface {
	CancelSessionRuns(ctx context.Context, tenantID uint64, sessionID string) (canceled []string, err error)
}

// craftRunCanceler adapts the existing run controller onto the canceler port.
type craftRunCanceler struct {
	db   *gorm.DB
	runs CraftRunController
}

// NewCraftSessionRunCanceler builds the production run canceler: non-terminal
// runs of the session are listed from the durable run table and each is
// canceled through the controller (best effort per run; the ids are returned
// so the tombstone record can name what was canceled).
func NewCraftSessionRunCanceler(db *gorm.DB, runs CraftRunController) CraftSessionRunCanceler {
	return &craftRunCanceler{db: db, runs: runs}
}

func (c *craftRunCanceler) CancelSessionRuns(ctx context.Context, tenantID uint64, sessionID string) ([]string, error) {
	if c == nil || c.db == nil || c.runs == nil {
		return nil, errors.New("craft: run canceler requires the run table and controller")
	}
	var runIDs []string
	err := c.db.WithContext(ctx).Table("agent_runs").
		Where("tenant_id = ? AND session_id = ?", tenantID, sessionID).
		Where("status IN ('queued','running','recovering','waiting_user')").
		Order("run_id").Pluck("run_id", &runIDs).Error
	if err != nil {
		return nil, err
	}
	canceled := make([]string, 0, len(runIDs))
	for _, runID := range runIDs {
		if cerr := c.runs.Cancel(ctx, agentruntime.RunKey{TenantID: tenantID, RunID: runID}); cerr != nil {
			logger.Warnf(ctx, "[CraftLifecycle] cancel run %s of session %s failed: %v", runID, sessionID, cerr)
			continue
		}
		canceled = append(canceled, runID)
	}
	return canceled, nil
}

// CraftObjectDeleter removes one stored object. The FileService adapter is
// CraftObjectDeleterFromFileService.
type CraftObjectDeleter interface {
	DeleteObject(ctx context.Context, tenantID uint64, ref string) error
}

type fileServiceObjectDeleter struct {
	files interface {
		DeleteFile(ctx context.Context, filePath string) error
	}
}

// CraftObjectDeleterFromFileService adapts the existing file service.
func CraftObjectDeleterFromFileService(files interface {
	DeleteFile(ctx context.Context, filePath string) error
}) CraftObjectDeleter {
	return fileServiceObjectDeleter{files: files}
}

func (d fileServiceObjectDeleter) DeleteObject(ctx context.Context, tenantID uint64, ref string) error {
	if d.files == nil {
		return errors.New("craft: object deleter is not wired")
	}
	return d.files.DeleteFile(ctx, ref)
}

// CraftLifecycleQuota answers whether the tenant is over its sandbox or
// storage quota. Over-limit gates ONLY new sandboxes (AdmitNewSandbox);
// downloads and cleanup never consult it.
type CraftLifecycleQuota interface {
	SandboxOverLimit(ctx context.Context, tenantID uint64) (bool, error)
	StorageOverLimit(ctx context.Context, tenantID uint64) (bool, error)
}

// CraftSandboxTTLSource answers a sandbox's provider TTL and whether the
// provider can extend it. An unknown TTL must surface workspace risk early —
// a container is never assumed to live forever.
type CraftSandboxTTLSource interface {
	TTL(ctx context.Context, tenantID uint64, sandboxID string) (expiresAt time.Time, extendable bool, err error)
}

// CraftLifecycleConfig assembles the lifecycle service.
type CraftLifecycleConfig struct {
	// DB is the migrated business database.
	DB *gorm.DB
	// Store persists the craft workspace binding (revision CAS clear).
	Store craft.Store
	// Bindings is the existing session sandbox binding store; the sweep runs
	// under its per-session lifecycle lock and clears bindings through its
	// compare-and-delete.
	Bindings sandbox.SessionSandboxBindingStore
	// ActiveRuns is the database active-run check (the real run query).
	ActiveRuns CraftRunActivity
	// SessionExists answers the durable session record (sandbox's checker).
	SessionExists sandbox.SessionExistenceChecker
	// SandboxDeleter deletes the provider sandbox.
	SandboxDeleter CraftSandboxDeleter
	// RunCanceler cancels a deleted session's live runs (optional; without it
	// TombstoneSession records the runs it could not cancel).
	RunCanceler CraftSessionRunCanceler
	// ObjectDeleter deletes orphan storage objects (optional; without it
	// object candidates are recorded but never deleted).
	ObjectDeleter CraftObjectDeleter
	// Quota gates new sandboxes (optional).
	Quota CraftLifecycleQuota
	// TTL answers provider TTLs for the early workspace risk (optional).
	TTL CraftSandboxTTLSource
	// Policy carries the configurable initial suggestion values.
	Policy craft.LifecyclePolicy
	// Now is injectable for tests.
	Now func() time.Time
}

// CraftLifecycle is the O03 reclamation program.
type CraftLifecycle struct {
	db          *gorm.DB
	store       craft.Store
	bindings    sandbox.SessionSandboxBindingStore
	activeRuns  CraftRunActivity
	sessionLife sandbox.SessionExistenceChecker
	deleter     CraftSandboxDeleter
	runCanceler CraftSessionRunCanceler
	objectDel   CraftObjectDeleter
	quota       CraftLifecycleQuota
	ttl         CraftSandboxTTLSource
	policy      craft.LifecyclePolicy
	now         func() time.Time
}

// NewCraftLifecycle validates the assembly and returns the service.
func NewCraftLifecycle(cfg CraftLifecycleConfig) (*CraftLifecycle, error) {
	if cfg.DB == nil || cfg.Store == nil || cfg.Bindings == nil ||
		cfg.ActiveRuns == nil || cfg.SessionExists == nil || cfg.SandboxDeleter == nil {
		return nil, errors.New("craft: lifecycle requires db, store, bindings, active-runs, session existence and a sandbox deleter")
	}
	now := cfg.Now
	if now == nil {
		now = func() time.Time { return time.Now().UTC() }
	}
	return &CraftLifecycle{
		db: cfg.DB, store: cfg.Store, bindings: cfg.Bindings,
		activeRuns: cfg.ActiveRuns, sessionLife: cfg.SessionExists,
		deleter: cfg.SandboxDeleter, runCanceler: cfg.RunCanceler,
		objectDel: cfg.ObjectDeleter, quota: cfg.Quota, ttl: cfg.TTL,
		policy: cfg.Policy.Normalize(), now: now,
	}, nil
}

// -----------------------------------------------------------------------------
// Durable rows (migration 000128_craft_lifecycle / 000048 sqlite)
// -----------------------------------------------------------------------------

type craftLifecycleStateRow struct {
	TenantID     uint64    `gorm:"column:tenant_id;primaryKey;autoIncrement:false"`
	SessionID    string    `gorm:"column:session_id;primaryKey"`
	ResourceKind string    `gorm:"column:resource_kind;primaryKey"`
	Provider     string    `gorm:"column:provider;primaryKey"`
	SandboxID    string    `gorm:"column:sandbox_id;primaryKey"`
	Generation   string    `gorm:"column:generation;primaryKey"`
	ResourceRef  string    `gorm:"column:resource_ref;primaryKey"`
	State        string    `gorm:"column:state"`
	Reason       string    `gorm:"column:reason"`
	EligibleAt   time.Time `gorm:"column:eligible_at"`
	Attempts     int64     `gorm:"column:attempts"`
	LastError    *string   `gorm:"column:last_error"`
	CreatedAt    time.Time `gorm:"column:created_at"`
	UpdatedAt    time.Time `gorm:"column:updated_at"`
}

func (craftLifecycleStateRow) TableName() string { return "craft_lifecycle_states" }

type craftLifecycleEventRow struct {
	ID         string    `gorm:"column:id;primaryKey"`
	TenantID   uint64    `gorm:"column:tenant_id"`
	Kind       string    `gorm:"column:kind"`
	SessionID  string    `gorm:"column:session_id"`
	SandboxID  string    `gorm:"column:sandbox_id"`
	Bytes      int64     `gorm:"column:bytes"`
	OccurredAt time.Time `gorm:"column:occurred_at"`
	RecordedAt time.Time `gorm:"column:recorded_at"`
}

func (craftLifecycleEventRow) TableName() string { return "craft_lifecycle_events" }

// lifecycleStateKey is the natural identity of one sweep-ledger row.
type lifecycleStateKey struct {
	TenantID                  uint64
	SessionID                 string
	Provider                  string
	SandboxID, Generation     string
	ResourceKind, ResourceRef string
}

func (k lifecycleStateKey) row(state, reason string, eligibleAt time.Time, now time.Time) craftLifecycleStateRow {
	return craftLifecycleStateRow{
		TenantID: k.TenantID, SessionID: k.SessionID, ResourceKind: k.ResourceKind,
		Provider: k.Provider, SandboxID: k.SandboxID, Generation: k.Generation,
		ResourceRef: k.ResourceRef, State: state, Reason: reason,
		EligibleAt: eligibleAt, CreatedAt: now, UpdatedAt: now,
	}
}

// craftSessionExistence is the production SessionExistenceChecker: a session
// exists while its durable row is not soft-deleted.
type craftSessionExistence struct{ db *gorm.DB }

// NewCraftSessionExistence builds the production session checker over the
// sessions table's soft-delete column.
func NewCraftSessionExistence(db *gorm.DB) sandbox.SessionExistenceChecker {
	return &craftSessionExistence{db: db}
}

func (c *craftSessionExistence) SessionExists(ctx context.Context, key sandbox.SessionSandboxKey) (bool, error) {
	if c == nil || c.db == nil {
		return false, errors.New("craft: session existence checker requires the database")
	}
	var count int64
	err := c.db.WithContext(ctx).Table("sessions").
		Where("id = ? AND tenant_id = ? AND deleted_at IS NULL", key.SessionID, key.TenantID).
		Count(&count).Error
	return count > 0, err
}

// -----------------------------------------------------------------------------
// Session tombstone (the session-deletion entry)
// -----------------------------------------------------------------------------

// CraftTombstoneResult reports what the tombstone pass did, in order: the
// tombstone itself, the runs it canceled, and the unknown remote tasks whose
// risk is now durably recorded.
type CraftTombstoneResult struct {
	Tombstoned   bool
	SandboxID    string
	Generation   string
	CanceledRuns []string
	UnknownTasks []string
	Note         string
}

// TombstoneSession starts the resource teardown of a session being deleted.
// The ORDER is the contract: the tombstone (state 'deleting') is written
// FIRST so no new dispatch and no snapshot restore can take the session over
// mid-teardown; run cancellation comes second; the resource-reference pass
// comes last and records the unknown remote tasks as a durable risk — an
// outcome that cannot be proven is never reclaimed, it is recorded.
func (s *CraftLifecycle) TombstoneSession(
	ctx context.Context, tenantID uint64, sessionID, reason string,
) (*CraftTombstoneResult, error) {
	if s == nil {
		return nil, fmt.Errorf("%w: lifecycle service is not assembled", craft.ErrInvalidInput)
	}
	if tenantID == 0 || strings.TrimSpace(sessionID) == "" {
		return nil, fmt.Errorf("%w: tombstone requires tenant and session", craft.ErrInvalidInput)
	}
	key := sandbox.SessionSandboxKey{TenantID: tenantID, SessionID: sessionID}
	now := s.now()

	// Read the current binding/workspace identity before anything else; this
	// is a pure read — the tombstone itself is the first write.
	provider, sandboxID, generation := "", "", ""
	if binding, err := s.bindings.Get(ctx, key); err == nil && binding != nil {
		provider, sandboxID, generation = string(binding.Provider), binding.SandboxID, binding.Generation
	}
	if workspace, err := s.readWorkspaceRow(ctx, tenantID, sessionID); err == nil && workspace != nil {
		if sandboxID == "" {
			sandboxID = workspace.SandboxID
		}
		if generation == "" {
			generation = workspace.Generation
		}
	}

	// 1. TOMBSTONE FIRST: the deleting mark blocks GuardDispatch/GuardRestore.
	stateKey := lifecycleStateKey{
		TenantID: tenantID, SessionID: sessionID, ResourceKind: craft.LifecycleResourceSandbox,
		Provider: provider, SandboxID: sandboxID, Generation: generation, ResourceRef: "",
	}
	tombstoned, err := s.markTombstone(ctx, stateKey, reason, now)
	if err != nil {
		return nil, err
	}

	result := &CraftTombstoneResult{
		Tombstoned: tombstoned, SandboxID: sandboxID, Generation: generation,
	}

	// 2. Run cancellation (best effort per run; the durable record names what
	// could not be canceled).
	if s.runCanceler != nil {
		canceled, cerr := s.runCanceler.CancelSessionRuns(ctx, tenantID, sessionID)
		if cerr != nil {
			result.Note = fmt.Sprintf("run cancellation incomplete: %v", cerr)
		}
		result.CanceledRuns = canceled
	}

	// 3. Resource references: unknown remote tasks are recorded in the
	// tombstone row itself, and the tombstone KEEPS blocking (state stays
	// 'deleting') — the sweep later re-derives the protection and owns the
	// kept/risk transition, exactly as it would after a crash.
	unknown, err := s.unfinishedDelegationIDs(ctx, tenantID, sessionID)
	if err != nil {
		result.Note = strings.TrimSpace(result.Note + " " + fmt.Sprintf("unknown-task scan failed: %v", err))
		return result, nil
	}
	result.UnknownTasks = unknown
	if len(unknown) > 0 {
		risk := fmt.Sprintf(
			"%d remote sub-execution(s) with unproven outcome (%s); the sandbox is retained until they resolve",
			len(unknown), strings.Join(unknown, ", "))
		if uerr := s.updateTombstoneReason(ctx, stateKey, risk, now); uerr != nil {
			result.Note = strings.TrimSpace(result.Note + " " + uerr.Error())
		}
	}
	return result, nil
}

// updateTombstoneReason records the unknown-task risk on the tombstone row
// without leaving the blocking 'deleting' state.
func (s *CraftLifecycle) updateTombstoneReason(
	ctx context.Context, key lifecycleStateKey, reason string, now time.Time,
) error {
	updated := s.stateScoped(ctx, key).
		Where("state = ?", craft.LifecycleStateDeleting).
		Updates(map[string]any{"reason": reason, "updated_at": now})
	if updated.Error != nil {
		return updated.Error
	}
	if updated.RowsAffected != 1 {
		return fmt.Errorf("%w: tombstone of session %s is no longer deleting", craft.ErrNotFound, key.SessionID)
	}
	return nil
}

// markTombstone durably writes the 'deleting' mark. It is idempotent: a row
// already deleting stays deleting; a terminal 'deleted' row stays deleted and
// reports tombstoned=false.
func (s *CraftLifecycle) markTombstone(
	ctx context.Context, key lifecycleStateKey, reason string, now time.Time,
) (bool, error) {
	if reason == "" {
		reason = "session deletion"
	}
	row := key.row(craft.LifecycleStateDeleting, reason, now, now)
	created := s.db.WithContext(ctx).Clauses(clause.OnConflict{DoNothing: true}).Create(&row)
	if created.Error != nil {
		return false, created.Error
	}
	if created.RowsAffected == 1 {
		return true, nil
	}
	updated := s.stateScoped(ctx, key).
		Where("state IN ?", []string{
			craft.LifecycleStateCandidate, craft.LifecycleStateFailed,
			craft.LifecycleStateKept, craft.LifecycleStateRisk,
		}).
		Updates(map[string]any{
			"state": craft.LifecycleStateDeleting, "reason": reason,
			"eligible_at": now, "updated_at": now,
		})
	if updated.Error != nil {
		return false, updated.Error
	}
	return updated.RowsAffected == 1, nil
}

// stateScoped returns a statement scoped to exactly one ledger row's natural
// identity. Each condition is a separate typed WHERE so the row identity is
// never subject to positional binding.
func (s *CraftLifecycle) stateScoped(ctx context.Context, key lifecycleStateKey) *gorm.DB {
	return s.db.WithContext(ctx).Model(&craftLifecycleStateRow{}).
		Where("tenant_id = ?", key.TenantID).
		Where("session_id = ?", key.SessionID).
		Where("resource_kind = ?", key.ResourceKind).
		Where("provider = ?", key.Provider).
		Where("sandbox_id = ?", key.SandboxID).
		Where("generation = ?", key.Generation).
		Where("resource_ref = ?", key.ResourceRef)
}

// markState transitions a ledger row's state, keeping the retry bookkeeping.
func (s *CraftLifecycle) markState(
	ctx context.Context, key lifecycleStateKey, state string, now time.Time, reason string,
) error {
	updates := map[string]any{"state": state, "updated_at": now}
	if reason != "" {
		updates["reason"] = reason
	}
	if state == craft.LifecycleStateDeleted {
		updates["eligible_at"] = now
	}
	updated := s.stateScoped(ctx, key).Updates(updates)
	if updated.Error != nil {
		return updated.Error
	}
	if updated.RowsAffected != 1 {
		return fmt.Errorf("%w: lifecycle row for session %s vanished", craft.ErrNotFound, key.SessionID)
	}
	return nil
}

// markFailure records a failed provider delete: the retry record survives
// (state 'failed', attempts+1, last_error, next eligibility after backoff).
func (s *CraftLifecycle) markFailure(
	ctx context.Context, key lifecycleStateKey, now time.Time, err error,
) error {
	message := err.Error()
	updates := map[string]any{
		"state":       craft.LifecycleStateFailed,
		"attempts":    gorm.Expr("attempts + 1"),
		"last_error":  message,
		"eligible_at": now.Add(s.policy.SweepRetryBackoff),
		"updated_at":  now,
	}
	updated := s.stateScoped(ctx, key).Updates(updates)
	if updated.Error != nil {
		return updated.Error
	}
	if updated.RowsAffected != 1 {
		return fmt.Errorf("%w: lifecycle row for session %s vanished", craft.ErrNotFound, key.SessionID)
	}
	return nil
}

// -----------------------------------------------------------------------------
// Dispatch / restore guards
// -----------------------------------------------------------------------------

// GuardDispatch refuses a NEW delegation dispatch while the session's sandbox
// is being cleaned up (the tombstone mark). The lock alone is not enough —
// a dispatch could start right after a sweep pass releases it — so the
// durable deleting mark is the authority.
func (s *CraftLifecycle) GuardDispatch(ctx context.Context, tenantID uint64, sessionID string) error {
	return s.guard(ctx, tenantID, sessionID, "dispatch")
}

// GuardRestore refuses a snapshot restore onto a session whose sandbox is
// being cleaned up: the restore would materialize a generation the sweeper is
// about to reclaim.
func (s *CraftLifecycle) GuardRestore(ctx context.Context, tenantID uint64, sessionID string) error {
	return s.guard(ctx, tenantID, sessionID, "restore")
}

func (s *CraftLifecycle) guard(ctx context.Context, tenantID uint64, sessionID, action string) error {
	if s == nil {
		return fmt.Errorf("%w: lifecycle service is not assembled", craft.ErrInvalidInput)
	}
	var row struct{ State string }
	err := s.db.WithContext(ctx).Model(&craftLifecycleStateRow{}).
		Select("state").
		Where("tenant_id = ? AND session_id = ? AND resource_kind = ? AND state = ?",
			tenantID, sessionID, craft.LifecycleResourceSandbox, craft.LifecycleStateDeleting).
		Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil
	}
	if err != nil {
		// Fail closed: an unreadable guard must not open the teardown window.
		return fmt.Errorf("%w: cannot check the lifecycle state of session %s: %v",
			craft.ErrBusy, sessionID, err)
	}
	return fmt.Errorf("%w: session %s is being cleaned up; %s refused",
		craft.ErrBusy, sessionID, action)
}

// -----------------------------------------------------------------------------
// Sweep
// -----------------------------------------------------------------------------

// Sweep processes one batch of reclaimable resources (default 100). Every
// decision re-derives the resource state from the real run relationships
// under the session's lifecycle lock; nothing is deleted while a run is
// active, a delegation outcome is unknown, a decision is pending or a
// reference is live. Failures keep their retry records; one bad resource
// never aborts the batch.
func (s *CraftLifecycle) Sweep(ctx context.Context, limit int) error {
	if s == nil {
		return fmt.Errorf("%w: lifecycle service is not assembled", craft.ErrInvalidInput)
	}
	if limit <= 0 {
		limit = s.policy.SweepBatchSize
	}
	now := s.now()

	// Discovery first: sessions deleted before this pass (or by deployments
	// that delete without the tombstone entry) get their tombstone rows now —
	// the mark precedes any destructive work, exactly like TombstoneSession.
	if err := s.discoverDeletedSessionTombstones(ctx, limit, now); err != nil {
		return err
	}

	// 'deleting' rows are processed too: a tombstone written by
	// TombstoneSession starts as 'deleting', and a crash mid-delete leaves
	// the same state — both are re-driven here under the lifecycle lock.
	var rows []craftLifecycleStateRow
	err := s.db.WithContext(ctx).Where("state IN ?", []string{
		craft.LifecycleStateCandidate, craft.LifecycleStateFailed,
		craft.LifecycleStateKept, craft.LifecycleStateRisk,
		craft.LifecycleStateDeleting,
	}).Order("eligible_at, session_id").Limit(limit).Find(&rows).Error
	if err != nil {
		return err
	}
	var failures []error
	for _, row := range rows {
		if err := ctx.Err(); err != nil {
			failures = append(failures, err)
			break
		}
		var err error
		switch row.ResourceKind {
		case craft.LifecycleResourceSandbox:
			err = s.sweepSandbox(ctx, row)
		case craft.LifecycleResourceObject:
			err = s.sweepObject(ctx, row)
		default:
			err = s.markState(ctx, lifecycleKeyOf(row), craft.LifecycleStateKept, s.now(),
				fmt.Sprintf("unknown resource kind %q", row.ResourceKind))
		}
		if err != nil && !errors.Is(err, context.Canceled) {
			failures = append(failures, fmt.Errorf("sweep %s of session %s: %w", row.ResourceKind, row.SessionID, err))
		}
	}
	s.refreshPendingDecisionGauge(ctx)
	return errors.Join(failures...)
}

// refreshPendingDecisionGauge refreshes the craft_pending_decisions gauge
// from the durable interaction table. The gauge is fleet-wide (no tenant
// label — identities never enter metric labels); a counting error only logs,
// it never fails the sweep pass that hosts the refresh.
func (s *CraftLifecycle) refreshPendingDecisionGauge(ctx context.Context) {
	var pending int64
	if err := s.db.WithContext(ctx).Table("craft_interactions").
		Where("status = ?", "pending").Count(&pending).Error; err != nil {
		logger.Warnf(ctx, "[CraftLifecycle] pending-decision gauge refresh failed: %v", err)
		return
	}
	metrics.SetCraftPendingDecisions(pending)
}

func lifecycleKeyOf(row craftLifecycleStateRow) lifecycleStateKey {
	return lifecycleStateKey{
		TenantID: row.TenantID, SessionID: row.SessionID, ResourceKind: row.ResourceKind,
		Provider: row.Provider, SandboxID: row.SandboxID, Generation: row.Generation,
		ResourceRef: row.ResourceRef,
	}
}

// discoverDeletedSessionTombstones writes tombstone rows for workspaces whose
// owning session row is soft-deleted and that carry a sandbox binding without
// a lifecycle row yet. Pure discovery: no destructive work happens here.
func (s *CraftLifecycle) discoverDeletedSessionTombstones(ctx context.Context, limit int, now time.Time) error {
	type discovery struct {
		TenantID   uint64
		SessionID  string
		SandboxID  string
		Generation string
	}
	var found []discovery
	err := s.db.WithContext(ctx).Table("craft_workspaces AS ws").
		Select("ws.tenant_id, ws.session_id, ws.sandbox_id, ws.generation").
		Joins("JOIN sessions s ON s.id = ws.session_id AND s.tenant_id = ws.tenant_id AND s.deleted_at IS NOT NULL").
		Joins("LEFT JOIN craft_lifecycle_states st ON st.tenant_id = ws.tenant_id AND st.session_id = ws.session_id AND st.resource_kind = 'sandbox'").
		Where("ws.sandbox_id <> '' AND st.session_id IS NULL").
		Limit(limit).Find(&found).Error
	if err != nil {
		return err
	}
	for _, d := range found {
		key := sandbox.SessionSandboxKey{TenantID: d.TenantID, SessionID: d.SessionID}
		provider := ""
		if binding, berr := s.bindings.Get(ctx, key); berr == nil && binding != nil {
			provider = string(binding.Provider)
		}
		stateKey := lifecycleStateKey{
			TenantID: d.TenantID, SessionID: d.SessionID,
			ResourceKind: craft.LifecycleResourceSandbox,
			Provider:     provider, SandboxID: d.SandboxID, Generation: d.Generation,
		}
		if _, terr := s.markTombstone(ctx, stateKey, "owning session deleted (discovered)", now); terr != nil {
			return terr
		}
	}
	return nil
}

// readWorkspaceRow reads the raw workspace row by tenant+session (no owner
// scope: the sweep runs outside any user request).
func (s *CraftLifecycle) readWorkspaceRow(ctx context.Context, tenantID uint64, sessionID string) (*craft.Workspace, error) {
	var row struct {
		ID                string
		TenantID          uint64
		SessionID         string
		OwnerID           string
		SandboxID         string
		Generation        string
		OpenCodeSessionID string
		RuntimeDigest     string
		Revision          int64
	}
	err := s.db.WithContext(ctx).Table("craft_workspaces").
		Select("id, tenant_id, session_id, owner_id, sandbox_id, generation, oc_session_id, runtime_digest, revision").
		Where("tenant_id = ? AND session_id = ?", tenantID, sessionID).
		Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &craft.Workspace{
		ID: row.ID,
		Scope: craft.Scope{
			TenantID: row.TenantID, UserID: row.OwnerID, SessionID: row.SessionID,
		},
		SandboxID:         row.SandboxID,
		Generation:        row.Generation,
		OpenCodeSessionID: row.OpenCodeSessionID,
		RuntimeDigest:     row.RuntimeDigest,
		Revision:          row.Revision,
	}, nil
}

// unfinishedDelegationIDs lists the session's delegations with no terminal
// result: their outcome is unknown and their resources are never reclaimed.
func (s *CraftLifecycle) unfinishedDelegationIDs(ctx context.Context, tenantID uint64, sessionID string) ([]string, error) {
	var ids []string
	err := s.db.WithContext(ctx).Table("craft_delegations AS d").
		Joins("JOIN craft_workspaces ws ON ws.id = d.workspace_id AND ws.tenant_id = d.tenant_id").
		Where("d.tenant_id = ? AND ws.session_id = ? AND d.result_json IS NULL", tenantID, sessionID).
		Order("d.created_at, d.id").Pluck("d.id", &ids).Error
	return ids, err
}

// pendingDecisions counts the session's pending interactions.
func (s *CraftLifecycle) pendingDecisions(ctx context.Context, tenantID uint64, sessionID string) (int64, error) {
	var count int64
	err := s.db.WithContext(ctx).Table("craft_interactions").
		Where("tenant_id = ? AND session_id = ? AND status = 'pending'", tenantID, sessionID).
		Count(&count).Error
	return count, err
}

// deriveSandboxState derives the reclaim verdict of one sandbox from the real
// run relationships. Referenced means the owning session still exists — a
// live session's sandbox keeps its artwork re-openable.
func (s *CraftLifecycle) deriveSandboxState(
	ctx context.Context, row craftLifecycleStateRow,
) (craft.ResourceState, error) {
	state := craft.ResourceState{EligibleAt: row.EligibleAt}
	key := sandbox.SessionSandboxKey{TenantID: row.TenantID, SessionID: row.SessionID}
	scope := craft.Scope{TenantID: row.TenantID, SessionID: row.SessionID}

	exists, err := s.sessionLife.SessionExists(ctx, key)
	if err != nil {
		return state, fmt.Errorf("check owning session: %w", err)
	}
	state.Referenced = exists

	if active, aerr := s.activeRuns(ctx, scope); aerr != nil {
		return state, aerr
	} else if active {
		state.Active = true
	}
	if unknown, uerr := s.unfinishedDelegationIDs(ctx, row.TenantID, row.SessionID); uerr != nil {
		return state, uerr
	} else if len(unknown) > 0 {
		state.Unknown = true
	}
	if pending, perr := s.pendingDecisions(ctx, row.TenantID, row.SessionID); perr != nil {
		return state, perr
	} else if pending > 0 {
		state.DecisionPending = true
	}
	return state, nil
}

// sweepSandbox reclaims one sandbox under its session's lifecycle lock:
// lock → re-derive state → CAS deleting → provider delete → generation-
// matched binding clear → terminal record.
func (s *CraftLifecycle) sweepSandbox(ctx context.Context, row craftLifecycleStateRow) error {
	key := sandbox.SessionSandboxKey{TenantID: row.TenantID, SessionID: row.SessionID}
	if err := key.Validate(); err != nil {
		return err
	}
	stateKey := lifecycleKeyOf(row)
	return s.bindings.WithLifecycleLock(ctx, key, func(lockCtx context.Context) error {
		now := s.now()

		// The binding may already be gone (another path cleaned it, or the
		// resolve destroyed it): if this row names no sandbox at all there is
		// nothing left to reclaim.
		binding, err := s.bindings.Get(lockCtx, key)
		if err != nil {
			return fmt.Errorf("read sandbox binding: %w", err)
		}
		if binding == nil && row.SandboxID == "" {
			return s.markState(lockCtx, stateKey, craft.LifecycleStateDeleted, now, "no sandbox binding existed")
		}

		state, err := s.deriveSandboxState(lockCtx, row)
		if err != nil {
			return err
		}
		if !craft.CanDeleteResource(state, now) {
			nextState, reason := craft.LifecycleStateKept, keepReason(state)
			if state.Unknown {
				// Unknown remote task: the risk record is durable, names the
				// tasks, and the sandbox stays until the outcome is proven.
				unknown, _ := s.unfinishedDelegationIDs(lockCtx, row.TenantID, row.SessionID)
				nextState = craft.LifecycleStateRisk
				reason = fmt.Sprintf("outcome-unknown sub-execution(s) %v retain the sandbox of session %s",
					unknown, row.SessionID)
			}
			return s.markState(lockCtx, stateKey, nextState, now, reason)
		}

		// CAS the deleting mark: only a non-terminal row transitions, so two
		// sweeping workers can never both believe they own the delete. A row
		// already 'deleting' (a tombstone, or a crash leftover) re-drives —
		// the lifecycle lock serializes the flight, this CAS keeps terminal
		// rows ('deleted') and vanished rows out.
		swapped := s.stateScoped(lockCtx, stateKey).
			Where("state IN ?", []string{
				craft.LifecycleStateCandidate, craft.LifecycleStateFailed,
				craft.LifecycleStateKept, craft.LifecycleStateRisk,
				craft.LifecycleStateDeleting,
			}).
			Updates(map[string]any{
				"state": craft.LifecycleStateDeleting, "updated_at": now, "eligible_at": now,
			})
		if swapped.Error != nil {
			return swapped.Error
		}
		if swapped.RowsAffected != 1 {
			return nil // another worker won the CAS (or the row is terminal)
		}

		// Provider delete. The deleting mark now blocks new dispatch and
		// restore (GuardDispatch/GuardRestore), and the lifecycle lock this
		// sweep holds blocks every resolve of the same session.
		if row.SandboxID != "" {
			delCtx, cancel := context.WithTimeout(lockCtx, craftLifecycleDeleteTimeout)
			err := s.deleter.Delete(delCtx, row.TenantID, row.SandboxID)
			cancel()
			if err != nil {
				return s.markFailure(lockCtx, stateKey, s.now(), err)
			}
		}

		// Generation-matched clear: a workspace rebound to a NEW generation
		// keeps its new binding — only the recorded generation is cleared.
		if workspace, werr := s.readWorkspaceRow(lockCtx, row.TenantID, row.SessionID); werr != nil {
			return werr
		} else if workspace != nil && workspace.Generation == row.Generation {
			cleared := *workspace
			cleared.SandboxID = ""
			cleared.Generation = ""
			cleared.OpenCodeSessionID = ""
			if _, perr := s.store.PutWorkspace(lockCtx, cleared, workspace.Revision); perr != nil {
				// The provider resource is already gone; this failure is a
				// bookkeeping retry, recorded honestly for the next pass.
				return s.markFailure(lockCtx, stateKey, s.now(), perr)
			}
		}
		if binding != nil && binding.SandboxID == row.SandboxID && row.SandboxID != "" {
			if _, derr := s.bindings.DeleteIfMatch(lockCtx, key, binding.Provider, binding.SandboxID); derr != nil {
				return s.markFailure(lockCtx, stateKey, s.now(), derr)
			}
		}
		if row.SandboxID != "" {
			_, _ = s.RecordSandboxEvent(lockCtx, row.TenantID, row.SessionID, row.SandboxID,
				craft.LifecycleEventSandboxStop, now)
		}
		return s.markState(lockCtx, stateKey, craft.LifecycleStateDeleted, now, "")
	})
}

// keepReason names what protected a kept resource — the record operators
// read when a cleanup did not happen.
func keepReason(state craft.ResourceState) string {
	switch {
	case state.Active:
		return "session still has an active run"
	case state.DecisionPending:
		return "session still has a pending decision"
	case state.Referenced:
		return "owning session still exists; the sandbox keeps the artwork re-openable"
	default:
		return "eligibility window not elapsed"
	}
}

// -----------------------------------------------------------------------------
// Orphan objects
// -----------------------------------------------------------------------------

// RecordOrphanCandidate records one storage object that looks unreferenced
// (an upload whose association never completed, a superseded export). The
// object is NOT deleted now: it waits out the configured candidate window
// (24h by default) and a later sweep re-checks its references before any
// deletion. Recording is idempotent per (tenant, session, ref).
func (s *CraftLifecycle) RecordOrphanCandidate(
	ctx context.Context, tenantID uint64, sessionID, ref, reason string,
) error {
	if s == nil {
		return fmt.Errorf("%w: lifecycle service is not assembled", craft.ErrInvalidInput)
	}
	if tenantID == 0 || strings.TrimSpace(sessionID) == "" || strings.TrimSpace(ref) == "" {
		return fmt.Errorf("%w: orphan candidate requires tenant, session and ref", craft.ErrInvalidInput)
	}
	now := s.now()
	key := lifecycleStateKey{
		TenantID: tenantID, SessionID: sessionID,
		ResourceKind: craft.LifecycleResourceObject, ResourceRef: ref,
	}
	row := key.row(craft.LifecycleStateCandidate, reason, now.Add(s.policy.OrphanCandidateWindow), now)
	created := s.db.WithContext(ctx).Clauses(clause.OnConflict{DoNothing: true}).Create(&row)
	return created.Error
}

// objectReferenced answers from the real manifests whether any live version
// file, snapshot object or workspace input still pins the ref. An unreadable
// manifest fails toward protection.
func (s *CraftLifecycle) objectReferenced(ctx context.Context, tenantID uint64, ref string) (bool, error) {
	var versions int64
	if err := s.db.WithContext(ctx).Table("craft_version_files AS vf").
		Joins("JOIN craft_versions v ON v.id = vf.version_id AND v.tenant_id = ?", tenantID).
		Where("vf.resource_ref = ?", ref).Count(&versions).Error; err != nil {
		return true, err
	}
	if versions > 0 {
		return true, nil
	}
	var snapshots int64
	if err := s.db.WithContext(ctx).Table("craft_snapshot_objects AS so").
		Joins("JOIN craft_snapshots sn ON sn.id = so.snapshot_id AND sn.tenant_id = ?", tenantID).
		Where("so.resource_ref = ?", ref).Count(&snapshots).Error; err != nil {
		return true, err
	}
	if snapshots > 0 {
		return true, nil
	}
	var inputs int64
	if err := s.db.WithContext(ctx).Table("craft_workspace_inputs AS wi").
		Joins("JOIN craft_workspaces ws ON ws.id = wi.workspace_id AND ws.tenant_id = ?", tenantID).
		Where("wi.ref = ?", ref).Count(&inputs).Error; err != nil {
		return true, err
	}
	return inputs > 0, nil
}

// sweepObject reclaims one matured orphan-object candidate: the window must
// have elapsed, the references are re-checked from the live manifests, and
// only an unreferenced object is deleted through the object deleter.
func (s *CraftLifecycle) sweepObject(ctx context.Context, row craftLifecycleStateRow) error {
	now := s.now()
	stateKey := lifecycleKeyOf(row)
	if now.Before(row.EligibleAt) {
		return nil // candidate window not elapsed: never delete early
	}
	referenced, err := s.objectReferenced(ctx, row.TenantID, row.ResourceRef)
	if err != nil {
		return s.markState(ctx, stateKey, craft.LifecycleStateKept, now,
			fmt.Sprintf("reference check failed (kept): %v", err))
	}
	state := craft.ResourceState{Referenced: referenced, EligibleAt: row.EligibleAt}
	if !craft.CanDeleteResource(state, now) {
		return s.markState(ctx, stateKey, craft.LifecycleStateKept, now,
			"object is still referenced by a version, snapshot or workspace input")
	}
	if s.objectDel == nil {
		return s.markState(ctx, stateKey, craft.LifecycleStateKept, now,
			"no object deleter wired; object recorded but not reclaimed")
	}
	// 'deleting' is a legal source state (O03 review liveness fix): a crash
	// between this CAS and DeleteObject/markState(deleted) must not strand the
	// row in deleting forever. DeleteObject is idempotent — the sandbox CAS
	// already relies on that for its own crash re-drive — so re-driving a
	// stranded deleting row is the fail-safe direction: the object is either
	// reclaimed or keeps an honest retry record, never a stale ledger row.
	swapped := s.stateScoped(ctx, stateKey).
		Where("state IN ?", []string{
			craft.LifecycleStateCandidate, craft.LifecycleStateFailed,
			craft.LifecycleStateKept, craft.LifecycleStateRisk,
			craft.LifecycleStateDeleting,
		}).
		Updates(map[string]any{"state": craft.LifecycleStateDeleting, "updated_at": now})
	if swapped.Error != nil {
		return swapped.Error
	}
	if swapped.RowsAffected != 1 {
		return nil
	}
	if err := s.objectDel.DeleteObject(ctx, row.TenantID, row.ResourceRef); err != nil {
		return s.markFailure(ctx, stateKey, s.now(), err)
	}
	return s.markState(ctx, stateKey, craft.LifecycleStateDeleted, now, "")
}

// -----------------------------------------------------------------------------
// Usage accounting (sandbox residency and storage bytes-day)
// -----------------------------------------------------------------------------

// CraftLifecycleUsage is the accounted residency and storage of one tenant
// over a window. Dwell sums real start→stop pairs; storage byte-days
// integrate the storage observations over the window.
type CraftLifecycleUsage struct {
	Starts          int
	Stops           int
	OpenStarts      int
	Dwell           time.Duration
	StorageBytes    int64
	StorageBytesDay float64
}

// RecordSandboxEvent records one sandbox start/stop fact. The id IS the
// content-identity dedup key, so redelivery is a no-op; the bool reports
// whether a NEW fact was recorded.
func (s *CraftLifecycle) RecordSandboxEvent(
	ctx context.Context, tenantID uint64, sessionID, sandboxID, kind string, occurredAt time.Time,
) (bool, error) {
	switch kind {
	case craft.LifecycleEventSandboxStart, craft.LifecycleEventSandboxStop:
	default:
		return false, fmt.Errorf("%w: lifecycle event kind %q", craft.ErrInvalidInput, kind)
	}
	if tenantID == 0 || strings.TrimSpace(sandboxID) == "" {
		return false, fmt.Errorf("%w: lifecycle event requires tenant and sandbox", craft.ErrInvalidInput)
	}
	if occurredAt.IsZero() {
		return false, fmt.Errorf("%w: lifecycle event requires its moment", craft.ErrInvalidInput)
	}
	row := craftLifecycleEventRow{
		ID:       craft.LifecycleEventKey(tenantID, sandboxID, kind, occurredAt),
		TenantID: tenantID, Kind: kind, SessionID: sessionID, SandboxID: sandboxID,
		OccurredAt: occurredAt.UTC(), RecordedAt: s.now(),
	}
	created := s.db.WithContext(ctx).Clauses(clause.OnConflict{DoNothing: true}).Create(&row)
	if created.Error != nil {
		return false, created.Error
	}
	return created.RowsAffected == 1, nil
}

// RecordStorageBytes records one storage observation (the bytes the tenant
// holds at that moment). Redelivery dedups the same way.
func (s *CraftLifecycle) RecordStorageBytes(
	ctx context.Context, tenantID uint64, sessionID, sandboxID string, bytes int64, at time.Time,
) (bool, error) {
	if tenantID == 0 || at.IsZero() {
		return false, fmt.Errorf("%w: storage observation requires tenant and moment", craft.ErrInvalidInput)
	}
	if bytes < 0 {
		return false, fmt.Errorf("%w: storage observation is negative", craft.ErrInvalidInput)
	}
	row := craftLifecycleEventRow{
		ID:       craft.LifecycleEventKey(tenantID, sandboxID, craft.LifecycleEventStorageBytes, at),
		TenantID: tenantID, Kind: craft.LifecycleEventStorageBytes,
		SessionID: sessionID, SandboxID: sandboxID, Bytes: bytes,
		OccurredAt: at.UTC(), RecordedAt: s.now(),
	}
	created := s.db.WithContext(ctx).Clauses(clause.OnConflict{DoNothing: true}).Create(&row)
	if created.Error != nil {
		return false, created.Error
	}
	return created.RowsAffected == 1, nil
}

// LifecycleUsage accounts one tenant's sandbox residency and storage over the
// window [since, now]: dwell is the sum of completed start→stop pairs per
// sandbox, and storage byte-days integrate the storage observations (a
// trailing observation is accounted up to now).
func (s *CraftLifecycle) LifecycleUsage(ctx context.Context, tenantID uint64, since time.Time) (CraftLifecycleUsage, error) {
	if tenantID == 0 {
		return CraftLifecycleUsage{}, fmt.Errorf("%w: usage accounting requires a tenant", craft.ErrInvalidInput)
	}
	return s.usageOf(ctx, tenantID, "", since)
}

// SessionLifecycleUsage accounts ONE session's sandbox residency and storage
// over the window — the O04 execution-detail view's "沙箱驻留" line. The
// semantics are exactly LifecycleUsage's, scoped to the session's own events.
func (s *CraftLifecycle) SessionLifecycleUsage(ctx context.Context, tenantID uint64, sessionID string, since time.Time) (CraftLifecycleUsage, error) {
	if s == nil {
		return CraftLifecycleUsage{}, fmt.Errorf("%w: lifecycle service is not assembled", craft.ErrInvalidInput)
	}
	if tenantID == 0 || strings.TrimSpace(sessionID) == "" {
		return CraftLifecycleUsage{}, fmt.Errorf("%w: session usage accounting requires tenant and session", craft.ErrInvalidInput)
	}
	return s.usageOf(ctx, tenantID, sessionID, since)
}

// usageOf loads one tenant's (or one session's) lifecycle events and folds
// them into residency and storage accounting.
func (s *CraftLifecycle) usageOf(ctx context.Context, tenantID uint64, sessionID string, since time.Time) (CraftLifecycleUsage, error) {
	var events []craftLifecycleEventRow
	// The tenant filter is SQL-side (indexed); the since-window is applied in
	// Go below — SQL-side time comparison on the stored column is not
	// portable across the dialects this store runs on.
	query := s.db.WithContext(ctx).
		Where("tenant_id = ?", tenantID)
	if sessionID != "" {
		query = query.Where("session_id = ?", sessionID)
	}
	err := query.Order("sandbox_id, occurred_at, kind").Find(&events).Error
	if err != nil {
		return CraftLifecycleUsage{}, err
	}
	filtered := make([]craftLifecycleEventRow, 0, len(events))
	for _, e := range events {
		if !e.OccurredAt.Before(since) {
			filtered = append(filtered, e)
		}
	}
	return craftLifecycleUsageOfEvents(filtered, s.now()), nil
}

// craftLifecycleUsageOfEvents folds the windowed events: dwell is the sum of
// completed start→stop pairs per sandbox, storage byte-days integrate the
// storage observations (a trailing observation is accounted up to now).
func craftLifecycleUsageOfEvents(events []craftLifecycleEventRow, now time.Time) CraftLifecycleUsage {
	usage := CraftLifecycleUsage{}

	// Residency: pair start→stop per sandbox.
	open := false
	var openedAt time.Time
	lastSandbox := ""
	for _, e := range events {
		if e.Kind != craft.LifecycleEventSandboxStart && e.Kind != craft.LifecycleEventSandboxStop {
			continue
		}
		if e.SandboxID != lastSandbox {
			if open {
				usage.OpenStarts++
			}
			open, lastSandbox = false, e.SandboxID
		}
		switch e.Kind {
		case craft.LifecycleEventSandboxStart:
			if open {
				usage.OpenStarts++ // a restart without a stop: honest, counted open
			}
			open, openedAt = true, e.OccurredAt
			usage.Starts++
		case craft.LifecycleEventSandboxStop:
			if open {
				usage.Dwell += e.OccurredAt.Sub(openedAt)
				open = false
			}
			usage.Stops++
		}
	}
	if open {
		usage.OpenStarts++
	}

	// Storage: integrate the observations (oldest first within the window).
	var observations []craftLifecycleEventRow
	for _, e := range events {
		if e.Kind == craft.LifecycleEventStorageBytes {
			observations = append(observations, e)
		}
	}
	sort.SliceStable(observations, func(i, j int) bool {
		return observations[i].OccurredAt.Before(observations[j].OccurredAt)
	})
	for i, e := range observations {
		usage.StorageBytes += e.Bytes
		end := now
		if i+1 < len(observations) {
			end = observations[i+1].OccurredAt
		}
		if span := end.Sub(e.OccurredAt); span > 0 {
			usage.StorageBytesDay += craft.BytesDay(e.Bytes, span)
		}
	}
	return usage
}

// -----------------------------------------------------------------------------
// Quota and TTL risk
// -----------------------------------------------------------------------------

// AdmitNewSandbox gates NEW sandboxes on the tenant's quota. An over-limit
// quota refuses the start (ErrCraftQuotaExceeded) while downloads and cleanup
// stay available — craft.QuotaAllows is the authority, this is its producer.
func (s *CraftLifecycle) AdmitNewSandbox(ctx context.Context, tenantID uint64) error {
	if s == nil {
		return fmt.Errorf("%w: lifecycle service is not assembled", craft.ErrInvalidInput)
	}
	if s.quota == nil {
		return nil
	}
	sandboxOver, err := s.quota.SandboxOverLimit(ctx, tenantID)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrCraftQuotaExceeded, err)
	}
	storageOver, err := s.quota.StorageOverLimit(ctx, tenantID)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrCraftQuotaExceeded, err)
	}
	if !craft.QuotaAllows(craft.LifecycleActionStartSandbox, sandboxOver, storageOver) {
		return fmt.Errorf("%w: new sandboxes are gated while the tenant is over quota; existing authorized downloads and cleanup remain available",
			ErrCraftQuotaExceeded)
	}
	return nil
}

// WorkspaceTTLRisk surfaces the sandbox-TTL risk of one session's workspace
// EARLY: an unextendable TTL that ends before the next idle window, or an
// unknown TTL, is reported now — the container is never assumed permanent.
func (s *CraftLifecycle) WorkspaceTTLRisk(ctx context.Context, tenantID uint64, sessionID string) (craft.TTLRisk, error) {
	if s == nil {
		return craft.TTLRisk{}, fmt.Errorf("%w: lifecycle service is not assembled", craft.ErrInvalidInput)
	}
	key := sandbox.SessionSandboxKey{TenantID: tenantID, SessionID: sessionID}
	binding, err := s.bindings.Get(ctx, key)
	if err != nil {
		return craft.TTLRisk{}, err
	}
	if binding == nil {
		return craft.TTLRisk{Reason: "session has no bound sandbox"}, nil
	}
	if s.ttl == nil {
		return craft.TTLRisk{
			Reason: "no TTL source wired; the sandbox's provider TTL is unknown",
		}, nil
	}
	expiresAt, extendable, err := s.ttl.TTL(ctx, tenantID, binding.SandboxID)
	if err != nil {
		return craft.TTLRisk{AtRisk: true, Reason: fmt.Sprintf(
			"provider TTL check failed; assume the sandbox may vanish: %v", err)}, nil
	}
	return craft.ProviderTTLRisk(expiresAt, extendable, s.now(), s.policy), nil
}
