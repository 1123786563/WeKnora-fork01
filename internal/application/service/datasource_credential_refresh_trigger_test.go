package service

import (
	"context"
	"encoding/json"
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/hibiken/asynq"

	appconnector "github.com/Tencent/WeKnora/internal/appconnector"
	apprepo "github.com/Tencent/WeKnora/internal/application/repository"
	"github.com/Tencent/WeKnora/internal/datasource"
	"github.com/Tencent/WeKnora/internal/datasource/connector/moauth"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// ──────────────────────────────────────────────────────────────────────
// SP2-b Task 6 credential refresh trigger (spec §6.3) — full-chain
// acceptance on the mock OAuth connector. Every credential below is a
// constructive fake ("mock-access-<n>" / repeated-letter AES keys): no real
// credential literal ever enters source or fixtures.
// ──────────────────────────────────────────────────────────────────────

const credentialTriggerTenantID uint64 = 21

// credentialTriggerBindingStore serves appconnector.SyncBindingStore from the
// same sqlite table the real repository writes, so currentSyncAuthVersion
// observes the auth-version bump end to end.
type credentialTriggerBindingStore struct {
	db     *gorm.DB
	tenant uint64
	dsID   string
}

func (s credentialTriggerBindingStore) FindSyncBinding(
	context.Context, uint64, string,
) (*appconnector.StoredSyncBinding, error) {
	var row apprepo.AppDataSourceBindingRow
	err := s.db.Table("app_datasource_bindings").
		Where("tenant_id = ? AND datasource_id = ?", s.tenant, s.dsID).
		Take(&row).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &appconnector.StoredSyncBinding{
		TenantID:       row.TenantID,
		DataSourceID:   row.DataSourceID,
		InstallationID: row.InstallationID,
		ConnectionID:   row.ConnectionID,
		AuthVersion:    row.AuthVersion,
	}, nil
}

// credentialTriggerSpaceState reports an active space connection on an active
// installation so AuthorizeSyncExecution does not pause the fixture's syncs.
type credentialTriggerSpaceState struct{}

func (credentialTriggerSpaceState) SpaceBindingState(
	context.Context, uint64, string,
) *appconnector.BindingState {
	return &appconnector.BindingState{
		ConnectionState:   appconnector.ConnectionActive,
		ConnectionKind:    appconnector.ConnectionKindSpace,
		InstallationState: appconnector.InstallationActive,
	}
}

// credentialTriggerFixture wires ProcessSync end to end against sqlite: the
// real DataSource repository (encrypted config round-trips, binding bump), a
// connector chosen by the test, and the A07 sync-execution hooks.
type credentialTriggerFixture struct {
	db      *gorm.DB
	dsRepo  interfaces.DataSourceRepository
	ds      *types.DataSource
	syncLog *types.SyncLog
	audit   *purgeAuditSink
	conn    datasource.Connector
	svc     *DataSourceService
	logRepo *processSyncSyncLogRepo
}

// newCredentialTriggerFixture assembles the service around one connector. The
// data source row is created with the given credentials through the real
// ToJSON encryption path.
func newCredentialTriggerFixture(
	t *testing.T, conn datasource.Connector, creds map[string]interface{},
) *credentialTriggerFixture {
	t.Helper()
	t.Setenv("SYSTEM_AES_KEY", strings.Repeat("c", 32))

	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "weknora.db")), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(
		&types.DataSource{}, &types.SyncLog{}, &apprepo.AppDataSourceBindingRow{},
	))
	// AppDataSourceBindingRow deliberately maps no timestamps, but the real
	// migration (000120) carries updated_at and the auth-version increment
	// writes it — add the column so sqlite matches production's shape.
	require.NoError(t, db.Exec(
		`ALTER TABLE app_datasource_bindings ADD COLUMN updated_at DATETIME`,
	).Error)

	dsRepo := apprepo.NewDataSourceRepository(db)
	cfg := &types.DataSourceConfig{Type: conn.Type(), Credentials: creds}
	blob, err := cfg.ToJSON()
	require.NoError(t, err)
	ds := &types.DataSource{
		ID:              "ds-cred-trigger",
		TenantID:        credentialTriggerTenantID,
		KnowledgeBaseID: "kb-cred-trigger",
		Name:            "Refresh Trigger Source",
		Type:            conn.Type(),
		Config:          blob,
		Status:          types.DataSourceStatusActive,
		SyncMode:        types.SyncModeIncremental,
		SyncDeletions:   true,
	}
	require.NoError(t, dsRepo.Create(context.Background(), ds))

	syncLog := &types.SyncLog{
		ID: "log-cred-trigger", DataSourceID: ds.ID, TenantID: ds.TenantID,
		Status: types.SyncLogStatusRunning, StartedAt: time.Now().UTC(),
	}
	logRepo := &processSyncSyncLogRepo{logs: map[string]*types.SyncLog{syncLog.ID: syncLog}}

	registry := datasource.NewConnectorRegistry()
	require.NoError(t, registry.Register(conn))

	f := &credentialTriggerFixture{
		db: db, dsRepo: dsRepo, ds: ds, syncLog: syncLog,
		audit: &purgeAuditSink{}, conn: conn, logRepo: logRepo,
	}
	svc := &DataSourceService{
		dsRepo:            dsRepo,
		syncLogRepo:       logRepo,
		kbService:         &processSyncKBService{kb: &types.KnowledgeBase{ID: ds.KnowledgeBaseID, TenantID: ds.TenantID}},
		connectorRegistry: registry,
		tenantRepo:        &processSyncTenantRepo{tenant: &types.Tenant{ID: ds.TenantID}},
		tagService:        &processSyncTagService{},
		audit:             f.audit,
	}
	svc.SetSyncExecution(
		credentialTriggerBindingStore{db: db, tenant: ds.TenantID, dsID: ds.ID},
		credentialTriggerSpaceState{}, nil,
	)
	f.svc = svc
	return f
}

