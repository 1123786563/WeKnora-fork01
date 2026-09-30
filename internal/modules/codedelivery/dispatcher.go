package codedelivery

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	deliveryrepo "github.com/Tencent/WeKnora/internal/modules/codedelivery/repository/codedelivery"
)

// DispatcherDeps wires the outbound delivery chain. Guard re-runs on every
// dispatch and recovery (A02: the persisted subject may still use the
// connection); ActionRows re-reads the approved snapshot.
type DispatcherDeps struct {
	Connections ConnectionReader
	Creds       CredentialResolver
	Guard       A02Guard
	GitHub      GitHubClientFactory
	GitLab      CodePlatformClientFactory
	Workspace   WorkspaceFileSource
	Store       *deliveryrepo.DeliveryStore
	ActionRows  ActionStoreSource
	Runs        RunReader
}

// clientForTarget routes the approved action's platform target to its
// adapter — the ONLY platform switch on the dispatch face (T24 #54).
func (d *DeliveryDispatcher) clientForTarget(target, token string, repo RepoRef) (CodePlatformClient, error) {
	provider, err := ProviderOfTarget(target)
	if err != nil {
		return nil, err
	}
	return clientForPlatform(d.deps.GitHub, d.deps.GitLab, provider, token, repo)
}

// DeliveryDispatcher implements the A03 ActionDispatcher and UnknownResolver
// for github.deliver actions. It is the ONLY outbound boundary: tokens
// resolve here, GitHub calls leave here, receipts land here. There is no
// merge path anywhere (AC1).
type DeliveryDispatcher struct{ deps DispatcherDeps }

func NewDeliveryDispatcher(deps DispatcherDeps) *DeliveryDispatcher {
	return &DeliveryDispatcher{deps: deps}
}

// Dispatch performs the approved delivery. Pre-send gates (snapshot parse,
// A02, credential) fail with ErrDispatchNotStarted; a GitHub response is a
// definite outcome; a transport error is unobservable and bubbles up as
// ErrGitHubTransport (the service parks unknown). Push-succeeded with a
// definite PR failure is the recorded PARTIAL completion `pushed`.
func (d *DeliveryDispatcher) Dispatch(ctx context.Context, snap ActionSnapshot, providerKey string) (DispatchOutcome, error) {
	material, err := ParseDeliveryMaterial(snap.Args)
	if err != nil {
		return DispatchOutcome{}, fmt.Errorf("%w: %v", ErrDispatchNotStarted, err)
	}
	if err := d.deps.Guard.Check(ctx,
		ActionSubject{TenantID: snap.TenantID, ActorID: snap.ActorID},
		snap.ConnectionID, snap.AuthVersion,
	); err != nil {
		return DispatchOutcome{}, fmt.Errorf("%w: a02: %v", ErrDispatchNotStarted, err)
	}
	token, err := d.tokenFor(ctx, snap)
	if err != nil {
		return DispatchOutcome{}, fmt.Errorf("%w: credential: %v", ErrDispatchNotStarted, err)
	}
	// 平台路由是前置门（T24 #54）：未知目标/未接线适配器在此拒绝，零远端调用。
	client, err := d.clientForTarget(snap.Target, token, material.Repo)
	if err != nil {
		return DispatchOutcome{}, fmt.Errorf("%w: %v", ErrDispatchNotStarted, err)
	}
	row, err := d.findByAction(ctx, snap.TenantID, snap.ID)
	if err != nil {
		return DispatchOutcome{}, fmt.Errorf("%w: delivery row: %v", ErrDispatchNotStarted, err)
	}
	return d.deliver(ctx, snap, material, row, client, false)
}

