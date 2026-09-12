import { useEffect, useMemo, useState, type ReactNode } from 'react';
import type { WeKnoraClient } from '@weknora/api-client';
import { ApiError } from '@weknora/api-client';
import type { RefundView } from '@weknora/contracts';
import { createScopeController } from '@weknora/domain/scope';
import { scopedKey } from '@weknora/domain';
import { Button, Card, Status } from '@weknora/ui';
import { refundMessage } from './refund-state.ts';

export interface AdminListRow { id: string; label: string; detail: string; }
export interface CatalogDraft { version: number; label: string; immutablePreview: string; }
/** Backend projection of REAL platform-operator capability. */
export interface CommercialOperatorCapability { operator: boolean; }
/** The refund under review plus the expectedVersion the reviewer last reconciled. */
export interface RefundReviewTarget { refundId: string; expectedVersion: number; }

const BLOCKED_ENV_NOTE = '服务端接口未接入（blocked-env）：列表数据等待平台端点 wiring，未用假数据填充。';

function formatCny(amountFen: string): string {
  return `¥${(Number.parseInt(amountFen, 10) / 100).toFixed(2)}`;
}

function isVersionConflict(error: unknown): boolean {
  return error instanceof ApiError
    && (error.status === 409 || error.code === 'VERSION_CONFLICT' || error.code === 'VERSION_MISMATCH');
}

function isRefundTerminal(refund: RefundView): boolean {
  return refund.state === 'completed' || refund.state === 'failed_confirmed' || refund.state === 'not_created_confirmed';
}

type ReviewState =
  | { status: 'idle' }
  | { status: 'loading' }
  | { status: 'error'; message: string }
  | { status: 'conflict'; message: string }
  | { status: 'done'; refund: RefundView };

function AdminList({ rows, blockedNote }: { rows?: AdminListRow[]; blockedNote: string }): ReactNode {
  if (!rows) return <Status>{blockedNote}</Status>;
  if (rows.length === 0) return <Status>暂无数据。</Status>;
  return (
    <ul className="wk-list">
      {rows.map((row) => <li key={row.id}><strong>{row.label}</strong><span>{row.detail}</span></li>)}
    </ul>
  );
}

interface AdminCommercialPageProps {
  client: WeKnoraClient;
  scopeController: ReturnType<typeof createScopeController>;
  /**
   * Real operator capability from the backend projection. Rendering this page
   * only for operators is a UI affordance, NOT authorization: server-side
   * capability enforcement stays authoritative for every admin action.
   */
  capability: CommercialOperatorCapability;
  /** Refund under review; the host supplies the reconciled expectedVersion. */
  reviewTarget?: RefundReviewTarget;
  /** True when the viewer is the space owner who filed the request: self-approval is never offered. */
  viewerIsRequester?: boolean;
  /** Catalog drafts; publish is only enabled after previewing the immutable version. */
  catalogDrafts?: CatalogDraft[];
  /** Publish wiring supplied by the host; absent = endpoint not wired (blocked-env). */
  onPublishDraft?: (version: number) => void;
  authorizationAudit?: AdminListRow[];
  paidPendingFulfillment?: AdminListRow[];
  settlementDiffs?: AdminListRow[];
}

