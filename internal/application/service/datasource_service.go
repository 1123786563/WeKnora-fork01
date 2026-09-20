package service

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"mime/multipart"
	"net/textproto"
	"reflect"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/Tencent/WeKnora/internal/appconnector"
	"github.com/Tencent/WeKnora/internal/application/access"
	"github.com/Tencent/WeKnora/internal/datasource"
	"github.com/Tencent/WeKnora/internal/datasource/connector/ima"
	"github.com/Tencent/WeKnora/internal/logger"
	"github.com/Tencent/WeKnora/internal/tracing/langfuse"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
	secutils "github.com/Tencent/WeKnora/internal/utils"
	"github.com/hibiken/asynq"
)

// DataSourceService implements the DataSourceService interface
type DataSourceService struct {
	dsRepo            interfaces.DataSourceRepository
	syncLogRepo       interfaces.SyncLogRepository
	knowledgeService  interfaces.KnowledgeService
	kbService         interfaces.KnowledgeBaseService
	taskEnqueuer      interfaces.TaskEnqueuer
	connectorRegistry *datasource.ConnectorRegistry
	scheduler         *datasource.Scheduler
	tenantRepo        interfaces.TenantRepository
	tagService        interfaces.KnowledgeTagService
	audit             interfaces.AuditLogService
	// taskInspector hard-cancels queued/running datasource:sync tasks on
	// delete/pause (SP2-a Task 5). nil keeps the legacy sweep-only behavior.
	taskInspector interfaces.TaskInspector

	// --- A07: scoped connector sync execution (additive, zero values = legacy path) ---
	syncBindingStore appconnector.SyncBindingStore
	syncSpaceState   SyncSpaceStateResolver
	syncPlanActive   func(ctx context.Context, tenantID uint64) bool
	syncFenceMu      sync.Mutex
	syncFences       map[string]int64
}

// NewDataSourceService creates a new data source service
func NewDataSourceService(
	dsRepo interfaces.DataSourceRepository,
	syncLogRepo interfaces.SyncLogRepository,
	knowledgeService interfaces.KnowledgeService,
	kbService interfaces.KnowledgeBaseService,
	taskEnqueuer interfaces.TaskEnqueuer,
	connectorRegistry *datasource.ConnectorRegistry,
	scheduler *datasource.Scheduler,
	tenantRepo interfaces.TenantRepository,
	tagService interfaces.KnowledgeTagService,
	audit interfaces.AuditLogService,
) interfaces.DataSourceService {
	return &DataSourceService{
		dsRepo:            dsRepo,
		syncLogRepo:       syncLogRepo,
		knowledgeService:  knowledgeService,
		kbService:         kbService,
		taskEnqueuer:      taskEnqueuer,
		connectorRegistry: connectorRegistry,
		scheduler:         scheduler,
		tenantRepo:        tenantRepo,
		tagService:        tagService,
		audit:             audit,
	}
}

// SyncSpaceStateResolver supplies the live installation/connection state behind
// a stored binding. Returning nil marks the credentials as legacy — personal or
// space ownership cannot be proven, so the binding requires reauthorization.
type SyncSpaceStateResolver interface {
	SpaceBindingState(ctx context.Context, tenantID uint64, connectionID string) *appconnector.BindingState
}

// SetSyncExecution installs the A07 scoped-sync hooks: the app_datasource_bindings
// store, the live space-connection state resolver, and the plan gate (false =
// expired/canceled plan → new syncs pause, persisted settlements still complete).
// All parameters are optional; nil components keep the legacy behavior.
func (s *DataSourceService) SetSyncExecution(
	store appconnector.SyncBindingStore, spaceState SyncSpaceStateResolver, planActive func(ctx context.Context, tenantID uint64) bool,
) {
	s.syncBindingStore = store
	s.syncSpaceState = spaceState
	s.syncPlanActive = planActive
	if s.syncFences == nil {
		s.syncFences = make(map[string]int64)
	}
}

// SetTaskInspector installs the queue inspector used to hard-cancel queued
// (and running) datasource:sync tasks when a data source is deleted or paused
// (SP2-a Task 5). Setter injection mirrors SetSyncExecution so the dig graph
// stays acyclic and tests can construct the service without a queue backend.
// nil degrades to the legacy sweep-only behavior.
func (s *DataSourceService) SetTaskInspector(inspector interfaces.TaskInspector) {
	s.taskInspector = inspector
}

// hardCancelSyncTasks removes this data source's queued datasource:sync tasks
// from the task backend and signals any active worker to stop. Best-effort:
// errors are logged and swallowed because CancelPendingByDataSource remains
// the durable stop signal — a Redis blip must not fail the delete/pause.
//
// kbID is deliberately passed as "" (verified against matchesKnowledgeBase,
// pinned by TestMatchesKnowledgeBaseEmptyKBIDScopesToDataSource): the
// datasource:sync payload carries no knowledge_base_id, and the matcher
// treats an empty kbID as "skip the KB filter" rather than matching-empty, so
// the call is scoped strictly to dataSourceIDs. Passing ds.KnowledgeBaseID
// instead would over-cancel every queued task of the whole knowledge base.
func (s *DataSourceService) hardCancelSyncTasks(ctx context.Context, dsID string) {
	if s.taskInspector == nil {
		logger.Warnf(ctx, "task inspector not configured; queued sync tasks for ds=%s not hard-cancelled", dsID)
		return
	}
	canceller, ok := s.taskInspector.(interfaces.KnowledgeBaseTaskCanceller)
	if !ok {
		// Lite-mode noop inspector: inline executors cannot be dequeued before
		// they start; CancelPendingByDataSource below remains the stop signal.
		return
	}
	deleted, cancelled, err := canceller.CancelTasksForKnowledgeBase(ctx, "", nil, []string{dsID})
	if err != nil {
		logger.Warnf(ctx, "failed to hard-cancel sync tasks for ds=%s: %v", dsID, err)
		return
	}
	if deleted > 0 || cancelled > 0 {
		logger.Infof(ctx, "hard-cancelled sync tasks for ds=%s: deleted_from_queue=%d active_cancel_signaled=%d",
			dsID, deleted, cancelled)
	}
}

// AuthorizeSyncExecution is the pre-dispatch gate for every team sync (A07).
// Order matters: the plan gate runs first and applies to EVERY data source of
// the tenant — legacy-path datasources cannot bypass an enabled space's budget —
// then a bound data source must hold an active space connection on an active
// installation; legacy credentials that cannot prove ownership, revoked or
// pending-reauthorization connections pause with reason permission.
func (s *DataSourceService) AuthorizeSyncExecution(ctx context.Context, ds *types.DataSource) error {
	if s.syncPlanActive != nil && !s.syncPlanActive(ctx, ds.TenantID) {
		return appconnector.NewSyncPausedError(appconnector.PauseReasonPlan,
			"plan expired; new syncs paused, persisted settlements still complete")
	}
	if s.syncBindingStore == nil || ds == nil {
		return nil
	}
	var state *appconnector.BindingState
	if s.syncSpaceState != nil {
		if row, err := s.syncBindingStore.FindSyncBinding(ctx, ds.TenantID, ds.ID); err == nil && row != nil {
			state = s.syncSpaceState.SpaceBindingState(ctx, ds.TenantID, row.ConnectionID)
		}
	}
	binding, err := appconnector.ResolveSyncBinding(ctx, s.syncBindingStore, ds.TenantID, ds.ID, state)
	if errors.Is(err, appconnector.ErrSyncBindingNotFound) {
		// No binding row: legacy execution path (still plan-gated above).
		return nil
	}
	if err != nil {
		return err
	}
	if binding.RequiresReauthorization {
		return appconnector.NewSyncPausedError(appconnector.PauseReasonPermission,
			"connection requires reauthorization before team sync can run")
	}
	return nil
}

// TakeSyncFence grants a new exclusive lease for the data source's sync run and
// returns the worker's fence token. A previous worker's checkpoints stop
// advancing as soon as a newer fence exists.
func (s *DataSourceService) TakeSyncFence(dsID string) int64 {
	s.syncFenceMu.Lock()
	defer s.syncFenceMu.Unlock()
	if s.syncFences == nil {
		s.syncFences = make(map[string]int64)
	}
	s.syncFences[dsID]++
	return s.syncFences[dsID]
}

// CurrentSyncFence reports the fence a checkpoint must match to advance.
func (s *DataSourceService) CurrentSyncFence(dsID string) int64 {
	s.syncFenceMu.Lock()
	defer s.syncFenceMu.Unlock()
	return s.syncFences[dsID]
}

// currentSyncAuthVersion returns the auth version of the data source's binding
// (0 when unbound — legacy credentials execute without a version stamp).
func (s *DataSourceService) currentSyncAuthVersion(ctx context.Context, ds *types.DataSource) int64 {
	if s.syncBindingStore == nil || ds == nil {
		return 0
	}
	row, err := s.syncBindingStore.FindSyncBinding(ctx, ds.TenantID, ds.ID)
	if err != nil || row == nil {
		return 0
	}
	return row.AuthVersion
}

// CreateDataSource creates a new data source configuration
func (s *DataSourceService) CreateDataSource(ctx context.Context, ds *types.DataSource) (*types.DataSource, error) {
	if ds == nil {
		return nil, datasource.ErrDataSourceInvalid
	}

	// Validate knowledge base exists
	kb, err := s.kbService.GetKnowledgeBaseByID(ctx, ds.KnowledgeBaseID)
	if err != nil || kb == nil {
		return nil, datasource.ErrKnowledgeBaseNotFound
	}
	if kb.TenantID != ds.TenantID {
		return nil, datasource.ErrKnowledgeBaseNotFound
	}

	// Validate connector type
	_, err = s.connectorRegistry.Get(ds.Type)
	if err != nil {
		return nil, err
	}

	// Validate configuration
	if cfg, err := ds.ParseConfig(); err == nil && cfg != nil {
		cfg.StripNonSecretCredentials(ds.Type)
		if blob, err := cfg.ToJSON(); err == nil {
			ds.Config = blob
		}
	}
	if err := s.validateDataSourceConfig(ctx, ds); err != nil {
		return nil, err
	}

	// Create in database
	if err := s.dsRepo.Create(ctx, ds); err != nil {
		logger.Errorf(ctx, "failed to create data source: %v", err)
		return nil, err
	}

	// Register cron schedule if configured
	if ds.SyncSchedule != "" && ds.Status == types.DataSourceStatusActive {
		if err := s.scheduler.AddOrUpdate(ds); err != nil {
			logger.Warnf(ctx, "failed to register cron for ds=%s: %v", ds.ID, err)
		}
	}

	logger.Infof(ctx, "data source created: id=%s type=%s kb=%s", ds.ID, ds.Type, ds.KnowledgeBaseID)
	recordKBActivity(ctx, s.audit, ds.TenantID, ds.KnowledgeBaseID, types.AuditActionDataSourceCreated,
		"data_source", ds.ID, types.AuditOutcomeSuccess,
		map[string]any{"name": ds.Name, "type": ds.Type})
	return ds, nil
}

// GetDataSource retrieves a data source by ID
func (s *DataSourceService) GetDataSource(ctx context.Context, id string) (*types.DataSource, error) {
	ds, err := s.dsRepo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	return ds, nil
}

// ListDataSources lists all data sources for a knowledge base
func (s *DataSourceService) ListDataSources(ctx context.Context, kbID string) ([]*types.DataSource, error) {
	dataSources, err := s.dsRepo.FindByKnowledgeBase(ctx, kbID)
	if err != nil {
		logger.Errorf(ctx, "failed to list data sources: %v", err)
		return nil, err
	}

	// Attach latest sync log to each data source
	for _, ds := range dataSources {
		log, _ := s.syncLogRepo.FindLatest(ctx, ds.ID)
		if log != nil {
			ds.LatestSyncLog = log
		}
	}

	return dataSources, nil
}

// UpdateDataSource updates an existing data source
func (s *DataSourceService) UpdateDataSource(ctx context.Context, ds *types.DataSource) (*types.DataSource, error) {
	if ds == nil || ds.ID == "" {
		return nil, datasource.ErrDataSourceInvalid
	}

	// Verify data source exists
	existing, err := s.dsRepo.FindByID(ctx, ds.ID)
	if err != nil {
		return nil, err
	}

	if ds.KnowledgeBaseID == "" {
		ds.KnowledgeBaseID = existing.KnowledgeBaseID
	}
	if ds.KnowledgeBaseID != existing.KnowledgeBaseID {
		return nil, fmt.Errorf("changing knowledge base is not allowed")
	}

	if ds.TenantID == 0 {
		ds.TenantID = existing.TenantID
	}
	if ds.TenantID != existing.TenantID {
		return nil, datasource.ErrDataSourceInvalid
	}

	// Credentials NEVER flow through this endpoint — they live behind the
	// /credentials subresource. Force-preserve the stored credentials map
	// regardless of what the body says. Log a warning if a stale caller
	// passes one so we can spot them and migrate later. Non-credential
	// fields of Config (Type / ResourceIDs / Settings) flow through.
	var mergedCfg, existingParsedCfg *types.DataSourceConfig
	if len(ds.Config) > 0 {
		incomingCfg, parseIncErr := ds.ParseConfig()
		existingCfg, parseExErr := existing.ParseConfig()
		if parseIncErr == nil && parseExErr == nil && incomingCfg != nil {
			if incomingCfg.HasCredentials() {
				logger.Warnf(ctx,
					"deprecated: credentials in PUT /datasource/%s body are ignored; use PUT /credentials instead",
					secutils.SanitizeForLog(ds.ID))
			}
			merged := *incomingCfg
			if existingCfg != nil {
				merged.Credentials = existingCfg.Credentials
			} else {
				merged.Credentials = nil
			}
			merged.StripNonSecretCredentials(ds.Type)
			if blob, err := merged.ToJSON(); err == nil {
				ds.Config = blob
			}
			mergedCfg = &merged
			existingParsedCfg = existingCfg
		}
	}

	// Validate new configuration if non-credential fields changed. Skip
	// when there are no stored credentials yet (validators would fail with
	// no token to call the live API) and when the parsed config is
	// structurally identical.
	configActuallyChanged := true
	if mergedCfg != nil && existingParsedCfg != nil {
		configActuallyChanged = !reflect.DeepEqual(*mergedCfg, *existingParsedCfg)
	}
	hasCreds := mergedCfg != nil && mergedCfg.HasConfiguredCredentials(ds.Type)
	if hasCreds && (ds.Type != existing.Type || configActuallyChanged) {
		if err := s.validateDataSourceConfig(ctx, ds); err != nil {
			return nil, err
		}
	}

	if err := s.dsRepo.Update(ctx, ds); err != nil {
		logger.Errorf(ctx, "failed to update data source: %v", err)
		return nil, err
	}

	// Update cron schedule
	if err := s.scheduler.AddOrUpdate(ds); err != nil {
		logger.Warnf(ctx, "failed to update cron for ds=%s: %v", ds.ID, err)
	}

	logger.Infof(ctx, "data source updated: id=%s", ds.ID)
	recordKBActivity(ctx, s.audit, ds.TenantID, ds.KnowledgeBaseID, types.AuditActionDataSourceUpdated,
		"data_source", ds.ID, types.AuditOutcomeSuccess,
		map[string]any{"name": ds.Name, "type": ds.Type, "changed_fields": []string{"settings"}})
	return ds, nil
}

