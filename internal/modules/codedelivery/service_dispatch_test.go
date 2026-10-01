package codedelivery

import (
	"context"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	appconnector "github.com/Tencent/WeKnora/internal/modules/appconnector"
	appconnectorrepo "github.com/Tencent/WeKnora/internal/modules/appconnector/repository/appconnector"
	appconnectorsvc "github.com/Tencent/WeKnora/internal/modules/appconnector/service/appconnector"
	"github.com/stretchr/testify/require"
)

// dispatchFixture：夹具内部已单实例装配 DeliveryDispatcher（Task 6 改造后），
// 直通即可；prepare+approve 完成后返回已批准待派发的种子。
func dispatchFixture(t *testing.T, mutate func(root string)) *deliveryFixture {
	return newDeliveryFixture(t, mutate)
}

// seededFixture：基线之上修改 main.go，prepare + approve 完成。
func seededFixture(t *testing.T) *deliveryFixture {
	f := dispatchFixture(t, func(root string) {
		dir := filepath.Join(root, "octocat/hello")
		require.NoError(t, os.MkdirAll(dir, 0o755))
		require.NoError(t, os.WriteFile(filepath.Join(dir, "README.md"), []byte("# hello\n"), 0o644))
		require.NoError(t, os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main\n\nfunc main() {}\n"), 0o644))
	})
	ctx := context.Background()
	view, err := f.svc.PrepareDelivery(ctx, prepareInput())
	require.NoError(t, err)
	require.NoError(t, f.actions.Approve(ctx, view.ActionID, "u1", view.Digest))
	return f
}

func dispatchInput(view DeliveryView) DispatchInput {
	return DispatchInput{TenantID: 7, CallerID: "u1", RunID: "run-1", DeliveryID: view.ID}
}

func firstDelivery(t *testing.T, f *deliveryFixture) DeliveryView {
	view, err := f.svc.GetDeliveryForRun(context.Background(), 7, "run-1")
	require.NoError(t, err)
	return view
}

// AC2 端到端：提交 SHA、审批内容（approver+digest）与 PR 回执全部落账。
func TestDispatchDeliversAndRecordsTraceableReceipts(t *testing.T) {
	f := seededFixture(t)
	ctx := context.Background()

	view, err := f.svc.DispatchDelivery(ctx, dispatchInput(firstDelivery(t, f)))
	require.NoError(t, err)
	require.Equal(t, string(DeliveryDelivered), view.State)
	require.NotEmpty(t, view.CommitSHA)
	require.EqualValues(t, 1, view.PRNumber)
	require.Contains(t, view.PRURL, "/pull/1")
	require.Equal(t, "octocat", view.RemoteLogin, "实际远端身份必须落账")
	require.Equal(t, "u1", view.Approver, "审批内容（批准人）必须可追溯")
	require.NotEmpty(t, view.Digest)

	// 远端事实：任务分支指向候选提交；默认分支纹丝不动。
	sha, ok := f.github.BranchCommit("weknora/task/s-1")
	require.True(t, ok)
	require.Equal(t, view.CommitSHA, sha)
	// 计划稿字面量笔误修正：基线 sha 是 "b"+39 个 0（40 字符，与
	// prepareInput()/模拟器 seed 同源），计划硬编码字面量多写了一个 0。
	require.Equal(t, "b"+strings.Repeat("0", 39), branchCommitOf(t, f, "main"))
	require.Empty(t, f.github.Violations())
}

func branchCommitOf(t *testing.T, f *deliveryFixture, branch string) string {
	t.Helper()
	sha, ok := f.github.BranchCommit(branch)
	require.True(t, ok)
	return sha
}

// AC1：派发全程零 merge、零保护分支写（模拟器违规计数器为证）。
func TestDispatchNeverWritesProtectedBranchOrMerges(t *testing.T) {
	f := seededFixture(t)
	ctx := context.Background()
	_, err := f.svc.DispatchDelivery(ctx, dispatchInput(firstDelivery(t, f)))
	require.NoError(t, err)
	require.Zero(t, f.github.Calls()["PUT /pulls/merge"])
	require.Empty(t, f.github.Violations())
}

// 部分完成（spec 故事 47 / CONTEXT.md 避免项）：推送成功、PR 创建确定性
// 失败 → 状态 pushed；恢复只补 PR，绝不重发 blobs/tree/commit/ref。
func TestPartialPushPRFailureRecoversWithoutRepush(t *testing.T) {
	f := seededFixture(t)
	ctx := context.Background()
	f.github.failNextPRCreation()
	view, err := f.svc.DispatchDelivery(ctx, dispatchInput(firstDelivery(t, f)))
	require.NoError(t, err)
	require.Equal(t, string(DeliveryPushed), view.State)
	require.NotEmpty(t, view.CommitSHA, "推送提交必须已落账")

	before := snapshotCalls(f)
	view, err = f.svc.DispatchDelivery(ctx, dispatchInput(view)) // PR-only 恢复
	require.NoError(t, err)
	require.Equal(t, string(DeliveryDelivered), view.State)
	after := snapshotCalls(f)
	require.Equal(t, before["POST /git/blobs"], after["POST /git/blobs"], "recovery must not re-send blobs")
	require.Equal(t, before["POST /git/trees"], after["POST /git/trees"], "recovery must not re-send trees")
	require.Equal(t, before["POST /git/commits"], after["POST /git/commits"], "recovery must not re-send commits")
	require.Equal(t, before["POST /git/refs"], after["POST /git/refs"], "recovery must not re-push the branch")
	require.Equal(t, before["PATCH /git/refs"], after["PATCH /git/refs"])
	require.Equal(t, before["POST /pulls"]+1, after["POST /pulls"], "recovery retries ONLY the PR creation")
}

func TestDispatchAndResolveRejectMismatchedPersistedRunAndOwner(t *testing.T) {
	for _, field := range []string{"run_id", "owner_id"} {
		t.Run(field, func(t *testing.T) {
			f := seededFixture(t)
			view := firstDelivery(t, f)
			before := snapshotCalls(f)
			column, value := "run_id", "run-other"
			if field == "owner_id" {
				column, value = "owner_id", "u-other"
			}
			require.NoError(t, f.svc.deps.Store.DB().Table("code_deliveries").Where("id = ?", view.ID).Update(column, value).Error)
			in := dispatchInput(view)
			_, err := f.svc.DispatchDelivery(context.Background(), in)
			require.ErrorIs(t, err, ErrNotDeliveryOwner)
			_, err = f.svc.ResolveDeliveryUnknown(context.Background(), in)
			require.ErrorIs(t, err, ErrNotDeliveryOwner)
			require.Equal(t, before, snapshotCalls(f), "mismatched identities must be rejected before any provider operation")
		})
	}
}

func snapshotCalls(f *deliveryFixture) map[string]int {
	out := map[string]int{}
	for k, v := range f.github.Calls() {
		out[k] = v
	}
	return out
}

// 审批锚点不可变（Review Focus 3 / spec「candidate code commits are
// immutable approval anchors」）：批准后内容被换=新 Prepare=新 digest；旧
// digest 的批准对第二个交付必然 digest mismatch 拒绝，且第二个交付在未获
// 得自己（新 digest）的批准前派发被拒——全程零 GitHub 调用。
func TestDispatchRefusesSecondDeliveryPreparedWithDifferentFiles(t *testing.T) {
	f := dispatchFixture(t, func(root string) {
		dir := filepath.Join(root, "octocat/hello")
		require.NoError(t, os.MkdirAll(dir, 0o755))
		require.NoError(t, os.WriteFile(filepath.Join(dir, "README.md"), []byte("# hello\n"), 0o644))
		require.NoError(t, os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main\n\nfunc main() {}\n"), 0o644))
	})
	ctx := context.Background()

	// 交付一：改 main.go → action A1 / digest D1。
	first, err := f.svc.PrepareDelivery(ctx, prepareInput())
	require.NoError(t, err)
	require.NoError(t, f.actions.Approve(ctx, first.ActionID, "u1", first.Digest))

	// 批准之后工作区内容再变（新增 util.go）→ 交付二必须走新 Prepare：
	// action A2 / digest D2 ≠ D1（immutable anchor：旧批准绝不迁移）。
	require.NoError(t, os.WriteFile(filepath.Join(f.root, "octocat/hello/util.go"), []byte("package main\n\nfunc util() {}\n"), 0o644))
	second, err := f.svc.PrepareDelivery(ctx, prepareInput())
	require.NoError(t, err)
	require.NotEqual(t, first.Digest, second.Digest, "content change must mint a NEW digest")
	require.NotEqual(t, first.ActionID, second.ActionID)

	// 旧 digest 的批准对 A2 拒绝（A03 digest 绑定），A2 停留在 awaiting_approval。
	require.Error(t, f.actions.Approve(ctx, second.ActionID, "u1", first.Digest))
	after, err := f.svc.GetDelivery(ctx, 7, second.ID)
	require.NoError(t, err)
	require.Equal(t, "awaiting_approval", after.ActionState)

	// 未获自己 digest 的批准即派发交付二：拒绝且零 GitHub 写调用。
	_, err = f.svc.DispatchDelivery(ctx, dispatchInput(after))
	require.Error(t, err)
	require.Zero(t, f.github.Calls()["POST /git/refs"])
	require.Zero(t, f.github.Calls()["POST /git/blobs"])
	require.Zero(t, f.github.Calls()["POST /pulls"])
	require.Empty(t, f.github.Violations())
}

// 未批准即派发：A03 拒绝（ErrActionState 族），零 GitHub 调用。
func TestDispatchWithoutApprovalConsumesNothing(t *testing.T) {
	f := dispatchFixture(t, func(root string) {
		dir := filepath.Join(root, "octocat/hello")
		require.NoError(t, os.MkdirAll(dir, 0o755))
		require.NoError(t, os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main\n\nfunc main() {}\n"), 0o644))
	})
	ctx := context.Background()
	view, err := f.svc.PrepareDelivery(ctx, prepareInput())
	require.NoError(t, err)
	_, err = f.svc.DispatchDelivery(ctx, dispatchInput(view))
	require.Error(t, err)
	require.Zero(t, f.github.Calls()["POST /git/refs"])
	require.Zero(t, f.github.Calls()["POST /pulls"])
}

// 篡改快照永不触达 GitHub：dispatcher 解析失败=ErrDispatchNotStarted。
func TestTamperedSnapshotNeverReachesGitHub(t *testing.T) {
	f := seededFixture(t)
	snap := appconnectorsvc.ActionSnapshot{ID: "act-x", TenantID: 7, ActorID: "u1", Args: []byte(`{"repo":"o/n"}`)}
	_, err := f.dispatcher.Dispatch(context.Background(), snap, "")
	require.ErrorIs(t, err, appconnectorsvc.ErrDispatchNotStarted)
	require.Zero(t, f.github.Calls()["POST /git/blobs"])
}

// 远端不可观测 → unknown 落账；ResolveUnknown 以远端事实收敛（分支已推、
// PR 缺席 → pushed），随后 PR-only 恢复完成交付。
func TestUnknownOutcomeResolvesFromRemoteFacts(t *testing.T) {
	f := seededFixture(t)
	ctx := context.Background()
	f.github.blackoutAfterRefCreate()
	view, err := f.svc.DispatchDelivery(ctx, dispatchInput(firstDelivery(t, f)))
	require.NoError(t, err)
	require.Equal(t, string(DeliveryUnknown), view.State)
	require.Equal(t, "unknown", view.ActionState)

	f.github.liftBlackout()
	in := dispatchInput(view)
	_, err = f.svc.ResolveDeliveryUnknown(ctx, in)
	require.ErrorIs(t, err, ErrDeliveryConfirmationRequired)
	require.Equal(t, string(DeliveryUnknown), firstDelivery(t, f).State)
	in.ConfirmNoMatchingPR = true
	view, err = f.svc.ResolveDeliveryUnknown(ctx, in)
	require.NoError(t, err)
	require.Equal(t, string(DeliveryPushed), view.State, "远端事实：分支已推、PR 缺席 → 部分完成")
	action, err := f.svc.deps.ActionRows.FindAction(ctx, view.ActionID)
	require.NoError(t, err)
	require.Equal(t, appconnector.ActionSucceeded, action.State, "A03 action and delivery must settle consistently")

	view, err = f.svc.DispatchDelivery(ctx, dispatchInput(view)) // PR-only 恢复
	require.NoError(t, err)
	require.Equal(t, string(DeliveryDelivered), view.State)
	require.Empty(t, f.github.Violations())
}

func TestDispatchedSucceededActionNeedsAndAcceptsConfirmation(t *testing.T) {
	f := seededFixture(t)
	ctx := context.Background()
	f.github.failNextPRCreation()
	view, err := f.svc.DispatchDelivery(ctx, dispatchInput(firstDelivery(t, f)))
	require.NoError(t, err)
	require.Equal(t, string(DeliveryPushed), view.State)
	require.NoError(t, f.svc.deps.Store.TransitionState(ctx, 7, view.ID, []string{string(DeliveryPushed)}, string(DeliveryDispatched), "simulated interrupted settlement"))
	in := dispatchInput(view)
	_, err = f.svc.ResolveDeliveryUnknown(ctx, in)
	require.ErrorIs(t, err, ErrDeliveryConfirmationRequired)
	require.Equal(t, string(DeliveryDispatched), firstDelivery(t, f).State)
	in.ConfirmNoMatchingPR = true
	resolved, err := f.svc.ResolveDeliveryUnknown(ctx, in)
	require.NoError(t, err)
	require.Equal(t, string(DeliveryPushed), resolved.State)
	require.NotEmpty(t, resolved.CommitSHA)
	action, err := f.svc.deps.ActionRows.FindAction(ctx, view.ActionID)
	require.NoError(t, err)
	require.Equal(t, appconnector.ActionSucceeded, action.State)
}

func TestDispatchedUnknownActionSettlesBothRecordsWithConfirmation(t *testing.T) {
	f := seededFixture(t)
	ctx := context.Background()
	f.github.blackoutAfterRefCreate()
	view, err := f.svc.DispatchDelivery(ctx, dispatchInput(firstDelivery(t, f)))
	require.NoError(t, err)
	require.Equal(t, string(DeliveryUnknown), view.State)
	f.github.liftBlackout()
	require.NoError(t, f.svc.deps.Store.TransitionState(ctx, 7, view.ID, []string{string(DeliveryUnknown)}, string(DeliveryDispatched), "simulated delayed action settlement"))
	in := dispatchInput(view)
	_, err = f.svc.ResolveDeliveryUnknown(ctx, in)
	require.ErrorIs(t, err, ErrDeliveryConfirmationRequired)
	in.ConfirmNoMatchingPR = true
	resolved, err := f.svc.ResolveDeliveryUnknown(ctx, in)
	require.NoError(t, err)
	require.Equal(t, string(DeliveryPushed), resolved.State)
	action, err := f.svc.deps.ActionRows.FindAction(ctx, view.ActionID)
	require.NoError(t, err)
	require.Equal(t, appconnector.ActionSucceeded, action.State)
}

func TestUnknownResolutionSettlementFailureLeavesDeliveryAmbiguousAndRetriesReadOnly(t *testing.T) {
	f := seededFixture(t)
	ctx := context.Background()
	f.github.blackoutAfterRefCreate()
	view, err := f.svc.DispatchDelivery(ctx, dispatchInput(firstDelivery(t, f)))
	require.NoError(t, err)
	f.github.liftBlackout()
	require.NoError(t, f.db.Exec(`CREATE TRIGGER fail_action_success BEFORE UPDATE OF state ON app_actions WHEN NEW.state = 'succeeded' BEGIN SELECT RAISE(FAIL, 'injected settlement failure'); END`).Error)
	in := dispatchInput(view)
	in.ConfirmNoMatchingPR = true
	before := snapshotCalls(f)
	_, err = f.svc.ResolveDeliveryUnknown(ctx, in)
	require.Error(t, err)
	require.Equal(t, string(DeliveryUnknown), firstDelivery(t, f).State)
	action, err := f.svc.deps.ActionRows.FindAction(ctx, view.ActionID)
	require.NoError(t, err)
	require.Equal(t, appconnector.ActionUnknown, action.State)
	_, err = f.svc.DispatchDelivery(ctx, in)
	require.ErrorIs(t, err, ErrDeliveryState)
	require.Equal(t, before["POST /pulls"], snapshotCalls(f)["POST /pulls"], "unknown action cannot enter create path")
	require.NoError(t, f.db.Exec(`DROP TRIGGER fail_action_success`).Error)
	before = snapshotCalls(f)
	resolved, err := f.svc.ResolveDeliveryUnknown(ctx, in)
	require.NoError(t, err)
	require.Equal(t, string(DeliveryPushed), resolved.State)
	action, err = f.svc.deps.ActionRows.FindAction(ctx, view.ActionID)
	require.NoError(t, err)
	require.Equal(t, appconnector.ActionSucceeded, action.State)
	require.Equal(t, before["POST /pulls"], snapshotCalls(f)["POST /pulls"], "resolve must remain read-only")
}

func TestUnknownResolutionPropagatesBranchLookupErrorWithoutWrites(t *testing.T) {
	f := seededFixture(t)
	ctx := context.Background()
	f.github.blackoutAfterRefCreate()
	view, err := f.svc.DispatchDelivery(ctx, dispatchInput(firstDelivery(t, f)))
	require.NoError(t, err)
	require.Equal(t, string(DeliveryUnknown), view.State)
	f.github.liftBlackout()
	f.github.failNextBranchRead()
	before := snapshotCalls(f)
	in := dispatchInput(view)
	in.ConfirmNoMatchingPR = true
	_, err = f.svc.ResolveDeliveryUnknown(ctx, in)
	require.Error(t, err)
	var apiErr *GitHubAPIError
	require.ErrorAs(t, err, &apiErr)
	require.Equal(t, http.StatusServiceUnavailable, apiErr.Status)
	require.Equal(t, string(DeliveryUnknown), firstDelivery(t, f).State)
	action, err := f.svc.deps.ActionRows.FindAction(ctx, view.ActionID)
	require.NoError(t, err)
	require.Equal(t, appconnector.ActionUnknown, action.State)
	require.Equal(t, before["POST /pulls"], snapshotCalls(f)["POST /pulls"])
}

func TestSucceededActionReconcilesAmbiguousDeliveryOnRetry(t *testing.T) {
	f := seededFixture(t)
	ctx := context.Background()
	f.github.blackoutAfterRefCreate()
	view, err := f.svc.DispatchDelivery(ctx, dispatchInput(firstDelivery(t, f)))
	require.NoError(t, err)
	f.github.liftBlackout()
	require.NoError(t, f.db.Exec(`CREATE TRIGGER fail_delivery_settle BEFORE UPDATE OF state ON code_deliveries WHEN NEW.state = 'pushed' BEGIN SELECT RAISE(FAIL, 'injected delivery settlement failure'); END`).Error)
	in := dispatchInput(view)
	in.ConfirmNoMatchingPR = true
	_, err = f.svc.ResolveDeliveryUnknown(ctx, in)
	require.Error(t, err)
	require.Equal(t, string(DeliveryUnknown), firstDelivery(t, f).State)
	action, err := f.svc.deps.ActionRows.FindAction(ctx, view.ActionID)
	require.NoError(t, err)
	require.Equal(t, appconnector.ActionSucceeded, action.State)
	before := snapshotCalls(f)
	_, err = f.svc.DispatchDelivery(ctx, in)
	require.ErrorIs(t, err, ErrDeliveryState)
	require.Equal(t, before["POST /pulls"], snapshotCalls(f)["POST /pulls"])
	require.NoError(t, f.db.Exec(`DROP TRIGGER fail_delivery_settle`).Error)
	before = snapshotCalls(f)
	resolved, err := f.svc.ResolveDeliveryUnknown(ctx, in)
	require.NoError(t, err)
	require.Equal(t, string(DeliveryPushed), resolved.State)
	require.Equal(t, before["POST /pulls"], snapshotCalls(f)["POST /pulls"])
}

// A02 拒绝（成员资格撤销）：动作不消费、零远端调用。
func TestDispatchFailsClosedWhenConnectionUnusable(t *testing.T) {
	f := seededFixture(t)
	ctx := context.Background()
	membersDrop(f, "u1")
	_, err := f.svc.DispatchDelivery(ctx, dispatchInput(firstDelivery(t, f)))
	require.Error(t, err)
	require.Zero(t, f.github.Calls()["POST /git/refs"])
	require.Zero(t, f.github.Calls()["POST /pulls"])
}

// 同任务分支二次交付（CONTEXT.md「创建或更新草稿 PR」迭代语义，最终修复
// 轮发现 1）：第二次交付的提交必须以任务分支现 head 为 parent。模拟器
// PATCH 现按 fast-forward 校验（真实 GitHub 同语义），parent 链错误时本
// 测试以 422 失败——这正是修复前实现（恒以 BaselineSHA 为 parent）在真实
// GitHub 必败的回归守卫。
func TestSecondDeliveryOnSameTaskBranchFastForwards(t *testing.T) {
	f := seededFixture(t)
	ctx := context.Background()

	first, err := f.svc.DispatchDelivery(ctx, dispatchInput(firstDelivery(t, f)))
	require.NoError(t, err)
	require.Equal(t, string(DeliveryDelivered), first.State)
	firstCommit := first.CommitSHA

	// 批准之后工作区再变 → 新 Prepare（新 digest）→ 新批准 → 二次交付。
	require.NoError(t, os.WriteFile(filepath.Join(f.root, "octocat/hello/main.go"),
		[]byte("package main\n\nfunc main() { _ = 1 }\n"), 0o644))
	second, err := f.svc.PrepareDelivery(ctx, prepareInput())
	require.NoError(t, err)
	require.NotEqual(t, first.Digest, second.Digest, "内容变化必须铸造新 digest")
	require.NoError(t, f.actions.Approve(ctx, second.ActionID, "u1", second.Digest))

	secondView, err := f.svc.DispatchDelivery(ctx, dispatchInput(second))
	require.NoError(t, err)
	require.Equal(t, string(DeliveryDelivered), secondView.State)
	require.NotEqual(t, firstCommit, secondView.CommitSHA)

	// fast-forward 事实链：分支现 head = 第二颗提交，且其 parent 是第一颗。
	head, ok := f.github.BranchCommit("weknora/task/s-1")
	require.True(t, ok)
	require.Equal(t, secondView.CommitSHA, head)
	require.Equal(t, []string{firstCommit}, f.github.ParentOf(secondView.CommitSHA),
		"二次交付的提交必须以分支现 head 为 parent（force:false 只接受 ff）")

	// 草稿 PR 迭代复用同一 head 的既有 PR（更新语义），不重复开 PR。
	require.Equal(t, first.PRNumber, secondView.PRNumber)
	// Repository 远端读恰好 4 次：两次 prepare 各 1 + 两次派发各 1（PR 半程
	// 复用推送半程已读的默认分支，最终修复轮发现 5 去掉了重复读——修复前
	// 每次派发读 2 次，总计 6）。
	require.Equal(t, 4, f.github.Calls()["GET /repos"], "每次 prepare/派发只读一次 Repository")
	require.Empty(t, f.github.Violations())
}

// 前置门拒绝自愈（最终修复轮发现 2）：派发期凭据解析失败 =
// ErrDispatchNotStarted → A03 落 action=failed 且 Execute 返回 nil。交付行
// 必须跟随落 failed——修复前它永久滞留 dispatched（再派发被 default 拒、
// resolve 因 action 非 unknown 被拒，无自愈路径）。
func TestPreSendGateFailureSettlesDeliveryFailedNotStranded(t *testing.T) {
	f := seededFixture(t)
	ctx := context.Background()
	breakCreds(f) // 准备已获批；此后凭据行「被删」→ 仅派发期 tokenFor 失败

	_, err := f.svc.DispatchDelivery(ctx, dispatchInput(firstDelivery(t, f)))
	require.ErrorIs(t, err, ErrDeliveryDispatchRejected)

	// 交付行已诚实落账 failed（含失败原因），不再是滞留的 dispatched。
	view, gerr := f.svc.GetDelivery(ctx, 7, firstDelivery(t, f).ID)
	require.NoError(t, gerr)
	require.Equal(t, string(DeliveryFailed), view.State)
	require.NotEmpty(t, view.Failure)
	// A03 侧一致：action 行 = failed（"dispatch rejected before send"）。
	var row appconnectorrepo.ActionRow
	require.NoError(t, f.db.Where("id = ?", view.ActionID).First(&row).Error)
	require.Equal(t, "failed", row.State)

	// 零远端调用：拒绝发生在任何出网之前（GET /repos//branches 各 1 次是
	// prepare 阶段的既有读；派发期新增为 0——派发期专属的 GET /git/ref 为 0）。
	require.Zero(t, f.github.Calls()["GET /git/ref"])
	require.Zero(t, f.github.Calls()["POST /git/blobs"])
	require.Zero(t, f.github.Calls()["POST /git/refs"])

	// 滞留态的两个死路现在是干净的终态拒绝：再派发 → 状态冲突；resolve →
	// action 非 unknown 被拒。行不再「卡死在中间态无声无息」。
	_, err = f.svc.DispatchDelivery(ctx, dispatchInput(view))
	require.ErrorIs(t, err, ErrDeliveryState)
	_, err = f.svc.ResolveDeliveryUnknown(ctx, dispatchInput(view))
	require.Error(t, err)
}
