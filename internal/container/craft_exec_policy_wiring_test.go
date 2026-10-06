package container

// T20 (#139) T03 deferred-item assembly gate: the Docker command faces are
// CONSTRUCTED through the production assembly factory with the SAME T03
// delegate-execution policy (and the production audit sink) attached, and
// the gate journey is re-run through that construction path — a denied
// uploaded-code command answers the member-visible refusal BEFORE any
// provider/engine call or durable send hold, and the denial persists an
// audit row through the REAL audit log service. The restricted face fails
// closed the same way. The sandbox ROUTE that exposes these faces is the
// T19 S3 identity-propagation item and stays default-off; this test proves
// the construction path the route will consume.

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/Tencent/WeKnora/internal/application/repository"
	"github.com/Tencent/WeKnora/internal/application/service"
	"github.com/Tencent/WeKnora/internal/modules/craft"
	"github.com/Tencent/WeKnora/internal/sandbox"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/stretchr/testify/require"
	gormsqlite "gorm.io/driver/sqlite"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"

	sql "database/sql"

	migrate "github.com/golang-migrate/migrate/v4"
	sqlite3migrate "github.com/golang-migrate/migrate/v4/database/sqlite3"
)

// openCraftT20PolicyDB applies the full sqlite migration chain (the drill
// fixture pattern) so the durable identity, claim, output and audit tables
// all exist.
func openCraftT20PolicyDB(t *testing.T) *gorm.DB {
	t.Helper()
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("t20 policy: cannot locate test file")
	}
	repoRoot := filepath.Clean(filepath.Join(filepath.Dir(filename), "../.."))
	dbPath := filepath.Join(t.TempDir(), "craft-t20-policy.db")
	dsn := "file:" + dbPath + "?_foreign_keys=on&_busy_timeout=5000"
	sqlDB, err := sql.Open("sqlite3", dsn)
	if err != nil {
		t.Fatalf("t20 policy: open sqlite: %v", err)
	}
	driver, err := sqlite3migrate.WithInstance(sqlDB, &sqlite3migrate.Config{NoTxWrap: true})
	if err != nil {
		t.Fatalf("t20 policy: migrate driver: %v", err)
	}
	migrator, err := migrate.NewWithDatabaseInstance("file://"+filepath.Join(repoRoot, "migrations/sqlite"), "sqlite3", driver)
	if err != nil {
		t.Fatalf("t20 policy: migrator: %v", err)
	}
	if err := migrator.Up(); err != nil {
		t.Fatalf("t20 policy: migrations up: %v", err)
	}
	_, _ = migrator.Close()
	db, err := gorm.Open(gormsqlite.Open(dsn), &gorm.Config{Logger: gormlogger.Default.LogMode(gormlogger.Silent)})
	if err != nil {
		t.Fatalf("t20 policy: gorm open: %v", err)
	}
	return db
}

// t20ExecPolicyHandle is a docker-provider sandbox handle for the gate
// journey (the gate runs before any engine touch, so a synthetic handle is
// exactly right).
type t20ExecPolicyHandle struct{}

func (t20ExecPolicyHandle) ID() string                       { return "sandbox-t20-policy" }
func (t20ExecPolicyHandle) Provider() sandbox.RemoteProvider { return sandbox.SandboxTypeDocker }
func (t20ExecPolicyHandle) Metadata() map[string]string      { return nil }

// t20CountingNormalProvider counts every engine touch; a denied review must
// leave all counters zero.
type t20CountingNormalProvider struct{ created, started, observed int }

func (p *t20CountingNormalProvider) CreateAttachedExec(context.Context, sandbox.RemoteSandboxHandle, sandbox.DockerNormalExecRequest) (sandbox.DockerNormalExecReceipt, error) {
	p.created++
	return sandbox.DockerNormalExecReceipt{}, nil
}
func (p *t20CountingNormalProvider) StartAttachedExecOnce(context.Context, sandbox.DockerNormalExecReceipt, bool, []byte, sandbox.DockerNormalExecChunkSink) (sandbox.DockerNormalExecOutcome, error) {
	p.started++
	return sandbox.DockerNormalExecOutcome{}, nil
}
func (p *t20CountingNormalProvider) ObserveAttachedExec(context.Context, sandbox.DockerNormalExecReceipt, bool) (sandbox.DockerNormalExecObservation, error) {
	p.observed++
	return sandbox.DockerNormalExecObservation{}, nil
}

// t20CountingOutputlessClient counts the restricted engine touches.
type t20CountingOutputlessClient struct{ created, started, observed int }

func (c *t20CountingOutputlessClient) CreateOutputlessExec(context.Context, sandbox.RemoteSandboxHandle, sandbox.DockerOutputlessExecRequest) (sandbox.DockerOutputlessExecReceipt, error) {
	c.created++
	return sandbox.DockerOutputlessExecReceipt{}, nil
}
func (c *t20CountingOutputlessClient) StartOutputlessExec(context.Context, sandbox.DockerOutputlessExecReceipt, time.Duration) error {
	c.started++
	return nil
}
func (c *t20CountingOutputlessClient) ObserveOutputlessExec(context.Context, sandbox.DockerOutputlessExecReceipt, bool) (sandbox.DockerOutputlessExecObservation, error) {
	c.observed++
	return sandbox.DockerOutputlessExecObservation{}, nil
}