// UpdateDataSourceCredentials replaces the connector credential map. This is
// a single atomic write; the previous credential set is discarded entirely
// (callers cannot patch individual keys because half-configured connector
// auth is meaningless). After persisting, the live connection is validated
// so the caller learns immediately if the new credentials are wrong.
func (s *DataSourceService) UpdateDataSourceCredentials(
	ctx context.Context, id string, credentials map[string]interface{},
) (*types.DataSource, error) {
	if id == "" {
		return nil, datasource.ErrDataSourceInvalid
	}
	existing, err := s.dsRepo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	parsed, err := existing.ParseConfig()
	if err != nil {
		return nil, err
	}
	if parsed == nil {
		parsed = &types.DataSourceConfig{Type: existing.Type}
	}
	parsed.Credentials = credentials
	parsed.StripNonSecretCredentials(existing.Type)
	blob, err := parsed.ToJSON()
	if err != nil {
		return nil, err
	}
	existing.Config = blob

	// Run live validation now that the credentials are in place — surfaces
	// "wrong token" feedback immediately to the user instead of waiting for
	// the next scheduled sync.
	if err := s.validateDataSourceConfig(ctx, existing); err != nil {
		return nil, err
	}
	if err := s.dsRepo.Update(ctx, existing); err != nil {
		return nil, err
	}
	logger.Infof(ctx, "DataSource credentials updated: id=%s", secutils.SanitizeForLog(id))
	recordKBActivity(ctx, s.audit, existing.TenantID, existing.KnowledgeBaseID, types.AuditActionDataSourceUpdated,
		"data_source", existing.ID, types.AuditOutcomeSuccess,
		map[string]any{"name": existing.Name, "type": existing.Type, "changed_fields": []string{"credentials"}})
	return existing, nil
}

// ClearDataSourceCredentials wipes the connector credential map without
// touching any other config field. Idempotent.
func (s *DataSourceService) ClearDataSourceCredentials(ctx context.Context, id string) error {
	if id == "" {
		return datasource.ErrDataSourceInvalid
	}
	existing, err := s.dsRepo.FindByID(ctx, id)
	if err != nil {
		return err
	}
	parsed, err := existing.ParseConfig()
	if err != nil {
		return err
	}
	if parsed == nil {
		return nil
	}
	parsed.StripNonSecretCredentials(existing.Type)
	if !parsed.HasConfiguredCredentials(existing.Type) {
		blob, err := parsed.ToJSON()
		if err != nil {
			return err
		}
		existing.Config = blob
		return s.dsRepo.Update(ctx, existing)
	}
	parsed.Credentials = nil
	blob, err := parsed.ToJSON()
	if err != nil {
		return err
	}
	existing.Config = blob
	if err := s.dsRepo.Update(ctx, existing); err != nil {
		return err
	}
	logger.Infof(ctx, "DataSource credentials cleared by user: id=%s", secutils.SanitizeForLog(id))
	recordKBActivity(ctx, s.audit, existing.TenantID, existing.KnowledgeBaseID, types.AuditActionDataSourceUpdated,
		"data_source", existing.ID, types.AuditOutcomeSuccess,
		map[string]any{"name": existing.Name, "type": existing.Type, "changed_fields": []string{"credentials"}})
	return nil
}

// RefreshDataSourceCredential is the machine write-back channel for token
// refresh (SP2-b §6.2). It deliberately differs from the user-facing
// UpdateDataSourceCredentials in four ways:
//
//  1. Anti-overwrite guard first: when no usable credentials are stored the
//     write is refused. This closes the key-rotation trap — after a rotated
//     or removed SYSTEM_AES_KEY, ParseConfig blanks values to "", and an
//     unconditional single-key write-back would re-encrypt those blanks and
//     permanently destroy the surviving ciphertext. Spec §7 hard constraint.
//  2. Single-key update: only the named key (plus the last_refreshed_at
//     bookkeeping stamp) changes; every other credential key round-trips
//     through decrypt → re-encrypt untouched.
//  3. No live validation: the refreshed token is naturally verified by the
//     next sync, so a flapping upstream must not fail the write-back.
//  4. Its own audit action (datasource.credential_auto_refreshed), keeping
//     machine rotations distinguishable from user PUTs in the audit feed.
func (s *DataSourceService) RefreshDataSourceCredential(ctx context.Context, dsID, key, value string) error {
	// An empty value would blank the key — the exact overwrite the guard
	// exists to prevent — so refuse it up front along with missing ids/keys.
	if dsID == "" || key == "" || value == "" {
		return datasource.ErrDataSourceInvalid
	}
	// 1. Read + decrypt the stored config.
	existing, err := s.dsRepo.FindByID(ctx, dsID)
	if err != nil {
		return err
	}
	parsed, err := existing.ParseConfig()
	if err != nil {
		return err
	}
	// 2. Anti-overwrite guard: nothing usable stored (or the stored blob no
	//    longer decrypts) → refuse rather than overwrite.
	if parsed == nil || !parsed.HasConfiguredCredentials(existing.Type) ||
		storedCredentialDecryptFailed(existing.Config, parsed) {
		logger.Warnf(ctx,
			"credential write-back rejected (no usable stored credentials): id=%s field=%s",
			secutils.SanitizeForLog(dsID), secutils.SanitizeForLog(key))
		return datasource.ErrCredentialRefreshRejected
	}
	// 3. Single-key update: the named key plus the last_refreshed_at stamp;
	//    all other keys keep their decrypted values.
	parsed.Credentials[key] = value
	parsed.Credentials[types.CredentialKeyLastRefreshedAt] = time.Now().UTC().Format(time.RFC3339)
	// 4. Re-encrypt and persist (ToJSON is the only credential write path).
	blob, err := parsed.ToJSON()
	if err != nil {
		return err
	}
	existing.Config = blob
	if err := s.dsRepo.Update(ctx, existing); err != nil {
		logger.Errorf(ctx, "failed to persist refreshed credential: id=%s: %v", secutils.SanitizeForLog(dsID), err)
		return err
	}
	// 5. Audit as its own action; the next sync validates the new token.
	logger.Infof(ctx, "DataSource credential auto-refreshed: id=%s field=%s",
		secutils.SanitizeForLog(dsID), secutils.SanitizeForLog(key))
	recordKBActivity(ctx, s.audit, existing.TenantID, existing.KnowledgeBaseID,
		types.AuditActionDataSourceCredentialAutoRefreshed,
		"data_source", existing.ID, types.AuditOutcomeSuccess,
		map[string]any{"name": existing.Name, "type": existing.Type, "field": key})
	return nil
}

// storedCredentialDecryptFailed reports whether any stored credential string
// failed to decrypt under the current SYSTEM_AES_KEY. ParseConfig blanks such
// values to "" (lenient load), so compare the decrypted map against the raw
// jsonb: a non-empty stored string that parsed back empty is a decrypt
// failure, and writing back in that state would overwrite ciphertext with
// blanks. Legitimately empty stored strings are not failures.
func storedCredentialDecryptFailed(raw types.JSON, parsed *types.DataSourceConfig) bool {
	if len(raw) == 0 || parsed == nil {
		return false
	}
	var probe struct {
		Credentials map[string]json.RawMessage `json:"credentials"`
	}
	if err := json.Unmarshal(raw, &probe); err != nil || len(probe.Credentials) == 0 {
		return false
	}
	for k, rawVal := range probe.Credentials {
		var stored string
		if err := json.Unmarshal(rawVal, &stored); err != nil || stored == "" {
			continue // non-string value, or stored-empty is not a decrypt failure
		}
		if decrypted, _ := parsed.Credentials[k].(string); decrypted == "" {
			return true
		}
	}
	return false
}

// credentialRefreshWindow is how close to expires_at a sync start proactively
// rotates the token (SP2-b §6.3): inside the window the machine refresh
// channel swaps the credential before the upstream can start rejecting it
// mid-run.
const credentialRefreshWindow = 5 * time.Minute

// syncAuthVersionCursorKey is the reserved ConnectorCursor key carrying the
// binding auth version a cursor was produced under. Reserved for the service:
// connectors must not use it for their own state.
const syncAuthVersionCursorKey = "_sync_auth_version"

// dataSourceBindingAuthBumper is the optional repository surface the refresh
// trigger uses to advance a binding's auth version after a persisted rotation.
// A local type assertion (the same pattern as dataSourceBindingCleaner)
// because the binding row is deliberately not part of
// interfaces.DataSourceRepository.
type dataSourceBindingAuthBumper interface {
	IncrementAppDataSourceBindingAuthVersion(ctx context.Context, tenantID uint64, dataSourceID string) error
}

// maybeRefreshSyncCredentials runs the SP2-b §6.3 proactive credential refresh
// at sync start (called right after ParseConfig):
//
//   - no expires_at (long-lived) or comfortably far from it → nothing to do;
//   - near expiry (0 < remaining < credentialRefreshWindow) and the connector
//     implements CredentialsRefresher → rotate through the refresher, write
//     every returned key back via the machine channel, advance the binding
//     auth version once (so stale cursors invalidate), merge the new values
//     into config so THIS run executes on the new token, and continue. A
//     failed refresh only logs a warning — the run finishes on the old
//     credentials and the next sync retries the rotation;
//   - already expired → same rotation attempt, but "expired and not refreshed"
//     (no refresher, refresher error, or nothing persisted) returns a
//     SyncPausedError(permission): the run pauses with a reauthorization hint
//     instead of producing fetch-failure error noise.
//
// A refresher result whose every write-back is refused (e.g. the
// anti-overwrite guard after a SYSTEM_AES_KEY rotation) persists nothing, so
// no audit, no auth-version bump and — when the credential was already
// expired — no pause on a token the connector just issued: the run continues
// on the merged in-memory values and the next sync retries the persistence.
// Partially persisted rotations merge only the persisted keys, keeping the
// in-memory config consistent with storage (an unpersisted expires_at would
// otherwise hide the next refresh trigger).
func (s *DataSourceService) maybeRefreshSyncCredentials(
	ctx context.Context, connector datasource.Connector, ds *types.DataSource, config *types.DataSourceConfig,
) error {
	if config == nil {
		return nil
	}
	until, hasExpiry := types.CredentialsExpiry(config)
	if !hasExpiry {
		return nil // long-lived credential: nothing to rotate
	}
	now := time.Now().UTC()
	expired := !until.After(now)
	if !expired && until.Sub(now) > credentialRefreshWindow {
		return nil // comfortably valid: no refresh needed yet
	}
	refresher, canRefresh := connector.(datasource.CredentialsRefresher)
	if !canRefresh {
		if expired {
			return appconnector.NewSyncPausedError(appconnector.PauseReasonPermission,
				"credentials expired and the connector cannot refresh them; reauthorize the data source")
		}
		return nil
	}
	updated, nextRefreshAt, err := refresher.RefreshCredentials(ctx, config)
	if err != nil || len(updated) == 0 {
		reason := "refresher returned no updated credentials"
		if err != nil {
			reason = err.Error()
		}
		if expired {
			return appconnector.NewSyncPausedError(appconnector.PauseReasonPermission,
				"credential refresh failed after expiry: "+reason)
		}
		logger.Warnf(ctx,
			"credential refresh failed near expiry; continuing with previous credentials: ds=%s err=%s",
			secutils.SanitizeForLog(ds.ID), reason)
		return nil
	}

	// Rotate every returned key through the machine channel (guard, encrypted
	// persistence, audit); merge into the in-memory config regardless so this
	// run executes on the token the connector just issued.
	persisted := 0
	for key, value := range updated {
		if werr := s.RefreshDataSourceCredential(ctx, ds.ID, key, value); werr != nil {
			logger.Warnf(ctx, "credential write-back rejected at sync start: ds=%s field=%s err=%v",
				secutils.SanitizeForLog(ds.ID), secutils.SanitizeForLog(key), werr)
			continue
		}
		persisted++
		config.Credentials[key] = value
	}
	// The stored row may have been rewritten by the write-back; re-read so the
	// caller's ds snapshot cannot clobber the fresh blob on a later full-row
	// Update (UpdateSyncState is column-scoped and never at risk).
	if persisted > 0 {
		if fresh, rerr := s.dsRepo.FindByID(ctx, ds.ID); rerr == nil && fresh != nil {
			ds.Config = fresh.Config
		}
		// One bump per rotation, however many keys it touched: the cursor only
		// needs to learn "the credential switched", not how many fields moved.
		// A missing binding row (legacy data source) is a silent skip — the
		// increment affects nothing.
		if bumper, ok := s.dsRepo.(dataSourceBindingAuthBumper); ok {
			if berr := bumper.IncrementAppDataSourceBindingAuthVersion(ctx, ds.TenantID, ds.ID); berr != nil {
				logger.Warnf(ctx, "failed to advance binding auth version after credential refresh: ds=%s err=%v",
					secutils.SanitizeForLog(ds.ID), berr)
			}
		} else {
			logger.Warnf(ctx,
				"data source repository cannot advance binding auth versions; auth version of ds=%s unchanged",
				secutils.SanitizeForLog(ds.ID))
		}
		logger.Infof(ctx, "credentials auto-refreshed at sync start: ds=%s keys=%d next_refresh_at=%s",
			secutils.SanitizeForLog(ds.ID), persisted, nextRefreshAt.Format(time.RFC3339))
	} else {
		logger.Warnf(ctx,
			"credential refresh persisted nothing (write-back refused); running on in-memory values only: ds=%s",
			secutils.SanitizeForLog(ds.ID))
		// The connector issued a usable token; hand it to this run even though
		// persistence was refused.
		for key, value := range updated {
			config.Credentials[key] = value
		}
	}
	return nil
}

// cursorAuthVersionStale reports whether a persisted cursor was produced under
// a different binding auth version than the current one (SP2-b §6.3). A cursor
// stamped with version v is stale unless the binding's current version equals
// v; an unstamped (pre-A07) cursor counts as version 0, stale exactly once a
// binding exists. An unparseable stamp is conservatively stale — resuming on a
// cursor of unknown provenance is the riskier branch.
func cursorAuthVersionStale(cursor *types.SyncCursor, current int64) bool {
	if cursor == nil {
		return false
	}
	raw, ok := cursor.ConnectorCursor[syncAuthVersionCursorKey]
	if !ok {
		return current > 0
	}
	switch v := raw.(type) {
	case float64: // shape after the JSON round-trip through ParseSyncCursor
		return int64(v) != current
	case int64:
		return v != current
	case int:
		return int64(v) != current
	default:
		return true
	}
}

// stampSyncAuthVersion records the binding auth version a cursor was produced
// under, so the next run can tell whether it is still valid to resume
// (cursorAuthVersionStale). No-op for a nil cursor or the legacy version 0.
func stampSyncAuthVersion(cursor *types.SyncCursor, authVersion int64) {
	if cursor == nil || authVersion <= 0 {
		return
	}
	if cursor.ConnectorCursor == nil {
		cursor.ConnectorCursor = map[string]interface{}{}
	}
	cursor.ConnectorCursor[syncAuthVersionCursorKey] = authVersion
}