export function AdminCommercialPage({
  client,
  scopeController,
  capability,
  reviewTarget,
  viewerIsRequester,
  catalogDrafts,
  onPublishDraft,
  authorizationAudit,
  paidPendingFulfillment,
  settlementDiffs,
}: AdminCommercialPageProps) {
  const scope = scopeController.current();
  const queryKey = useMemo(() => scopedKey(scope.scope, 'commercial-admin'), [scope.scope]);
  const [refund, setRefund] = useState<RefundView | null>(null);
  const [review, setReview] = useState<ReviewState>({ status: 'idle' });
  const [reloadToken, setReloadToken] = useState(0);
  const [conflictedVersion, setConflictedVersion] = useState<number | null>(null);
  const [previewedVersion, setPreviewedVersion] = useState<number | null>(null);
  const [publishNote, setPublishNote] = useState('');

  // A version conflict is cleared only when the host supplies a newly
  // reconciled reviewTarget (refundId or expectedVersion change).
  useEffect(() => {
    setConflictedVersion(null);
    setReview({ status: 'idle' });
  }, [reviewTarget?.refundId, reviewTarget?.expectedVersion]);

  useEffect(() => {
    if (!reviewTarget) return undefined;
    let active = true;
    const currentScope = scopeController.current();
    setReview({ status: 'loading' });
    void client.commercial.getRefund(reviewTarget.refundId, currentScope.signal).then((value) => {
      if (active && scopeController.isCurrent(currentScope.scope)) {
        setRefund(value);
        setReview({ status: 'idle' });
      }
    }).catch((error: unknown) => {
      if (!active || currentScope.signal.aborted || !scopeController.isCurrent(currentScope.scope)) return;
      setReview({ status: 'error', message: error instanceof Error ? error.message : 'Unable to load refund for review' });
    });
    return () => { active = false; };
  }, [client, scopeController, reviewTarget?.refundId, reloadToken]);

  if (!capability.operator) {
    return (
      <main className="wk-page">
        <header className="wk-header">
          <div>
            <p className="wk-eyebrow">Commercial</p>
            <h1>平台商业运营</h1>
            <p className="wk-muted">Operator-only console</p>
          </div>
        </header>
        <Card>
          <Status tone="error">
            当前账号不具备平台运营能力（capability.operator=false），平台审核面板不渲染。隐藏入口不是授权：服务端能力校验才是权威。
          </Status>
        </Card>
      </main>
    );
  }

  const submitReview = (decision: 'approve' | 'reject'): void => {
    if (!reviewTarget) return;
    const currentScope = scopeController.current();
    setReview({ status: 'loading' });
    // Refresh the refundable projection BEFORE deciding, then review the SAME
    // refund id with the currently reconciled expectedVersion — never a new
    // business operation.
    void client.commercial.getRefund(reviewTarget.refundId, currentScope.signal).then((fresh) => {
      if (scopeController.isCurrent(currentScope.scope)) setRefund(fresh);
      if (isRefundTerminal(fresh)) {
        if (scopeController.isCurrent(currentScope.scope)) setReview({ status: 'done', refund: fresh });
        return null;
      }
      return client.commercial.reviewRefund(reviewTarget.refundId, decision, reviewTarget.expectedVersion, currentScope.signal).then((updated) => {
        if (scopeController.isCurrent(currentScope.scope)) {
          setRefund(updated);
          setReview({ status: 'done', refund: updated });
        }
      });
    }).catch((error: unknown) => {
      if (currentScope.signal.aborted || !scopeController.isCurrent(currentScope.scope)) return;
      if (isVersionConflict(error)) {
        setConflictedVersion(reviewTarget.expectedVersion);
        setReview({ status: 'conflict', message: '退款单版本已变化，请重新核对后再审批。' });
        setReloadToken((value) => value + 1);
        return;
      }
      setReview({ status: 'error', message: error instanceof Error ? error.message : 'Review request failed' });
    });
  };

  const reviewDisabled = review.status === 'loading' || conflictedVersion !== null;
  const previewedDraft = previewedVersion === null ? undefined : catalogDrafts?.find((draft) => draft.version === previewedVersion);

  return (
    <main className="wk-page">
      <header className="wk-header">
        <div>
          <p className="wk-eyebrow">Commercial</p>
          <h1>平台商业运营</h1>
          <p className="wk-muted">目录发布 · 授权审计 · 退款审核 · 履约与结算</p>
        </div>
      </header>
      <Card>
        <p className="wk-debug">scope key: {JSON.stringify(queryKey)}</p>

        <section aria-label="目录草稿与发布">
          <h2>目录草稿 / 发布</h2>
          {!catalogDrafts ? (
            <Status>{BLOCKED_ENV_NOTE}</Status>
          ) : (
            <>
              <ul className="wk-list">
                {catalogDrafts.map((draft) => (
                  <li key={draft.version}>
                    <strong>{draft.label}</strong>
                    <span>
                      v{draft.version}
                      <Button type="button" onClick={() => setPreviewedVersion(draft.version)}>预览不可变版本</Button>
                      <Button
                        type="button"
                        disabled={previewedVersion !== draft.version || !onPublishDraft}
                        onClick={() => {
                          if (previewedVersion !== draft.version || !onPublishDraft) return;
                          onPublishDraft(draft.version);
                          setPublishNote(`已提交发布版本 ${draft.version}（仅发布预览过的不可变版本）。`);
                        }}
                      >
                        发布 v{draft.version}
                      </Button>
                    </span>
                  </li>
                ))}
              </ul>
              {previewedDraft ? (
                <div>
                  <h3>不可变版本预览（v{previewedDraft.version}）</h3>
                  <p>{previewedDraft.immutablePreview}</p>
                  <p className="wk-muted">发布动作只对预览过的不可变版本开放；未预览前发布键保持禁用。</p>
                </div>
              ) : <p className="wk-muted">发布前必须先预览目标不可变版本。</p>}
              {!onPublishDraft ? <Status>发布端点未接入（blocked-env）：发布键禁用，等待平台端点 wiring。</Status> : null}
              {publishNote ? <Status>{publishNote}</Status> : null}
            </>
          )}
        </section>

        <section aria-label="授权审计">
          <h2>授权审计（Owner 委派账单管理员 / 未授权 Admin）</h2>
          <AdminList rows={authorizationAudit} blockedNote={BLOCKED_ENV_NOTE} />
        </section>

        <section aria-label="退款审核">
          <h2>退款审核（平台）</h2>
          {!reviewTarget ? (
            <Status>未指定审核对象（reviewTarget）：平台待审列表接口未接入（blocked-env），由宿主注入审核对象与核对版本。</Status>
          ) : (
            <>
              <ul className="wk-list">
                <li><strong>退款单号</strong><span>{reviewTarget.refundId}</span></li>
                <li><strong>当前状态</strong><span>{refund ? refundMessage(refund.state) : '加载中'}</span></li>
                <li><strong>核算可退本金</strong><span>{refund ? formatCny(refund.amount_fen) : '—'}</span></li>
                <li><strong>核算锁定额度</strong><span>{refund ? refund.locked_credits : '—'}</span></li>
                <li><strong>expectedVersion</strong><span>{reviewTarget.expectedVersion}</span></li>
              </ul>
              {viewerIsRequester ? (
                <Status tone="error">
                  不能自审自批：该退款由当前查看者（空间 Owner）发起，审核须由其他平台运营执行。UI 不提供自批入口；服务端强制为权威。
                </Status>
              ) : (
                <>
                  <Button type="button" disabled={review.status === 'loading'} onClick={() => setReloadToken((value) => value + 1)}>
                    重新核对（刷新可退核算）
                  </Button>
                  {' '}
                  <Button type="button" disabled={reviewDisabled} onClick={() => submitReview('approve')}>批准（按当前 expectedVersion）</Button>
                  {' '}
                  <Button type="button" disabled={reviewDisabled} onClick={() => submitReview('reject')}>拒绝（按当前 expectedVersion）</Button>
                </>
              )}
              {review.status === 'loading' ? <Status>核对/提交中…</Status> : null}
              {review.status === 'error' ? <Status tone="error">{review.message}</Status> : null}
              {review.status === 'conflict' ? (
                <Status tone="error">
                  版本冲突（expectedVersion {conflictedVersion} 已过期）：{review.message} 已自动重新拉取，请基于新核对版本重试。
                </Status>
              ) : null}
              {review.status === 'done' ? <Status>审核结果：{refundMessage(review.refund.state)}</Status> : null}
            </>
          )}
        </section>

        <section aria-label="paid 待履约">
          <h2>paid 待履约</h2>
          <AdminList rows={paidPendingFulfillment} blockedNote={BLOCKED_ENV_NOTE} />
        </section>

        <section aria-label="结算差异">
          <h2>结算差异</h2>
          <AdminList rows={settlementDiffs} blockedNote={BLOCKED_ENV_NOTE} />
        </section>
      </Card>
    </main>
  );
}