// RecoverPullRequest completes the PR half of a PARTIAL delivery (state
// pushed) under the SAME approval: A02 re-check, then PR-only. It never
// re-sends blobs/tree/commit/ref (CONTEXT.md 代码交付避免项).
func (d *DeliveryDispatcher) RecoverPullRequest(ctx context.Context, tenantID uint64, deliveryID string) error {
	row, err := d.deps.Store.GetDelivery(ctx, tenantID, deliveryID)
	if err != nil {
		return err
	}
	if row.State != string(DeliveryPushed) {
		return fmt.Errorf("%w: recovery from %s (resolve unknown first)", ErrDeliveryState, row.State)
	}
	snap, err := d.snapshotOfDelivery(ctx, row)
	if err != nil {
		return err
	}
	material, err := ParseDeliveryMaterial(snap.Args)
	if err != nil {
		return err
	}
	if err := d.deps.Guard.Check(ctx,
		ActionSubject{TenantID: snap.TenantID, ActorID: snap.ActorID},
		snap.ConnectionID, snap.AuthVersion,
	); err != nil {
		return err
	}
	token, err := d.tokenFor(ctx, snap)
	if err != nil {
		return err
	}
	client, err := d.clientForTarget(snap.Target, token, material.Repo)
	if err != nil {
		return err
	}
	_, err = d.deliver(ctx, snap, material, row, client, true)
	return err
}

// deliver runs the delivery chain. partialRecovery=true skips the push half
// entirely — it already happened under the SAME approval.
func (d *DeliveryDispatcher) deliver(ctx context.Context, snap ActionSnapshot, material DeliveryMaterial, row deliveryrepo.DeliveryRow, client GitHubClient, partialRecovery bool) (DispatchOutcome, error) {
	// 默认分支只读一次：推送半程已取（护栏 1）时 PR 半程直接复用，只有
	// 恢复半程（跳过了推送半程）才自取（最终修复轮：去掉重复远端读）。
	defaultBranch := ""
	if !partialRecovery {
		info, err := client.Repository(ctx)
		if err != nil {
			return DispatchOutcome{}, err
		}
		defaultBranch = info.DefaultBranch
		protected, err := client.BranchProtected(ctx, material.Branch)
		if err != nil {
			return DispatchOutcome{}, err
		}
		// AC1 双保险：派发前复验目标分支不是默认分支/未被远端标记保护。
		if err := RefuseProtectedTarget(material.Branch, defaultBranch, protected); err != nil {
			return DispatchOutcome{}, fmt.Errorf("%w: %v", ErrDispatchNotStarted, err)
		}
		// 内容从会话工作区读取（令牌只留在服务端，永不进沙箱）。
		run, err := d.deps.Runs.GetOwnedRun(ctx, snap.TenantID, snap.ActorID, row.RunID)
		if err != nil || run.SessionID == "" {
			return DispatchOutcome{}, fmt.Errorf("%w: run %s", ErrDispatchNotStarted, row.RunID)
		}
		// 同任务分支迭代语义（CONTEXT.md「创建或更新草稿 PR」）：分支已
		// 存在时新提交必须以分支现 head 为 parent——GitHub 的 force:false
		// ref 更新只接受 fast-forward，恒以 BaselineSHA 为 parent 的第二颗
		// 提交与分支现 head 互不为后代，真实 GitHub 会 422 拒绝。
		parent := material.BaselineSHA
		head, exists, herr := client.BranchHead(ctx, material.Branch)
		if herr != nil {
			return DispatchOutcome{}, herr
		}
		if exists {
			parent = head
		}
		root := WorkspaceRepoRoot(material.Repo)
		entries := make([]TreeEntry, 0, len(material.Files))
		for _, change := range material.Files {
			if change.Deleted {
				entries = append(entries, TreeEntry{Path: change.Path})
				continue
			}
			content, rerr := d.deps.Workspace.ReadSessionFile(ctx, run.SessionID, root+"/"+change.Path)
			if rerr != nil {
				return DispatchOutcome{}, fmt.Errorf("%w: workspace read %s: %v", ErrDispatchNotStarted, change.Path, rerr)
			}
			blobSHA, berr := client.CreateBlob(ctx, content)
			if berr != nil {
				return DispatchOutcome{}, berr
			}
			entries = append(entries, TreeEntry{Path: change.Path, SHA: blobSHA})
		}
		baseTree, terr := client.CommitTree(ctx, material.BaselineSHA)
		if terr != nil {
			return DispatchOutcome{}, terr
		}
		treeSHA, trerr := client.CreateTree(ctx, baseTree, entries)
		if trerr != nil {
			return DispatchOutcome{}, trerr
		}
		commitSHA, cerr := client.CreateCommit(ctx, parent, treeSHA, material.CommitMessage)
		if cerr != nil {
			return DispatchOutcome{}, cerr
		}
		if err := client.EnsureBranch(ctx, material.Branch, commitSHA); err != nil {
			return DispatchOutcome{}, err
		}
		// 权威回执以远端为准（T24 #54）：推送后读回分支现 head——GitHub 上
		// 它等于 CreateCommit 的结果；GitLab 的 commits API 由服务端定 sha，
		// 本地占位值绝不进入台账。
		head, pushed, herr := client.BranchHead(ctx, material.Branch)
		if herr != nil {
			return DispatchOutcome{}, herr
		}
		if !pushed {
			return DispatchOutcome{}, fmt.Errorf("%w: branch %s absent after push", ErrGitHubTransport, material.Branch)
		}
		commitSHA = head
		if err := d.deps.Store.RecordReceipts(ctx, snap.TenantID, row.ID, deliveryrepo.ReceiptUpdate{CommitSHA: commitSHA}); err != nil {
			return DispatchOutcome{}, err
		}
		if err := d.deps.Store.TransitionState(ctx, snap.TenantID, row.ID,
			[]string{string(DeliveryDispatched), string(DeliveryPrepared), string(DeliveryUnknown)}, string(DeliveryPushed), ""); err != nil {
			return DispatchOutcome{}, err
		}
	}
	// —— PR 半程 ——（恢复路径只走这里；默认分支复用推送半程已读事实）
	base := defaultBranch
	if base == "" {
		repoInfo, err := client.Repository(ctx)
		if err != nil {
			return DispatchOutcome{}, err
		}
		base = repoInfo.DefaultBranch
	}
	receipt, prerr := client.DraftPullRequest(ctx, PullRequestInput{
		Title: material.PRTitle, Head: material.Repo.Owner + ":" + material.Branch, Base: base,
	})
	var login string
	if prerr == nil {
		login, prerr = client.CurrentLogin(ctx)
	}
	if prerr != nil {
		var apiErr *GitHubAPIError
		if errors.As(prerr, &apiErr) {
			// 确定性 PR 失败：若已推（pushed）保持部分完成可恢复；否则 failed。
			current, gerr := d.deps.Store.GetDelivery(ctx, snap.TenantID, row.ID)
			if gerr == nil && current.State == string(DeliveryPushed) {
				return DispatchOutcome{Status: "succeeded", ProviderResult: partialReceipt(current.CommitSHA)}, nil
			}
			return DispatchOutcome{}, fmt.Errorf("%w: pr: %v", ErrDispatchNotStarted, apiErr)
		}
		return DispatchOutcome{}, prerr // 传输不可观测 → 上层落 unknown
	}
	if err := d.deps.Store.RecordReceipts(ctx, snap.TenantID, row.ID, deliveryrepo.ReceiptUpdate{
		PRNumber: receipt.Number, PRURL: receipt.URL, RemoteLogin: login,
	}); err != nil {
		return DispatchOutcome{}, err
	}
	if err := d.deps.Store.TransitionState(ctx, snap.TenantID, row.ID,
		[]string{string(DeliveryPushed), string(DeliveryPrepared), string(DeliveryDispatched), string(DeliveryUnknown)},
		string(DeliveryDelivered), ""); err != nil {
		return DispatchOutcome{}, err
	}
	return DispatchOutcome{
		Status:         "succeeded",
		ProviderResult: deliveredReceipt(receipt.Number, receipt.URL, login),
	}, nil
}