// DeleteDataSource deletes a data source (soft delete). With
// purgeDocuments=true (SP2-a Task 8) it appends one step at the end of the
// existing sequence: enqueue the async datasource:purge task that drains every
// document the source synced. Without the flag the behavior is byte-identical
// with the pre-SP2-a delete. The purge order is a hard constraint (spec §4.2):
// queued/running syncs are already cancelled above, so the drain cannot race a
// sync rebuilding what it deletes.
func (s *DataSourceService) DeleteDataSource(ctx context.Context, id string, purgeDocuments bool) error {
	// Verify data source exists
	existing, err := s.dsRepo.FindByID(ctx, id)
	if err != nil {
		return err
	}

	if err := s.dsRepo.Delete(ctx, id); err != nil {
		logger.Errorf(ctx, "failed to delete data source: %v", err)
		return err
	}

	// Remove cron schedule
	s.scheduler.Remove(id)

	// Hard-cancel queued (and running) sync tasks BEFORE the durable sweep
	// (SP2-a Task 5): the inspector also deletes the retry records a killed
	// active task transitions into, so flipping the rows first would race a
	// task that the sweep alone cannot stop.
	s.hardCancelSyncTasks(ctx, id)

	// Cancel any pending/running sync logs so queued asynq tasks won't retry
	if err := s.syncLogRepo.CancelPendingByDataSource(ctx, id); err != nil {
		logger.Warnf(ctx, "failed to cancel pending sync logs for ds=%s: %v", id, err)
	}

	auditDetails := map[string]any{"name": existing.Name, "type": existing.Type}
	if purgeDocuments {
		// Best-effort like hardCancelSyncTasks above: the delete itself is
		// already durable, so an enqueue outage must not fail the request —
		// the audit carries the outcome instead.
		if err := s.enqueueDocumentPurge(ctx, existing); err != nil {
			logger.Errorf(ctx, "failed to enqueue purge task for ds=%s (documents retained): %v", id, err)
			auditDetails["purge_documents"] = "enqueue_failed"
		} else {
			auditDetails["purge_documents"] = true
		}
	}

	logger.Infof(ctx, "data source deleted: id=%s purge_documents=%t", id, purgeDocuments)
	recordKBActivity(ctx, s.audit, existing.TenantID, existing.KnowledgeBaseID, types.AuditActionDataSourceDeleted,
		"data_source", existing.ID, types.AuditOutcomeSuccess,
		auditDetails)
	return nil
}

// enqueueDocumentPurge queues the async cascade that removes every document
// the (already soft-deleted) data source synced into its knowledge base. The
// payload snapshots ds.Name as the auto-tag name because the row — and with it
// the name — may be gone by the time the worker runs.
func (s *DataSourceService) enqueueDocumentPurge(ctx context.Context, ds *types.DataSource) error {
	if s.taskEnqueuer == nil {
		return errors.New("task enqueuer not configured")
	}
	payload := types.DataSourcePurgePayload{
		TenantID:        ds.TenantID,
		KnowledgeBaseID: ds.KnowledgeBaseID,
		DataSourceID:    ds.ID,
		TagName:         ds.Name,
		Initiator:       types.TaskInitiatorFromContext(ctx),
	}
	langfuse.InjectTracing(ctx, &payload)
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal purge payload: %w", err)
	}
	task := asynq.NewTask(types.TypeDataSourcePurge, payloadBytes,
		asynq.Queue(types.QueueMaintenance), asynq.MaxRetry(3), asynq.Timeout(2*time.Hour))
	info, err := s.taskEnqueuer.Enqueue(task)
	if err != nil {
		return fmt.Errorf("enqueue purge task: %w", err)
	}
	logger.Infof(ctx, "purge task enqueued: ds=%s kb=%s taskID=%s", ds.ID, ds.KnowledgeBaseID, info.ID)
	return nil
}

// dataSourcePurgeBatchSize is the drain batch of the delete-source cascade,
// aligned with the batch-delete ceiling in internal/handler/knowledge.go.
const dataSourcePurgeBatchSize = 200

// dataSourceBindingCleaner is the optional surface the purge tail uses to
// drop app_datasource_bindings rows. A local type assertion (the same pattern
// as the Task 5 inspector cast) because the binding row is deliberately not
// part of interfaces.DataSourceRepository; repositories without the method
// keep their rows and the miss is only logged.
type dataSourceBindingCleaner interface {
	DeleteAppDataSourceBindingsByDataSource(ctx context.Context, tenantID uint64, dataSourceID string) error
}

// ProcessDataSourcePurge handles the asynq datasource:purge task (SP2-a
// Task 8): unmarshal, install initiator/task metadata, run the drain.
func (s *DataSourceService) ProcessDataSourcePurge(ctx context.Context, task *asynq.Task) error {
	var payload types.DataSourcePurgePayload
	if err := json.Unmarshal(task.Payload(), &payload); err != nil {
		logger.Errorf(ctx, "failed to unmarshal purge payload: %v", err)
		return fmt.Errorf("invalid purge payload: %v: %w", err, asynq.SkipRetry)
	}
	ctx = payload.Initiator.Apply(ctx)
	taskID, _ := asynq.GetTaskID(ctx)
	ctx = withKBActivityTask(ctx, taskID, kbActivityTrigger(ctx))
	return s.PurgeDataSourceDocuments(ctx, payload)
}

// PurgeDataSourceDocuments drains every document one (deleted) data source
// synced into one knowledge base, then cleans up the source's tail: the
// per-source auto-tag (only when nothing references it anymore) and the
// app_datasource_bindings rows.
//
// Each round fetches up to dataSourcePurgeBatchSize live ids scoped to the
// (tenant, kb, data source) triple — other sources' documents are never
// selected — pushes them through the existing batched delete pipeline
// (vectors, graph, wiki pages, chunks, physical files) with the same cleanup
// scope ProcessKnowledgeListDelete uses, then removes the soft-deleted
// tombstones. ctx.Err() is checked between batches so an asynq cancellation
// interrupts the drain; every step is idempotent and a retry continues with
// the rows still live. A cancel landing inside a batch (after its soft
// delete, before its hard delete) leaves that one batch's ≤200 rows as
// tombstones — harmless residue the retry skips, since the deleted source
// can never re-sync those external ids.
func (s *DataSourceService) PurgeDataSourceDocuments(ctx context.Context, payload types.DataSourcePurgePayload) error {
	if payload.TenantID == 0 || payload.KnowledgeBaseID == "" || payload.DataSourceID == "" {
		return fmt.Errorf("invalid purge scope: %w", asynq.SkipRetry)
	}
	if s.knowledgeService == nil {
		return fmt.Errorf("knowledge service unavailable: %w", asynq.SkipRetry)
	}
	knowledgeRepo := s.knowledgeService.GetRepository()
	ctx = types.WithExecutionTenant(ctx, payload.TenantID)

	purged := 0
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		ids, err := knowledgeRepo.FindKnowledgeIDsByDataSourceID(
			ctx, payload.TenantID, payload.KnowledgeBaseID, payload.DataSourceID, dataSourcePurgeBatchSize)
		if err != nil {
			return fmt.Errorf("find purge batch: %w", err)
		}
		if len(ids) == 0 {
			break
		}
		bindings := make(map[string]string, len(ids))
		for _, id := range ids {
			bindings[id] = payload.KnowledgeBaseID
		}
		if err := s.knowledgeService.DeleteKnowledgeList(
			withKnowledgeCleanup(ctx, payload.TenantID, bindings), ids,
		); err != nil {
			return fmt.Errorf("delete purge batch (%d documents): %w", len(ids), err)
		}
		if err := knowledgeRepo.HardDeleteKnowledgeList(ctx, payload.TenantID, ids); err != nil {
			return fmt.Errorf("hard-delete purge batch (%d documents): %w", len(ids), err)
		}
		purged += len(ids)
		logger.Infof(ctx, "purged %d documents of ds=%s (cumulative %d)", len(ids), payload.DataSourceID, purged)
	}

	// Tail cleanup (spec §4.2 step 5). Errors return so an asynq retry re-runs
	// the (now empty) drain and retries just the tail.
	if payload.TagName != "" && s.tagService != nil {
		if err := s.tagService.DeleteOrphanTagByName(ctx, payload.KnowledgeBaseID, payload.TagName); err != nil {
			return fmt.Errorf("delete orphan tag %q: %w", payload.TagName, err)
		}
	}
	if cleaner, ok := s.dsRepo.(dataSourceBindingCleaner); ok {
		if err := cleaner.DeleteAppDataSourceBindingsByDataSource(ctx, payload.TenantID, payload.DataSourceID); err != nil {
			return fmt.Errorf("delete app binding rows: %w", err)
		}
	} else {
		logger.Warnf(ctx, "data source repository cannot delete binding rows; app_datasource_bindings rows of ds=%s retained",
			payload.DataSourceID)
	}

	if purged > 0 {
		logger.Infof(ctx, "purge complete: purged_documents=%d ds=%s kb=%s",
			purged, payload.DataSourceID, payload.KnowledgeBaseID)
		recordKBActivity(ctx, s.audit, payload.TenantID, payload.KnowledgeBaseID, types.AuditActionDataSourceDeleted,
			"data_source", payload.DataSourceID, types.AuditOutcomeSuccess,
			map[string]any{"purge_completed": true, "purged_documents": purged})
	}
	return nil
}

// CountDataSourceDocuments returns how many live documents one data source
// synced into its knowledge base (SP2-a spec §4.1). The handler has already
// proven tenant ownership via getOwnedDataSource; the count re-scopes to the
// same (tenant, kb, data source) triple the delete-source cascade drains, so
// the number shown in the confirmation dialog matches exactly what
// purge_documents=true will remove.
func (s *DataSourceService) CountDataSourceDocuments(ctx context.Context, tenantID uint64, dsID string) (int64, error) {
	ds, err := s.dsRepo.FindByID(ctx, dsID)
	if err != nil {
		return 0, err
	}
	return s.knowledgeService.GetRepository().CountKnowledgeByDataSourceID(ctx, tenantID, ds.KnowledgeBaseID, dsID)
}

// ValidateConnection tests the connection to an external data source
func (s *DataSourceService) ValidateConnection(ctx context.Context, dsID string) error {
	ds, err := s.GetDataSource(ctx, dsID)
	if err != nil {
		return err
	}

	// Get connector
	connector, err := s.connectorRegistry.Get(ds.Type)
	if err != nil {
		return err
	}

	// Parse configuration
	config, err := ds.ParseConfig()
	if err != nil {
		return datasource.ErrInvalidConfig
	}

	// Validate connection
	if err := connector.Validate(ctx, config); err != nil {
		// Update data source with error
		ds.Status = types.DataSourceStatusError
		ds.ErrorMessage = err.Error()
		_ = s.dsRepo.Update(ctx, ds)
		return err
	}

	// Clear error if it was previously in error state
	if ds.Status == types.DataSourceStatusError {
		ds.Status = types.DataSourceStatusActive
		ds.ErrorMessage = ""
		_ = s.dsRepo.Update(ctx, ds)
	}

	return nil
}

// ListAvailableResources lists resources available for sync in the external system.
// parentID enables lazy (on-demand) loading of hierarchical resources: pass "" to
// list the top level, or a resource's ExternalID to list only its direct children.
func (s *DataSourceService) ListAvailableResources(
	ctx context.Context, dsID string, parentID string,
) ([]types.Resource, error) {
	ds, err := s.GetDataSource(ctx, dsID)
	if err != nil {
		return nil, err
	}

	// Get connector
	connector, err := s.connectorRegistry.Get(ds.Type)
	if err != nil {
		return nil, err
	}

	// Parse configuration
	config, err := ds.ParseConfig()
	if err != nil {
		return nil, datasource.ErrInvalidConfig
	}

	// List resources
	resources, err := connector.ListResources(ctx, config, parentID)
	if err != nil {
		logger.Errorf(ctx, "failed to list resources: %v", err)
		return nil, err
	}

	return resources, nil
}

// ResolveResourceAncestors resolves the ancestor ExternalIDs needed to reveal the
// given resources in a lazily-loaded picker (see the connector method for details).
func (s *DataSourceService) ResolveResourceAncestors(
	ctx context.Context, dsID string, resourceIDs []string,
) ([]string, error) {
	if len(resourceIDs) == 0 {
		return []string{}, nil
	}

	ds, err := s.GetDataSource(ctx, dsID)
	if err != nil {
		return nil, err
	}

	connector, err := s.connectorRegistry.Get(ds.Type)
	if err != nil {
		return nil, err
	}

	config, err := ds.ParseConfig()
	if err != nil {
		return nil, datasource.ErrInvalidConfig
	}

	ancestors, err := connector.ResolveResourceAncestors(ctx, config, resourceIDs)
	if err != nil {
		logger.Errorf(ctx, "failed to resolve resource ancestors: %v", err)
		return nil, err
	}

	return ancestors, nil
}

