package codedelivery

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	agentruntime "github.com/Tencent/WeKnora/internal/modules/agentruntime/agent/runtime"
	appconnector "github.com/Tencent/WeKnora/internal/modules/appconnector"
	appconnectorsvc "github.com/Tencent/WeKnora/internal/modules/appconnector/service/appconnector"
	deliveryrepo "github.com/Tencent/WeKnora/internal/modules/codedelivery/repository/codedelivery"
	"github.com/google/uuid"
)

// Delivery target constant of every A03 action this module prepares.
const DeliveryActionTarget = "github.deliver"

// maxBaselineBytes bounds the materialized baseline payload. It must stay
// <= maxRawBodyBytes (gitlab_client.go) — the raw blob cap is the runaway
// guard for the blobs this ceiling admits, and must never refuse one.
const maxBaselineBytes = 16 << 20

// maxWorkspaceFileBytes bounds one workspace file considered for a diff.
const maxWorkspaceFileBytes = 4 << 20

var (
	// ErrNotDeliveryOwner: caller is not the run owner (initiator predicate).
	ErrNotDeliveryOwner = errors.New("code_delivery_not_owner")
	// ErrConnectionNotUsable: not a personal connection of the caller, or not active.
	ErrConnectionNotUsable = errors.New("code_delivery_connection_not_usable")
)

// RunReader mirrors session.OwnedRunReader (production: *repository.AgentRunStore).
type RunReader interface {
	GetOwnedRun(ctx context.Context, tenantID uint64, ownerID, runID string) (agentruntime.Run, error)
}

// ConnectionReader is the connection lookup the owner-only rule needs.
type ConnectionReader interface {
	FindConnectionByID(ctx context.Context, connectionID string) (appconnector.Connection, error)
}

// CodeDeliveryDeps wires the module. Actions is the DELIVERY-DEDICATED
// ActionService instance (its dispatcher becomes the DeliveryDispatcher in
// Task 6); the container builds it so the global OC dispatcher is untouched.
// Creds resolves the delivery token AFTER the permission chain — the bytes
// never enter any response, log, or the sandbox.
type CodeDeliveryDeps struct {
	Store       *deliveryrepo.DeliveryStore
	Actions     *appconnectorsvc.ActionService
	ActionRows  appconnectorsvc.ActionStoreSource
	Connections ConnectionReader
	Creds       appconnectorsvc.CredentialResolver
	// GitHub 与 GitLab 是统一 seam 背后的两个平台适配器（T24 #54）；缺失
	// 的适配器让对应平台在 prepare 面即 fail closed。
	GitHub GitHubClientFactory
	GitLab CodePlatformClientFactory
	// Providers 把连接解析到其安装 app id（=平台名）。生产实现是
	// appconnectorrepo.InstallationStore。提供者是服务端权威事实、绝不静默
	// 猜测：Providers nil ⇒ prepare 面对包括 GitHub 在内的一切连接 fail
	// closed（零值 =「无交付」现状，不是 GitHub-only 降级）。生产容器必须
	// 接线（container/code_delivery.go，Task 4）；未接线部署的交付功能整体
	// 不可用是本语义的有意结果（fail-closed 方向，回归测试钉死于
	// TestUnwiredProvidersFailsClosedEvenForGitHub）。
	Providers ProviderSource
	Workspace WorkspaceFileSource
	Runs      RunReader
	// Dispatcher executes approved deliveries and recovers pushed ones
	// (Task 6). Nil keeps the prepare-only wiring usable; a dispatch on a
	// pushed row without it fails closed.
	Dispatcher *DeliveryDispatcher
}

type CodeDeliveryService struct {
	deps CodeDeliveryDeps
}

func NewCodeDeliveryService(deps CodeDeliveryDeps) *CodeDeliveryService {
	return &CodeDeliveryService{deps: deps}
}

// BaselineInput names the fixed baseline to materialize.
type BaselineInput struct {
	TenantID     uint64
	CallerID     string
	RunID        string
	ConnectionID string
	Repo         RepoRef
	BaselineSHA  string
}

type BaselineReceipt struct {
	Files int
	Root  string
}