// withBinding seeds an app_datasource_bindings row at the given auth version.
func (f *credentialTriggerFixture) withBinding(t *testing.T, authVersion int64) {
	t.Helper()
	require.NoError(t, f.db.Create(&apprepo.AppDataSourceBindingRow{
		ID: "bind-cred-trigger", TenantID: f.ds.TenantID, DataSourceID: f.ds.ID,
		InstallationID: "inst-cred-trigger", ConnectionID: "conn-cred-trigger",
		AuthVersion: authVersion,
	}).Error)
}

// bindingAuthVersion reads the live binding row (0 when absent).
func (f *credentialTriggerFixture) bindingAuthVersion(t *testing.T) int64 {
	t.Helper()
	var row apprepo.AppDataSourceBindingRow
	err := f.db.Table("app_datasource_bindings").
		Where("tenant_id = ? AND datasource_id = ?", f.ds.TenantID, f.ds.ID).
		Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return 0
	}
	require.NoError(t, err)
	return row.AuthVersion
}

// seedCursor stores a persisted cursor stamped with the given auth version; a
// version of 0 leaves the stamp absent (a pre-A07 cursor shape).
func (f *credentialTriggerFixture) seedCursor(t *testing.T, authVersion int64) {
	t.Helper()
	inner := map[string]interface{}{"marker": "previous-run"}
	if authVersion > 0 {
		inner[syncAuthVersionCursorKey] = float64(authVersion)
	}
	blob, err := json.Marshal(&types.SyncCursor{ConnectorCursor: inner})
	require.NoError(t, err)
	f.ds.LastSyncCursor = types.JSON(blob)
	require.NoError(t, f.dsRepo.Update(context.Background(), f.ds))
}

// storedConfig decrypts the persisted config row.
func (f *credentialTriggerFixture) storedConfig(t *testing.T) *types.DataSourceConfig {
	t.Helper()
	stored, err := f.dsRepo.FindByID(context.Background(), f.ds.ID)
	require.NoError(t, err)
	cfg, err := stored.ParseConfig()
	require.NoError(t, err)
	return cfg
}

