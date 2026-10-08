package container

import (
	"context"
	"database/sql"
	"encoding/json"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	agentruntime "github.com/Tencent/WeKnora/internal/agent/runtime"
	"github.com/Tencent/WeKnora/internal/application/repository"
	"github.com/Tencent/WeKnora/internal/application/service"
	"github.com/Tencent/WeKnora/internal/craft"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
	"github.com/golang-migrate/migrate/v4"
	sqlite3migrate "github.com/golang-migrate/migrate/v4/database/sqlite3"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// Assembly-layer proof for the C02 registrar wiring (C06 review blocker 1):
// the dig cycle break must keep interaction.pending flowing through the
// REAL production registration chain. The inner executor reads
// runtime.emit INDIRECTLY (newCraftRuntimeExecutor), so after
// wireCraftInteractionRegistrar installs craftInteractionRegistrar around
// that emitter, one interaction.pending emission must land the durable
// craft_interactions row AND park the run at waiting_user — exactly what
// BASE's construction-time wrap did. The e2e seeds rows directly and
// cannot see this chain; this test drives it.

// wiringWorkspaceStore answers the workspace lookup for one bound scope.
type wiringWorkspaceStore struct {
	craft.Store
	ws craft.Workspace
}

func (s *wiringWorkspaceStore) GetWorkspace(context.Context, craft.Scope) (craft.Workspace, error) {
	return s.ws, nil
}

// wiringRunController records the waiting_user parks.
type wiringRunController struct {
	service.CraftRunController
	parks []agentruntime.Fence
}

func (c *wiringRunController) WaitForDecision(_ context.Context, fence agentruntime.Fence, _ string) error {
	c.parks = append(c.parks, fence)
	return nil
}

func wiringTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	_, filename, _, ok := runtime.Caller(0)
	require.True(t, ok)
	repoRoot := filepath.Clean(filepath.Join(filepath.Dir(filename), "../.."))
	dbPath := filepath.Join(t.TempDir(), "wiring.db")
	dsn := "file:" + dbPath + "?_foreign_keys=on&_busy_timeout=5000"
	sqlDB, err := sql.Open("sqlite3", dsn)
	require.NoError(t, err)
	driver, err := sqlite3migrate.WithInstance(sqlDB, &sqlite3migrate.Config{NoTxWrap: true})
	require.NoError(t, err)
	migrator, err := migrate.NewWithDatabaseInstance(
		"file:"+filepath.Join(repoRoot, "migrations/sqlite"), "sqlite3", driver)
	require.NoError(t, err)
	require.NoError(t, migrator.Up())
	_, _ = migrator.Close()
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	require.NoError(t, db.Exec(
		"INSERT INTO tenants (id, name, business) VALUES (1, 'tenant-1', 'test')").Error)
	require.NoError(t, db.Exec(
		"INSERT INTO sessions (id, tenant_id, title, user_id, engine_type) VALUES ('s-wiring', 1, 'wiring', 'u-wiring', 'trpc')").Error)
	return db
}

