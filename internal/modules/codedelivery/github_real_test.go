package codedelivery

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"testing"
	"time"

	appconnector "github.com/Tencent/WeKnora/internal/modules/appconnector"
	appconnectorrepo "github.com/Tencent/WeKnora/internal/modules/appconnector/repository/appconnector"
	appconnectorsvc "github.com/Tencent/WeKnora/internal/modules/appconnector/service/appconnector"
	deliveryrepo "github.com/Tencent/WeKnora/internal/modules/codedelivery/repository/codedelivery"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// TestGitHubClientAgainstRealGitHub 需要 WEKNORA_GITHUB_TEST_TOKEN（经典
// PAT，repo 只读作用域）与 WEKNORA_GITHUB_TEST_REPO（owner/name，公开仓库）。
// 缺 env 即 skip——真实 GitHub 属 true external（blocked-env），不伪造。
func TestGitHubClientAgainstRealGitHub(t *testing.T) {
	token := os.Getenv("WEKNORA_GITHUB_TEST_TOKEN")
	repoRaw := os.Getenv("WEKNORA_GITHUB_TEST_REPO")
	if token == "" || repoRaw == "" {
		t.Skip("WEKNORA_GITHUB_TEST_TOKEN/WEKNORA_GITHUB_TEST_REPO not set (blocked-env)")
	}
	repo, err := ParseRepoRef(repoRaw)
	require.NoError(t, err)
	client := NewGitHubClientFactory(httpClientDefault(), GitHubAPIBaseURL)(token, repo)
	ctx := context.Background()

	info, err := client.Repository(ctx)
	require.NoError(t, err)
	require.NotEmpty(t, info.DefaultBranch)
	login, err := client.CurrentLogin(ctx)
	require.NoError(t, err)
	t.Logf("real github: repo=%s default=%s login=%s", repo, info.DefaultBranch, login)
}

// realLoopConnections mirrors fixtureConnections (service_prepare_test.go:41)
// but hands out the REAL token from the environment — credentials are read
// from env only, never literals.
type realLoopConnections struct {
	db      *gorm.DB
	members map[string]bool
}

func (s *realLoopConnections) FindConnectionByID(ctx context.Context, id string) (appconnector.Connection, error) {
	var row appconnectorrepo.ConnectionRow
	if err := s.db.WithContext(ctx).Where("id = ?", id).First(&row).Error; err != nil {
		return appconnector.Connection{}, err
	}
	return appconnector.Connection{
		ID: row.ID, InstallationID: row.InstallationID, Kind: row.Kind,
		OwnerID: row.OwnerID, CredentialRef: row.CredentialRef,
		State: row.State, TenantID: row.TenantID, AuthVersion: row.AuthVersion,
	}, nil
}

func (s *realLoopConnections) LoadCredential(ctx context.Context, c appconnector.Connection) ([]byte, error) {
	return []byte(os.Getenv("WEKNORA_GITHUB_TEST_TOKEN")), nil
}

func (s *realLoopConnections) MemberActive(ctx context.Context, tenantID uint64, userID string) (bool, error) {
	return s.members[userID], nil
}

func (s *realLoopConnections) TryAcquireRefreshLease(ctx context.Context, c appconnector.Connection, leaseID string, until time.Time) (bool, error) {
	return true, nil
}

func (s *realLoopConnections) Resolve(ctx context.Context, connectionID string, expectedVersion int64) ([]byte, error) {
	return []byte(os.Getenv("WEKNORA_GITHUB_TEST_TOKEN")), nil
}

// realLoopBaselineSHA anchors the delivery on the default branch's CURRENT
// head (a real 40-hex commit sha the real API accepts).
func realLoopBaselineSHA(t *testing.T, client GitHubClient) string {
	t.Helper()
	info, err := client.Repository(context.Background())
	require.NoError(t, err)
	head, exists, err := client.BranchHead(context.Background(), info.DefaultBranch)
	require.NoError(t, err)
	require.True(t, exists, "default branch %q is absent; refusing to deliver from a missing baseline", info.DefaultBranch)
	require.Len(t, head, 40)
	return head
}