// ManualSync triggers an immediate sync for a data source. forceFull asks the
// worker to drop any persisted cursor and reconcile the whole source (the
// payload field always existed; the API flag now exposes it).
func (s *DataSourceService) ManualSync(ctx context.Context, dsID string, forceFull bool) (*types.SyncLog, error) {
	ds, err := s.GetDataSource(ctx, dsID)
	if err != nil {
		return nil, err
	}

	if ds.Status != types.DataSourceStatusActive &&
		ds.Status != types.DataSourceStatusError &&
		ds.Status != types.DataSourceStatusPaused {
		return nil, datasource.ErrDataSourceNotActive
	}

	// A07: resolve the space connection and current budget/plan state BEFORE
	// scheduling. A pause keeps the persisted cursor (resume continues from the
	// last checkpoint) and is recorded with its machine-readable reason.
	if err := s.AuthorizeSyncExecution(ctx, ds); err != nil {
		logger.Warnf(ctx, "sync for ds=%s paused before scheduling: %v", dsID, err)
		var paused *appconnector.SyncPausedError
		reason := ""
		if errors.As(err, &paused) {
			reason = paused.Reason
		}
		recordKBActivity(ctx, s.audit, ds.TenantID, ds.KnowledgeBaseID, types.AuditActionDataSourceSyncFailed,
			"data_source", ds.ID, types.AuditOutcomeFailed,
			map[string]any{"name": ds.Name, "type": ds.Type, "pause_reason": reason, "trigger": "manual"})
		return nil, err
	}

	// Create sync log
	syncLog := &types.SyncLog{
		DataSourceID: dsID,
		TenantID:     ds.TenantID,
		Status:       types.SyncLogStatusRunning,
		StartedAt:    time.Now().UTC(),
	}

	if err := s.syncLogRepo.Create(ctx, syncLog); err != nil {
		logger.Errorf(ctx, "failed to create sync log: %v", err)
		return nil, err
	}

	// Enqueue sync task
	payload := &types.DataSourceSyncPayload{
		DataSourceID: dsID,
		TenantID:     ds.TenantID,
		SyncLogID:    syncLog.ID,
		ForceFull:    forceFull,
		Initiator:    types.TaskInitiatorFromContext(ctx),
		Trigger:      "manual",
	}
	langfuse.InjectTracing(ctx, payload)

	payloadJSON, _ := json.Marshal(payload)
	task := asynq.NewTask(types.TypeDataSourceSync, payloadJSON,
		asynq.Queue(types.QueueSync), asynq.MaxRetry(5), asynq.Timeout(2*time.Hour))

	info, err := s.taskEnqueuer.Enqueue(task)
	if err != nil {
		logger.Errorf(ctx, "failed to enqueue sync task: %v", err)
		syncLog.Status = types.SyncLogStatusFailed
		syncLog.FinishedAt = timePtr(time.Now().UTC())
		syncLog.ErrorMessage = err.Error()
		_ = s.syncLogRepo.Update(ctx, syncLog)
		if ds.Status != types.DataSourceStatusPaused {
			ds.Status = types.DataSourceStatusError
		}
		ds.ErrorMessage = fmt.Sprintf("Failed to enqueue sync: %v", err)
		_ = s.dsRepo.Update(ctx, ds)
		recordKBActivity(ctx, s.audit, ds.TenantID, ds.KnowledgeBaseID, types.AuditActionDataSourceSyncFailed,
			"data_source", ds.ID, types.AuditOutcomeFailed,
			map[string]any{"name": ds.Name, "type": ds.Type, "sync_log_id": syncLog.ID, "trigger": "manual"})
		return nil, err
	}

	// Correlate the log row with its queue record (SP2-a Task 5) so cancel
	// flows and the runtime dashboard can reach the task by id. Best-effort:
	// the sync itself is already queued.
	if err := s.syncLogRepo.UpdateAsynqTaskID(ctx, syncLog.ID, info.ID); err != nil {
		logger.Warnf(ctx, "failed to record asynq task id for syncLog=%s: %v", syncLog.ID, err)
	} else {
		syncLog.AsynqTaskID = info.ID
	}

	logger.Infof(ctx, "sync task enqueued: ds=%s syncLog=%s taskID=%s", dsID, syncLog.ID, info.ID)
	recordKBActivity(ctx, s.audit, ds.TenantID, ds.KnowledgeBaseID, types.AuditActionDataSourceSyncStarted,
		"data_source", ds.ID, types.AuditOutcomeAccepted,
		map[string]any{
			"name": ds.Name, "type": ds.Type, "sync_log_id": syncLog.ID,
			"task_id": info.ID, "trigger": "manual", "processing_status": "pending",
		})
	return syncLog, nil
}

// syncTriggerManualReindex marks a scoped reindex run's payload (SP2-b §5.3).
const syncTriggerManualReindex = "manual_reindex"

// Per-item failure codes recorded by the scoped reindex path. The frontend
// localises them via datasource.syncError.<code> (Task 7).
const (
	// syncErrTargetedUnsupported: the connector cannot refetch a single item
	// (no TargetedFetcher implementation, or a connector-specific degradation
	// like IMA's hashed external id). Remedy: a normal sync.
	syncErrTargetedUnsupported = "targeted_unsupported"
	// syncErrNotFound: the item no longer exists at the source.
	syncErrNotFound = "not_found"
	// syncErrFetchFailed: the refetch errored; Message carries the raw cause so
	// later consumers can tell transient from permanent failures.
	syncErrFetchFailed = "fetch_failed"
)

// ErrReindexDuplicateRequest is returned by ReindexItems when a reindex with
// the same non-empty request_id is already enqueued: the deterministic asynq
// TaskID collides (ErrTaskIDConflict), so the duplicate request is rejected
// instead of queueing a second run of the same items.
var ErrReindexDuplicateRequest = errors.New(
	"a reindex request with this request_id is already enqueued")

// reindexTaskID builds the deterministic asynq task id for a scoped reindex.
// The manual-sync and scheduler ids share the "dssync:" prefix but never this
// suffix, so reindex runs are recognizable in the queue and a repeated
// request_id collides in Redis (idempotency) rather than double-running.
func reindexTaskID(tenantID uint64, dsID, requestID string) string {
	return fmt.Sprintf("dssync:%d:%s:reindex:%s", tenantID, dsID, requestID)
}

// ReindexItems schedules a scoped (targeted) reindex run (SP2-b §5.3): only
// the listed external ids are refetched, converging into the run's own SyncLog
// with Trigger=manual_reindex. Unlike ManualSync it never mutates data source
// state on failure (Ruling P-4) — the whole-source runs own ds.Status and
// LastSyncResult. A non-empty requestID makes the enqueue idempotent: a repeat
// with the same id while the task is still enqueued returns
// ErrReindexDuplicateRequest.
func (s *DataSourceService) ReindexItems(
	ctx context.Context, dsID string, externalIDs []string, requestID string,
) (string, error) {
	ds, err := s.GetDataSource(ctx, dsID)
	if err != nil {
		return "", err
	}

	// Same lifecycle gate as ManualSync: active/error/paused sources can be
	// repaired by a retry round (failed samples typically live on an errored
	// source), a deleted one cannot.
	if ds.Status != types.DataSourceStatusActive &&
		ds.Status != types.DataSourceStatusError &&
		ds.Status != types.DataSourceStatusPaused {
		return "", datasource.ErrDataSourceNotActive
	}

	// Same pre-dispatch gate as every other team sync (A07): a pause here is
	// recorded with its machine-readable reason and no run is queued.
	if err := s.AuthorizeSyncExecution(ctx, ds); err != nil {
		logger.Warnf(ctx, "scoped reindex for ds=%s paused before scheduling: %v", dsID, err)
		var paused *appconnector.SyncPausedError
		reason := ""
		if errors.As(err, &paused) {
			reason = paused.Reason
		}
		recordKBActivity(ctx, s.audit, ds.TenantID, ds.KnowledgeBaseID, types.AuditActionDataSourceSyncFailed,
			"data_source", ds.ID, types.AuditOutcomeFailed,
			map[string]any{"name": ds.Name, "type": ds.Type, "pause_reason": reason,
				"trigger": syncTriggerManualReindex})
		return "", err
	}

	syncLog := &types.SyncLog{
		DataSourceID: dsID,
		TenantID:     ds.TenantID,
		Status:       types.SyncLogStatusRunning,
		StartedAt:    time.Now().UTC(),
	}
	if err := s.syncLogRepo.Create(ctx, syncLog); err != nil {
		logger.Errorf(ctx, "failed to create scoped reindex sync log: %v", err)
		return "", err
	}

	payload := &types.DataSourceSyncPayload{
		DataSourceID: dsID,
		TenantID:     ds.TenantID,
		SyncLogID:    syncLog.ID,
		Initiator:    types.TaskInitiatorFromContext(ctx),
		Trigger:      syncTriggerManualReindex,
		// The scoped item list is itself the run's item bound; MaxItems mirrors
		// it so queue introspection sees the true size.
		MaxItems: len(externalIDs),
		Scope:    &types.SyncScope{ExternalIDs: externalIDs},
	}
	langfuse.InjectTracing(ctx, payload)

	payloadJSON, _ := json.Marshal(payload)
	task := asynq.NewTask(types.TypeDataSourceSync, payloadJSON)
	opts := []asynq.Option{
		asynq.Queue(types.QueueSync), asynq.MaxRetry(5), asynq.Timeout(2 * time.Hour),
	}
	if requestID != "" {
		opts = append(opts, asynq.TaskID(reindexTaskID(ds.TenantID, dsID, requestID)))
	}

	info, err := s.taskEnqueuer.Enqueue(task, opts...)
	if err != nil {
		if errors.Is(err, asynq.ErrTaskIDConflict) {
			// Idempotent rejection: the first request with this request_id owns
			// the queue slot. Cancel the log row just created for this attempt
			// so no orphaned "running" row is left behind.
			syncLog.Status = types.SyncLogStatusCanceled
			syncLog.FinishedAt = timePtr(time.Now().UTC())
			syncLog.ErrorMessage = "duplicate reindex request: this request_id is already enqueued"
			_ = s.syncLogRepo.Update(ctx, syncLog)
			logger.Warnf(ctx, "scoped reindex rejected as duplicate: ds=%s requestID=%s", dsID, requestID)
			return "", ErrReindexDuplicateRequest
		}
		logger.Errorf(ctx, "failed to enqueue scoped reindex task: %v", err)
		syncLog.Status = types.SyncLogStatusFailed
		syncLog.FinishedAt = timePtr(time.Now().UTC())
		syncLog.ErrorMessage = err.Error()
		_ = s.syncLogRepo.Update(ctx, syncLog)
		// Ruling P-4: a scoped run's failure never touches ds state — unlike
		// ManualSync's enqueue failure, no ds.Status flip happens here.
		return "", err
	}

	// Correlate the log row with its queue record (best-effort, as ManualSync).
	if err := s.syncLogRepo.UpdateAsynqTaskID(ctx, syncLog.ID, info.ID); err != nil {
		logger.Warnf(ctx, "failed to record asynq task id for scoped reindex log=%s: %v", syncLog.ID, err)
	} else {
		syncLog.AsynqTaskID = info.ID
	}

	logger.Infof(ctx, "scoped reindex task enqueued: ds=%s syncLog=%s taskID=%s items=%d requestID=%q",
		dsID, syncLog.ID, info.ID, len(externalIDs), requestID)
	return syncLog.ID, nil
}

// PauseDataSource pauses a data source's scheduled syncs
func (s *DataSourceService) PauseDataSource(ctx context.Context, id string) error {
	ds, err := s.GetDataSource(ctx, id)
	if err != nil {
		return err
	}

	ds.Status = types.DataSourceStatusPaused
	if err := s.dsRepo.Update(ctx, ds); err != nil {
		logger.Errorf(ctx, "failed to pause data source: %v", err)
		return err
	}

	// Remove cron schedule
	s.scheduler.Remove(id)

	// Pause also stops work already in flight (SP2-a Task 5): hard-cancel
	// queued/running tasks first, then flip the log rows so a killed worker's
	// log does not stay "running" forever.
	s.hardCancelSyncTasks(ctx, id)
	if err := s.syncLogRepo.CancelPendingByDataSource(ctx, id); err != nil {
		logger.Warnf(ctx, "failed to cancel pending sync logs for paused ds=%s: %v", id, err)
	}

	logger.Infof(ctx, "data source paused: id=%s", id)
	recordKBActivity(ctx, s.audit, ds.TenantID, ds.KnowledgeBaseID, types.AuditActionDataSourcePaused,
		"data_source", ds.ID, types.AuditOutcomeSuccess, map[string]any{"name": ds.Name, "type": ds.Type})
	return nil
}

// ResumeDataSource resumes a paused data source
func (s *DataSourceService) ResumeDataSource(ctx context.Context, id string) error {
	ds, err := s.GetDataSource(ctx, id)
	if err != nil {
		return err
	}

	ds.Status = types.DataSourceStatusActive
	if err := s.dsRepo.Update(ctx, ds); err != nil {
		logger.Errorf(ctx, "failed to resume data source: %v", err)
		return err
	}

	// Re-register cron schedule
	if err := s.scheduler.AddOrUpdate(ds); err != nil {
		logger.Warnf(ctx, "failed to re-register cron for ds=%s: %v", ds.ID, err)
	}

	logger.Infof(ctx, "data source resumed: id=%s", id)
	recordKBActivity(ctx, s.audit, ds.TenantID, ds.KnowledgeBaseID, types.AuditActionDataSourceResumed,
		"data_source", ds.ID, types.AuditOutcomeSuccess, map[string]any{"name": ds.Name, "type": ds.Type})
	return nil
}

// GetSyncLogs retrieves sync history for a data source
func (s *DataSourceService) GetSyncLogs(ctx context.Context, dsID string, limit int, offset int) ([]*types.SyncLog, error) {
	logs, err := s.syncLogRepo.FindByDataSource(ctx, dsID, limit, offset)
	if err != nil {
		logger.Errorf(ctx, "failed to get sync logs: %v", err)
		return nil, err
	}
	return logs, nil
}

// GetSyncLog retrieves a specific sync log entry
func (s *DataSourceService) GetSyncLog(ctx context.Context, syncLogID string) (*types.SyncLog, error) {
	log, err := s.syncLogRepo.FindByID(ctx, syncLogID)
	if err != nil {
		return nil, err
	}
	return log, nil
}

// ErrSyncLogNotFound is returned by CancelSyncLog when the requested sync log
// does not exist, does not belong to the caller's tenant and data source, or is
// not currently running. All three cases answer identically so the API leaks no
// information about other tenants' sync logs.
var ErrSyncLogNotFound = errors.New("sync log not found")

// syncCanceledMessage is the terminal error message recorded when a run exits
// because a user requested cancellation (SP2-a §3.3).
const syncCanceledMessage = "canceled by user"

// errSyncCanceled is the in-band sentinel that unwinds a sync loop after a
// cooperative cancel was observed at a checkpoint or batch boundary. The sync
// paths convert it to a graceful canceled terminal state; ProcessSync
// additionally normalizes it to nil so asynq never treats a user cancel as a
// failure and retries the run.
var errSyncCanceled = errors.New("sync canceled by user")

// CancelSyncLog flags a running sync for cooperative cancellation (SP2-a
// §3.3). The sync loop observes the flag at its throttled checkpoint/batch
// pulse (see syncPulse) and exits gracefully: status=canceled, error_message
// "canceled by user", the cursor left where the last checkpoint put it so the
// next sync resumes instead of restarting. The log must belong to dsID within
// tenantID and still be running, otherwise ErrSyncLogNotFound — a terminal log
// cannot be canceled. Flagging an already-flagged log is idempotent.
func (s *DataSourceService) CancelSyncLog(ctx context.Context, tenantID uint64, dsID, logID string) error {
	log, err := s.syncLogRepo.FindByID(ctx, logID)
	if err != nil {
		return ErrSyncLogNotFound
	}
	if log.DataSourceID != dsID || log.TenantID != tenantID || log.Status != types.SyncLogStatusRunning {
		return ErrSyncLogNotFound
	}
	return s.syncLogRepo.RequestCancel(ctx, logID)
}

// ProcessSync handles the actual sync operation (called by asynq task)
func (s *DataSourceService) ProcessSync(ctx context.Context, task *asynq.Task) error {
	err := s.processSync(ctx, task)
	// SP2-a §3.3: a cooperative cancel is a graceful exit, not a failure. The
	// sync paths handle the sentinel locally; this normalization is the
	// belt-and-braces guarantee that a sentinel leaking from any path never
	// makes asynq retry a run the user asked to stop.
	if errors.Is(err, errSyncCanceled) {
		return nil
	}
	return err
}

