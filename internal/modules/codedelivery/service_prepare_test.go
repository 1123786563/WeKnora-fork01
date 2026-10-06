package codedelivery

import (
	"context"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	agentruntime "github.com/Tencent/WeKnora/internal/agent/runtime"
	appconnector "github.com/Tencent/WeKnora/internal/modules/appconnector"
	appconnectorrepo "github.com/Tencent/WeKnora/internal/modules/appconnector/repository/appconnector"
	appconnectorsvc "github.com/Tencent/WeKnora/internal/modules/appconnector/service/appconnector"
	deliveryrepo "github.com/Tencent/WeKnora/internal/modules/codedelivery/repository/codedelivery"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// --- 夹具：真实 sqlite（AutoMigrate，全量迁移轨道被预存在 000112 撞号
// 破坏——见计划差异记录 5）+ 真实 A03 行为 + 真实 ocAuthorizer ---

type fixtureRun struct {
	sessionID string
}

func (f fixtureRun) GetOwnedRun(ctx context.Context, tenantID uint64, ownerID, runID string) (agentruntime.Run, error) {
	if runID != "run-1" || ownerID != "u1" || tenantID != 7 {
		return agentruntime.Run{}, errors.New("run_not_found")
	}
	return agentruntime.Run{Key: agentruntime.RunKey{TenantID: tenantID, RunID: runID}, SessionID: f.sessionID, Owner: ownerID}, nil
}

// fixtureConnections 同时实现 ConnectionReader 与 CredentialResolver（与生产
// MCPOAuthBindingStore 的双角色一致）；指针接收者使 membersDrop/breakCreds 生效。
type fixtureConnections struct {
	db          *gorm.DB
	members     map[string]bool
	credsBroken bool // 模拟凭据行被删后 Resolve 失败（前置门拒绝注入点）
}

func (f *fixtureConnections) FindConnectionByID(ctx context.Context, id string) (appconnector.Connection, error) {
	var row appconnectorrepo.ConnectionRow
	if err := f.db.WithContext(ctx).Where("id = ?", id).First(&row).Error; err != nil {
		return appconnector.Connection{}, err
	}
	return appconnector.Connection{
		ID: row.ID, InstallationID: row.InstallationID, Kind: row.Kind,
		OwnerID: row.OwnerID, CredentialRef: row.CredentialRef,
		State: row.State, TenantID: row.TenantID, AuthVersion: row.AuthVersion,
	}, nil
}

func (f *fixtureConnections) LoadCredential(ctx context.Context, c appconnector.Connection) ([]byte, error) {
	if strings.HasSuffix(c.CredentialRef, ":gitlab") {
		return []byte("glpat-testtoken"), nil
	}
	return []byte("gho_testtoken"), nil
}

func (f *fixtureConnections) MemberActive(ctx context.Context, tenantID uint64, userID string) (bool, error) {
	return f.members[userID], nil
}

func (f *fixtureConnections) TryAcquireRefreshLease(ctx context.Context, c appconnector.Connection, leaseID string, until time.Time) (bool, error) {
	return true, nil
}

func (f *fixtureConnections) Resolve(ctx context.Context, connectionID string, expectedVersion int64) ([]byte, error) {
	if f.credsBroken {
		return nil, errors.New("credential row deleted")
	}
	if strings.HasSuffix(connectionID, "-gl") {
		return []byte("glpat-testtoken"), nil
	}
	return []byte("gho_testtoken"), nil
}

func (f *fixtureConnections) drop(userID string) { delete(f.members, userID) }

type deliveryFixture struct {
	db          *gorm.DB
	store       *deliveryrepo.DeliveryStore
	actions     *appconnectorsvc.ActionService
	svc         *CodeDeliveryService
	github      *githubEmulator
	gitlab      *gitLabEmulator
	workspace   WorkspaceFileSource
	root        string
	connections *fixtureConnections
	dispatcher  *DeliveryDispatcher
}

// newDeliveryFixture 装配真实 sqlite（AutoMigrate）+ 真实 A03 ActionStore +
// 真实 ocAuthorizer + httptest GitHub 模拟器 + 本地目录工作区。
// Task 6 起为 dispatcher 内部单实例构造：dispatcher 与 service 共享同一
// db/emulator/connections/workspace/store 实例，绝无两套底层件。
func newDeliveryFixture(t *testing.T, mutate func(root string)) *deliveryFixture {
	t.Helper()
	dsn := "file:" + filepath.Join(t.TempDir(), "svc.db") + "?_foreign_keys=on&_busy_timeout=5000"
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(
		&appconnectorrepo.ActionRow{}, &appconnectorrepo.ApprovalRow{}, &appconnectorrepo.PreAuthorizationRow{},
		&appconnectorrepo.ConnectionRow{}, &appconnectorrepo.InstallationRow{}, &deliveryrepo.DeliveryRow{},
	))
	// 个人 GitHub 连接：inst-gh / conn-gh，owner=u1，active。
	require.NoError(t, db.Create(&appconnectorrepo.InstallationRow{ID: "inst-gh", AppID: "github", AppVersion: "1", State: appconnector.InstallationActive, TenantID: 7}).Error)
	require.NoError(t, db.Create(&appconnectorrepo.ConnectionRow{
		ID: "conn-gh", InstallationID: "inst-gh", Kind: appconnector.ConnectionKindPersonal,
		OwnerID: "u1", CredentialRef: "mcp:conn-gh:github", State: appconnector.ConnectionActive,
		TenantID: 7, AuthVersion: 1,
	}).Error)
	// 个人 GitLab 连接：inst-gl / conn-gl，owner=u1，active（T24 #54）。
	require.NoError(t, db.Create(&appconnectorrepo.InstallationRow{ID: "inst-gl", AppID: "gitlab", AppVersion: "1", State: appconnector.InstallationActive, TenantID: 7}).Error)
	require.NoError(t, db.Create(&appconnectorrepo.ConnectionRow{
		ID: "conn-gl", InstallationID: "inst-gl", Kind: appconnector.ConnectionKindPersonal,
		OwnerID: "u1", CredentialRef: "mcp:conn-gl:gitlab", State: appconnector.ConnectionActive,
		TenantID: 7, AuthVersion: 1,
	}).Error)

	e := newGitHubEmulator(t)
	gl := newGitLabEmulator(t)
	gitlabFactory := NewGitLabClientFactory(http.DefaultClient, gl.srv.URL)
	providers := appconnectorrepo.NewInstallationStore(db)
	root := t.TempDir()
	if mutate != nil {
		mutate(root)
	}
	workspace, err := NewLocalWorkspaceSource(root)
	require.NoError(t, err)

	actionStore := appconnectorrepo.NewActionStore(db)
	connections := &fixtureConnections{db: db, members: map[string]bool{"u1": true, "u2": true}}
	guard := appconnectorsvc.NewSubjectGuard(connections) // 单参 permission-only guard：个人连接 owner-only + 成员资格
	factory := NewGitHubClientFactory(http.DefaultClient, e.srv.URL)
	store := deliveryrepo.NewDeliveryStore(db)
	dispatcher := NewDeliveryDispatcher(DispatcherDeps{
		Connections: connections, Creds: connections, Guard: guard,
		GitHub: factory, GitLab: gitlabFactory, Workspace: workspace, Store: store,
		ActionRows: actionStore, Runs: fixtureRun{sessionID: "s-1"},
	})
	actions := appconnectorsvc.NewActionService(actionStore, guard, nil, dispatcher, dispatcher)
	svc := NewCodeDeliveryService(CodeDeliveryDeps{
		Store: store, Actions: actions, ActionRows: actionStore,
		Connections: connections, Creds: connections,
		GitHub: factory, GitLab: gitlabFactory, Providers: providers,
		Workspace: workspace, Runs: fixtureRun{sessionID: "s-1"},
		Dispatcher: dispatcher,
	})
	return &deliveryFixture{db: db, store: store, actions: actions, svc: svc, github: e, gitlab: gl, workspace: workspace, root: root, connections: connections, dispatcher: dispatcher}
}