// wiringRuntime assembles the local runtime through the REAL constructor
// (env-driven, like the container) and returns it with its assembly.
func wiringRuntime(t *testing.T) (*localCraftRuntime, *CraftInteractionAssembly, *wiringRunController, agentruntime.Fence, craft.Task) {
	t.Helper()
	db := wiringTestDB(t)
	// Admissions are actor-fenced: AgentRunStore.Admit requires an active user
	// and tenant-membership row for the acting principal. Seed the durable
	// identity chain the production admission path reads.
	require.NoError(t, db.Exec("INSERT INTO users (id,username,email,password_hash,tenant_id) VALUES ('u-wiring','u-wiring','u-wiring@example.test','x',1)").Error)
	require.NoError(t, db.Exec("INSERT INTO tenant_members (tenant_id,user_id,role,status,joined_at,created_at,updated_at) VALUES (1,'u-wiring','owner','active',CURRENT_TIMESTAMP,CURRENT_TIMESTAMP,CURRENT_TIMESTAMP)").Error)

	runs := repository.NewAgentRunStore(db)
	user, err := json.Marshal(map[string]any{"role": "user", "content": "build"})
	require.NoError(t, err)
	assistant, err := json.Marshal(map[string]any{"role": "assistant", "content": ""})
	require.NoError(t, err)
	key := agentruntime.RunKey{TenantID: 1, RunID: "run-wiring-1"}
	_, err = runs.Admit(context.Background(), agentruntime.Admission{
		Key: key, SessionID: "s-wiring", UserID: "u-wiring", RequestID: "req-wiring",
		AssistantMessageID: "asst-wiring", RequestHash: "rh-wiring",
		Snapshot:    json.RawMessage(`{"version":1,"craft":true}`),
		UserMessage: user, AssistantMessage: assistant,
		Deadline: time.Now().Add(time.Hour),
	})
	require.NoError(t, err)
	fence, err := runs.Claim(context.Background(), key, "worker-wiring", time.Hour)
	require.NoError(t, err)

	scope := craft.Scope{TenantID: 1, UserID: "u-wiring", SessionID: "s-wiring"}
	store := &wiringWorkspaceStore{ws: craft.Workspace{
		ID: "ws-wiring", Scope: scope, SandboxID: "local-opencode-serve",
		Generation: "1", OpenCodeSessionID: "oc-wiring",
	}}
	t.Setenv(craftOpenCodeBaseURLEnv, "http://127.0.0.1:1") // no listener: the registrar's snapshot read fails fast and falls back
	t.Setenv(craftOpenCodeWorkDirEnv, t.TempDir())
	// The embedded-interface stubs satisfy the constructor's non-nil
	// checks without touching any backend: the emission path under test
	// never calls them.
	executor, err := newCraftRuntimeExecutor(
		db, store, runs,
		struct{ interfaces.FileService }{},
		struct{ craft.VersionStore }{},
		nil, nil, nil,
	)
	require.NoError(t, err)
	runtime, ok := executor.(*localCraftRuntime)
	require.True(t, ok)

	controller := &wiringRunController{}
	assembly := &CraftInteractionAssembly{
		Store: service.NewGormCraftInteractionStore(db),
		Runs:  controller,
	}
	task := craft.Task{
		ID: "dlg_wiring", ToolCallID: "call_wiring", Scope: scope,
		WorkspaceID: "ws-wiring", Fence: fence,
		Prompt: "build the report", RequestHash: "rh-wiring",
	}
	return runtime, assembly, controller, fence, task
}

func TestWireCraftInteractionRegistrarRegistersPendingInteractions(t *testing.T) {
	runtime, assembly, controller, fence, task := wiringRuntime(t)
	ctx := context.Background()
	payload, err := json.Marshal(map[string]string{"kind": "question", "part_id": "qst_wiring_1"})
	require.NoError(t, err)

	// Before the wiring: the plain emitter only appends the run event; no
	// interaction row exists (this is the regression the review caught —
	// the production registration must NOT depend on this state).
	require.NoError(t, runtime.emit(ctx, task, "interaction.pending", payload))
	var before int64
	require.NoError(t, runtime.db.Table("craft_interactions").Count(&before).Error)
	require.Zero(t, before, "plain emitter must not register interactions")

	// The container's boot-time wiring installs the registrar around the
	// SAME emitter the inner executor reads.
	wireCraftInteractionRegistrar(runtime, assembly)

	// One interaction.pending through the wired emitter: the durable row
	// lands through the REAL store, the run parks at waiting_user, and the
	// event still passes through to the durable run event stream.
	require.NoError(t, runtime.emit(ctx, task, "interaction.pending", payload))
	var after int64
	require.NoError(t, runtime.db.Table("craft_interactions").Count(&after).Error)
	require.Equal(t, int64(1), after, "interaction.pending must register the durable interaction")

	var row struct {
		Kind        string
		RunID       string
		TaskID      string
		OcRequestID string `gorm:"column:oc_request_id"`
	}
	require.NoError(t, runtime.db.Table("craft_interactions").Take(&row).Error)
	require.Equal(t, craft.InteractionQuestion, row.Kind)
	require.Equal(t, "run-wiring-1", row.RunID)
	require.Equal(t, task.ID, row.TaskID)
	require.Equal(t, "qst_wiring_1", row.OcRequestID)

	require.Len(t, controller.parks, 1, "the run must park at waiting_user")
	require.Equal(t, fence.RunID, controller.parks[0].RunID)

	// The event itself still reaches the run stream (registrar passthrough).
	var events int64
	require.NoError(t, runtime.db.Table("agent_run_events").
		Where("tenant_id = ? AND run_id = ?", fence.TenantID, fence.RunID).Count(&events).Error)
	require.Equal(t, int64(2), events, "both emissions must append durable run events")
}

