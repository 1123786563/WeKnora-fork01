package codedelivery

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	appconnector "github.com/Tencent/WeKnora/internal/modules/appconnector"
	appconnectorsvc "github.com/Tencent/WeKnora/internal/modules/appconnector/service/appconnector"
	deliveryrepo "github.com/Tencent/WeKnora/internal/modules/codedelivery/repository/codedelivery"
)

// DispatcherDeps wires the outbound delivery chain. Guard re-runs on every
// dispatch and recovery (A02: the persisted subject may still use the
// connection); ActionRows re-reads the approved snapshot.
type DispatcherDeps struct {
	Connections ConnectionReader
	Creds       appconnectorsvc.CredentialResolver
	Guard       appconnectorsvc.A02Guard
	GitHub      GitHubClientFactory
	Workspace   WorkspaceFileSource
	Store       *deliveryrepo.DeliveryStore
	ActionRows  appconnectorsvc.ActionStoreSource
	Runs        RunReader
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
func (d *DeliveryDispatcher) Dispatch(ctx context.Context, snap appconnectorsvc.ActionSnapshot, providerKey string) (appconnectorsvc.DispatchOutcome, error) {
	material, err := ParseDeliveryMaterial(snap.Args)
	if err != nil {
		return appconnectorsvc.DispatchOutcome{}, fmt.Errorf("%w: %v", appconnectorsvc.ErrDispatchNotStarted, err)
	}
	if err := d.deps.Guard.Check(ctx,
		appconnector.OCSubject{TenantID: snap.TenantID, ActorID: snap.ActorID},
		snap.ConnectionID, snap.AuthVersion,
	); err != nil {
		return appconnectorsvc.DispatchOutcome{}, fmt.Errorf("%w: a02: %v", appconnectorsvc.ErrDispatchNotStarted, err)
	}
	token, err := d.tokenFor(ctx, snap)
	if err != nil {
		return appconnectorsvc.DispatchOutcome{}, fmt.Errorf("%w: credential: %v", appconnectorsvc.ErrDispatchNotStarted, err)
	}
	row, err := d.findByAction(ctx, snap.TenantID, snap.ID)
	if err != nil {
		return appconnectorsvc.DispatchOutcome{}, fmt.Errorf("%w: delivery row: %v", appconnectorsvc.ErrDispatchNotStarted, err)
	}
	client := d.deps.GitHub(token, material.Repo)
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
		appconnector.OCSubject{TenantID: snap.TenantID, ActorID: snap.ActorID},
		snap.ConnectionID, snap.AuthVersion,
	); err != nil {
		return err
	}
	token, err := d.tokenFor(ctx, snap)
	if err != nil {
		return err
	}
	_, err = d.deliver(ctx, snap, material, row, d.deps.GitHub(token, material.Repo), true)
	return err
}