// MaterializeBaseline writes the fixed baseline tree into the run's session
// workspace (User Story 41: execution starts from a reproducible state).
// Owner-only: the caller must own the run AND the personal connection.
func (s *CodeDeliveryService) MaterializeBaseline(ctx context.Context, in BaselineInput) (BaselineReceipt, error) {
	sessionID, conn, err := s.authorize(ctx, in.TenantID, in.CallerID, in.RunID, in.ConnectionID)
	if err != nil {
		return BaselineReceipt{}, err
	}
	if !baselineSHALegal(in.BaselineSHA) {
		return BaselineReceipt{}, fmt.Errorf("%w: %q", ErrInvalidBaselineSHA, in.BaselineSHA)
	}
	provider, err := s.platformProvider(ctx, conn)
	if err != nil {
		return BaselineReceipt{}, err
	}
	token, err := s.tokenFor(ctx, in.ConnectionID)
	if err != nil {
		return BaselineReceipt{}, err
	}
	client, err := s.clientFor(provider, token, in.Repo)
	if err != nil {
		return BaselineReceipt{}, err
	}
	tree, err := s.baselineTree(ctx, client, in.BaselineSHA)
	if err != nil {
		return BaselineReceipt{}, err
	}
	if len(tree) > MaxDeliveryFiles {
		return BaselineReceipt{}, fmt.Errorf("%w: baseline has %d files", ErrBaselineTooLarge, len(tree))
	}
	root := WorkspaceRepoRoot(in.Repo)
	files := make([]WorkspaceFileWrite, 0, len(tree))
	total := 0
	for p, blobSHA := range tree {
		content, err := client.Blob(ctx, blobSHA)
		if err != nil {
			return BaselineReceipt{}, err
		}
		total += len(content)
		if total > maxBaselineBytes {
			return BaselineReceipt{}, fmt.Errorf("%w: baseline exceeds %d bytes", ErrBaselineTooLarge, maxBaselineBytes)
		}
		files = append(files, WorkspaceFileWrite{Path: root + "/" + p, Content: content})
	}
	if err := s.deps.Workspace.WriteSessionWorkspaceFiles(ctx, sessionID, files); err != nil {
		return BaselineReceipt{}, err
	}
	return BaselineReceipt{Files: len(files), Root: root}, nil
}

// PrepareInput anchors one delivery candidate. Branch empty → TaskBranchOf(taskID).
type PrepareInput struct {
	TenantID      uint64
	CallerID      string
	RunID         string
	ConnectionID  string
	Repo          RepoRef
	BaselineSHA   string
	Branch        string // 空 = TaskBranchOf(sessionID)
	CommitMessage string
	PRTitle       string
}