// QueryProvider resolves an unknown delivery from REMOTE FACTS only: a draft
// PR for the task head → delivered; otherwise the task branch ref → pushed.
// It never re-sends anything.
func (d *DeliveryDispatcher) QueryProvider(ctx context.Context, snap ActionSnapshot, providerKey string) (DispatchOutcome, error) {
	material, err := ParseDeliveryMaterial(snap.Args)
	if err != nil {
		return DispatchOutcome{}, err
	}
	row, err := d.findByAction(ctx, snap.TenantID, snap.ID)
	if err != nil {
		return DispatchOutcome{}, err
	}
	token, err := d.tokenFor(ctx, snap)
	if err != nil {
		return DispatchOutcome{}, err
	}
	client, err := d.clientForTarget(snap.Target, token, material.Repo)
	if err != nil {
		return DispatchOutcome{}, fmt.Errorf("%w: %v", ErrDispatchUnknown, err)
	}
	// MR identity carries the target dimension (R5-F8): resolve the repo's
	// default branch — the same source of truth the dispatch half's PR leg
	// uses (client.Repository) — so a same-source foreign-target MR can
	// never be mistaken for this delivery's receipt.
	info, ierr := client.Repository(ctx)
	if ierr != nil {
		return DispatchOutcome{}, ierr
	}
	head := material.Repo.Owner + ":" + material.Branch
	if receipt, rerr := client.PullRequestForHead(ctx, head, info.DefaultBranch); rerr == nil && receipt != nil {
		if err := d.deps.Store.RecordReceipts(ctx, snap.TenantID, row.ID, deliveryrepo.ReceiptUpdate{
			PRNumber: receipt.Number, PRURL: receipt.URL,
		}); err != nil {
			return DispatchOutcome{}, err
		}
		if err := d.deps.Store.TransitionState(ctx, snap.TenantID, row.ID,
			[]string{string(DeliveryUnknown)}, string(DeliveryDelivered), ""); err != nil {
			return DispatchOutcome{}, err
		}
		return DispatchOutcome{Status: "succeeded", ProviderResult: "resolved: draft PR exists"}, nil
	}
	if sha, exists, berr := client.BranchHead(ctx, material.Branch); berr == nil && exists {
		if err := d.deps.Store.RecordReceipts(ctx, snap.TenantID, row.ID, deliveryrepo.ReceiptUpdate{CommitSHA: sha}); err != nil {
			return DispatchOutcome{}, err
		}
		if err := d.deps.Store.TransitionState(ctx, snap.TenantID, row.ID,
			[]string{string(DeliveryUnknown)}, string(DeliveryPushed), ""); err != nil {
			return DispatchOutcome{}, err
		}
		return DispatchOutcome{Status: "succeeded", ProviderResult: "resolved: branch pushed, draft PR absent"}, nil
	}
	return DispatchOutcome{}, fmt.Errorf("%w: no remote fact for %s yet", ErrDispatchUnknown, head)
}

