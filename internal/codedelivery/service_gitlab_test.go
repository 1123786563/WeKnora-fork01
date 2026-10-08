package codedelivery

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	appconnector "github.com/Tencent/WeKnora/internal/appconnector"
	appconnectorrepo "github.com/Tencent/WeKnora/internal/appconnector/repository/appconnector"
	appconnectorsvc "github.com/Tencent/WeKnora/internal/appconnector/service/appconnector"
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
// fail closed；派发面对未知 target 在前置门拒绝（终审修复：派发面拆成
// 两条 leg 分别钉住 A02 门与平台路由门的拒绝点，均为零远端调用——原
// leg 未设 ConnectionID 时实际由 A02 门拒绝，注释声称的平台路由分支由
// 新 leg 真正触达；ProviderOfTarget 的解析面另有 TestDraftMRTitleAndProtectedGlob 单元钉）。
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

	// 派发面 leg 1：合法材料 + 无连接快照（未设 ConnectionID）→ A02 门在
	// 守卫的连接行查询处拒绝，零远端调用。错误链携带 "a02"（与 leg 2 的
	// 平台路由拒绝可区分）。
	before := snapshotGitLabCalls(f)
	snap := appconnectorsvc.ActionSnapshot{ID: "act-x", TenantID: 7, ActorID: "u1", Target: "notion.deliver", Args: mustMaterialJSON(t)}
	_, err = f.dispatcher.Dispatch(ctx, snap, "")
	require.ErrorIs(t, err, appconnectorsvc.ErrDispatchNotStarted)
	require.ErrorContains(t, err, "a02", "无连接快照必须由 A02 门拒绝")
	require.Equal(t, before, snapshotGitLabCalls(f), "未知快照的派发必须零远端调用")

	// 派发面 leg 2（终审修复：真正触达平台路由门）：合法材料 + 可用连接
	// conn-notion（A02 与凭据解析均放行）+ 未知 target → ProviderOfTarget/
	// clientForPlatform 在零远端调用处拒绝。错误链携带 unsupported_provider，
	// 外来 target 永不触达任何适配器。
	snap = appconnectorsvc.ActionSnapshot{ID: "act-x", TenantID: 7, ActorID: "u1",
		ConnectionID: "conn-notion", AuthVersion: 1, Target: "notion.deliver", Args: mustMaterialJSON(t)}
	_, err = f.dispatcher.Dispatch(ctx, snap, "")
	require.ErrorIs(t, err, appconnectorsvc.ErrDispatchNotStarted)
	require.ErrorContains(t, err, "code_delivery_unsupported_provider", "未知 target 必须由平台路由门拒绝而非 A02 门")
	require.Equal(t, before, snapshotGitLabCalls(f), "外来 target 的派发必须零远端调用")
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