// deliver runs the delivery chain. partialRecovery=true skips the push half
// entirely — it already happened under the SAME approval.
func (d *DeliveryDispatcher) deliver(ctx context.Context, snap appconnectorsvc.ActionSnapshot, material DeliveryMaterial, row deliveryrepo.DeliveryRow, client GitHubClient, partialRecovery bool) (appconnectorsvc.DispatchOutcome, error) {
	if !partialRecovery {
		info, err := client.Repository(ctx)
		if err != nil {
			return appconnectorsvc.DispatchOutcome{}, err
		}
		protected, err := client.BranchProtected(ctx, material.Branch)
		if err != nil {
			return appconnectorsvc.DispatchOutcome{}, err
		}
		// AC1 双保险：派发前复验目标分支不是默认分支/未被远端标记保护。
		if err := RefuseProtectedTarget(material.Branch, info.DefaultBranch, protected); err != nil {
			return appconnectorsvc.DispatchOutcome{}, fmt.Errorf("%w: %v", appconnectorsvc.ErrDispatchNotStarted, err)
		}
		// 内容从会话工作区读取（令牌只留在服务端，永不进沙箱）。
		run, err := d.deps.Runs.GetOwnedRun(ctx, snap.TenantID, snap.ActorID, row.RunID)
		if err != nil || run.SessionID == "" {
			return appconnectorsvc.DispatchOutcome{}, fmt.Errorf("%w: run %s", appconnectorsvc.ErrDispatchNotStarted, row.RunID)
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
				return appconnectorsvc.DispatchOutcome{}, fmt.Errorf("%w: workspace read %s: %v", appconnectorsvc.ErrDispatchNotStarted, change.Path, rerr)
			}
			blobSHA, berr := client.CreateBlob(ctx, content)
			if berr != nil {
				return appconnectorsvc.DispatchOutcome{}, berr
			}
			entries = append(entries, TreeEntry{Path: change.Path, SHA: blobSHA})
		}
		baseTree, terr := client.CommitTree(ctx, material.BaselineSHA)
		if terr != nil {
			return appconnectorsvc.DispatchOutcome{}, terr
		}
		treeSHA, trerr := client.CreateTree(ctx, baseTree, entries)
		if trerr != nil {
			return appconnectorsvc.DispatchOutcome{}, trerr
		}
		commitSHA, cerr := client.CreateCommit(ctx, material.BaselineSHA, treeSHA, material.CommitMessage)
		if cerr != nil {
			return appconnectorsvc.DispatchOutcome{}, cerr
		}
		if err := d.deps.Store.RecordReceipts(ctx, snap.TenantID, row.ID, deliveryrepo.ReceiptUpdate{CommitSHA: commitSHA}); err != nil {
			return appconnectorsvc.DispatchOutcome{}, err
		}
		if err := client.EnsureBranch(ctx, material.Branch, commitSHA); err != nil {
			return appconnectorsvc.DispatchOutcome{}, err
		}
		if err := d.deps.Store.TransitionState(ctx, snap.TenantID, row.ID,
			[]string{string(DeliveryDispatched), string(DeliveryPrepared), string(DeliveryUnknown)}, string(DeliveryPushed), ""); err != nil {
			return appconnectorsvc.DispatchOutcome{}, err
		}
	}
	// —— PR 半程 ——（恢复路径只走这里）
	repoInfo, err := client.Repository(ctx)
	if err != nil {
		return appconnectorsvc.DispatchOutcome{}, err
	}
	receipt, prerr := client.DraftPullRequest(ctx, PullRequestInput{
		Title: material.PRTitle, Head: material.Repo.Owner + ":" + material.Branch, Base: repoInfo.DefaultBranch,
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
				return appconnectorsvc.DispatchOutcome{Status: appconnector.ActionSucceeded, ProviderResult: partialReceipt(current.CommitSHA)}, nil
			}
			return appconnectorsvc.DispatchOutcome{}, fmt.Errorf("%w: pr: %v", appconnectorsvc.ErrDispatchNotStarted, apiErr)
		}
		return appconnectorsvc.DispatchOutcome{}, prerr // 传输不可观测 → 上层落 unknown
	}
	if err := d.deps.Store.RecordReceipts(ctx, snap.TenantID, row.ID, deliveryrepo.ReceiptUpdate{
		PRNumber: receipt.Number, PRURL: receipt.URL, RemoteLogin: login,
	}); err != nil {
		return appconnectorsvc.DispatchOutcome{}, err
	}
	if err := d.deps.Store.TransitionState(ctx, snap.TenantID, row.ID,
		[]string{string(DeliveryPushed), string(DeliveryPrepared), string(DeliveryDispatched), string(DeliveryUnknown)},
		string(DeliveryDelivered), ""); err != nil {
		return appconnectorsvc.DispatchOutcome{}, err
	}
	return appconnectorsvc.DispatchOutcome{
		Status:         appconnector.ActionSucceeded,
		ProviderResult: deliveredReceipt(receipt.Number, receipt.URL, login),
	}, nil
}