// storedRow re-reads the raw data source row.
func (f *credentialTriggerFixture) storedRow(t *testing.T) *types.DataSource {
	t.Helper()
	stored, err := f.dsRepo.FindByID(context.Background(), f.ds.ID)
	require.NoError(t, err)
	return stored
}

// runSync dispatches one ProcessSync run over the fixture.
func (f *credentialTriggerFixture) runSync(t *testing.T) error {
	t.Helper()
	payload, err := json.Marshal(types.DataSourceSyncPayload{
		DataSourceID: f.ds.ID, TenantID: f.ds.TenantID, SyncLogID: f.syncLog.ID,
	})
	require.NoError(t, err)
	return f.svc.ProcessSync(context.Background(), asynq.NewTask(types.TypeDataSourceSync, payload))
}

func mockExpiringCreds(remaining time.Duration) map[string]interface{} {
	return map[string]interface{}{
		"access_token":  "mock-access-0",
		"refresh_token": "mock-refresh-0",
		"expires_at":    time.Now().UTC().Add(remaining).Format(time.RFC3339),
	}
}

// ── Full chain: 临期 → refresher → encrypted write-back → audit →
// binding AuthVersion bump → stale cursor dropped for full reconciliation.
func TestProcessSync_ExpiringCredentialsFullChain(t *testing.T) {
	conn := moauth.New(time.Hour, nil)
	f := newCredentialTriggerFixture(t, conn, mockExpiringCreds(3*time.Minute))
	f.withBinding(t, 1)
	f.seedCursor(t, 1)
	orgBlob := f.storedRow(t).Config

	require.NoError(t, f.runSync(t))

	// 1. Refresher was invoked exactly once at sync start.
	assert.Equal(t, 1, conn.RefreshCalls(), "the trigger must call RefreshCredentials once")

	// 2. Write-back persisted through the encrypted channel: ciphertext only
	//    in the row, and decrypting recovers the rotated token.
	stored := f.storedRow(t)
	assert.NotContains(t, string(stored.Config), "mock-access-1",
		"the rotated token must never land in the row as plaintext")
	assert.NotEqual(t, orgBlob, stored.Config, "the stored blob must have been rotated")
	cfg := f.storedConfig(t)
	assert.Equal(t, "mock-access-1", cfg.Credentials["access_token"],
		"the refreshed token must round-trip through decrypt")
	assert.Equal(t, "mock-refresh-0", cfg.Credentials["refresh_token"],
		"unrelated keys must survive the single-key rotation")
	newExpiry, ok := types.ParseCredentialTimestamp(cfg, types.CredentialKeyExpiresAt)
	require.True(t, ok)
	assert.True(t, newExpiry.After(time.Now().UTC().Add(50*time.Minute)),
		"expires_at must now carry the refreshed token's TTL")
	stamp, ok := types.ParseCredentialTimestamp(cfg, types.CredentialKeyLastRefreshedAt)
	require.True(t, ok, "the machine write-back stamps last_refreshed_at")
	assert.WithinDuration(t, time.Now().UTC(), stamp, time.Minute)

	// 3. Audit action for the machine rotation.
	entries := f.audit.findByAction(types.AuditActionDataSourceCredentialAutoRefreshed)
	assert.NotEmpty(t, entries, "the rotation must audit as credential_auto_refreshed")

	// 4. Binding auth version advanced exactly once.
	assert.Equal(t, int64(2), f.bindingAuthVersion(t),
		"a persisted rotation must bump ConnectionAuthVer by one")

	// 5. The stale cursor (stamped with the pre-rotation version) was dropped:
	//    the incremental fetch ran from scratch.
	assert.True(t, conn.LastFetchCursorWasNil(),
		"a cursor produced under the old auth version must be dropped for full reconciliation")

	// 6. This run executed on the new token.
	assert.Equal(t, "mock-access-1", conn.LastFetchAccessToken())

	// 7. The run itself completed (log success, no ds error flip).
	assert.Equal(t, types.SyncLogStatusSuccess, f.logRepo.logs[f.syncLog.ID].Status)
	assert.Equal(t, types.DataSourceStatusActive, f.storedRow(t).Status)
}

