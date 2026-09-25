import React from 'react';

// T11 (#128): sharing a restricted-source result is server authority this
// panel only projects. The server derives the restricted contribution from
// recorded evidence and reduces consent to a state; the panel never derives
// authority client-side and never renders original-source material.

export type CraftShareStatus = 'private' | 'consented' | 'declined';
export type CraftShareRole = 'owner' | 'collaborator' | 'viewer';
export type CraftShareDecisionKind = 'approved' | 'rejected' | 'unknown';

export interface CraftShareDecision {
  version_id: string;
  evidence_digest: string;
  owner_id: string;
  decision: CraftShareDecisionKind;
}

export interface CraftShareView {
  versionId: string;
  restricted: boolean;
  evidenceDigest: string;
  status: CraftShareStatus;
  decision: CraftShareDecision | null;
  expiresAt: string | null;
}

const shareStatuses: readonly CraftShareStatus[] = ['private', 'consented', 'declined'];
const shareDecisions: readonly CraftShareDecisionKind[] = ['approved', 'rejected', 'unknown'];

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === 'object' && value !== null;
}

function optionalString(value: unknown): string | null {
  return typeof value === 'string' && value.length > 0 ? value : null;
}

// projectShareView normalizes the wire payload into the typed view. The
// projection is fail-closed: malformed shapes are dropped, not repaired,
// and a decision that does not bind the CURRENT version and evidence
// digest is stale — it never projects as consented authority.
export function projectShareView(raw: unknown): CraftShareView | null {
  if (!isRecord(raw)) return null;
  const versionId = optionalString(raw.version_id);
  const evidenceDigest = optionalString(raw.evidence_digest);
  const status = raw.status;
  if (!versionId || !evidenceDigest) return null;
  if (typeof raw.restricted !== 'boolean') return null;
  if (typeof status !== 'string' || !shareStatuses.includes(status as CraftShareStatus)) return null;

  let decision: CraftShareDecision | null = null;
  if (isRecord(raw.decision)) {
    const d = raw.decision;
    const dVersion = optionalString(d.version_id);
    const dDigest = optionalString(d.evidence_digest);
    const dOwner = optionalString(d.owner_id);
    const dKind = d.decision;
    if (dVersion && dDigest && dOwner && typeof dKind === 'string' && shareDecisions.includes(dKind as CraftShareDecisionKind)) {
      // The binding guard mirrors the server rule: a decision of another
      // version or other evidence is history, never live authority.
      if (dVersion === versionId && dDigest === evidenceDigest) {
        decision = { version_id: dVersion, evidence_digest: dDigest, owner_id: dOwner, decision: dKind as CraftShareDecisionKind };
      }
    }
  }

  // If the wire claims consent without a binding decision the panel stays
  // private: authority is never inferred, only projected from a bound decision.
  const effectiveStatus: CraftShareStatus =
    status === 'consented' && decision?.decision !== 'approved' ? 'private' : (status as CraftShareStatus);

  return {
    versionId,
    restricted: raw.restricted,
    evidenceDigest,
    status: effectiveStatus,
    decision,
    expiresAt: optionalString(raw.expires_at),
  };
}

export interface CraftSharePanelProps {
  locale: 'zh' | 'en';
  role: CraftShareRole;
  view: CraftShareView;
  onDecide(decision: 'approved' | 'rejected'): void | Promise<void>;
  onRevoke(): void | Promise<void>;
}

interface CraftShareLabels {
  heading: string;
  version: string;
  digest: string;
  states: Record<CraftShareStatus, string>;
  restricted: string;
  unrestricted: string;
  confirm: string;
  decline: string;
  revoke: string;
  decidedBy: string;
  expires: string;
  awaiting: string;
}

function shareLabels(locale: 'zh' | 'en'): CraftShareLabels {
  if (locale === 'en') {
    return {
      heading: 'Share restricted-source result',
      version: 'Version',
      digest: 'Evidence digest',
      states: { private: 'Private', consented: 'Consented', declined: 'Declined' },
      restricted: 'This version derives from restricted (shared-library) sources. Sharing the result needs your explicit decision and never shares the originals.',
      unrestricted: 'This version derives from unrestricted sources only.',
      confirm: 'Confirm share',
      decline: 'Decline',
      revoke: 'Revoke consent',
      decidedBy: 'Decided by',
      expires: 'Expires',
      awaiting: 'Awaiting owner decision',
    };
  }
  return {
    heading: '共享受限来源结果',
    version: '版本',
    digest: '证据摘要',
    states: { private: '私有', consented: '已同意共享', declined: '已拒绝' },
    restricted: '该版本派生自受限（共享库）来源。共享结果需要你的明确决定，且不会共享原始来源。',
    unrestricted: '该版本仅派生自非受限来源。',
    confirm: '确认共享',
    decline: '拒绝共享',
    revoke: '撤回共享同意',
    decidedBy: '决定人',
    expires: '有效期至',
    awaiting: '等待所有者决定',
  };
}

// CraftSharePanel projects the server's consent summary. The owner sees the
// exact immutable version and evidence digest they are deciding on; every
// other member sees the state without controls. No original-source material
// (refs, excerpts, titles) is ever rendered here.
export function CraftSharePanel({ locale, role, view, onDecide, onRevoke }: CraftSharePanelProps) {
  const labels = shareLabels(locale);
  const isOwner = role === 'owner';
  const awaitingDecision = view.restricted && view.status === 'private';

  return <section aria-label={labels.heading} className="wk-craft-share" data-status={view.status} data-restricted={view.restricted}>
    <h4>{labels.heading}</h4>
    <p className="wk-craft-share-summary">
      <span className="wk-craft-share-version">{labels.version}: <code>{view.versionId}</code></span>
      <span className="wk-craft-share-digest">{labels.digest}: <code>{view.evidenceDigest.slice(0, 16)}…</code></span>
      <span className="wk-craft-share-state" data-state={view.status}>{labels.states[view.status]}</span>
    </p>
    <p className="wk-craft-share-notice">{view.restricted ? labels.restricted : labels.unrestricted}</p>
    {view.decision && view.status !== 'private' && <p className="wk-craft-share-decision">
      {labels.decidedBy}: <code>{view.decision.owner_id}</code>
      {view.expiresAt ? <span className="wk-craft-share-expiry"> · {labels.expires}: {view.expiresAt}</span> : null}
    </p>}
    {awaitingDecision && !view.decision && <p className="wk-craft-share-awaiting">{labels.awaiting}</p>}
    {isOwner && awaitingDecision && <div className="wk-craft-share-actions">
      <button type="button" className="wk-craft-share-confirm" onClick={() => void onDecide('approved')}>{labels.confirm}</button>
      <button type="button" className="wk-craft-share-decline" onClick={() => void onDecide('rejected')}>{labels.decline}</button>
    </div>}
    {isOwner && view.status === 'consented' && view.restricted && <div className="wk-craft-share-actions">
      <button type="button" className="wk-craft-share-revoke" onClick={() => void onRevoke()}>{labels.revoke}</button>
    </div>}
  </section>;
}