// QueryProvider resolves an unknown delivery from REMOTE FACTS only: a draft
// PR for the task head → delivered; otherwise the task branch ref → pushed.
// It never re-sends anything.
func (d *DeliveryDispatcher) QueryProvider(ctx context.Context, snap appconnectorsvc.ActionSnapshot, providerKey string) (appconnectorsvc.DispatchOutcome, error) {
	material, err := ParseDeliveryMaterial(snap.Args)
	if err != nil {
		return appconnectorsvc.DispatchOutcome{}, err
	}
	row, err := d.findByAction(ctx, snap.TenantID, snap.ID)
	if err != nil {
		return appconnectorsvc.DispatchOutcome{}, err
	}
	token, err := d.tokenFor(ctx, snap)
	if err != nil {
		return appconnectorsvc.DispatchOutcome{}, err
	}
	client := d.deps.GitHub(token, material.Repo)
	head := material.Repo.Owner + ":" + material.Branch
	if receipt, rerr := client.PullRequestForHead(ctx, head); rerr == nil && receipt != nil {
		if err := d.deps.Store.RecordReceipts(ctx, snap.TenantID, row.ID, deliveryrepo.ReceiptUpdate{
			PRNumber: receipt.Number, PRURL: receipt.URL,
		}); err != nil {
			return appconnectorsvc.DispatchOutcome{}, err
		}
		if err := d.deps.Store.TransitionState(ctx, snap.TenantID, row.ID,
			[]string{string(DeliveryUnknown)}, string(DeliveryDelivered), ""); err != nil {
			return appconnectorsvc.DispatchOutcome{}, err
		}
		return appconnectorsvc.DispatchOutcome{Status: appconnector.ActionSucceeded, ProviderResult: "resolved: draft PR exists"}, nil
	}
	if sha, exists, berr := client.BranchHead(ctx, material.Branch); berr == nil && exists {
		if err := d.deps.Store.RecordReceipts(ctx, snap.TenantID, row.ID, deliveryrepo.ReceiptUpdate{CommitSHA: sha}); err != nil {
			return appconnectorsvc.DispatchOutcome{}, err
		}
		if err := d.deps.Store.TransitionState(ctx, snap.TenantID, row.ID,
			[]string{string(DeliveryUnknown)}, string(DeliveryPushed), ""); err != nil {
			return appconnectorsvc.DispatchOutcome{}, err
		}
		return appconnectorsvc.DispatchOutcome{Status: appconnector.ActionSucceeded, ProviderResult: "resolved: branch pushed, draft PR absent"}, nil
	}
	return appconnectorsvc.DispatchOutcome{}, fmt.Errorf("%w: no remote fact for %s yet", appconnectorsvc.ErrDispatchUnknown, head)
}

func (d *DeliveryDispatcher) tokenFor(ctx context.Context, snap appconnectorsvc.ActionSnapshot) (string, error) {
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
	err := d.deps.Store.DB().WithContext(ctx).
		Where("tenant_id = ? AND action_id = ?", tenantID, actionID).
		Order("created_at DESC").First(&row).Error
	if err != nil {
		return deliveryrepo.DeliveryRow{}, deliveryrepo.ErrDeliveryNotFound
	}
	return row, nil
}

func (d *DeliveryDispatcher) snapshotOfDelivery(ctx context.Context, row deliveryrepo.DeliveryRow) (appconnectorsvc.ActionSnapshot, error) {
	actionRow, err := d.deps.ActionRows.FindAction(ctx, row.ActionID)
	if err != nil {
		return appconnectorsvc.ActionSnapshot{}, err
	}
	return appconnectorsvc.ActionSnapshot{
		ID: actionRow.ID, TenantID: actionRow.TenantID, ActorID: actionRow.ActorID,
		ConnectionID: actionRow.ConnectionID, Version: actionRow.AppVersion,
		Target: actionRow.Target, Risk: actionRow.Risk, Digest: actionRow.ArgsDigest,
		State: actionRow.State, Fence: actionRow.Fence, Args: []byte(actionRow.ArgsSnapshot),
		AuthVersion: actionRow.AuthVersion, DigestVersion: int(actionRow.DigestVersion),
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
