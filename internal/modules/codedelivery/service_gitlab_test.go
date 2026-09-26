package codedelivery

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	appconnector "github.com/Tencent/WeKnora/internal/modules/appconnector"
	appconnectorrepo "github.com/Tencent/WeKnora/internal/modules/appconnector/repository/appconnector"
	appconnectorsvc "github.com/Tencent/WeKnora/internal/modules/appconnector/service/appconnector"
	"github.com/stretchr/testify/require"
)

func gitlabPrepareInput() PrepareInput {
	return PrepareInput{
		TenantID: 7, CallerID: "u1", RunID: "run-1", ConnectionID: "conn-gl",
		Repo:          RepoRef{Owner: "octocat", Name: "hello"},
		BaselineSHA:   "b" + strings.Repeat("0", 39),
		CommitMessage: "fix: greeting", PRTitle: "WeKnora task s-1",
	}
}

// seededGitLabFixture：工作区在基线之上修改 main.go，prepare + approve 完成
// （GitLab 通路；与 github 侧 seededFixture 同构）。
func seededGitLabFixture(t *testing.T) *deliveryFixture {
	f := newDeliveryFixture(t, func(root string) {
		dir := filepath.Join(root, "octocat/hello")
		require.NoError(t, os.MkdirAll(dir, 0o755))
		require.NoError(t, os.WriteFile(filepath.Join(dir, "README.md"), []byte("# hello\n"), 0o644))
		require.NoError(t, os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main\n\nfunc main() {}\n"), 0o644))
	})
	ctx := context.Background()
	view, err := f.svc.PrepareDelivery(ctx, gitlabPrepareInput())
	require.NoError(t, err)
	require.NoError(t, f.actions.Approve(ctx, view.ActionID, "u1", view.Digest))
	return f
}

func snapshotGitLabCalls(f *deliveryFixture) map[string]int {
	out := map[string]int{}
	for k, v := range f.gitlab.Calls() {
		out[k] = v
	}
	return out
}

func mustBranchCommit(t *testing.T, e *gitLabEmulator, branch string) string {
	t.Helper()
	sha, ok := e.BranchCommit(branch)
	require.True(t, ok)
	return sha
}

func mustMaterialJSON(t *testing.T) []byte {
	t.Helper()
	mat := DeliveryMaterial{
		Repo: RepoRef{Owner: "octocat", Name: "hello"}, BaselineSHA: "b" + strings.Repeat("0", 39),
		Branch: TaskBranchOf("s-9"), Files: []FileChange{{Path: "main.go"}},
		CommitMessage: "m", PRTitle: "t",
	}
	raw, err := mat.CanonicalJSON()
	require.NoError(t, err)
	return raw
}

// AC（验收 2 权限面）：提供者从连接的安装 app id 服务端权威解析；A03 行以
// gitlab.deliver 锚定；回执（服务端 SHA/MR/远端身份/批准人）全部落账，且
// 提交 SHA 是推送后读回的远端权威 head。
func TestGitLabDeliveryE2E_RecordsTraceableReceipts(t *testing.T) {
	f := seededGitLabFixture(t)
	ctx := context.Background()

	view := firstDelivery(t, f)
	var row appconnectorrepo.ActionRow
	require.NoError(t, f.db.Where("id = ?", view.ActionID).First(&row).Error)
	require.Equal(t, "gitlab.deliver", row.Target)
	require.Equal(t, "deliver", row.Risk)

	view, err := f.svc.DispatchDelivery(ctx, dispatchInput(view))
	require.NoError(t, err)
	require.Equal(t, string(DeliveryDelivered), view.State)
	require.NotEmpty(t, view.CommitSHA)
	require.EqualValues(t, 1, view.PRNumber)
	require.Contains(t, view.PRURL, "/-/merge_requests/1")
	require.Equal(t, "gl-user", view.RemoteLogin, "实际远端身份必须落账")
	require.Equal(t, "u1", view.Approver, "审批内容（批准人）必须可追溯")
	require.NotEmpty(t, view.Digest)

	sha, ok := f.gitlab.BranchCommit("weknora/task/s-1")
	require.True(t, ok)
	require.Equal(t, view.CommitSHA, sha, "回执必须是读回的远端权威 head")
	require.Equal(t, "b"+strings.Repeat("0", 39), mustBranchCommit(t, f.gitlab, "main"))
	require.Empty(t, f.gitlab.Violations())
}

// 基线物化走同一 seam：GitLab 适配器的 Tree/Blob 把基线树写入工作区固定根。
func TestGitLabMaterializeBaselineWritesFixedTree(t *testing.T) {
	f := newDeliveryFixture(t, nil)
	ctx := context.Background()

	in := BaselineInput{
		TenantID: 7, CallerID: "u1", RunID: "run-1", ConnectionID: "conn-gl",
		Repo:        RepoRef{Owner: "octocat", Name: "hello"},
		BaselineSHA: "b" + strings.Repeat("0", 39),
	}
	receipt, err := f.svc.MaterializeBaseline(ctx, in)
	require.NoError(t, err)
	require.Equal(t, 2, receipt.Files)
	require.Equal(t, "/workspace/octocat/hello", receipt.Root)
	raw, err := os.ReadFile(filepath.Join(f.root, "octocat/hello/main.go"))
	require.NoError(t, err)
	require.Equal(t, "package main\n", string(raw))
	require.Empty(t, f.gitlab.Violations())
}

// Review Focus 1：连接背后的安装 app 不是代码平台 → 任何远端调用之前
// fail closed；派发面对未知 target 同样拒绝（零远端调用）。
func TestGitLabUnsupportedProviderFailsClosed(t *testing.T) {
	f := newDeliveryFixture(t, func(root string) {
		dir := filepath.Join(root, "octocat/hello")
		require.NoError(t, os.MkdirAll(dir, 0o755))
		require.NoError(t, os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main\n\nfunc main() {}\n"), 0o644))
	})
	ctx := context.Background()

	require.NoError(t, f.db.Create(&appconnectorrepo.InstallationRow{ID: "inst-notion", AppID: "notion", AppVersion: "1", State: appconnector.InstallationActive, TenantID: 7}).Error)
	require.NoError(t, f.db.Create(&appconnectorrepo.ConnectionRow{
		ID: "conn-notion", InstallationID: "inst-notion", Kind: appconnector.ConnectionKindPersonal,
		OwnerID: "u1", CredentialRef: "mcp:conn-notion:notion", State: appconnector.ConnectionActive,
		TenantID: 7, AuthVersion: 1,
	}).Error)

	in := gitlabPrepareInput()
	in.ConnectionID = "conn-notion"
	_, err := f.svc.PrepareDelivery(ctx, in)
	require.ErrorIs(t, err, ErrUnsupportedProvider)
	require.Zero(t, f.gitlab.Calls()["GET /project"], "提供者拒绝必须发生在任何远端读之前")
	require.Zero(t, f.gitlab.Calls()["GET /repository/tree"])

	// 派发面：合法材料 + 未知 target → 前置门拒绝，零远端调用。
	snap := appconnectorsvc.ActionSnapshot{ID: "act-x", TenantID: 7, ActorID: "u1", Target: "notion.deliver", Args: mustMaterialJSON(t)}
	_, err = f.dispatcher.Dispatch(ctx, snap, "")
	require.ErrorIs(t, err, appconnectorsvc.ErrDispatchNotStarted)
	require.Zero(t, f.gitlab.Calls()["POST /repository/commits"])
	require.Zero(t, f.gitlab.Calls()["POST /merge_requests"])
}

// Review Focus 3（篡改面）：派发器解析失败 = ErrDispatchNotStarted，零 GitLab 调用
// （prepare 阶段的远端读已发生，故以「派发前后调用计数不变」精确断言）。
func TestGitLabTamperedSnapshotNeverReachesGitLab(t *testing.T) {
	f := seededGitLabFixture(t)
	before := snapshotGitLabCalls(f)
	snap := appconnectorsvc.ActionSnapshot{ID: "act-x", TenantID: 7, ActorID: "u1", Target: "gitlab.deliver", Args: []byte(`{"repo":"o/n"}`)}
	_, err := f.dispatcher.Dispatch(context.Background(), snap, "")
	require.ErrorIs(t, err, appconnectorsvc.ErrDispatchNotStarted)
	after := snapshotGitLabCalls(f)
	require.Equal(t, before, after, "篡改快照的派发必须零远端调用")
}