// materializeRealLoopBaseline mirrors a checkout: every baseline blob is
// written before the test adds its one new marker file. This keeps the real
// delivery diff limited to that marker instead of accidentally deleting the
// repository's existing files.
func materializeRealLoopBaseline(t *testing.T, client GitHubClient, baselineSHA, repoDir string) {
	t.Helper()
	treeSHA, err := client.CommitTree(context.Background(), baselineSHA)
	require.NoError(t, err)
	entries, err := client.Tree(context.Background(), treeSHA)
	require.NoError(t, err)
	paths := make([]string, 0, len(entries))
	for path := range entries {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	for _, path := range paths {
		clean := filepath.Clean(filepath.FromSlash(path))
		require.True(t, filepath.IsLocal(clean), "GitHub tree path must stay inside the test repository: %q", path)
		content, err := client.Blob(context.Background(), entries[path])
		require.NoError(t, err)
		full := filepath.Join(repoDir, clean)
		require.NoError(t, os.MkdirAll(filepath.Dir(full), 0o755))
		require.NoError(t, os.WriteFile(full, content, 0o644))
	}
}

// TestGitHubRealRecoveryLoopNoRepeatPush 需要 WEKNORA_GITHUB_TEST_TOKEN（对
// WEKNORA_GITHUB_TEST_REPO 有写权限的 PAT）、WEKNORA_GITHUB_TEST_REPO（owner/name，
// 专用测试仓库——本测试会真实推送任务分支并创建草稿 PR）与
// WEKNORA_GITHUB_TEST_WRITABLE=1 三者同时在场。缺任一即 skip（blocked-env，
// 不伪造）。真实部分成功（pushed）需要平台侧 PR 失败注入、不可确定性制造：
// 该分支的等价证据是 delivery_recovery_http_test.go 的 wire 契约 e2e；本测试
// 覆盖真实环境可确定性的恢复收敛面：完整交付→delivered→重复 dispatch 被状态
// 机拒绝（不重复推送）→读面幂等。
func TestGitHubRealRecoveryLoopNoRepeatPush(t *testing.T) {
	token := os.Getenv("WEKNORA_GITHUB_TEST_TOKEN")
	repoRaw := os.Getenv("WEKNORA_GITHUB_TEST_REPO")
	if token == "" || repoRaw == "" || os.Getenv("WEKNORA_GITHUB_TEST_WRITABLE") != "1" {
		t.Skip("WEKNORA_GITHUB_TEST_TOKEN/WEKNORA_GITHUB_TEST_REPO/WEKNORA_GITHUB_TEST_WRITABLE not set (blocked-env)")
	}
	repo, err := ParseRepoRef(repoRaw)
	require.NoError(t, err)

	// 装配镜像 newDeliveryFixture（service_prepare_test.go:102-149），但 GitHub
	// factory 指向真实 API；workspace/DB/审批链全部真实（同包私有类型复用）。
	dsn := "file:" + filepath.Join(t.TempDir(), "real.db") + "?_foreign_keys=on&_busy_timeout=5000"
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(
		&appconnectorrepo.ActionRow{}, &appconnectorrepo.ApprovalRow{}, &appconnectorrepo.PreAuthorizationRow{},
		&appconnectorrepo.ConnectionRow{}, &appconnectorrepo.InstallationRow{}, &deliveryrepo.DeliveryRow{},
	))
	require.NoError(t, db.Create(&appconnectorrepo.InstallationRow{ID: "inst-gh", AppID: "github", AppVersion: "1", State: appconnector.InstallationActive, TenantID: 7}).Error)
	require.NoError(t, db.Create(&appconnectorrepo.ConnectionRow{
		ID: "conn-gh", InstallationID: "inst-gh", Kind: appconnector.ConnectionKindPersonal,
		OwnerID: "u1", CredentialRef: "mcp:conn-gh:github", State: appconnector.ConnectionActive,
		TenantID: 7, AuthVersion: 1,
	}).Error)

	factory := NewGitHubClientFactory(httpClientDefault(), GitHubAPIBaseURL)
	client := factory(token, repo)
	baseline := realLoopBaselineSHA(t, client)
	info, err := client.Repository(context.Background())
	require.NoError(t, err)

	root := t.TempDir()
	sessionID := fmt.Sprintf("s-real-%d", time.Now().UnixNano())
	dir := filepath.Join(root, repo.Owner, repo.Name)
	require.NoError(t, os.MkdirAll(dir, 0o755))
	materializeRealLoopBaseline(t, client, baseline, dir)
	marker := filepath.Join(dir, "weknora-real-recovery-loop.txt")
	_, err = os.Stat(marker)
	require.True(t, os.IsNotExist(err), "the marker must be a new path so the baseline remains untouched")
	require.NoError(t, os.WriteFile(marker, []byte("recovery loop marker\n"), 0o644))
	workspace, err := NewLocalWorkspaceSource(root)
	require.NoError(t, err)

	actionStore := appconnectorrepo.NewActionStore(db)
	store := deliveryrepo.NewDeliveryStore(db)
	connections := &realLoopConnections{db: db, members: map[string]bool{"u1": true}}
	guard := appconnectorsvc.NewSubjectGuard(connections)
	dispatcher := NewDeliveryDispatcher(DispatcherDeps{
		Connections: connections, Creds: connections, Guard: guard,
		GitHub: factory, Workspace: workspace, Store: store,
		ActionRows: actionStore, Runs: fixtureRun{sessionID: sessionID},
	})
	actions := appconnectorsvc.NewActionService(actionStore, guard, nil, dispatcher, dispatcher)
	svc := NewCodeDeliveryService(CodeDeliveryDeps{
		Store: store, Actions: actions, ActionRows: actionStore,
		Connections: connections, Creds: connections,
		GitHub: factory, Providers: appconnectorrepo.NewInstallationStore(db),
		Workspace: workspace, Runs: fixtureRun{sessionID: sessionID}, Dispatcher: dispatcher,
	})

	ctx := context.Background()
	view, err := svc.PrepareDelivery(ctx, PrepareInput{
		TenantID: 7, CallerID: "u1", RunID: "run-1", ConnectionID: "conn-gh",
		Repo: repo, BaselineSHA: baseline, CommitMessage: "t25 real recovery loop", PRTitle: "t25 real recovery loop",
	})
	require.NoError(t, err)
	require.NoError(t, actions.Approve(ctx, view.ActionID, "u1", view.Digest))
	view, err = svc.DispatchDelivery(ctx, DispatchInput{TenantID: 7, CallerID: "u1", RunID: "run-1", DeliveryID: view.ID})
	require.NoError(t, err)
	require.Equal(t, string(DeliveryDelivered), view.State)
	require.NotEmpty(t, view.PRURL)
	receipt, err := client.PullRequestForHead(ctx, repo.Owner+":"+view.Branch, info.DefaultBranch)
	require.NoError(t, err)
	require.NotNil(t, receipt)
	require.True(t, receipt.Draft, "the real PR must remain a draft")

	// The duplicate dispatch must be refused by the state machine — no
	// second push leaves the process.
	_, err = svc.DispatchDelivery(ctx, DispatchInput{TenantID: 7, CallerID: "u1", RunID: "run-1", DeliveryID: view.ID})
	require.ErrorIs(t, err, ErrDeliveryState)
	again, err := svc.GetDelivery(ctx, 7, view.ID)
	require.NoError(t, err)
	require.Equal(t, string(DeliveryDelivered), again.State, "the read face stays idempotent after the refused duplicate")
}