// Comfortably-valid credentials never trigger the refresher.
func TestProcessSync_FarFromExpirySkipsRefresh(t *testing.T) {
	conn := moauth.New(time.Hour, nil)
	f := newCredentialTriggerFixture(t, conn, mockExpiringCreds(time.Hour))
	f.withBinding(t, 1)

	require.NoError(t, f.runSync(t))
	assert.Zero(t, conn.RefreshCalls())
	assert.Equal(t, int64(1), f.bindingAuthVersion(t), "no rotation: no bump")
	assert.Empty(t, f.audit.findByAction(types.AuditActionDataSourceCredentialAutoRefreshed))
}

// Long-lived credentials (no expires_at) never trigger the refresher.
func TestProcessSync_LongLivedCredentialsSkipRefresh(t *testing.T) {
	conn := moauth.New(time.Hour, nil)
	f := newCredentialTriggerFixture(t, conn, map[string]interface{}{
		"access_token": "mock-access-0",
	})
	f.withBinding(t, 1)

	require.NoError(t, f.runSync(t))
	assert.Zero(t, conn.RefreshCalls())
}

// 临期 but the refresh fails: warn and finish the run on the OLD credentials
// (the next sync retries the rotation).
func TestProcessSync_NearExpiryRefreshFailureContinuesOnOldCredentials(t *testing.T) {
	conn := moauth.New(time.Hour, errors.New("mock refresh endpoint unreachable"))
	f := newCredentialTriggerFixture(t, conn, mockExpiringCreds(3*time.Minute))
	f.withBinding(t, 1)
	orgBlob := f.storedRow(t).Config

	require.NoError(t, f.runSync(t))

	assert.Equal(t, 1, conn.RefreshCalls())
	assert.Equal(t, "mock-access-0", conn.LastFetchAccessToken(),
		"the run must keep executing on the previous credentials")
	assert.Equal(t, orgBlob, f.storedRow(t).Config, "nothing was persisted")
	assert.Equal(t, int64(1), f.bindingAuthVersion(t), "no persisted rotation: no bump")
	assert.Empty(t, f.audit.findByAction(types.AuditActionDataSourceCredentialAutoRefreshed))
	assert.Equal(t, types.SyncLogStatusSuccess, f.logRepo.logs[f.syncLog.ID].Status)
}

// 过期 + refresh failure → semantic pause: the sync log fails with the pause
// message, ProcessSync returns nil (no asynq error noise), and the data source
// is never flipped into error state.
func TestProcessSync_ExpiredUnrefreshablePausesWithoutErrorFlip(t *testing.T) {
	conn := moauth.New(time.Hour, errors.New("mock refresh endpoint unreachable"))
	f := newCredentialTriggerFixture(t, conn, mockExpiringCreds(-time.Minute))
	f.withBinding(t, 1)
	orgBlob := f.storedRow(t).Config

	require.NoError(t, f.runSync(t), "a pause must not surface as an asynq failure")

	log := f.logRepo.logs[f.syncLog.ID]
	assert.Equal(t, types.SyncLogStatusFailed, log.Status)
	assert.Contains(t, log.ErrorMessage, "credential")
	require.NotNil(t, log.FinishedAt)

	// Not an error flip: status stays, cursor stays, config stays.
	stored := f.storedRow(t)
	assert.Equal(t, types.DataSourceStatusActive, stored.Status)
	assert.Equal(t, orgBlob, stored.Config)

	// The pause is audited with its machine-readable reason.
	failed := f.audit.findByAction(types.AuditActionDataSourceSyncFailed)
	require.Len(t, failed, 1)
	assert.Contains(t, string(failed[0].Details), appconnector.PauseReasonPermission)
}