// PrepareDelivery computes the workspace diff against the fixed baseline,
// anchors it as an A03 action (risk=deliver → always awaiting_approval), and
// persists the traceability row. AC1 guardrails run BEFORE any GitHub write.
func (s *CodeDeliveryService) PrepareDelivery(ctx context.Context, in PrepareInput) (DeliveryView, error) {
	sessionID, conn, err := s.authorize(ctx, in.TenantID, in.CallerID, in.RunID, in.ConnectionID)
	if err != nil {
		return DeliveryView{}, err
	}
	if in.CommitMessage == "" || in.PRTitle == "" {
		return DeliveryView{}, fmt.Errorf("%w: commit_message and pr_title are required", ErrInvalidMaterial)
	}
	if !baselineSHALegal(in.BaselineSHA) {
		return DeliveryView{}, fmt.Errorf("%w: %q", ErrInvalidBaselineSHA, in.BaselineSHA)
	}
	provider, err := s.platformProvider(ctx, conn)
	if err != nil {
		return DeliveryView{}, err
	}
	token, terr := s.tokenFor(ctx, in.ConnectionID)
	if terr != nil {
		return DeliveryView{}, terr
	}
	client, err := s.clientFor(provider, token, in.Repo)
	if err != nil {
		return DeliveryView{}, err
	}
	// 护栏 1（顺序有意为之）：先判「目标是否撞默认分支/远端 protected」再验
	// 任务分支前缀白名单——任何形状的分支（包括误填 "main"）都必须先撞上
	// 保护分支拒绝（AC1 的第一道闸），前缀白名单是第二道。
	if in.Branch == "" {
		in.Branch = TaskBranchOf(sessionID)
	}
	// 纯本地形状闸（先于任何远端读）：连合法 refname 都不是的分支字符串
	// 不应花费一次 provider 往返，也不应参与 URL 构建。默认分支/远端
	// protected 判定仍在其后——"main" 形状合法，仍先撞保护分支拒绝
	//（AC1 第一道闸次序不变）。
	if !branchShapeLegal(in.Branch) {
		return DeliveryView{}, fmt.Errorf("%w: %q is not a legal refname", ErrInvalidBranch, in.Branch)
	}
	info, err := client.Repository(ctx)
	if err != nil {
		return DeliveryView{}, err
	}
	protected, err := client.BranchProtected(ctx, in.Branch)
	if err != nil {
		return DeliveryView{}, err
	}
	if err := RefuseProtectedTarget(in.Branch, info.DefaultBranch, protected); err != nil {
		return DeliveryView{}, err
	}
	if err := ValidateTaskBranch(in.Branch); err != nil {
		return DeliveryView{}, err
	}
	// 护栏 2：材料=工作区 diff（按真实 git blob sha 判定修改）。
	baselineTree, err := s.baselineTree(ctx, client, in.BaselineSHA)
	if err != nil {
		return DeliveryView{}, err
	}
	if len(baselineTree) > MaxDeliveryFiles {
		return DeliveryView{}, fmt.Errorf("%w: baseline has %d files", ErrBaselineTooLarge, len(baselineTree))
	}
	workspaceTree, err := s.workspaceTree(ctx, sessionID, in.Repo)
	if err != nil {
		return DeliveryView{}, err
	}
	changes := DiffAgainstBaseline(baselineTree, workspaceTree)
	if len(changes) == 0 {
		return DeliveryView{}, fmt.Errorf("%w: workspace matches the baseline; nothing to deliver", ErrInvalidMaterial)
	}
	material := DeliveryMaterial{
		Repo: in.Repo, BaselineSHA: in.BaselineSHA, Branch: in.Branch,
		Files: changes, CommitMessage: in.CommitMessage, PRTitle: in.PRTitle,
	}
	if len(material.Files) > MaxDeliveryFiles {
		return DeliveryView{}, fmt.Errorf("%w: %d changed files", ErrBaselineTooLarge, len(material.Files))
	}
	args, err := material.CanonicalJSON()
	if err != nil {
		return DeliveryView{}, err
	}
	// 锚定 A03：digest 绑定 repo/基线/分支/文件清单/提交信息/PR 标题 + 连接
	// 版本；内容变化=新 digest=旧批准失效（immutable approval anchor）。
	// conn 已由 authorize 装载（T24 #54：authorize 返回连接供提供者解析）。
	actionID, err := s.deps.Actions.Prepare(ctx, appconnector.Action{
		TenantID: in.TenantID, ActorID: in.CallerID, ConnectionID: in.ConnectionID,
		Target: DeliveryTargetOf(provider), Risk: appconnector.RiskDeliver,
		AuthVersion: conn.AuthVersion, Args: args,
	})
	if err != nil {
		return DeliveryView{}, err
	}
	// 结构性断言：交付动作必须生而 awaiting_approval（deliver 不在任何预授
	// 权白名单内；即便有人造出 deliver 预授权也在此 fail closed）。
	row, err := s.deps.ActionRows.FindAction(ctx, actionID)
	if err != nil {
		return DeliveryView{}, err
	}
	if row.State != appconnector.ActionAwaitingApproval {
		return DeliveryView{}, fmt.Errorf("%w: delivery action must await approval, got %s", ErrInvalidMaterial, row.State)
	}
	drow := deliveryrepo.DeliveryRow{
		ID: "dlv_" + uuid.NewString(), TenantID: in.TenantID,
		TaskID: sessionID, RunID: in.RunID, OwnerID: in.CallerID,
		ActionID: actionID, ConnectionID: in.ConnectionID,
		Repo: in.Repo.String(), BaselineSHA: in.BaselineSHA, Branch: in.Branch,
		State: string(DeliveryPrepared),
	}
	if err := s.deps.Store.CreateDelivery(ctx, drow); err != nil {
		return DeliveryView{}, err
	}
	return s.viewOf(ctx, drow)
}

// GetDeliveryForRun returns the latest delivery of a run (read face).
func (s *CodeDeliveryService) GetDeliveryForRun(ctx context.Context, tenantID uint64, runID string) (DeliveryView, error) {
	row, err := s.deps.Store.LatestForRun(ctx, tenantID, runID)
	if err != nil {
		return DeliveryView{}, err
	}
	return s.viewOf(ctx, row)
}