func membersDrop(f *deliveryFixture, userID string) { f.connections.drop(userID) }

func breakCreds(f *deliveryFixture) { f.connections.credsBroken = true }

func baselineInput() BaselineInput {
	return BaselineInput{
		TenantID: 7, CallerID: "u1", RunID: "run-1", ConnectionID: "conn-gh",
		Repo:        RepoRef{Owner: "octocat", Name: "hello"},
		BaselineSHA: "b" + strings.Repeat("0", 39),
	}
}

func prepareInput() PrepareInput {
	return PrepareInput{
		TenantID: 7, CallerID: "u1", RunID: "run-1", ConnectionID: "conn-gh",
		Repo:          RepoRef{Owner: "octocat", Name: "hello"},
		BaselineSHA:   "b" + strings.Repeat("0", 39),
		CommitMessage: "fix: greeting", PRTitle: "WeKnora task s-1",
	}
}

func TestMaterializeBaselineWritesFixedTreeIntoWorkspace(t *testing.T) {
	f := newDeliveryFixture(t, nil)
	ctx := context.Background()

	receipt, err := f.svc.MaterializeBaseline(ctx, baselineInput())
	require.NoError(t, err)
	require.Equal(t, 2, receipt.Files)
	require.Equal(t, "/workspace/octocat/hello", receipt.Root)
	raw, err := os.ReadFile(filepath.Join(f.root, "octocat/hello/main.go"))
	require.NoError(t, err)
	require.Equal(t, "package main\n", string(raw))
}

