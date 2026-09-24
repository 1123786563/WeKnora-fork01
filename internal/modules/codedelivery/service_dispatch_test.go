package codedelivery

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

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
	view, err = f.svc.ResolveDeliveryUnknown(ctx, dispatchInput(view))
	require.NoError(t, err)
	require.Equal(t, string(DeliveryPushed), view.State, "远端事实：分支已推、PR 缺席 → 部分完成")

	view, err = f.svc.DispatchDelivery(ctx, dispatchInput(view)) // PR-only 恢复
	require.NoError(t, err)
	require.Equal(t, string(DeliveryDelivered), view.State)
	require.Empty(t, f.github.Violations())
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