// 回归钉（审查修复轮 1）：提供者解析是服务端权威事实、绝不静默猜测——
// Providers 未接线的部署（本任务与容器接线任务之间的中间态）对包括 GitHub
// 在内的一切连接在 prepare 面一律 fail closed、零远端调用。零值是「无交付」
// 现状而非 GitHub-only 降级；生产容器接线落地后本测试继续钉住 nil 语义。
func TestUnwiredProvidersFailsClosedEvenForGitHub(t *testing.T) {
	f := newDeliveryFixture(t, func(root string) {
		dir := filepath.Join(root, "octocat/hello")
		require.NoError(t, os.MkdirAll(dir, 0o755))
		require.NoError(t, os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main\n\nfunc main() {}\n"), 0o644))
	})
	unwired := NewCodeDeliveryService(CodeDeliveryDeps{
		Store: f.svc.deps.Store, Actions: f.svc.deps.Actions, ActionRows: f.svc.deps.ActionRows,
		Connections: f.svc.deps.Connections, Creds: f.svc.deps.Creds,
		GitHub: f.svc.deps.GitHub, GitLab: f.svc.deps.GitLab,
		Workspace: f.svc.deps.Workspace, Runs: f.svc.deps.Runs,
		Dispatcher: f.svc.deps.Dispatcher,
	})
	ctx := context.Background()

	_, err := unwired.PrepareDelivery(ctx, prepareInput())
	require.ErrorIs(t, err, ErrUnsupportedProvider)
	_, err = unwired.MaterializeBaseline(ctx, baselineInput())
	require.ErrorIs(t, err, ErrUnsupportedProvider)

	require.Zero(t, f.github.Calls()["GET /repos"], "提供者拒绝必须发生在任何远端读之前")
	require.Zero(t, f.github.Calls()["GET /branches"])
	require.Zero(t, f.github.Calls()["POST /git/refs"])
}

// Review Focus 5（护栏次序）：目标分支 = 默认分支，或命中 GitLab 通配保护
// 模式 → ErrProtectedBranch 且零远端写。通配匹配发生在适配器内（差异隐藏）。
func TestGitLabPrepareRefusesProtectedBranchWithZeroRemoteWrites(t *testing.T) {
	f := newDeliveryFixture(t, func(root string) {
		dir := filepath.Join(root, "octocat/hello")
		require.NoError(t, os.MkdirAll(dir, 0o755))
		require.NoError(t, os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main\n\nfunc main() {}\n"), 0o644))
	})
	ctx := context.Background()

	// 仓库默认分支是 main：与 GitHub 同序，先撞保护分支闸。
	in := gitlabPrepareInput()
	in.Branch = "main"
	_, err := f.svc.PrepareDelivery(ctx, in)
	require.ErrorIs(t, err, ErrProtectedBranch)

	// GitLab 保护分支是「精确名或通配模式」：模拟器追加 weknora/task/* 模式，
	// 适配器必须本地完成通配匹配后拒绝。
	in = gitlabPrepareInput()
	in.Branch = TaskBranchOf("s-stable")
	f.gitlab.protectPattern("weknora/task/*")
	_, err = f.svc.PrepareDelivery(ctx, in)
	require.ErrorIs(t, err, ErrProtectedBranch)

	require.Zero(t, f.gitlab.Calls()["POST /repository/commits"])
	require.Zero(t, f.gitlab.Calls()["POST /merge_requests"])
	require.Empty(t, f.gitlab.Violations())
}

// Review Focus 2（部分完成）：推送成功、MR 创建确定性失败 → pushed；恢复
// 只补 MR，绝不重发 commits actions（调用计数为证）。
func TestGitLabPartialPushMRFailureRecoversWithoutRepush(t *testing.T) {
	f := seededGitLabFixture(t)
	ctx := context.Background()
	f.gitlab.failNextMRCreation()
	view, err := f.svc.DispatchDelivery(ctx, dispatchInput(firstDelivery(t, f)))
	require.NoError(t, err)
	require.Equal(t, string(DeliveryPushed), view.State)
	require.NotEmpty(t, view.CommitSHA, "推送提交必须已落账（读回的远端 head）")

	before := snapshotGitLabCalls(f)
	view, err = f.svc.DispatchDelivery(ctx, dispatchInput(view)) // MR-only 恢复
	require.NoError(t, err)
	require.Equal(t, string(DeliveryDelivered), view.State)
	after := snapshotGitLabCalls(f)
	require.Equal(t, before["POST /repository/commits"], after["POST /repository/commits"], "恢复不得重发提交")
	require.Equal(t, before["POST /merge_requests"]+1, after["POST /merge_requests"], "恢复只补 MR")
	require.Empty(t, f.gitlab.Violations())
}

// Review Focus 4（不可观测）：commits POST 落地后断网 → unknown；resolve
// 以远端事实收敛（分支已收敛、MR 缺席 → pushed）；随后 MR-only 恢复完成交付。
func TestGitLabUnknownOutcomeResolvesFromRemoteFacts(t *testing.T) {
	f := seededGitLabFixture(t)
	ctx := context.Background()
	f.gitlab.blackoutAfterCommitCreate()
	view, err := f.svc.DispatchDelivery(ctx, dispatchInput(firstDelivery(t, f)))
	require.NoError(t, err)
	require.Equal(t, string(DeliveryUnknown), view.State)
	require.Equal(t, "unknown", view.ActionState)

	f.gitlab.liftBlackout()
	in := dispatchInput(view)
	_, err = f.svc.ResolveDeliveryUnknown(ctx, in)
	require.ErrorIs(t, err, ErrDeliveryConfirmationRequired)
	require.Equal(t, string(DeliveryUnknown), firstDelivery(t, f).State)
	in.ConfirmNoMatchingPR = true
	view, err = f.svc.ResolveDeliveryUnknown(ctx, in)
	require.NoError(t, err)
	require.Equal(t, string(DeliveryPushed), view.State, "远端事实：分支已收敛、MR 缺席 → 部分完成")

	view, err = f.svc.DispatchDelivery(ctx, dispatchInput(view))
	require.NoError(t, err)
	require.Equal(t, string(DeliveryDelivered), view.State)
	require.Empty(t, f.gitlab.Violations())
}

// 同任务分支二次交付（CONTEXT.md「创建或更新草稿 PR」迭代语义）：GitLab 的
// commits API 在分支现 tip 之上追加提交，把分支收敛到新预期树；草稿 MR 复用
// 同 source branch 的既有开放 MR；祖先链证明永不改写历史。
func TestGitLabSecondDeliveryConvergesTaskBranchAndReusesMR(t *testing.T) {
	f := seededGitLabFixture(t)
	ctx := context.Background()

	first, err := f.svc.DispatchDelivery(ctx, dispatchInput(firstDelivery(t, f)))
	require.NoError(t, err)
	require.Equal(t, string(DeliveryDelivered), first.State)
	firstCommit := first.CommitSHA

	// 批准之后工作区再变 → 新 Prepare（新 digest）→ 新批准 → 二次交付。
	require.NoError(t, os.WriteFile(filepath.Join(f.root, "octocat/hello/main.go"),
		[]byte("package main\n\nfunc main() { _ = 1 }\n"), 0o644))
	second, err := f.svc.PrepareDelivery(ctx, gitlabPrepareInput())
	require.NoError(t, err)
	require.NotEqual(t, first.Digest, second.Digest, "内容变化必须铸造新 digest")
	require.NoError(t, f.actions.Approve(ctx, second.ActionID, "u1", second.Digest))

	secondView, err := f.svc.DispatchDelivery(ctx, dispatchInput(second))
	require.NoError(t, err)
	require.Equal(t, string(DeliveryDelivered), secondView.State)
	require.NotEqual(t, firstCommit, secondView.CommitSHA)

	// 收敛不变量：任务分支 tip 树恰好 = 基线树 + 批准变更（无残留、无多余）。
	tip := f.gitlab.BranchTree("weknora/task/s-1")
	require.Len(t, tip, 2)
	require.Equal(t, GitBlobSHA([]byte("package main\n\nfunc main() { _ = 1 }\n")), tip["main.go"])
	require.Equal(t, GitBlobSHA([]byte("# hello\n")), tip["README.md"])
	// 祖先链：新提交以分支现 tip 为 parent（永不 force）。
	require.Equal(t, []string{firstCommit}, f.gitlab.ParentOf(secondView.CommitSHA))
	// 草稿 MR 迭代复用（同 source branch 的开放 MR 不重复开）。
	require.Equal(t, first.PRNumber, secondView.PRNumber)
	require.Empty(t, f.gitlab.Violations())
}

// 删除型交付：基线有而工作区无的文件 → commits API delete action，分支
// tip 树精确收敛为「基线 − 删除 + 新增」。
func TestGitLabDeliveryAppliesDeletionActions(t *testing.T) {
	// 工作区只写 README（基线的 main.go 不在工作区）→ diff = main.go 删除。
	f := newDeliveryFixture(t, func(root string) {
		dir := filepath.Join(root, "octocat/hello")
		require.NoError(t, os.MkdirAll(dir, 0o755))
		require.NoError(t, os.WriteFile(filepath.Join(dir, "README.md"), []byte("# hello\n"), 0o644))
	})
	ctx := context.Background()
	view, err := f.svc.PrepareDelivery(ctx, gitlabPrepareInput())
	require.NoError(t, err)
	require.NoError(t, f.actions.Approve(ctx, view.ActionID, "u1", view.Digest))

	view, err = f.svc.DispatchDelivery(ctx, dispatchInput(view))
	require.NoError(t, err)
	require.Equal(t, string(DeliveryDelivered), view.State)

	tip := f.gitlab.BranchTree("weknora/task/s-1")
	require.Len(t, tip, 1, "删除必须体现在分支 tip 树上")
	_, hasMain := tip["main.go"]
	require.False(t, hasMain)
	require.Empty(t, f.gitlab.Violations())
}

// Review Focus 3（A02 面）：连接 owner 失去成员资格 → A02 拒绝，零远端调用。
func TestGitLabDispatchFailsClosedWhenConnectionUnusable(t *testing.T) {
	f := seededGitLabFixture(t)
	ctx := context.Background()
	membersDrop(f, "u1")
	_, err := f.svc.DispatchDelivery(ctx, dispatchInput(firstDelivery(t, f)))
	require.Error(t, err)
	require.Zero(t, f.gitlab.Calls()["POST /repository/commits"])
	require.Zero(t, f.gitlab.Calls()["POST /merge_requests"])
}

// R5-F8（对账面）：仅存在同 source、target=其他 base 的开放 MR 时，
// QueryProvider 不得凭 head 单维命中误判 delivered（错误终态无自愈）。
// 形态：source 分支已删（MR 仍开放——真实 GitLab 语义）、MR target 指向
// release-9；仓库默认分支 main 上没有任何 MR → 无远端事实 → 仍
// ErrDispatchUnknown，不写回执、不迁移终态。
func TestQueryProviderIgnoresForeignTargetMR(t *testing.T) {
	f := seededGitLabFixture(t)
	ctx := context.Background()
	f.gitlab.blackoutAfterCommitCreate()
	view, err := f.svc.DispatchDelivery(ctx, dispatchInput(firstDelivery(t, f)))
	require.NoError(t, err)
	require.Equal(t, string(DeliveryUnknown), view.State)

	f.gitlab.liftBlackout()
	// 残留形态：同 source 的开放 MR 指向无关 target，且任务分支已被删除。
	f.gitlab.addMR("weknora/task/s-1", "release-9")
	f.gitlab.deleteBranch("weknora/task/s-1")

	view, err = f.svc.GetDelivery(ctx, 7, view.ID)
	require.NoError(t, err)
	_, err = f.svc.ResolveDeliveryUnknown(ctx, dispatchInput(view))
	require.ErrorIs(t, err, appconnectorsvc.ErrDispatchUnknown, "a foreign-target MR is not a remote fact of THIS delivery")

	after, gerr := f.svc.GetDelivery(ctx, 7, view.ID)
	require.NoError(t, gerr)
	require.Equal(t, string(DeliveryUnknown), after.State, "no terminal transition may be written")
	require.Zero(t, after.PRNumber, "no PR receipt may be recorded from a foreign-target MR")
}

// R5-F13（settle 契约级）：EnsureBranch 的空收敛是 commits API 之前的本地
// 可证拒绝（分支/MR 均未创建）。批准已消耗后，若该拒绝不携带
// ErrDispatchNotStarted，settle 会落 unknown——而 QueryProvider 永远查不到
// 任何远端事实，交付永久滞留。批准后把工作区内容回卷成基线内容，派发时
// staged blob 与基线一致 → 空收敛 → 必须落 failed（可重开计划）。
func TestDispatchSettlesFailedOnEnsureBranchLocalRejection(t *testing.T) {
	f := seededGitLabFixture(t)
	ctx := context.Background()
	view := firstDelivery(t, f)

	// 批准之后、派发之前：工作区 main.go 回卷为基线内容（等价 blob sha）。
	require.NoError(t, os.WriteFile(filepath.Join(f.root, "octocat/hello/main.go"), []byte("package main\n"), 0o644))

	_, err := f.svc.DispatchDelivery(ctx, dispatchInput(view))
	require.ErrorIs(t, err, ErrDeliveryDispatchRejected, "a proven pre-send rejection settles failed")

	after, gerr := f.svc.GetDelivery(ctx, 7, view.ID)
	require.NoError(t, gerr)
	require.Equal(t, string(DeliveryFailed), after.State, "the delivery must settle FAILED, not strand in unknown")
	require.Zero(t, f.gitlab.Calls()["POST /repository/commits"], "nothing may leave")
	require.Zero(t, f.gitlab.Calls()["POST /merge_requests"])

	var row appconnectorrepo.ActionRow
	require.NoError(t, f.db.Where("id = ?", after.ActionID).First(&row).Error)
	require.Equal(t, "failed", row.State, "the action settles ActionFailed via the ErrDispatchNotStarted contract")
}