func (s *DataSourceService) processSync(ctx context.Context, task *asynq.Task) error {
	var payload types.DataSourceSyncPayload
	if err := json.Unmarshal(task.Payload(), &payload); err != nil {
		logger.Errorf(ctx, "failed to unmarshal sync payload: %v", err)
		return err
	}
	ctx = payload.Initiator.Apply(ctx)
	taskID, _ := asynq.GetTaskID(ctx)
	ctx = withKBActivityTask(ctx, taskID, payload.Trigger)

	logger.Infof(ctx, "processing data source sync: ds=%s syncLog=%s", payload.DataSourceID, payload.SyncLogID)

	// Get data source
	ds, err := s.GetDataSource(ctx, payload.DataSourceID)
	if err != nil {
		logger.Warnf(ctx, "data source not found (likely deleted), cancelling sync: ds=%s err=%v", payload.DataSourceID, err)
		if syncLog, slErr := s.syncLogRepo.FindByID(ctx, payload.SyncLogID); slErr == nil && syncLog != nil {
			syncLog.Status = types.SyncLogStatusCanceled
			syncLog.FinishedAt = timePtr(time.Now().UTC())
			syncLog.ErrorMessage = "data source has been deleted"
			_ = s.syncLogRepo.Update(ctx, syncLog)
		}
		return nil
	}

	// Get sync log
	syncLog, err := s.syncLogRepo.FindByID(ctx, payload.SyncLogID)
	if err != nil {
		logger.Errorf(ctx, "failed to get sync log: %v", err)
		return nil
	}

	kb, kbErr := s.kbService.GetKnowledgeBaseByID(ctx, ds.KnowledgeBaseID)
	if kbErr != nil {
		logger.Warnf(ctx, "knowledge base not found (likely deleted), cancelling sync: kb=%s ds=%s err=%v",
			ds.KnowledgeBaseID, payload.DataSourceID, kbErr)
		syncLog.Status = types.SyncLogStatusCanceled
		syncLog.FinishedAt = timePtr(time.Now().UTC())
		syncLog.ErrorMessage = "knowledge base has been deleted"
		_ = s.syncLogRepo.Update(ctx, syncLog)
		return nil
	}

	ctx, err = access.WithKBTaskWrite(ctx, kb, ds.TenantID)
	if err != nil {
		return fmt.Errorf("%w: data source KB does not belong to its tenant", asynq.SkipRetry)
	}
	wasPaused := ds.Status == types.DataSourceStatusPaused

	// A07: gate team-sync execution on the resolved space connection and the
	// current budget/plan state. A pause records the reason, keeps the persisted
	// cursor and does not flip the data source into error state; persisted
	// settlements already handed off still complete (MaySettlePersistedSync).
	if err := s.AuthorizeSyncExecution(ctx, ds); err != nil {
		logger.Warnf(ctx, "sync for ds=%s paused before execution: %v", payload.DataSourceID, err)
		syncLog.Status = types.SyncLogStatusFailed
		syncLog.FinishedAt = timePtr(time.Now().UTC())
		syncLog.ErrorMessage = err.Error()
		_ = s.syncLogRepo.Update(ctx, syncLog)
		var paused *appconnector.SyncPausedError
		reason := ""
		if errors.As(err, &paused) {
			reason = paused.Reason
		}
		recordKBActivity(ctx, s.audit, ds.TenantID, ds.KnowledgeBaseID, types.AuditActionDataSourceSyncFailed,
			"data_source", ds.ID, types.AuditOutcomeFailed,
			map[string]any{"name": ds.Name, "type": ds.Type, "pause_reason": reason})
		return nil
	}

	// Get connector
	connector, err := s.connectorRegistry.Get(ds.Type)
	if err != nil {
		logger.Errorf(ctx, "connector not found: type=%s", ds.Type)
		syncLog.Status = types.SyncLogStatusFailed
		syncLog.FinishedAt = timePtr(time.Now().UTC())
		syncLog.ErrorMessage = fmt.Sprintf("Connector not found: %s", ds.Type)
		_ = s.syncLogRepo.Update(ctx, syncLog)
		if !wasPaused {
			ds.Status = types.DataSourceStatusError
		}
		ds.ErrorMessage = syncLog.ErrorMessage
		_ = s.dsRepo.Update(ctx, ds)
		return err
	}

	// Parse configuration
	config, err := ds.ParseConfig()
	if err != nil {
		logger.Errorf(ctx, "failed to parse config: %v", err)
		syncLog.Status = types.SyncLogStatusFailed
		syncLog.FinishedAt = timePtr(time.Now().UTC())
		syncLog.ErrorMessage = fmt.Sprintf("Invalid configuration: %v", err)
		_ = s.syncLogRepo.Update(ctx, syncLog)
		if !wasPaused {
			ds.Status = types.DataSourceStatusError
		}
		ds.ErrorMessage = syncLog.ErrorMessage
		_ = s.dsRepo.Update(ctx, ds)
		return err
	}
	// Surface the KB's multimodal/VLM state to the connector so it only extracts
	// embedded images for OCR when the KB can actually ingest them (never persisted).
	config.MultimodalEnabled = kb.IsMultimodalEnabled()

	// Scoped reindex run (SP2-b §5.3): a payload naming specific items takes a
	// dedicated per-item refetch path. It must branch before the streaming and
	// batch paths — neither walks individual external ids — and it converges
	// into its own SyncLog without touching whole-source state.
	if payload.Scope != nil && len(payload.Scope.ExternalIDs) > 0 {
		return s.runScopedReindex(ctx, connector, ds, syncLog, config, &payload)
	}

	// SP2-b §6.3: proactive credential refresh at sync start, on the decrypted
	// config and before any fetch. Deliberately after the scoped branch: a
	// scoped reindex converges into its own run (Ruling P-4) and must neither
	// rotate whole-source credentials nor pause on their expiry. An expired
	// credential that cannot be refreshed pauses here with the A07 permission
	// semantics — the same state machine as AuthorizeSyncExecution above: the
	// log records the pause reason, the persisted cursor stays, the data
	// source never flips to error and ProcessSync returns nil (no asynq noise).
	if err := s.maybeRefreshSyncCredentials(ctx, connector, ds, config); err != nil {
		logger.Warnf(ctx, "sync for ds=%s paused before fetch: %v", payload.DataSourceID, err)
		syncLog.Status = types.SyncLogStatusFailed
		syncLog.FinishedAt = timePtr(time.Now().UTC())
		syncLog.ErrorMessage = err.Error()
		_ = s.syncLogRepo.Update(ctx, syncLog)
		var paused *appconnector.SyncPausedError
		reason := ""
		if errors.As(err, &paused) {
			reason = paused.Reason
		}
		recordKBActivity(ctx, s.audit, ds.TenantID, ds.KnowledgeBaseID, types.AuditActionDataSourceSyncFailed,
			"data_source", ds.ID, types.AuditOutcomeFailed,
			map[string]any{"name": ds.Name, "type": ds.Type, "pause_reason": reason})
		return nil
	}

	// Streaming path: connectors that support it interleave fetch→ingest→
	// checkpoint so a large sync bounds memory and resumes after a timeout
	// instead of restarting (Tencent/WeKnora#2136). Others fall back below.
	if sc, ok := connector.(datasource.StreamingConnector); ok {
		return s.processSyncStreaming(ctx, sc, ds, syncLog, config, payload, wasPaused)
	}

	// The binding auth version this run executes under (0 = legacy/unbound).
	// Read AFTER the refresh trigger so a rotation performed above is visible
	// here and to the streaming path below.
	authVersion := s.currentSyncAuthVersion(ctx, ds)

	// Fetch items based on sync mode
	var items []types.FetchedItem
	var nextCursor *types.SyncCursor
	var fetchErr error

	if payload.ForceFull || ds.SyncMode == types.SyncModeFull {
		// Full sync. FullSyncWithCursor connectors (DingTalk) re-fetch every
		// document while still reconciling deletions against the previous
		// cursor; plain FetchAll connectors keep the no-cursor behaviour.
		if full, ok := connector.(datasource.FullSyncWithCursor); ok {
			cursor, _ := ds.ParseSyncCursor()
			items, nextCursor, fetchErr = full.FetchAllFromCursor(ctx, config, config.ResourceIDs, cursor)
		} else {
			items, fetchErr = connector.FetchAll(ctx, config, config.ResourceIDs)
		}
		logger.Infof(ctx, "full sync fetched %d items", len(items))
	} else {
		// Incremental sync
		cursor, _ := ds.ParseSyncCursor()
		// SP2-b §6.3: a cursor produced under a different binding auth version
		// (credential rotation) must not be resumed across the token switch —
		// drop it and let the connector walk everything (ForceFull semantics),
		// so nothing changed under the old token slips through incrementally.
		if cursorAuthVersionStale(cursor, authVersion) {
			logger.Infof(ctx, "auth version changed, full reconciliation: ds=%s cursor_version_dropped", payload.DataSourceID)
			cursor = nil
		}
		items, nextCursor, fetchErr = connector.FetchIncremental(ctx, config, cursor)
		logger.Infof(ctx, "incremental sync fetched %d items", len(items))
	}

	var fetchWarnings []string
	var partialFetch *datasource.PartialFetchError
	if errors.As(fetchErr, &partialFetch) {
		fetchWarnings = partialFetch.Details
		fetchErr = nil
	}

	if fetchErr != nil {
		// Persist connector cursor even when fetch failed so transient outages
		// (e.g. RSS feed downtime) do not force a full re-ingest on recovery.
		if nextCursor != nil {
			stampSyncAuthVersion(nextCursor, authVersion)
			if cursorJSON, cerr := nextCursor.ToJSON(); cerr == nil {
				ds.LastSyncCursor = cursorJSON
				if uerr := s.dsRepo.UpdateSyncState(ctx, ds); uerr != nil {
					logger.Warnf(ctx, "failed to persist sync cursor after fetch error: %v", uerr)
				}
			}
		}
		logger.Errorf(ctx, "fetch operation failed: %v", fetchErr)
		syncLog.Status = types.SyncLogStatusFailed
		syncLog.FinishedAt = timePtr(time.Now().UTC())
		syncLog.ErrorMessage = fmt.Sprintf("Fetch failed: %v", fetchErr)
		_ = s.syncLogRepo.Update(ctx, syncLog)
		if !wasPaused {
			ds.Status = types.DataSourceStatusError
		}
		ds.ErrorMessage = syncLog.ErrorMessage
		_ = s.dsRepo.Update(ctx, ds)
		return fetchErr
	}

	// Process fetched items and write to knowledge base
	result := &types.SyncResult{
		Total: len(items),
	}

	// Set tenant context so KnowledgeService can resolve tenant info correctly
	ctx = context.WithValue(ctx, types.TenantIDContextKey, ds.TenantID)

	tenant, err := s.tenantRepo.GetTenantByID(ctx, ds.TenantID)
	if err != nil {
		logger.Errorf(ctx, "failed to get tenant info: %v", err)
		syncLog.Status = types.SyncLogStatusFailed
		syncLog.FinishedAt = timePtr(time.Now().UTC())
		syncLog.ErrorMessage = fmt.Sprintf("Failed to get tenant info: %v", err)
		_ = s.syncLogRepo.Update(ctx, syncLog)
		if !wasPaused {
			ds.Status = types.DataSourceStatusError
		}
		ds.ErrorMessage = syncLog.ErrorMessage
		_ = s.dsRepo.Update(ctx, ds)
		return err
	}
	ctx = context.WithValue(ctx, types.TenantInfoContextKey, tenant)

	// Auto-tag: find or create a tag for this data source so synced items are easily identifiable
	autoTagIDs := s.resolveAutoTagIDs(ctx, ds)

	// Heartbeat throttle state for the batch loop; the first item writes
	// immediately and later beats are spaced by syncHeartbeatInterval.
	var lastBeat time.Time
	for i, item := range items {
		item := item
		s.applyFetchedItem(withKBActivitySuppressed(ctx), ds, &item, autoTagIDs, result)
		// SP2-a §3.3: the same throttled pulse as the streaming path —
		// heartbeat plus cooperative-cancel check. The batch path has no
		// checkpoints, so its cursor is only persisted on completion: on cancel
		// the run stops here and the stored cursor keeps its previous value,
		// which is exactly the resume semantics.
		if syncPulse(ctx, s.syncLogRepo, syncLog.ID, &lastBeat) {
			s.cancelSyncRun(ctx, syncLog, result)
			logger.Infof(ctx, "batch sync canceled by user after %d/%d item(s): ds=%s",
				i+1, len(items), payload.DataSourceID)
			return nil
		}
		// Every batchProgressInterval items, mirror the running counts into the
		// sync log (same six fields as a streaming checkpoint) so a long batch
		// sync shows mid-flight progress instead of jumping from 0 to done.
		// Best-effort, like Checkpoint's progress write.
		if (i+1)%batchProgressInterval == 0 {
			syncLog.ItemsTotal = result.Total
			syncLog.ItemsCreated = result.Created
			syncLog.ItemsUpdated = result.Updated
			syncLog.ItemsDeleted = result.Deleted
			syncLog.ItemsSkipped = result.Skipped
			syncLog.ItemsFailed = result.Failed
			if err := s.syncLogRepo.UpdateResult(ctx, syncLog); err != nil {
				logger.Warnf(ctx, "failed to persist sync log progress at batch boundary: %v", err)
			}
		}
	}

	resultJSON, _ := result.ToJSON()
	if err := allFetchedItemsFailedError(result); err != nil {
		logger.Errorf(ctx, "data source sync failed while processing fetched items: %v", err)
		s.updateSyncRunResult(ctx, ds, syncLog, result, resultJSON, types.SyncLogStatusFailed, err.Error(), wasPaused)
		return err
	}

	// Update cursor for next incremental sync
	if nextCursor != nil {
		// Stamp the auth version the cursor was produced under so the next
		// run's staleness check (see the incremental branch above) can match
		// it — the batch path has no Checkpoint hook to do this mid-run.
		stampSyncAuthVersion(nextCursor, authVersion)
		cursorJSON, _ := nextCursor.ToJSON()
		ds.LastSyncCursor = cursorJSON
	}

	ds.LastSyncAt = timePtr(time.Now().UTC())
	syncStatus := types.SyncLogStatusSuccess
	syncErrorMessage := ""
	if len(fetchWarnings) > 0 {
		syncStatus = types.SyncLogStatusPartial
		syncErrorMessage = fmt.Sprintf("Some feeds failed: %s", strings.Join(fetchWarnings, "; "))
		for _, w := range fetchWarnings {
			result.Errors = append(result.Errors, types.SyncItemError{Message: w})
		}
		resultJSON, _ = result.ToJSON()
	}
	if result.Failed > 0 {
		// Per-document failures flip the sync to partial so the drawer shows
		// which docs didn't make it (mirrors the streaming path). Deletion
		// failures additionally only retry on a later full sync.
		syncStatus = types.SyncLogStatusPartial
		if syncErrorMessage != "" {
			syncErrorMessage += "; "
		}
		syncErrorMessage += fmt.Sprintf("%d document(s) failed to sync", result.Failed)
		if result.DeletionFailed > 0 {
			syncErrorMessage += fmt.Sprintf(
				"; %d deletion failure(s) will only retry on the next full sync", result.DeletionFailed)
		}
	}
	s.updateSyncRunResult(ctx, ds, syncLog, result, resultJSON, syncStatus, syncErrorMessage, wasPaused)

	logger.Infof(ctx, "data source sync completed: ds=%s created=%d updated=%d deleted=%d",
		payload.DataSourceID, syncLog.ItemsCreated, syncLog.ItemsUpdated, syncLog.ItemsDeleted)

	return nil
}