// GetDelivery returns one delivery by id, tenant-scoped (read face; the
// signature is frozen here, Task 6 reuses it after state transitions).
func (s *CodeDeliveryService) GetDelivery(ctx context.Context, tenantID uint64, deliveryID string) (DeliveryView, error) {
	row, err := s.deps.Store.GetDelivery(ctx, tenantID, deliveryID)
	if err != nil {
		return DeliveryView{}, err
	}
	return s.viewOf(ctx, row)
}

// DispatchInput addresses one delivery. CallerID must be the run owner.
type DispatchInput struct {
	TenantID            uint64
	CallerID            string
	RunID               string
	DeliveryID          string
	ConfirmNoMatchingPR bool
}

// ErrDeliveryState guards the delivery state machine at the service seam.
var ErrDeliveryState = errors.New("code_delivery_state_conflict")
var ErrDeliveryConfirmationRequired = errors.New("code_delivery_owner_confirmation_required")

// ErrDeliveryDispatchRejected: the approved dispatch was refused BEFORE any
// remote call left the process (A03 settled the action failed via
// ErrDispatchNotStarted — snapshot drift, A02, credential or workspace-read
// gate). The delivery row settles failed with the action; the approval is
// consumed, so a retry goes through a fresh Prepare.
var ErrDeliveryDispatchRejected = errors.New("code_delivery_dispatch_rejected")

// DispatchDelivery executes the approved delivery. prepared → consume the
// approval through the dedicated A03 instance; pushed → PR-only recovery
// under the SAME approval (never re-push); unknown → provider query only.
func (s *CodeDeliveryService) DispatchDelivery(ctx context.Context, in DispatchInput) (DeliveryView, error) {
	if _, err := s.deps.Runs.GetOwnedRun(ctx, in.TenantID, in.CallerID, in.RunID); err != nil {
		return DeliveryView{}, ErrNotDeliveryOwner
	}
	row, err := s.deps.Store.GetDelivery(ctx, in.TenantID, in.DeliveryID)
	if err != nil {
		return DeliveryView{}, err
	}
	if row.RunID != in.RunID || row.OwnerID != in.CallerID {
		return DeliveryView{}, ErrNotDeliveryOwner
	}
	switch DeliveryState(row.State) {
	case DeliveryPrepared:
		if err := s.deps.Store.TransitionState(ctx, in.TenantID, row.ID,
			[]string{string(DeliveryPrepared)}, string(DeliveryDispatched), ""); err != nil {
			return DeliveryView{}, err
		}
		if err := s.deps.Actions.Execute(ctx, row.ActionID); err != nil {
			if errors.Is(err, appconnectorsvc.ErrDispatchUnknown) {
				_ = s.deps.Store.TransitionState(ctx, in.TenantID, row.ID,
					[]string{string(DeliveryDispatched), string(DeliveryPushed)}, string(DeliveryUnknown), "")
				return s.viewAfter(ctx, in, row.ID)
			}
			after, gerr := s.deps.Store.GetDelivery(ctx, in.TenantID, row.ID)
			if gerr == nil && after.State == string(DeliveryPushed) {
				// 部分完成已由 dispatcher 落账：等待 PR-only 恢复，不算失败。
				return s.viewOf(ctx, after)
			}
			_ = s.deps.Store.TransitionState(ctx, in.TenantID, row.ID,
				[]string{string(DeliveryDispatched), string(DeliveryPrepared)}, string(DeliveryFailed), err.Error())
			return DeliveryView{}, err
		}
		// Execute 返回 nil 不等于成功：settleOutcome 会把「可证未出网的
		// 前置门拒绝」（ErrDispatchNotStarted——快照损坏/A02/凭据/工作区
		// 读失败）落成 action=failed 且不再上抛。交付行必须跟随落
		// failed，否则永久滞留 dispatched（再派发被 default 拒、resolve
		// 因 action 非 unknown 被拒，无自愈路径）。
		action, gerr := s.deps.ActionRows.FindAction(ctx, row.ActionID)
		if gerr != nil {
			return DeliveryView{}, gerr
		}
		if action.State == appconnector.ActionFailed {
			failure := action.ProviderResult
			if failure == "" {
				failure = "dispatch rejected before send"
			}
			_ = s.deps.Store.TransitionState(ctx, in.TenantID, row.ID,
				[]string{string(DeliveryDispatched), string(DeliveryPrepared)}, string(DeliveryFailed), failure)
			return DeliveryView{}, fmt.Errorf("%w: %s", ErrDeliveryDispatchRejected, failure)
		}
	case DeliveryPushed:
		// CAS claim is the cross-process recovery linearization point.
		if err := s.deps.Store.TransitionState(ctx, in.TenantID, row.ID,
			[]string{string(DeliveryPushed)}, string(DeliveryDispatched), ""); err != nil {
			return DeliveryView{}, err
		}
		// dispatched → PR-only recovery; the dispatcher never repeats push steps.
		if s.deps.Dispatcher == nil {
			claimErr := fmt.Errorf("%w: dispatcher not wired", ErrDeliveryState)
			if settleErr := s.settleRecoveryClaim(ctx, in, row.ID, DeliveryPushed, claimErr); settleErr != nil {
				return DeliveryView{}, errors.Join(claimErr, settleErr)
			}
			return DeliveryView{}, claimErr
		}
		if err := s.deps.Dispatcher.RecoverPullRequest(ctx, in.TenantID, row.ID); err != nil {
			var rejected *PRWriteRejectedError
			var notStarted *RecoveryNotStartedError
			next := DeliveryUnknown
			if errors.As(err, &rejected) || errors.As(err, &notStarted) {
				next = DeliveryPushed
			}
			if settleErr := s.settleRecoveryClaim(ctx, in, row.ID, next, err); settleErr != nil {
				return DeliveryView{}, errors.Join(err, settleErr)
			}
			return DeliveryView{}, err
		}
	default:
		return DeliveryView{}, fmt.Errorf("%w: %s", ErrDeliveryState, row.State)
	}
	return s.viewAfter(ctx, in, row.ID)
}