// 过期 + connector without a refresher → same permission pause.
func TestProcessSync_ExpiredWithoutRefresherPauses(t *testing.T) {
	conn := &credentialRefreshProbeConnector{} // Task 5 fixture connector: no CredentialsRefresher
	// The probe connector type has no expiring-credential semantics of its own;
	// drive the pause purely through the stored expires_at.
	f := newCredentialTriggerFixture(t, conn, mockExpiringCreds(-time.Minute))

	require.NoError(t, f.runSync(t))
	log := f.logRepo.logs[f.syncLog.ID]
	assert.Equal(t, types.SyncLogStatusFailed, log.Status)
	assert.Contains(t, log.ErrorMessage, "reauthorize")
}

// 过期 + refresher succeeds → the sync continues on the rotated token (the
// pause only covers "expired and NOT refreshed").
func TestProcessSync_ExpiredButRefreshedContinues(t *testing.T) {
	conn := moauth.New(time.Hour, nil)
	f := newCredentialTriggerFixture(t, conn, mockExpiringCreds(-time.Minute))
	f.withBinding(t, 1)

	require.NoError(t, f.runSync(t))
	assert.Equal(t, 1, conn.RefreshCalls())
	assert.Equal(t, "mock-access-1", conn.LastFetchAccessToken())
	assert.Equal(t, int64(2), f.bindingAuthVersion(t))
	assert.Equal(t, types.SyncLogStatusSuccess, f.logRepo.logs[f.syncLog.ID].Status)
}

// Guard interplay (§8), two SYSTEM_AES_KEY states:
//
// (a) rotated away after the blob was encrypted: ParseConfig blanks every
//
//	encrypted value, so expires_at is unreadable and the trigger never even
//	consults the refresher — nothing is attempted, the surviving ciphertext
//	stays byte-identical, no audit noise, no auth-version bump. The trigger
//	cannot bypass the Task 5 anti-overwrite guard because it cannot see a
//	parseable expiry on an undecryptable blob.
func TestProcessSync_RotatedKeyLeavesEncryptedBlobUntouched(t *testing.T) {
	conn := moauth.New(time.Hour, nil)
	f := newCredentialTriggerFixture(t, conn, mockExpiringCreds(3*time.Minute))
	f.withBinding(t, 1)
	orgBlob := f.storedRow(t).Config

	// The operator rotates the key between write and sync.
	t.Setenv("SYSTEM_AES_KEY", strings.Repeat("d", 32))

	require.NoError(t, f.runSync(t))

	assert.Zero(t, conn.RefreshCalls(),
		"the encrypted expires_at is unreadable under the rotated key, so no rotation is attempted")
	assert.Equal(t, orgBlob, f.storedRow(t).Config,
		"the undecryptable blob must be preserved verbatim, never blanked")
	assert.Equal(t, int64(1), f.bindingAuthVersion(t), "nothing persisted: no bump")
	assert.Empty(t, f.audit.findByAction(types.AuditActionDataSourceCredentialAutoRefreshed))
}

// (b) missing at both write and refresh time: ToJSON does not encrypt without
//
//	a key (existing, pre-Task-6 behavior), the guard passes because nothing
//	failed to decrypt, and the rotation persists in plaintext — the
//	documented no-key behavior of the single encryption gate, unchanged.
func TestProcessSync_MissingKeyPersistsUnencrypted_ExistingBehavior(t *testing.T) {
	conn := moauth.New(time.Hour, nil)
	f := newCredentialTriggerFixture(t, conn, mockExpiringCreds(3*time.Minute))
	f.withBinding(t, 1)

	// Operator never configured a key: rewrite the stored blob in plaintext
	// through the same ToJSON gate the service uses.
	t.Setenv("SYSTEM_AES_KEY", "")
	plain, err := (&types.DataSourceConfig{Type: conn.Type(), Credentials: mockExpiringCreds(3 * time.Minute)}).ToJSON()
	require.NoError(t, err)
	f.ds.Config = plain
	require.NoError(t, f.dsRepo.Update(context.Background(), f.ds))

	require.NoError(t, f.runSync(t))

	assert.Equal(t, 1, conn.RefreshCalls(), "the plaintext expiry is readable, so the rotation runs")
	stored := f.storedRow(t).Config
	assert.Contains(t, string(stored), "mock-access-1",
		"without SYSTEM_AES_KEY ToJSON does not encrypt (existing behavior, asserted as a pin)")
	assert.Equal(t, int64(2), f.bindingAuthVersion(t))
	assert.NotEmpty(t, f.audit.findByAction(types.AuditActionDataSourceCredentialAutoRefreshed))
	assert.Equal(t, types.SyncLogStatusSuccess, f.logRepo.logs[f.syncLog.ID].Status)
}