// runScopedReindex executes a scoped reindex run (SP2-b §5.3): each external id
// in payload.Scope is refetched through the connector's TargetedFetcher
// implementation and ingested via the shared applyFetchedItem core (same
// delete-before-create and SubtreeKeep semantics as a whole-source sync).
// Connectors without the interface degrade per item with a "run a normal sync"
// hint instead of failing the run.
//
// Convergence is deliberately isolated from whole-source syncs (Ruling P-4):
// only this run's SyncLog is finalized — DataSource.LastSyncResult, Status,
// ErrorMessage and the persisted cursor are never touched, so a retry round
// can neither mask nor reset the last full sync's outcome. Per the status
// ruling, per-item failures live in the result (Failed count + error samples),
// never in the run's status or in ds state.
func (s *DataSourceService) runScopedReindex(
	ctx context.Context, connector datasource.Connector,
	ds *types.DataSource, syncLog *types.SyncLog,
	config *types.DataSourceConfig, payload *types.DataSourceSyncPayload,
) error {
	externalIDs := payload.Scope.ExternalIDs
	result := &types.SyncResult{Total: len(externalIDs)}

	// Tenant + auto-tag setup precedes ingestion, same as both whole-source
	// paths (KnowledgeService resolves tenant info from the context).
	ctx = context.WithValue(ctx, types.TenantIDContextKey, ds.TenantID)
	tenant, err := s.tenantRepo.GetTenantByID(ctx, ds.TenantID)
	if err != nil {
		logger.Errorf(ctx, "scoped reindex: failed to get tenant info: %v", err)
		s.finishScopedReindex(ctx, ds, syncLog, result,
			types.SyncLogStatusFailed, fmt.Sprintf("Failed to get tenant info: %v", err))
		return err
	}
	ctx = context.WithValue(ctx, types.TenantInfoContextKey, tenant)
	autoTagIDs := s.resolveAutoTagIDs(ctx, ds)

	fetcher, supported := connector.(datasource.TargetedFetcher)

	// Heartbeat/cancel pulse between items, same contract as the batch loop.
	var lastBeat time.Time
	for _, externalID := range externalIDs {
		if syncPulse(ctx, s.syncLogRepo, syncLog.ID, &lastBeat) {
			s.cancelSyncRun(ctx, syncLog, result)
			logger.Infof(ctx, "scoped reindex canceled by user: ds=%s processed=%d/%d item(s)",
				payload.DataSourceID, result.Created+result.Updated+result.Deleted+result.Skipped+result.Failed, len(externalIDs))
			return nil
		}

		if !supported {
			result.Failed++
			recordSyncError(result, types.SyncItemError{
				ExternalID: externalID,
				Code:       syncErrTargetedUnsupported,
				Message: fmt.Sprintf("connector %q does not support targeted reindex; "+
					"run a normal sync so the item is re-listed", ds.Type),
			})
			continue
		}

		item, err := fetcher.FetchByExternalID(ctx, config, externalID)
		if err != nil {
			recordScopedFetchFailure(ctx, result, externalID, err)
			continue
		}
		if item == nil {
			// Defensive: the contract says not-found must be an error, but a
			// nil item must never panic the run.
			recordScopedFetchFailure(ctx, result, externalID,
				fmt.Errorf("%w: connector returned no item for %q", datasource.ErrItemNotFound, externalID))
			continue
		}
		s.applyFetchedItem(withKBActivitySuppressed(ctx), ds, item, autoTagIDs, result)
	}

	s.finishScopedReindex(ctx, ds, syncLog, result, types.SyncLogStatusSuccess, "")
	logger.Infof(ctx, "scoped reindex completed: ds=%s total=%d created=%d updated=%d failed=%d",
		payload.DataSourceID, result.Total, result.Created, result.Updated, result.Failed)
	return nil
}

// recordScopedFetchFailure maps a FetchByExternalID error onto the scoped
// run's per-item failure sample. The Message keeps the raw cause text so
// downstream consumers (frontend, Task 2 of the retry loop) can distinguish
// transient fetch failures from permanent ones.
func recordScopedFetchFailure(ctx context.Context, result *types.SyncResult, externalID string, err error) {
	code := syncErrFetchFailed
	switch {
	case errors.Is(err, datasource.ErrItemNotFound):
		code = syncErrNotFound
	case errors.Is(err, ima.ErrTargetedRefetchUnsupported):
		// IMA's external id is a one-way hash; its error text carries the
		// actionable remedy, so surface it verbatim under the shared code.
		code = syncErrTargetedUnsupported
	default:
		logger.Warnf(ctx, "scoped reindex: refetch of %q failed: %v", externalID, err)
	}
	result.Failed++
	recordSyncError(result, types.SyncItemError{
		ExternalID: externalID,
		Code:       code,
		Message:    err.Error(),
	})
}

// finishScopedReindex is the scoped run's isolated convergence (Ruling P-4):
// it finalizes ONLY this run's SyncLog — never DataSource.LastSyncResult,
// Status, ErrorMessage or the cursor, i.e. none of updateSyncRunResult's
// ds-side effects — and writes the single manual_reindex audit entry with the
// per-item summary.
func (s *DataSourceService) finishScopedReindex(
	ctx context.Context, ds *types.DataSource, syncLog *types.SyncLog,
	result *types.SyncResult, status, errorMessage string,
) {
	resultJSON, _ := result.ToJSON()
	syncLog.ItemsTotal = result.Total
	syncLog.ItemsCreated = result.Created
	syncLog.ItemsUpdated = result.Updated
	syncLog.ItemsDeleted = result.Deleted
	syncLog.ItemsSkipped = result.Skipped
	syncLog.ItemsFailed = result.Failed
	syncLog.Status = status
	syncLog.ErrorMessage = errorMessage
	syncLog.Result = resultJSON
	syncLog.FinishedAt = timePtr(time.Now().UTC())
	if err := s.syncLogRepo.UpdateResult(ctx, syncLog); err != nil {
		logger.Errorf(ctx, "failed to update scoped reindex sync log: %v", err)
	}

	outcome := types.AuditOutcomeSuccess
	if status == types.SyncLogStatusFailed {
		outcome = types.AuditOutcomeFailed
	} else if result.Failed > 0 {
		outcome = types.AuditOutcomePartial
	}
	recordKBActivity(ctx, s.audit, ds.TenantID, ds.KnowledgeBaseID, types.AuditActionDataSourceSyncCompleted,
		"data_source", ds.ID, outcome,
		map[string]any{
			"name": ds.Name, "type": ds.Type,
			"trigger":     syncTriggerManualReindex,
			"sync_log_id": syncLog.ID,
			"total":       result.Total, "created": result.Created, "updated": result.Updated,
			"deleted": result.Deleted, "skipped": result.Skipped, "failed": result.Failed,
		})
}

// resolveAutoTagIDs finds or creates the per-data-source tag applied to every
// synced item so results are identifiable in the KB. A tag failure is
// non-fatal: the sync proceeds untagged.
func (s *DataSourceService) resolveAutoTagIDs(ctx context.Context, ds *types.DataSource) []string {
	autoTagIDs := []string{}
	if autoTag, tagErr := s.tagService.FindOrCreateTagByName(ctx, ds.KnowledgeBaseID, ds.Name); tagErr != nil {
		logger.Warnf(ctx, "failed to find/create auto-tag %q: %v (proceeding without tag)", ds.Name, tagErr)
	} else if autoTag != nil {
		autoTagIDs = append(autoTagIDs, autoTag.ID)
		logger.Infof(ctx, "using auto-tag %q (id=%s) for data source sync", ds.Name, autoTag.ID)
	}
	return autoTagIDs
}

// maxSyncResultErrors bounds the per-item error sample retained in
// SyncResult.Errors. That slice is persisted as jsonb and returned in every
// sync-log list response, so an unbounded list on a sync that fails thousands of
// documents means multi-MB DB rows and payloads. The accurate failure count
// lives in SyncResult.Failed (a bounded int); this list only keeps a sample for
// display (Tencent/WeKnora#2136 / #1262).
const maxSyncResultErrors = 100

// batchProgressInterval is how often the batch (non-streaming) sync loop
// persists its running counts to the sync log: often enough for live progress
// on large batches, rare enough not to matter as write load. Streaming
// connectors checkpoint at page boundaries instead.
const batchProgressInterval = 20

// recordSyncError appends an error sample to result.Errors, capped at
// maxSyncResultErrors. Callers still increment result.Failed for the exact count.
func recordSyncError(result *types.SyncResult, item types.SyncItemError) {
	if len(result.Errors) < maxSyncResultErrors {
		result.Errors = append(result.Errors, item)
	}
}

// fetchFailureSyncError maps a connector error item into a structured, user-
// facing sample. Connectors that classify their errors (Feishu) provide a stable
// i18n code + params via metadata so the frontend localises it to the viewer's
// language; the raw status/body/log_id never leaves the server logs. Connectors
// without codes keep the raw text as a Message fallback. Best practice per
// Airbyte/Fivetran/Onyx: humanised, actionable, localised UI; raw detail in logs.
func fetchFailureSyncError(item *types.FetchedItem, rawMsg string) types.SyncItemError {
	e := types.SyncItemError{
		Title:      item.Title,
		ExternalID: item.ExternalID,
	}
	if code := item.Metadata["error_reason_code"]; code != "" {
		e.Code = code
		if v := item.Metadata["error_reason_code_value"]; v != "" {
			e.Params = map[string]string{"code": v}
		}
		e.Message = item.Metadata["error_reason"] // fallback if the client lacks the key
	} else {
		e.Message = rawMsg
	}
	return e
}

// applyFetchedItem writes a single fetched item into the knowledge base and
// updates result counters. It is the shared core of the batch loop and the
// streaming handler so item classification (deleted / empty / ingest outcome)
// stays identical across both fetch paths.
func (s *DataSourceService) applyFetchedItem(
	ctx context.Context, ds *types.DataSource, item *types.FetchedItem,
	tagIDs []string, result *types.SyncResult,
) {
	if item.IsDeleted {
		if !ds.SyncDeletions {
			// Sync deletion disabled: neither count nor delete.
			return
		}
		if item.ExternalID == "" {
			logger.Warnf(ctx, "skipping deletion for item %q: empty external_id", item.Title)
			result.Skipped++
			return
		}
		// Perform real KB deletion, scoped to items owned by this data source
		// so identical external IDs from different data sources cannot collide.
		repo := s.knowledgeService.GetRepository()
		existing, lookupErr := repo.FindByDataSourceExternalID(
			ctx, ds.TenantID, ds.KnowledgeBaseID, ds.ID, item.ExternalID,
		)
		if lookupErr != nil {
			logger.Errorf(ctx, "failed to find deleted knowledge for external_id=%s (ds=%s, kb=%s): %v",
				item.ExternalID, ds.ID, ds.KnowledgeBaseID, lookupErr)
			result.Failed++
			result.DeletionFailed++
			recordSyncError(result, types.SyncItemError{
				Title:   item.Title,
				Code:    "deletion_lookup_failed",
				Message: "Failed to look up the item before deletion; see server logs",
			})
			return
		}
		if existing == nil {
			// Deletion is idempotent: the source item may already have been
			// removed manually or by an earlier sync.
			result.Skipped++
			return
		}
		if deleteErr := s.knowledgeService.DeleteKnowledge(ctx, existing.ID); deleteErr != nil {
			// The cursor is already past this item, so a failed deletion normally
			// retries only on a later full sync. Counted separately so the
			// sync-log message can warn the operator about this gap.
			result.Failed++
			result.DeletionFailed++
			logger.Errorf(ctx, "failed to delete knowledge %s for external_id=%s (ds=%s): %v",
				existing.ID, item.ExternalID, ds.ID, deleteErr)
			recordSyncError(result, types.SyncItemError{
				Title:   item.Title,
				Code:    "deletion_failed",
				Message: "Deletion failed; see server logs",
			})
			return
		}
		if herr := repo.HardDeleteKnowledge(ctx, ds.TenantID, existing.ID); herr != nil {
			result.Failed++
			result.DeletionFailed++
			logger.Errorf(ctx, "failed to hard-delete knowledge %s for external_id=%s (ds=%s): %v",
				existing.ID, item.ExternalID, ds.ID, herr)
			recordSyncError(result, types.SyncItemError{
				Title:   item.Title,
				Code:    "deletion_failed",
				Message: "Deletion failed; see server logs",
			})
			return
		}
		result.Deleted++
		return
	}

	if len(item.Content) == 0 && item.URL == "" {
		// Check if this is an error item from the connector (failed to fetch content)
		if errMsg, hasErr := item.Metadata["error"]; hasErr {
			logger.Warnf(ctx, "item %q (external_id=%s) fetch failed: %s", item.Title, item.ExternalID, errMsg)
			result.Failed++
			recordSyncError(result, fetchFailureSyncError(item, errMsg))
		} else {
			logger.Infof(ctx, "skipping item %q (external_id=%s): no content or URL", item.Title, item.ExternalID)
			result.Skipped++
		}
		return
	}

	isUpdate, err := s.ingestItem(ctx, ds, item, tagIDs)
	if err != nil {
		var dupErr *types.DuplicateKnowledgeError
		switch {
		case errors.As(err, &dupErr):
			// Duplicate file/URL is not a failure — count as skipped.
			logger.Infof(ctx, "item %q (external_id=%s) already exists, skipping", item.Title, item.ExternalID)
			result.Skipped++
		case item.Metadata["embedded_image"] == "true":
			// An image extracted from a document for OCR is a best-effort
			// enrichment, not the document itself. If the KB cannot ingest it
			// (VLM/object-storage not configured for images, or a transient error),
			// skip it rather than failing the whole sync: the doc body already
			// synced, and the image stays in SubtreeKeep for a later retry once the
			// KB is configured.
			logger.Infof(ctx, "skipping embedded image %q (external_id=%s), not ingested: %v",
				item.Title, item.ExternalID, err)
			result.Skipped++
		default:
			logger.Warnf(ctx, "failed to ingest item %q (external_id=%s): %v", item.Title, item.ExternalID, err)
			result.Failed++
			recordSyncError(result, types.SyncItemError{
				Title:      item.Title,
				ExternalID: item.ExternalID,
				Code:       "ingest_failed",
				Message:    "Ingest failed; see server logs",
			})
		}
	} else if isUpdate {
		result.Updated++
	} else {
		result.Created++
	}
}

// streamingFetch dispatches to FetchFullStream when a connector can re-fetch
// every item while keeping the stored cursor as the deletion baseline. Other
// streaming connectors keep FetchStream, including force-full runs that drop
// the cursor on the first attempt via streamStartCursor.
func streamingFetch(
	ctx context.Context,
	sc datasource.StreamingConnector,
	config *types.DataSourceConfig,
	forceFull bool,
	startCursor, fullBaseline *types.SyncCursor,
	h datasource.StreamHandler,
) (*types.SyncCursor, error) {
	if forceFull {
		if full, ok := sc.(datasource.FullStreamingConnector); ok {
			return full.FetchFullStream(ctx, config, fullBaseline, h)
		}
	}
	return sc.FetchStream(ctx, config, startCursor, h)
}

// streamStartCursor decides which cursor a streaming fetch should resume from.
// A user-triggered full sync on its first attempt drops the cursor so every
// item is re-fetched; a retried full sync (attempt > 0) and every incremental
// sync resume from the last persisted checkpoint so a timed-out run converges
// instead of restarting from scratch.
func streamStartCursor(ds *types.DataSource, forceFull bool, attempt int) (*types.SyncCursor, error) {
	if forceFull && attempt == 0 {
		return nil, nil
	}
	return ds.ParseSyncCursor()
}