func TestMaterializeBaselineRejectsNonOwnerAndForeignConnection(t *testing.T) {
	f := newDeliveryFixture(t, nil)
	ctx := context.Background()

	// 非本人 run（u2 调用 owner=u1 的 run）：run 归属谓词拒绝。
	in := baselineInput()
	in.CallerID = "u2"
	_, err := f.svc.MaterializeBaseline(ctx, in)
	require.Error(t, err) // fixtureRun 对非 owner 返回 run_not_found

	// 连接不是个人连接或调用者非 owner：ErrConnectionNotUsable。
	require.NoError(t, f.db.Create(&appconnectorrepo.ConnectionRow{
		ID: "conn-other", InstallationID: "inst-gh", Kind: appconnector.ConnectionKindPersonal,
		OwnerID: "u2", CredentialRef: "mcp:conn-other:github", State: appconnector.ConnectionActive, TenantID: 7, AuthVersion: 1,
	}).Error)
	in = baselineInput()
	in.ConnectionID = "conn-other"
	_, err = f.svc.MaterializeBaseline(ctx, in)
	require.ErrorIs(t, err, ErrConnectionNotUsable)
}

func TestPrepareDeliveryAnchorsApprovalAndDiff(t *testing.T) {
	// 工作区已在基线之上修改 main.go 并新增 util.go（经物化后覆写）。
	f := newDeliveryFixture(t, func(root string) {
		dir := filepath.Join(root, "octocat/hello")
		require.NoError(t, os.MkdirAll(dir, 0o755))
		require.NoError(t, os.WriteFile(filepath.Join(dir, "README.md"), []byte("# hello\n"), 0o644))
		require.NoError(t, os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main\n\nfunc main() {}\n"), 0o644))
		require.NoError(t, os.WriteFile(filepath.Join(dir, "util.go"), []byte("package main\n\nfunc util() {}\n"), 0o644))
	})
	ctx := context.Background()

	view, err := f.svc.PrepareDelivery(ctx, prepareInput())
	require.NoError(t, err)
	require.Equal(t, string(DeliveryPrepared), view.State)
	require.Equal(t, "weknora/task/s-1", view.Branch) // 默认分支=TaskBranchOf(sessionID)
	require.Equal(t, 2, view.Files)                   // main.go 修改 + util.go 新增
	require.NotEmpty(t, view.ActionID)
	require.NotEmpty(t, view.Digest)
	require.Equal(t, "awaiting_approval", view.ActionState)

	// A03 行真实落库：risk=deliver、target=github.deliver、args=归一化材料。
	var row appconnectorrepo.ActionRow
	require.NoError(t, f.db.Where("id = ?", view.ActionID).First(&row).Error)
	require.Equal(t, "deliver", row.Risk)
	require.Equal(t, "github.deliver", row.Target)
	mat, err := ParseDeliveryMaterial([]byte(row.ArgsSnapshot))
	require.NoError(t, err)
	require.Len(t, mat.Files, 2)
	// diff 语义必须区分「修改/新增」与「删除」：两项都不是删除。
	for _, fc := range mat.Files {
		require.False(t, fc.Deleted, "modified/added files must not be marked deleted: %s", fc.Path)
	}
}

func TestPrepareDeliveryRefusesProtectedBranchWithZeroRemoteWrites(t *testing.T) {
	f := newDeliveryFixture(t, nil)
	ctx := context.Background()

	// 仓库默认分支是 main；显式指定 branch=main 必须以 ErrProtectedBranch
	// 拒绝（实现把默认分支/保护判定放在前缀白名单之前）且零 ref 写。
	in := prepareInput()
	in.Branch = "main"
	_, err := f.svc.PrepareDelivery(ctx, in)
	require.ErrorIs(t, err, ErrProtectedBranch)

	// 分支远端标记 protected=true（模拟器 isProtected 按客户端查询的完整
	// 分支名精确匹配——与真实 GitHub GET /branches/{branch} 同语义）。
	in = prepareInput()
	in.Branch = "weknora/task/prod-branch"
	f.github.protectBranch("weknora/task/prod-branch")
	_, err = f.svc.PrepareDelivery(ctx, in)
	require.ErrorIs(t, err, ErrProtectedBranch)

	require.Zero(t, f.github.Calls()["POST /git/refs"])
	require.Zero(t, f.github.Calls()["PATCH /git/refs"])
	require.Zero(t, f.github.Calls()["POST /pulls"])
	require.Empty(t, f.github.Violations())
}

// 非法形状分支在任何远端读之前就被本地拒绝（最终修复轮发现 3：不再先打
// BranchProtected 浪费 provider 往返）；而形状合法的 "main" 仍先撞保护
// 分支闸——AC1 第一道闸的次序不变。
func TestPrepareDeliveryRejectsIllegalBranchShapeBeforeAnyRemoteRead(t *testing.T) {
	f := newDeliveryFixture(t, nil)
	ctx := context.Background()

	for _, branch := range []string{"weknora/task/bad branch", "weknora/task/a..b", "main/../escape"} {
		in := prepareInput()
		in.Branch = branch
		_, err := f.svc.PrepareDelivery(ctx, in)
		require.ErrorIs(t, err, ErrInvalidBranch, "branch %q must be refused locally", branch)
	}
	require.Zero(t, f.github.Calls()["GET /repos"], "非法分支不得花费任何远端读")
	require.Zero(t, f.github.Calls()["GET /branches"])
}

func TestPrepareDeliveryAlwaysAwaitsApprovalEvenWithWritePreAuthorization(t *testing.T) {
	f := newDeliveryFixture(t, func(root string) {
		dir := filepath.Join(root, "octocat/hello")
		require.NoError(t, os.MkdirAll(dir, 0o755))
		require.NoError(t, os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main\n\nfunc main() {}\n"), 0o644))
	})
	// 租户级 write 预授权（甚至 deliver 预授权）存在时……deliver 不在白名单
	// 风险类中，PreAuthorizationCovers 按 AllowedRisks 精确匹配，不会命中。
	require.NoError(t, f.db.Create(&appconnectorrepo.PreAuthorizationRow{
		ID: "pre-1", TenantID: 7, AllowedRisksJSON: `["write"]`, ConnectionID: "*", TargetScope: "*",
		ValidFrom: time.Now().Add(-time.Minute), ValidUntil: time.Now().Add(time.Hour), BudgetCapMicro: 0,
	}).Error)
	ctx := context.Background()
	view, err := f.svc.PrepareDelivery(ctx, prepareInput())
	require.NoError(t, err)
	require.Equal(t, "awaiting_approval", view.ActionState, "a write pre-authorization must never auto-authorize delivery")
}

func TestLocalWorkspaceSourceRefusesTraversal(t *testing.T) {
	root := t.TempDir()
	src, err := NewLocalWorkspaceSource(root)
	require.NoError(t, err)
	err = src.WriteSessionWorkspaceFiles(context.Background(), "s-1", []WorkspaceFileWrite{{Path: "../escape.txt", Content: []byte("x")}})
	require.ErrorIs(t, err, ErrInvalidMaterial)
}

// TestStoreStateConstantsMirrorDeliveryState 钉住 repository 包镜像常量与
// 模块级 DeliveryState 的等值性：store.go 为避免导入环（repo 不得反向导入
// 父包）改用字面量，本测试防止两侧字面量静默漂移（T22 #52 task 5）。
func TestStoreStateConstantsMirrorDeliveryState(t *testing.T) {
	require.Equal(t, string(DeliveryPrepared), deliveryrepo.StatePrepared)
	require.Equal(t, string(DeliveryDispatched), deliveryrepo.StateDispatched)
	require.Equal(t, string(DeliveryPushed), deliveryrepo.StatePushed)
	require.Equal(t, string(DeliveryDelivered), deliveryrepo.StateDelivered)
	require.Equal(t, string(DeliveryFailed), deliveryrepo.StateFailed)
	require.Equal(t, string(DeliveryUnknown), deliveryrepo.StateUnknown)
}

// TestDeliveryViewCarriesInitiatorAttribution pins the initiator leg of the
// traceability triple (CONTEXT.md 代码平台连接: 每次远端写入记录发起成员、
// 批准成员与实际远端身份). Approver/RemoteLogin already surface; the row's
// OwnerID must reach the read face too.
func TestDeliveryViewCarriesInitiatorAttribution(t *testing.T) {
	f := newDeliveryFixture(t, nil)

	view, err := f.svc.PrepareDelivery(context.Background(), prepareInput())

	require.NoError(t, err)
	require.Equal(t, "u1", view.Initiator,
		"交付读面必须携带发起者（DeliveryRow.OwnerID 在 prepare 时已落库）")
}