func TestNewCraftInteractionAssemblyInjectsCurrentTaskAccess(t *testing.T) {
	db := wiringTestDB(t)
	ctx := context.Background()
	require.NoError(t, db.Exec("INSERT INTO craft_sessions (session_id, tenant_id, kind) VALUES ('s-wiring', 1, 'web')").Error)
	require.NoError(t, db.Exec(`INSERT INTO users (id,username,email,password_hash,tenant_id) VALUES
		('u-wiring','u-wiring','u-wiring@example.test','x',1),
		('collaborator','collaborator','collaborator@example.test','x',1)`).Error)
	require.NoError(t, db.Exec(`INSERT INTO tenant_members (tenant_id,user_id,role,status,joined_at,created_at,updated_at) VALUES
		(1,'u-wiring','owner','active',CURRENT_TIMESTAMP,CURRENT_TIMESTAMP,CURRENT_TIMESTAMP),
		(1,'collaborator','contributor','active',CURRENT_TIMESTAMP,CURRENT_TIMESTAMP,CURRENT_TIMESTAMP)`).Error)
	access := service.NewCraftAccessService(db)
	ownerScope := craft.Scope{TenantID: 1, UserID: "u-wiring", SessionID: "s-wiring"}
	require.NoError(t, access.Grant(ctx, ownerScope, "collaborator", craft.TaskRoleCollaborator))
	craftStore := repository.NewCraftStore(db)
	_, err := craftStore.PutWorkspace(ctx, craft.Workspace{
		Scope: ownerScope, SandboxID: "sandbox-wiring-access", Generation: "1",
		OpenCodeSessionID: "opencode-wiring-access", RuntimeDigest: "runtime-wiring-access",
	}, 0)
	require.NoError(t, err)

	runsRepo := repository.NewAgentRunStore(db)
	key := agentruntime.RunKey{TenantID: 1, RunID: "run-wiring-access"}
	_, err = runsRepo.Admit(ctx, agentruntime.Admission{
		Key: key, SessionID: ownerScope.SessionID, UserID: ownerScope.UserID, ActorUserID: "collaborator",
		RequestID: "request-wiring-access", AssistantMessageID: "assistant-wiring-access",
		RequestHash: "hash-wiring-access", Snapshot: json.RawMessage(`{"version":1,"craft_input_manifest":[]}`),
		UserMessage:      json.RawMessage(`{"role":"user","content":"build"}`),
		AssistantMessage: json.RawMessage(`{"role":"assistant","content":""}`),
		Deadline:         time.Now().Add(time.Hour),
	})
	require.NoError(t, err)

	t.Setenv(craftOpenCodeBaseURLEnv, "")
	assembly := newCraftInteractionAssembly(db, craftStore, &AgentRuntime{Runs: service.NewAgentRunService(runsRepo)})
	status, err := assembly.Control.Stop(types.WithCaller(ctx, types.Caller{TenantID: 1, UserID: "collaborator"}), service.CraftStopRequest{
		Scope: ownerScope, RunKey: key, TaskID: "missing-delegation",
	})
	require.NoError(t, err, "the production assembly must accept the initiating Collaborator's current TaskWrite")
	require.Equal(t, "stopping", status.Phase)
	_, err = repository.NewCraftStopIntentStore(db).GetStopIntent(ctx, ownerScope, key.RunID)
	require.NoError(t, err, "accepted stop must use the durable stop-intent store")
}