// streamSyncHandler adapts a streaming fetch to the knowledge-base ingest path.
// Emit ingests each item as it arrives (bounding memory) and Checkpoint persists
// the connector cursor plus live progress counts at page boundaries.
type streamSyncHandler struct {
	svc     *DataSourceService
	ds      *types.DataSource
	tagIDs  []string
	result  *types.SyncResult
	syncLog *types.SyncLog
	// lastBeat is the caller-owned heartbeat/cancel-check throttle state; the
	// zero value means the first checkpoint pulses immediately. Connectors
	// like Confluence checkpoint per document, so the pulse must be throttled
	// or every document would cost an extra row write (SP2-a Task 2 condition).
	lastBeat time.Time
	// A07 lease fence of this worker; currentFence reports the live holder.
	workerFence  int64
	currentFence func() int64
	// authVersion is the binding version the run executes under (0 = legacy).
	authVersion int64
}

// Emit ingests one streamed item. A canceled context aborts the stream so the
// connector stops fetching; per-item ingest failures are recorded in result and
// do NOT abort (matching the batch loop, which never fails the whole sync for
// one bad document).
func (h *streamSyncHandler) Emit(ctx context.Context, item types.FetchedItem) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	h.result.Total++
	h.svc.applyFetchedItem(withKBActivitySuppressed(ctx), h.ds, &item, h.tagIDs, h.result)
	return nil
}

// Checkpoint persists the connector cursor onto the data source and mirrors the
// running counts into the sync log so progress survives a crash and the UI can
// reflect a long sync mid-flight instead of jumping from 0 to done.
func (h *streamSyncHandler) Checkpoint(ctx context.Context, cursor *types.SyncCursor) error {
	if cursor == nil {
		return nil
	}
	// A07: a checkpoint only advances after the emitted content and its
	// index-task handoff are durably persisted — Emit is synchronous, so by the
	// time the connector offers a checkpoint everything before it is committed —
	// and only while this worker still holds the lease fence. A stale worker
	// that lost its lease must not overwrite the current holder's cursor.
	if h.currentFence != nil && !appconnector.CanAdvanceCheckpoint(true, h.currentFence(), h.workerFence) {
		logger.Warnf(ctx, "checkpoint refused for ds=%s: worker fence %d is no longer current",
			h.ds.ID, h.workerFence)
		return nil
	}
	// The persisted cursor carries the binding auth version and lease fence so
	// a resume can tell which credential version and worker produced it
	// (consumed by cursorAuthVersionStale). stampSyncAuthVersion skips the
	// legacy version 0 without initializing the map, so the fence write below
	// still needs its own nil guard.
	stampSyncAuthVersion(cursor, h.authVersion)
	if cursor.ConnectorCursor == nil {
		cursor.ConnectorCursor = map[string]interface{}{}
	}
	cursor.ConnectorCursor["_sync_fence"] = h.workerFence
	cursorJSON, err := cursor.ToJSON()
	if err != nil {
		return err
	}
	h.ds.LastSyncCursor = cursorJSON
	if err := h.svc.dsRepo.UpdateSyncState(ctx, h.ds); err != nil {
		return err
	}

	// SP2-a §3.2/§3.3: one throttled pulse per window refreshes the heartbeat
	// (stall detection sees page-boundary liveness) and observes the
	// cooperative-cancel flag on the same cadence. Connectors that checkpoint
	// per document (Confluence) would otherwise hammer the row once per item.
	canceled := syncPulse(ctx, h.svc.syncLogRepo, h.syncLog.ID, &h.lastBeat)

	if canceled {
		// SP2-a §3.3: the terminal canceled state rides the progress write;
		// the cursor persisted above stays at this checkpoint so a later run
		// resumes right after the already-ingested documents.
		h.svc.cancelSyncRun(ctx, h.syncLog, h.result)
		logger.Infof(ctx, "sync canceled by user at checkpoint: ds=%s log=%s", h.ds.ID, h.syncLog.ID)
		return errSyncCanceled
	}

	// Best-effort live progress; a failure here must not abort the sync.
	h.syncLog.ItemsTotal = h.result.Total
	h.syncLog.ItemsCreated = h.result.Created
	h.syncLog.ItemsUpdated = h.result.Updated
	h.syncLog.ItemsDeleted = h.result.Deleted
	h.syncLog.ItemsSkipped = h.result.Skipped
	h.syncLog.ItemsFailed = h.result.Failed
	if err := h.svc.syncLogRepo.UpdateResult(ctx, h.syncLog); err != nil {
		logger.Warnf(ctx, "failed to persist sync log progress at checkpoint: %v", err)
	}
	return nil
}

// syncHeartbeatInterval is the minimum spacing between heartbeat writes on the
// batch path: a per-item write would hammer the sync_logs row once per
// document, while a 30s cadence stays far inside the stall window (task
// timeout + buffer) yet keeps liveness views current.
const syncHeartbeatInterval = 30 * time.Second

// maybeHeartbeat refreshes the sync-log heartbeat at most once per
// syncHeartbeatInterval, writing immediately on the first call. lastBeat is the
// caller-owned in-memory throttle state (zero value = never beaten); it is
// advanced on every out-of-window attempt, including failed ones, so a
// struggling database does not turn the batch loop into one write per item.
// Failures are logged and swallowed — a heartbeat is advisory liveness and must
// never abort a sync. The return value reports whether this call was a pulse
// point (throttle window elapsed); SP2-a §3.3's cooperative-cancel check rides
// exactly those points on both sync paths via syncPulse.
func maybeHeartbeat(ctx context.Context, syncLogRepo interfaces.SyncLogRepository, logID string, lastBeat *time.Time) bool {
	if lastBeat == nil {
		return false
	}
	now := time.Now().UTC()
	if !lastBeat.IsZero() && now.Sub(*lastBeat) < syncHeartbeatInterval {
		return false
	}
	*lastBeat = now
	if err := syncLogRepo.UpdateHeartbeat(ctx, logID, now); err != nil {
		logger.Warnf(ctx, "failed to persist sync heartbeat for log %s: %v", logID, err)
	}
	return true
}

// checkCancelRequested reports whether a cooperative cancel has been requested
// for the sync log. It is queried on the throttled pulse of maybeHeartbeat, so
// a flag raised mid-run lands within one window plus one checkpoint/batch
// boundary (~30s at the default interval). A missing row or a query failure
// reads as "not canceled": a broken read must never abort a healthy sync.
func checkCancelRequested(ctx context.Context, syncLogRepo interfaces.SyncLogRepository, logID string) bool {
	log, err := syncLogRepo.FindByID(ctx, logID)
	if err != nil || log == nil {
		return false
	}
	return log.CancelRequested
}

// syncPulse is the single throttled observation point shared by both sync
// paths: it refreshes the heartbeat (at most once per syncHeartbeatInterval)
// and, on the same pulse, checks the cooperative-cancel flag. It reports
// whether the run should stop because a user asked to cancel it.
func syncPulse(ctx context.Context, syncLogRepo interfaces.SyncLogRepository, logID string, lastBeat *time.Time) bool {
	if !maybeHeartbeat(ctx, syncLogRepo, logID, lastBeat) {
		return false
	}
	return checkCancelRequested(ctx, syncLogRepo, logID)
}

// processSyncStreaming runs a sync through a StreamingConnector, ingesting each
// item as it arrives and checkpointing progress so the run is memory-bounded and
// resumable after a timeout.
func (s *DataSourceService) processSyncStreaming(
	ctx context.Context, sc datasource.StreamingConnector,
	ds *types.DataSource, syncLog *types.SyncLog,
	config *types.DataSourceConfig, payload types.DataSourceSyncPayload, wasPaused bool,
) error {
	// Tenant + auto-tag setup must precede fetching because the stream ingests
	// each item on the fly.
	ctx = context.WithValue(ctx, types.TenantIDContextKey, ds.TenantID)
	tenant, err := s.tenantRepo.GetTenantByID(ctx, ds.TenantID)
	if err != nil {
		logger.Errorf(ctx, "failed to get tenant info: %v", err)
		s.updateSyncRunResult(ctx, ds, syncLog, &types.SyncResult{}, nil,
			types.SyncLogStatusFailed, fmt.Sprintf("Failed to get tenant info: %v", err), wasPaused)
		return err
	}
	ctx = context.WithValue(ctx, types.TenantInfoContextKey, tenant)

	autoTagIDs := s.resolveAutoTagIDs(ctx, ds)

	forceFull := payload.ForceFull || ds.SyncMode == types.SyncModeFull
	attempt, _ := asynq.GetRetryCount(ctx)
	startCursor, err := streamStartCursor(ds, forceFull, attempt)
	if err != nil {
		logger.Errorf(ctx, "failed to parse sync cursor: %v", err)
		s.updateSyncRunResult(ctx, ds, syncLog, &types.SyncResult{}, nil,
			types.SyncLogStatusFailed, fmt.Sprintf("Invalid cursor: %v", err), wasPaused)
		return err
	}

	result := &types.SyncResult{}
	// A07: acquire this run's exclusive lease fence; the checkpoint guard above
	// refuses advances from any earlier worker still draining a timed-out run.
	workerFence := s.TakeSyncFence(ds.ID)
	authVersion := s.currentSyncAuthVersion(ctx, ds)
	// SP2-b §6.3: a cursor produced under a different binding auth version
	// (credential rotation, including one performed by this run's refresh
	// trigger) must not be resumed across the token switch — drop it so the
	// stream walks everything from the beginning (the same ForceFull first-
	// attempt semantics). The deletion baseline below is deliberately kept:
	// reconciling deletions against the previous cursor is exactly what a full
	// reconciliation wants.
	if cursorAuthVersionStale(startCursor, authVersion) {
		logger.Infof(ctx, "auth version changed, full reconciliation: ds=%s log=%s cursor_version_dropped",
			payload.DataSourceID, syncLog.ID)
		startCursor = nil
	}
	handler := &streamSyncHandler{
		svc: s, ds: ds, tagIDs: autoTagIDs, result: result, syncLog: syncLog,
		workerFence:  workerFence,
		currentFence: func() int64 { return s.CurrentSyncFence(ds.ID) },
		authVersion:  authVersion,
	}

	// Full-stream connectors (Confluence) re-fetch every item on a force-full
	// run while retaining the stored cursor purely as the deletion baseline,
	// so Asynq retries continue instead of restarting.
	fullBaseline := startCursor
	if forceFull {
		if _, ok := sc.(datasource.FullStreamingConnector); ok {
			baseline, cursorErr := ds.ParseSyncCursor()
			if cursorErr != nil {
				logger.Errorf(ctx, "failed to parse full-sync cursor: %v", cursorErr)
				s.updateSyncRunResult(ctx, ds, syncLog, &types.SyncResult{}, nil,
					types.SyncLogStatusFailed, fmt.Sprintf("Invalid cursor: %v", cursorErr), wasPaused)
				return cursorErr
			}
			fullBaseline = baseline
		}
	}
	nextCursor, fetchErr := streamingFetch(ctx, sc, config, forceFull, startCursor, fullBaseline, handler)
	if errors.Is(fetchErr, errSyncCanceled) {
		// SP2-a §3.3: the checkpoint that observed the flag already persisted
		// the terminal canceled state and the resumed cursor. Report success so
		// asynq does not treat a user cancel as a failure and retry the run.
		logger.Infof(ctx, "streaming sync canceled by user: ds=%s log=%s created=%d updated=%d",
			payload.DataSourceID, syncLog.ID, result.Created, result.Updated)
		return nil
	}
	if fetchErr != nil {
		// Progress so far is already checkpointed onto ds.LastSyncCursor; leave
		// it in place so the Asynq retry resumes from there. Persist counts.
		logger.Errorf(ctx, "streaming fetch failed: %v", fetchErr)
		resultJSON, _ := result.ToJSON()
		s.updateSyncRunResult(ctx, ds, syncLog, result, resultJSON,
			types.SyncLogStatusFailed, fmt.Sprintf("Fetch failed: %v", fetchErr), wasPaused)
		return fetchErr
	}

	resultJSON, _ := result.ToJSON()
	if err := allFetchedItemsFailedError(result); err != nil {
		logger.Errorf(ctx, "streaming sync failed while processing fetched items: %v", err)
		s.updateSyncRunResult(ctx, ds, syncLog, result, resultJSON, types.SyncLogStatusFailed, err.Error(), wasPaused)
		return err
	}

	// Persist the final cursor for the next incremental sync. Connectors that
	// checkpoint mid-stream already carry the auth-version stamp; this also
	// covers a final cursor the connector returned without a final Checkpoint.
	if nextCursor != nil {
		stampSyncAuthVersion(nextCursor, authVersion)
		if cursorJSON, cerr := nextCursor.ToJSON(); cerr == nil {
			ds.LastSyncCursor = cursorJSON
		}
	}
	ds.LastSyncAt = timePtr(time.Now().UTC())

	// Surface per-document failures as a partial sync (not silent success), so
	// the sync-log drawer's failure detail explains which docs didn't make it —
	// the visibility gap behind "status normal but not everything syncs"
	// (Tencent/WeKnora#2136). Fetch failures abort the stream before the failed
	// page is checkpointed, so the next run retries them; deletion failures are
	// past the cursor and only retry on a full sync in the normal case (see
	// applyFetchedItem).
	status := types.SyncLogStatusSuccess
	errMsg := ""
	if result.Failed > 0 {
		status = types.SyncLogStatusPartial
		errMsg = fmt.Sprintf("%d document(s) failed to sync", result.Failed)
		if result.DeletionFailed > 0 {
			errMsg += fmt.Sprintf("; %d deletion failure(s) will only retry on the next full sync", result.DeletionFailed)
		}
	}
	s.updateSyncRunResult(ctx, ds, syncLog, result, resultJSON, status, errMsg, wasPaused)
	logger.Infof(ctx, "streaming sync completed: ds=%s created=%d updated=%d deleted=%d skipped=%d failed=%d",
		payload.DataSourceID, result.Created, result.Updated, result.Deleted, result.Skipped, result.Failed)
	return nil
}

// cancelSyncRun writes the cooperative-cancel terminal state (SP2-a §3.3):
// status=canceled, error_message="canceled by user", finished_at=now, with the
// live progress counts. The data source row is deliberately untouched — a user
// cancel is not a failure — and the cursor stays where the last checkpoint put
// it, so the next sync resumes instead of restarting.
func (s *DataSourceService) cancelSyncRun(ctx context.Context, syncLog *types.SyncLog, result *types.SyncResult) {
	syncLog.ItemsTotal = result.Total
	syncLog.ItemsCreated = result.Created
	syncLog.ItemsUpdated = result.Updated
	syncLog.ItemsDeleted = result.Deleted
	syncLog.ItemsSkipped = result.Skipped
	syncLog.ItemsFailed = result.Failed
	syncLog.Status = types.SyncLogStatusCanceled
	syncLog.ErrorMessage = syncCanceledMessage
	syncLog.FinishedAt = timePtr(time.Now().UTC())
	if err := s.syncLogRepo.UpdateResult(ctx, syncLog); err != nil {
		logger.Errorf(ctx, "failed to persist canceled sync log %s: %v", syncLog.ID, err)
	}
}