// settleRecoveryClaim must outlive the request: a canceled HTTP context after
// a possible provider write cannot be allowed to strand the durable claim.
// The bounded detached context is only used for the local CAS settlement.
func (s *CodeDeliveryService) settleRecoveryClaim(ctx context.Context, in DispatchInput, id string, to DeliveryState, cause error) error {
	settleCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()
	if err := s.deps.Store.TransitionState(settleCtx, in.TenantID, id,
		[]string{string(DeliveryDispatched)}, string(to), cause.Error()); err != nil {
		return fmt.Errorf("settle interrupted recovery claim to %s: %w", to, err)
	}
	return nil
}

// ResolveDeliveryUnknown settles an unknown delivery from remote facts only.
func (s *CodeDeliveryService) ResolveDeliveryUnknown(ctx context.Context, in DispatchInput) (DeliveryView, error) {
	if _, err := s.deps.Runs.GetOwnedRun(ctx, in.TenantID, in.CallerID, in.RunID); err != nil {
		return DeliveryView{}, ErrNotDeliveryOwner
	}
	row, err := s.deps.Store.GetDelivery(ctx, in.TenantID, in.DeliveryID)
	if err != nil {
		return DeliveryView{}, err
	}
	if row.RunID != in.RunID || row.OwnerID != in.CallerID {
		return DeliveryView{}, ErrNotDeliveryOwner
	}
	if row.State != string(DeliveryUnknown) && row.State != string(DeliveryDispatched) {
		return DeliveryView{}, fmt.Errorf("%w: resolve from %s", ErrDeliveryState, row.State)
	}
	action, err := s.deps.ActionRows.FindAction(ctx, row.ActionID)
	if err != nil {
		return DeliveryView{}, err
	}
	if action.State == appconnector.ActionUnknown {
		if err := s.deps.Actions.ResolveUnknownConfirmed(ctx, row.ActionID, in.ConfirmNoMatchingPR); err != nil {
			return DeliveryView{}, err
		}
		action, err = s.deps.ActionRows.FindAction(ctx, row.ActionID)
		if err != nil {
			return DeliveryView{}, err
		}
	}
	if action.State == appconnector.ActionSucceeded {
		if s.deps.Dispatcher == nil {
			return DeliveryView{}, fmt.Errorf("%w: dispatcher not wired", ErrDeliveryState)
		}
		snap, err := s.deps.Dispatcher.snapshotOfDelivery(ctx, row)
		if err != nil {
			return DeliveryView{}, err
		}
		if _, err = s.deps.Dispatcher.QueryProviderConfirmed(ctx, snap, "", in.ConfirmNoMatchingPR); err != nil {
			return DeliveryView{}, err
		}
	} else if action.State != appconnector.ActionUnknown {
		return DeliveryView{}, fmt.Errorf("%w: action state %s", ErrDeliveryState, action.State)
	}
	return s.viewAfter(ctx, in, row.ID)
}