// Unbound (legacy) data source: the rotation persists but there is no binding
// row to bump — the skip is silent, not an error.
func TestProcessSync_LegacyUnboundRotationSkipsBindingBump(t *testing.T) {
	conn := moauth.New(time.Hour, nil)
	f := newCredentialTriggerFixture(t, conn, mockExpiringCreds(3*time.Minute))
	f.seedCursor(t, 0)

	require.NoError(t, f.runSync(t))

	assert.Equal(t, 1, conn.RefreshCalls())
	assert.Equal(t, "mock-access-1", f.storedConfig(t).Credentials["access_token"])
	assert.Equal(t, int64(0), f.bindingAuthVersion(t), "no binding row: no bump, no error")
}

// The repository-level bump is scoped to (tenant, data source): a sibling
// binding is never advanced, and a missing row is a silent no-op.
func TestIncrementAppDataSourceBindingAuthVersion_Scoping(t *testing.T) {
	t.Setenv("SYSTEM_AES_KEY", strings.Repeat("c", 32))
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "weknora.db")), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&apprepo.AppDataSourceBindingRow{}))
	require.NoError(t, db.Exec(`ALTER TABLE app_datasource_bindings ADD COLUMN updated_at DATETIME`).Error)
	repo := apprepo.NewDataSourceRepository(db)
	ctx := context.Background()
	// The service reaches the bump through the same local type assertion
	// (dataSourceBindingAuthBumper); pin that the real repository satisfies it.
	bumper, ok := repo.(dataSourceBindingAuthBumper)
	require.True(t, ok, "the real repository must satisfy the bump surface the service asserts")

	require.NoError(t, db.Create(&apprepo.AppDataSourceBindingRow{
		ID: "bind-a", TenantID: 1, DataSourceID: "ds-a",
		InstallationID: "inst-1", ConnectionID: "conn-1", AuthVersion: 4,
	}).Error)
	require.NoError(t, db.Create(&apprepo.AppDataSourceBindingRow{
		ID: "bind-b", TenantID: 1, DataSourceID: "ds-b",
		InstallationID: "inst-1", ConnectionID: "conn-2", AuthVersion: 7,
	}).Error)

	// Missing row: silent no-op (legacy data source), not an error.
	require.NoError(t, bumper.IncrementAppDataSourceBindingAuthVersion(ctx, 1, "ds-missing"))

	require.NoError(t, bumper.IncrementAppDataSourceBindingAuthVersion(ctx, 1, "ds-a"))
	require.NoError(t, bumper.IncrementAppDataSourceBindingAuthVersion(ctx, 1, "ds-a"))

	read := func(dsID string) int64 {
		var row apprepo.AppDataSourceBindingRow
		require.NoError(t, db.Table("app_datasource_bindings").
			Where("tenant_id = ? AND datasource_id = ?", 1, dsID).Take(&row).Error)
		return row.AuthVersion
	}
	assert.Equal(t, int64(6), read("ds-a"), "two increments advance the row by exactly two")
	assert.Equal(t, int64(7), read("ds-b"), "sibling bindings stay untouched")
}