func TestCraftT20ExecPolicyAssemblyGateJourney(t *testing.T) {
	db := openCraftT20PolicyDB(t)
	const tenant = uint64(9501)
	const runID = "run-t20-policy"
	const sessionID = "sess-t20-policy"

	// Minimal durable identity: the session, its run and the uploaded python
	// input inside the run's admitted manifest (the exact shape the policy
	// re-loads by identity).
	now := time.Now().UTC()
	require.NoError(t, db.Exec("INSERT INTO sessions (id, tenant_id, title, user_id, engine_type) VALUES (?, ?, 't20 policy', 'owner', 'trpc')", sessionID, tenant).Error)
	require.NoError(t, db.Exec(`INSERT INTO agent_runs
		(tenant_id, run_id, session_id, owner_id, request_id, assistant_message_id, request_hash, snapshot, status, wait_reason, deadline, created_at, updated_at)
		VALUES (?, ?, ?, 'owner', 'req-t20-policy', 'am-t20-policy', 'rh', '{}', 'running', '', ?, ?, ?)`,
		tenant, runID, sessionID, now.Add(time.Hour), now, now).Error)
	content := []byte("print('data')\n")
	sum := sha256.Sum256(content)
	digest := hex.EncodeToString(sum[:])
	uploaded := craft.Input{Ref: "resource://t20-input-1", Name: "analyze.py", SHA256: digest, Bytes: int64(len(content)), CitationID: digest}
	snapshot, err := json.Marshal(map[string]any{
		"version": 1, "query": "build", "model_id": "model-1",
		"agent_config":         json.RawMessage(`{}`),
		"craft_input_manifest": []craft.Input{uploaded},
		"craft_workspace_seed": service.CraftWorkspaceSeedSnapshot{WorkspaceID: "ws-t20-policy", State: craft.DraftHeadEmpty},
	})
	require.NoError(t, err)
	require.NoError(t, db.Exec("UPDATE agent_runs SET snapshot = ? WHERE tenant_id = ? AND run_id = ?", string(snapshot), tenant, runID).Error)

	// The PRODUCTION audit sink (the same service the container provides).
	auditRepo := repository.NewAuditLogRepository(db)
	audit := service.NewAuditLogService(auditRepo)

	normalProvider := &t20CountingNormalProvider{}
	outputless := &t20CountingOutputlessClient{}
	budget, err := service.NewCraftBudgetService(db, nil, service.CraftBudgetPolicy{
		GrantWindow: time.Hour, MaxCalls: 5, CallUpper: 1, TaskLimit: 10,
	})
	require.NoError(t, err)

	// The production construction path under test.
	faces, err := newCraftDockerExecCommandFaces(db, repository.NewCraftStore(db), budget, audit, normalProvider, outputless, time.Second)
	require.NoError(t, err)
	require.NotNil(t, faces.Normal)
	require.NotNil(t, faces.Restricted)

	ctx := context.Background()

	// Normal face: executing the UPLOADED input is denied with the
	// member-visible refusal before any engine touch.
	_, err = faces.Normal.Execute(ctx, "grant-t20", "activity-t20-normal",
		service.CraftCallBinding{ModelID: "lead", Funding: "platform"}, t20ExecPolicyHandle{},
		repository.CraftDockerNormalInputRequest{
			TenantID: tenant, RunID: runID, TaskID: sessionID, WorkingDir: "/workspace",
			Command: []string{"python3", "/workspace/inputs/analyze.py"},
		})
	require.ErrorIs(t, err, craft.ErrForbidden)
	require.Contains(t, err.Error(), "Allowed alternative", "the refusal must stay member-visible")
	require.Zero(t, normalProvider.created, "a denied review must never reach the engine")
	require.Zero(t, normalProvider.started)
	require.Zero(t, normalProvider.observed)

	// A generated Workspace script passes the SAME constructed gate.
	require.NoError(t, faces.Policy.ReviewNormalExec(ctx, repository.CraftDockerNormalInputRequest{
		TenantID: tenant, RunID: runID, TaskID: sessionID, WorkingDir: "/workspace",
		Command: []string{"/workspace/rv-t20/output/build.sh"},
	}))

	// The denial persisted an audit row through the REAL audit service.
	var denials int64
	require.NoError(t, db.Table("audit_logs").
		Where("tenant_id = ? AND scope_type = ? AND scope_id = ? AND outcome = ?",
			tenant, "craft_run", runID, types.AuditOutcomeDenied).Count(&denials).Error)
	require.Positive(t, denials, "the production audit sink must persist the gate denial")

	// Restricted face: with no durable Run identity the constructed gate
	// refuses BEFORE any durable send hold or engine touch.
	_, err = faces.Restricted.Start(ctx, "grant-t20", "activity-t20-restricted",
		service.CraftCallBinding{}, t20ExecPolicyHandle{},
		service.CraftDockerOutputlessRequest{DiscardOutput: true, Exec: sandbox.RemoteExecRequest{Command: "true"}})
	require.ErrorIs(t, err, craft.ErrForbidden)
	require.Contains(t, err.Error(), "no durable Run identity")
	require.Zero(t, outputless.created, "the denied restricted review must never reach the engine")
	var holds int64
	require.NoError(t, db.Table("craft_charge_start_journal").
		Where("activity_key = ?", "activity-t20-restricted").Count(&holds).Error)
	require.Zero(t, holds, "the denied review must precede the durable send hold")
}