func (s *CodeDeliveryService) viewAfter(ctx context.Context, in DispatchInput, deliveryID string) (DeliveryView, error) {
	row, err := s.deps.Store.GetDelivery(ctx, in.TenantID, deliveryID)
	if err != nil {
		return DeliveryView{}, err
	}
	return s.viewOf(ctx, row)
}

// DeliveryView is the wire/read projection carrying the full traceability
// chain: approval anchor (action/digest/approver) + remote receipts.
type DeliveryView struct {
	ID          string `json:"id"`
	TaskID      string `json:"task_id"`
	RunID       string `json:"run_id"`
	Initiator   string `json:"initiator"`
	State       string `json:"state"`
	Repo        string `json:"repo"`
	BaselineSHA string `json:"baseline_sha"`
	Branch      string `json:"branch"`
	CommitSHA   string `json:"commit_sha"`
	PRNumber    int64  `json:"pr_number"`
	PRURL       string `json:"pr_url"`
	RemoteLogin string `json:"remote_login"`
	ActionID    string `json:"action_id"`
	ActionState string `json:"action_state"`
	Digest      string `json:"digest"`
	Approver    string `json:"approver"`
	Failure     string `json:"failure"`
	Files       int    `json:"files"`
	CreatedAt   string `json:"created_at"`
	UpdatedAt   string `json:"updated_at"`
}

// viewOf joins the A03 action row (state/digest/approved material) and the
// approval actor onto one delivery row — the Task/Run read-back face
// (Issue #52 AC2). Join failures degrade the EXTRA fields only; the row's
// own facts always surface.
func (s *CodeDeliveryService) viewOf(ctx context.Context, row deliveryrepo.DeliveryRow) (DeliveryView, error) {
	view := DeliveryView{
		ID: row.ID, TaskID: row.TaskID, RunID: row.RunID, Initiator: row.OwnerID, State: row.State,
		Repo: row.Repo, BaselineSHA: row.BaselineSHA, Branch: row.Branch,
		CommitSHA: row.CommitSHA, PRNumber: row.PRNumber, PRURL: row.PRURL,
		RemoteLogin: row.RemoteLogin, ActionID: row.ActionID,
		Failure:   row.Failure,
		CreatedAt: row.CreatedAt.UTC().Format(time.RFC3339),
		UpdatedAt: row.UpdatedAt.UTC().Format(time.RFC3339),
	}
	if row.ActionID != "" {
		if action, err := s.deps.ActionRows.FindAction(ctx, row.ActionID); err == nil {
			view.ActionState = action.State
			view.Digest = action.ArgsDigest
			mat, merr := ParseDeliveryMaterial([]byte(action.ArgsSnapshot))
			if merr == nil {
				view.Files = len(mat.Files)
			}
		}
		if approver, ok, err := s.deps.Store.LatestApproverForAction(ctx, row.ActionID); err == nil && ok {
			view.Approver = approver
		}
	}
	return view, nil
}

// authorize is the shared owner-only predicate: caller owns the run AND uses
// their own personal connection (CONTEXT.md 个人连接只能由其所有者使用).
// It returns the run's sessionID (= taskID, ADR-0004) and the loaded
// connection (provider resolution needs its installation); the service keeps
// no mutable state.
func (s *CodeDeliveryService) authorize(ctx context.Context, tenantID uint64, callerID, runID, connectionID string) (string, appconnector.Connection, error) {
	run, err := s.deps.Runs.GetOwnedRun(ctx, tenantID, callerID, runID)
	if err != nil || run.SessionID == "" {
		return "", appconnector.Connection{}, fmt.Errorf("%w: run %s", ErrNotDeliveryOwner, runID)
	}
	conn, err := s.deps.Connections.FindConnectionByID(ctx, connectionID)
	if err != nil {
		return "", appconnector.Connection{}, err
	}
	if conn.Kind != appconnector.ConnectionKindPersonal || conn.OwnerID != callerID ||
		conn.TenantID != tenantID || conn.State != appconnector.ConnectionActive {
		return "", appconnector.Connection{}, ErrConnectionNotUsable
	}
	return run.SessionID, conn, nil
}