// Cursor validity matrix at the consumption point.
func TestCursorAuthVersionStale(t *testing.T) {
	stamped := func(v float64) *types.SyncCursor {
		return &types.SyncCursor{ConnectorCursor: map[string]interface{}{
			syncAuthVersionCursorKey: v,
		}}
	}
	unstamped := &types.SyncCursor{ConnectorCursor: map[string]interface{}{"marker": "pre-a07"}}

	cases := []struct {
		name    string
		cursor  *types.SyncCursor
		current int64
		want    bool
	}{
		{"nil cursor never stale", nil, 7, false},
		{"matching version", stamped(3), 3, false},
		{"newer binding version", stamped(3), 4, true},
		{"older binding version", stamped(3), 2, true},
		{"stamped cursor, binding gone", stamped(3), 0, true},
		{"unstamped cursor, legacy", unstamped, 0, false},
		{"unstamped cursor, now bound", unstamped, 1, true},
		{"unparseable stamp is conservative", &types.SyncCursor{
			ConnectorCursor: map[string]interface{}{syncAuthVersionCursorKey: "three"},
		}, 3, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, cursorAuthVersionStale(tc.cursor, tc.current))
		})
	}
}

// Matching auth version: the cursor is resumed, not dropped.
func TestProcessSync_MatchingAuthVersionResumesCursor(t *testing.T) {
	conn := moauth.New(time.Hour, nil)
	f := newCredentialTriggerFixture(t, conn, mockExpiringCreds(time.Hour))
	f.withBinding(t, 1)
	f.seedCursor(t, 1)

	require.NoError(t, f.runSync(t))
	assert.False(t, conn.LastFetchCursorWasNil(),
		"a cursor matching the current auth version must be resumed")
}

// Streaming consumption point: a stale cursor is dropped before FetchStream
// (ForceFull semantics), a current one is resumed.
func TestProcessSyncStreaming_StaleAuthVersionDropsCursor(t *testing.T) {
	run := func(t *testing.T, seedVersion, bindingVersion int64) bool {
		t.Helper()
		conn := &refreshTriggerStreamConnector{}
		f := newCredentialTriggerFixture(t, conn, map[string]interface{}{
			"access_token": "mock-access-0",
		})
		f.withBinding(t, bindingVersion)
		f.seedCursor(t, seedVersion)
		require.NoError(t, f.runSync(t))
		require.True(t, conn.fetchStreamCalled, "FetchStream must have run")
		return conn.receivedCursorWasNil
	}

	t.Run("stale seed drops cursor", func(t *testing.T) {
		assert.True(t, run(t, 1, 2))
	})
	t.Run("current seed resumes cursor", func(t *testing.T) {
		assert.False(t, run(t, 2, 2))
	})
}

// refreshTriggerStreamConnector is a minimal streaming connector that records
// the cursor FetchStream was started from.
type refreshTriggerStreamConnector struct {
	fetchStreamCalled    bool
	receivedCursorWasNil bool
}

func (c *refreshTriggerStreamConnector) Type() string { return "test-refresh-trigger-stream" }
func (c *refreshTriggerStreamConnector) Validate(context.Context, *types.DataSourceConfig) error {
	return nil
}
func (c *refreshTriggerStreamConnector) ListResources(
	context.Context, *types.DataSourceConfig, string,
) ([]types.Resource, error) {
	return nil, nil
}
func (c *refreshTriggerStreamConnector) ResolveResourceAncestors(
	context.Context, *types.DataSourceConfig, []string,
) ([]string, error) {
	return nil, nil
}
func (c *refreshTriggerStreamConnector) FetchAll(
	context.Context, *types.DataSourceConfig, []string,
) ([]types.FetchedItem, error) {
	return nil, nil
}
func (c *refreshTriggerStreamConnector) FetchIncremental(
	context.Context, *types.DataSourceConfig, *types.SyncCursor,
) ([]types.FetchedItem, *types.SyncCursor, error) {
	return nil, nil, nil
}
func (c *refreshTriggerStreamConnector) FetchStream(
	_ context.Context, _ *types.DataSourceConfig, cursor *types.SyncCursor, _ datasource.StreamHandler,
) (*types.SyncCursor, error) {
	c.fetchStreamCalled = true
	c.receivedCursorWasNil = cursor == nil
	return cursor, nil
}

var _ datasource.StreamingConnector = (*refreshTriggerStreamConnector)(nil)