func (s *DataSourceService) updateSyncRunResult(
	ctx context.Context,
	ds *types.DataSource,
	syncLog *types.SyncLog,
	result *types.SyncResult,
	resultJSON types.JSON,
	status string,
	errorMessage string,
	wasPaused bool,
) {
	syncLog.ItemsTotal = result.Total
	syncLog.ItemsCreated = result.Created
	syncLog.ItemsUpdated = result.Updated
	syncLog.ItemsDeleted = result.Deleted
	syncLog.ItemsSkipped = result.Skipped
	syncLog.ItemsFailed = result.Failed
	syncLog.Status = status
	syncLog.FinishedAt = timePtr(time.Now().UTC())
	syncLog.ErrorMessage = errorMessage
	syncLog.Result = resultJSON
	if err := s.syncLogRepo.UpdateResult(ctx, syncLog); err != nil {
		logger.Errorf(ctx, "failed to update sync log: %v", err)
	}

	if status == types.SyncLogStatusFailed {
		if !wasPaused {
			ds.Status = types.DataSourceStatusError
		}
	} else if wasPaused {
		ds.Status = types.DataSourceStatusPaused
	} else {
		ds.Status = types.DataSourceStatusActive
	}
	ds.ErrorMessage = errorMessage
	ds.LastSyncResult = resultJSON
	if err := s.dsRepo.UpdateSyncState(ctx, ds); err != nil {
		logger.Errorf(ctx, "failed to update data source: %v", err)
	}
	action := types.AuditActionDataSourceSyncCompleted
	outcome := types.AuditOutcomeSuccess
	if status == types.SyncLogStatusFailed {
		action = types.AuditActionDataSourceSyncFailed
		outcome = types.AuditOutcomeFailed
	} else if status == types.SyncLogStatusPartial {
		outcome = types.AuditOutcomePartial
	}
	recordKBActivity(ctx, s.audit, ds.TenantID, ds.KnowledgeBaseID, action,
		"data_source", ds.ID, outcome,
		map[string]any{
			"name": ds.Name, "type": ds.Type, "sync_log_id": syncLog.ID,
			"total": result.Total, "created": result.Created, "updated": result.Updated,
			"deleted": result.Deleted, "skipped": result.Skipped, "failed": result.Failed,
		})
}

func allFetchedItemsFailedError(result *types.SyncResult) error {
	if result == nil || result.Total == 0 {
		return nil
	}
	if result.Failed != result.Total || result.Created != 0 || result.Updated != 0 ||
		result.Deleted != 0 || result.Skipped != 0 {
		return nil
	}

	detail := ""
	if len(result.Errors) > 0 {
		detail = result.Errors[0].Display()
		const maxDetailLen = 500
		if len(detail) > maxDetailLen {
			detail = detail[:maxDetailLen] + "..."
		}
	}
	if detail == "" {
		return fmt.Errorf("all fetched items failed during sync (%d/%d)", result.Failed, result.Total)
	}
	return fmt.Errorf("all fetched items failed during sync (%d/%d): %s", result.Failed, result.Total, detail)
}

// ValidateCredentials tests connectivity using raw credentials without persisting anything.
func (s *DataSourceService) ValidateCredentials(ctx context.Context, connectorType string, credentials map[string]interface{}) error {
	connector, err := s.connectorRegistry.Get(connectorType)
	if err != nil {
		return err
	}
	config := &types.DataSourceConfig{
		Type:        connectorType,
		Credentials: credentials,
	}
	if err := connector.Validate(ctx, config); err != nil {
		return err
	}

	return nil
}

// Helper functions

func (s *DataSourceService) validateDataSourceConfig(ctx context.Context, ds *types.DataSource) error {
	connector, err := s.connectorRegistry.Get(ds.Type)
	if err != nil {
		return err
	}

	config, err := ds.ParseConfig()
	if err != nil {
		return datasource.ErrInvalidConfig
	}

	return connector.Validate(ctx, config)
}

// ingestItem writes a single FetchedItem into the knowledge base.
// If a knowledge item with the same external_id already exists, it is deleted first (update = delete + re-create).
//
// Routing logic:
//   - Has Content bytes → CreateKnowledgeFromFile (走完整的文档解析 pipeline)
//   - Has URL only      → CreateKnowledgeFromURL  (让 WeKnora 下载并解析)
//
// Returns (isUpdate, error) — isUpdate is true when an existing item was replaced.
func (s *DataSourceService) ingestItem(ctx context.Context, ds *types.DataSource, item *types.FetchedItem, tagIDs []string) (bool, error) {
	// Channel decides the knowledge "source" label shown in the UI. Prefer the
	// connector-supplied metadata["channel"] (e.g. Feishu Drive sets it to
	// "feishu" so Drive docs share the wiki's "飞书" label instead of showing
	// "unknown" for the raw ds.Type "feishu_drive"). Fall back to ds.Type so
	// connectors that don't set metadata.channel still get a meaningful label.
	channel := ds.Type // e.g. "feishu", "notion"
	if item.Metadata != nil {
		if mc, ok := item.Metadata["channel"]; ok && mc != "" {
			channel = mc
		}
	}

	metadata := map[string]string{
		"external_id":        item.ExternalID,
		"source_resource_id": item.SourceResourceID,
		"datasource_id":      ds.ID,
	}
	// The source system's own last-modified time, when the connector supplied
	// one. The knowledge row's UpdatedAt moves on every re-parse, so this is
	// the only record of how old the document itself is.
	if !item.UpdatedAt.IsZero() {
		metadata["source_updated_at"] = item.UpdatedAt.UTC().Format(time.RFC3339)
	}
	if !item.CreatedAt.IsZero() {
		metadata["source_created_at"] = item.CreatedAt.UTC().Format(time.RFC3339)
	}
	for k, v := range item.Metadata {
		metadata[k] = v
	}

	// Check if a knowledge item with this external_id already exists → delete it first (update)
	isUpdate := false
	if item.ExternalID != "" {
		repo := s.knowledgeService.GetRepository()
		// Scope the lookup to items owned by this data source so identical
		// external IDs from two data sources cannot collide or overwrite each
		// other during updates.
		existing, err := repo.FindByDataSourceExternalID(ctx, ds.TenantID, ds.KnowledgeBaseID, ds.ID, item.ExternalID)
		if err != nil {
			logger.Warnf(ctx, "failed to check existing knowledge for external_id=%s: %v", item.ExternalID, err)
			// Non-fatal: proceed with creation (may produce duplicate)
		} else if existing != nil {
			logger.Infof(ctx, "found existing knowledge %s for external_id=%s, deleting for update", existing.ID, item.ExternalID)
			if err := s.knowledgeService.DeleteKnowledge(ctx, existing.ID); err != nil {
				logger.Warnf(ctx, "failed to delete existing knowledge %s: %v", existing.ID, err)
			} else {
				if herr := repo.HardDeleteKnowledge(ctx, ds.TenantID, existing.ID); herr != nil {
					logger.Warnf(ctx, "failed to hard-delete replaced knowledge %s: %v", existing.ID, herr)
				}
				isUpdate = true
			}
		}
	}

	// Case 1: content already fetched → build a FileHeader from bytes and call CreateKnowledgeFromFile
	if len(item.Content) > 0 {
		fh, err := bytesToFileHeader(item.Content, item.FileName)
		if err != nil {
			return isUpdate, fmt.Errorf("build file header: %w", err)
		}
		if _, err := s.knowledgeService.CreateKnowledgeFromFile(
			ctx,
			ds.KnowledgeBaseID,
			fh,
			metadata,
			nil,           // use KB default for multimodal
			item.FileName, // customFileName — must include extension for file-type validation
			tagIDs,        // auto-tag from data source
			channel,
			nil,
		); err != nil {
			var dupErr *types.DuplicateKnowledgeError
			if errors.As(err, &dupErr) && dupIsSameNode(dupErr, item) {
				// Identical content is already present in the KB under THIS node's
				// own external_id, so the parent effectively exists — reconcile the
				// subtree so children removed from the doc do not linger.
				s.sweepStaleSubtree(ctx, ds, item)
			}
			return isUpdate, err
		}
		s.sweepStaleSubtree(ctx, ds, item)
		return isUpdate, nil
	}

	// Case 2: only a remote URL — let WeKnora handle downloading and parsing
	if item.URL != "" {
		created, err := s.knowledgeService.CreateKnowledgeFromURL(
			ctx,
			ds.KnowledgeBaseID,
			item.URL,
			item.FileName,
			"",  // auto-detect file type
			nil, // use KB default for multimodal
			item.Title,
			tagIDs, // auto-tag from data source
			channel,
			nil,
		)
		if err != nil {
			var dupErr *types.DuplicateKnowledgeError
			if errors.As(err, &dupErr) && dupIsSameNode(dupErr, item) {
				// Identical content is already present in the KB under THIS node's
				// own external_id, so the parent effectively exists — reconcile the
				// subtree so children removed from the doc do not linger.
				s.sweepStaleSubtree(ctx, ds, item)
			}
			return isUpdate, err
		}
		// URL-created knowledge has no metadata, so a later deletion could
		// never find it. Attach the datasource keys on fresh creation only;
		// the duplicate path reuses an existing row that must not be re-tagged.
		if created != nil {
			metadataBytes, mErr := json.Marshal(metadata)
			if mErr != nil {
				return isUpdate, fmt.Errorf("marshal datasource metadata: %w", mErr)
			}
			created.Metadata = types.JSON(metadataBytes)
			if uErr := s.knowledgeService.GetRepository().UpdateKnowledge(ctx, created); uErr != nil {
				return isUpdate, fmt.Errorf("attach datasource metadata: %w", uErr)
			}
		}
		s.sweepStaleSubtree(ctx, ds, item)
		return isUpdate, nil
	}

	return isUpdate, fmt.Errorf("item has neither content nor URL")
}

// dupIsSameNode reports whether a duplicate-content error means the parent still
// exists in the KB *under this item's own external_id* — i.e. a content-dedup hit
// against this same node, so reconciling its subtree is safe. File deduplication
// keys on file_hash plus file_type (CheckKnowledgeExists), so an updated node whose rebuilt body
// happens to hash-collide with a DIFFERENT knowledge item (another node, or a
// manually-uploaded file with no external_id) would otherwise sweep this node's
// children even though its own parent row was just deleted for the update and
// never recreated — deleting those children with no parent to replace them. In
// that case the matched row's external_id differs (or is absent), so we skip the
// sweep and leave the children intact.
func dupIsSameNode(dupErr *types.DuplicateKnowledgeError, item *types.FetchedItem) bool {
	return dupErr != nil && dupErr.Knowledge != nil &&
		dupErr.Knowledge.GetMetadata()["external_id"] == item.ExternalID
}

// sweepStaleSubtree deletes STALE sub-items of item — knowledge whose external_id
// is prefixed with "<item.ExternalID>#" (e.g. attachment children of a docx node)
// that is NOT listed in item.SubtreeKeep, i.e. no longer present in the source.
//
// It runs only AFTER the parent item exists in the KB (freshly (re)created, or
// confirmed present via a duplicate-hash error), so a genuinely failed parent
// write never destroys existing children. Children still present in the source
// are preserved via SubtreeKeep even when they could not be re-ingested this
// cycle (e.g. a transient attachment download failure), so a still-present
// attachment never loses its previously-synced good copy. The "<id>#" prefix
// never matches the parent's own "<id>" external_id, so the parent is never
// self-swept.
func (s *DataSourceService) sweepStaleSubtree(ctx context.Context, ds *types.DataSource, item *types.FetchedItem) {
	if !item.ReplacesSubtree || item.ExternalID == "" {
		return
	}
	repo := s.knowledgeService.GetRepository()
	children, err := repo.FindByMetadataKeyPrefix(ctx, ds.TenantID, ds.KnowledgeBaseID, "external_id", types.SubtreeChildPrefix(item.ExternalID))
	if err != nil {
		logger.Warnf(ctx, "failed to list subtree of external_id=%s: %v", item.ExternalID, err)
		return
	}
	if len(children) == 0 {
		return
	}
	ids := make([]string, 0, len(children))
	for _, child := range children {
		// Scope to this data source so identical external_id prefixes from
		// another connector in the same KB cannot be swept.
		if child.GetMetadata()["datasource_id"] != ds.ID {
			continue
		}
		// A child still present in the source is preserved even if it could not be
		// re-ingested this sync; only children that vanished from the source are
		// stale and swept. Every child here was selected by the external_id-prefix
		// query, so its external_id is guaranteed present and readable (a malformed
		// row could not have matched the SQL predicate), and GetMetadata resolves
		// it identically to the keep-set entries the connector built. SubtreeKeep
		// holds one entry per still-present sub-item of this node (a small set), so
		// a linear scan is cheaper than materializing a lookup map.
		if slices.Contains(item.SubtreeKeep, child.GetMetadata()["external_id"]) {
			continue
		}
		ids = append(ids, child.ID)
	}
	if len(ids) == 0 {
		return
	}
	// Batch the deletion so a node whose attachment set shrank from N pays one
	// round of the delete fan-out rather than N sequential ones.
	if derr := s.knowledgeService.DeleteKnowledgeList(ctx, ids); derr != nil {
		logger.Warnf(ctx, "failed to delete %d stale sub-item(s) of external_id=%s: %v",
			len(ids), item.ExternalID, derr)
	} else if herr := repo.HardDeleteKnowledgeList(ctx, ds.TenantID, ids); herr != nil {
		logger.Warnf(ctx, "failed to hard-delete %d stale sub-item(s) of external_id=%s: %v",
			len(ids), item.ExternalID, herr)
	}
}

// bytesToFileHeader wraps a []byte into a *multipart.FileHeader so it can be
// consumed by KnowledgeService.CreateKnowledgeFromFile.
func bytesToFileHeader(data []byte, filename string) (*multipart.FileHeader, error) {
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)

	// Create a form file part
	partHeader := make(textproto.MIMEHeader)
	partHeader.Set("Content-Disposition", fmt.Sprintf(`form-data; name="file"; filename="%s"`, filename))
	partHeader.Set("Content-Type", "application/octet-stream")

	part, err := writer.CreatePart(partHeader)
	if err != nil {
		return nil, fmt.Errorf("create multipart part: %w", err)
	}

	if _, err := part.Write(data); err != nil {
		return nil, fmt.Errorf("write data to part: %w", err)
	}

	if err := writer.Close(); err != nil {
		return nil, fmt.Errorf("close multipart writer: %w", err)
	}

	// Parse the multipart data to get a FileHeader
	reader := multipart.NewReader(&buf, writer.Boundary())
	form, err := reader.ReadForm(int64(len(data)) + 1024)
	if err != nil {
		return nil, fmt.Errorf("read multipart form: %w", err)
	}

	files := form.File["file"]
	if len(files) == 0 {
		return nil, fmt.Errorf("no file in multipart form")
	}

	return files[0], nil
}

func timePtr(t time.Time) *time.Time {
	utc := t.UTC()
	return &utc
}