// platformProvider resolves the delivery platform from the connection's
// installation app id — server-side authority, never client input (T24 #54).
func (s *CodeDeliveryService) platformProvider(ctx context.Context, conn appconnector.Connection) (string, error) {
	if s.deps.Providers == nil {
		return "", fmt.Errorf("%w: provider source not wired", ErrUnsupportedProvider)
	}
	inst, err := s.deps.Providers.GetInstallationByID(ctx, conn.TenantID, conn.InstallationID)
	if err != nil {
		return "", err
	}
	if inst.AppID != ProviderGitHub && inst.AppID != ProviderGitLab {
		return "", fmt.Errorf("%w: connection app %q is not a code platform", ErrUnsupportedProvider, inst.AppID)
	}
	return inst.AppID, nil
}

// clientFor picks the platform adapter hidden behind the unified seam.
func (s *CodeDeliveryService) clientFor(provider, token string, repo RepoRef) (CodePlatformClient, error) {
	return clientForPlatform(s.deps.GitHub, s.deps.GitLab, provider, token, repo)
}

// tokenFor resolves the delivery token AFTER the permission chain (the
// CredentialResolver contract: credentials are for the calling adapter only).
func (s *CodeDeliveryService) tokenFor(ctx context.Context, connectionID string) (string, error) {
	conn, err := s.deps.Connections.FindConnectionByID(ctx, connectionID)
	if err != nil {
		return "", err
	}
	raw, err := s.deps.Creds.Resolve(ctx, connectionID, conn.AuthVersion)
	if err != nil {
		return "", err
	}
	token := strings.TrimSpace(string(raw))
	if token == "" {
		return "", ErrConnectionNotUsable
	}
	return token, nil
}

// baselineTree resolves a baseline COMMIT sha to its path→blob-sha tree.
// The git-trees endpoint accepts tree shas only (real GitHub, and the wire
// contract pinned by github_wire_test.go:365), so the commit is dereferenced
// through CommitTree first (偏差 D1：计划稿直接把基线 commit sha 传给 Tree，
// 模拟器与真实 API 都会 404).
func (s *CodeDeliveryService) baselineTree(ctx context.Context, client GitHubClient, baselineSHA string) (map[string]string, error) {
	treeSHA, err := client.CommitTree(ctx, baselineSHA)
	if err != nil {
		return nil, err
	}
	return client.Tree(ctx, treeSHA)
}

// workspaceTree projects the session workspace repo root as path→git blob sha.
func (s *CodeDeliveryService) workspaceTree(ctx context.Context, sessionID string, repo RepoRef) (map[string]string, error) {
	root := WorkspaceRepoRoot(repo)
	entries, err := s.deps.Workspace.ListSessionFiles(ctx, sessionID, root)
	if err != nil {
		return nil, err
	}
	if len(entries) > MaxDeliveryFiles {
		return nil, fmt.Errorf("%w: workspace has %d files", ErrBaselineTooLarge, len(entries))
	}
	tree := make(map[string]string, len(entries))
	rootPrefix := strings.TrimSuffix(root, "/") + "/"
	for _, entry := range entries {
		if entry.IsDir {
			continue
		}
		// 会话沙箱返回以请求目录为前缀的路径（"/workspace/<owner>/<name>/…"）。
		rel := strings.TrimPrefix(entry.Path, rootPrefix)
		if rel == entry.Path { // outside the repo root
			continue
		}
		if entry.Size > maxWorkspaceFileBytes {
			return nil, fmt.Errorf("%w: %s is %d bytes", ErrBaselineTooLarge, rel, entry.Size)
		}
		content, err := s.deps.Workspace.ReadSessionFile(ctx, sessionID, entry.Path)
		if err != nil {
			return nil, err
		}
		tree[rel] = GitBlobSHA(content)
	}
	return tree, nil
}

func baselineSHALegal(sha string) bool {
	return len(sha) == 40 && strings.Trim(sha, "0123456789abcdef") == ""
}