func (d *DeliveryDispatcher) tokenFor(ctx context.Context, snap ActionSnapshot) (string, error) {
	raw, err := d.deps.Creds.Resolve(ctx, snap.ConnectionID, snap.AuthVersion)
	if err != nil {
		return "", err
	}
	if len(raw) == 0 {
		return "", ErrConnectionNotUsable
	}
	return string(raw), nil
}

func (d *DeliveryDispatcher) findByAction(ctx context.Context, tenantID uint64, actionID string) (deliveryrepo.DeliveryRow, error) {
	var row deliveryrepo.DeliveryRow
	// created_at 并列（sqlite DATETIME 秒级精度下可能发生）时以 id DESC
	// 决出全序——单一 created_at 排序的并列行次序未定，读面可能取错行。
	err := d.deps.Store.DB().WithContext(ctx).
		Where("tenant_id = ? AND action_id = ?", tenantID, actionID).
		Order("created_at DESC").Order("id DESC").First(&row).Error
	if err != nil {
		return deliveryrepo.DeliveryRow{}, deliveryrepo.ErrDeliveryNotFound
	}
	return row, nil
}

func (d *DeliveryDispatcher) snapshotOfDelivery(ctx context.Context, row deliveryrepo.DeliveryRow) (ActionSnapshot, error) {
	actionRow, err := d.deps.ActionRows.FindAction(ctx, row.ActionID)
	if err != nil {
		return ActionSnapshot{}, err
	}
	return ActionSnapshot{
		ID: actionRow.ID, TenantID: actionRow.TenantID, ActorID: actionRow.ActorID,
		ConnectionID: actionRow.ConnectionID, Target: actionRow.Target, Args: []byte(actionRow.ArgsSnapshot), AuthVersion: actionRow.AuthVersion,
	}, nil
}

func partialReceipt(commitSHA string) string {
	raw, _ := json.Marshal(map[string]any{"partial": "pr", "commit_sha": commitSHA})
	return string(raw)
}

func deliveredReceipt(prNumber int64, prURL, login string) string {
	raw, _ := json.Marshal(map[string]any{
		"pr_number": prNumber, "pr_url": prURL, "remote_login": login,
	})
	return string(raw)
}
