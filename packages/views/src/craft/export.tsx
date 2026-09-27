import React, { useCallback, useState } from 'react';
import type {
  CraftExportDecision as CraftExportDecisionContract,
  CraftExportOriginRef,
} from '@weknora/contracts';

// T13 (#133): exporting restricted derived data is server authority this
// panel only PROJECTS. The server classifies every derived file from its
// recorded origins and reduces the owner's decision to a state bound to
// the immutable manifest digest; the panel never derives authority
// client-side and never renders a URL — origins render as the opaque
// identity they are, and opening an original is a separate, freshly
// authorized act.

export type CraftExportConsentStatus = 'none' | 'awaiting' | 'consented' | 'declined';
export type CraftExportRole = 'owner' | 'collaborator' | 'viewer';
export type CraftExportDecisionKind = 'approved' | 'rejected' | 'unknown';

// The decision shape is the FROZEN @weknora/contracts export; the panel
// only adds the 'unknown' parse residual on top of it.
export type CraftExportDecision = CraftExportDecisionContract & { decision: CraftExportDecisionKind };

export interface CraftExportConsentFile {
  path: string;
  sha256: string;
  origins: CraftExportOriginRef[];
  /** Server classification: a restricted origin contributed to this member. */
  restrictedDerived: boolean;
}

export interface CraftExportConsentView {
  versionId: string;
  manifestDigest: string;
  state: CraftExportConsentStatus;
  restrictedDerived: string[];
  files: CraftExportConsentFile[];
  decision: CraftExportDecision | null;
}

const consentStatuses: readonly CraftExportConsentStatus[] = ['none', 'awaiting', 'consented', 'declined'];
const exportDecisions: readonly CraftExportDecisionKind[] = ['approved', 'rejected', 'unknown'];
const originKinds: readonly string[] = ['knowledge', 'input', 'artifact'];

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === 'object' && value !== null;
}

function optionalString(value: unknown): string | null {
  return typeof value === 'string' && value.length > 0 ? value : null;
}

// projectExportConsentView normalizes the wire payload into the typed
// view. The projection is fail-closed: malformed shapes are dropped, not
// repaired, and authority is never inferred — a decision that does not
// bind the CURRENT version and manifest digest is stale, a consented
// claim without a binding approved decision downgrades to awaiting, and a
// state that contradicts the recorded origins (restricted derived files
// claimed as "none") resolves closed.
export function projectExportConsentView(raw: unknown): CraftExportConsentView | null {
  if (!isRecord(raw)) return null;
  const versionId = optionalString(raw.version_id);
  const manifestDigest = optionalString(raw.manifest_digest);
  const state = raw.state;
  if (!versionId || !manifestDigest) return null;
  if (typeof state !== 'string' || !consentStatuses.includes(state as CraftExportConsentStatus)) return null;

  if (!Array.isArray(raw.files)) return null;
  const files: CraftExportConsentFile[] = [];
  for (const item of raw.files) {
    if (!isRecord(item)) return null;
    const path = optionalString(item.path);
    const sha256 = optionalString(item.sha256);
    if (!path || !sha256 || !Array.isArray(item.origins)) return null;
    const origins: CraftExportOriginRef[] = [];
    for (const originItem of item.origins) {
      if (!isRecord(originItem)) return null;
      const kind = originItem.kind;
      const ref = optionalString(originItem.ref);
      const originDigest = optionalString(originItem.sha256);
      if (!ref || !originDigest || typeof kind !== 'string' || !originKinds.includes(kind)) return null;
      if (typeof originItem.restricted !== 'boolean') return null;
      origins.push({ kind: kind as CraftExportOriginRef['kind'], ref, sha256: originDigest, restricted: originItem.restricted });
    }
    files.push({ path, sha256, origins, restrictedDerived: origins.some((origin) => origin.restricted) });
  }

  let decision: CraftExportDecision | null = null;
  if (isRecord(raw.decision)) {
    const d = raw.decision;
    const dVersion = optionalString(d.version_id);
    const dDigest = optionalString(d.manifest_digest);
    const dOwner = optionalString(d.owner_id);
    const dKind = d.decision;
    if (dVersion && dDigest && dOwner && typeof dKind === 'string' && exportDecisions.includes(dKind as CraftExportDecisionKind)) {
      // The binding guard mirrors the server rule: a decision of another
      // version or another manifest digest is history, never authority.
      if (dVersion === versionId && dDigest === manifestDigest) {
        decision = { version_id: dVersion, manifest_digest: dDigest, owner_id: dOwner, decision: dKind as CraftExportDecisionKind };
      }
    }
  }

  // Authority is never inferred: a consented claim needs the binding
  // approved decision, and a "none" claim cannot stand over restricted
  // derived members the recorded origins themselves prove.
  let effectiveStatus = state as CraftExportConsentStatus;
  if (effectiveStatus === 'consented' && decision?.decision !== 'approved') effectiveStatus = 'awaiting';
  if (effectiveStatus === 'none' && files.some((file) => file.restrictedDerived)) effectiveStatus = 'awaiting';

  const restrictedDerived = files.filter((file) => file.restrictedDerived).map((file) => file.path);
  return {
    versionId,
    manifestDigest,
    state: effectiveStatus,
    restrictedDerived,
    files,
    decision,
  };
}

export interface CraftExportConsentPanelProps {
  locale: 'zh' | 'en';
  role: CraftExportRole;
  view: CraftExportConsentView;
  onDecide(decision: 'approved' | 'rejected'): void | Promise<void>;
}

interface CraftExportLabels {
  heading: string;
  version: string;
  digest: string;
  states: Record<CraftExportConsentStatus, string>;
  restrictedBadge: string;
  origins: string;
  needsConsent: string;
  noConsentNeeded: string;
  approve: string;
  safeExport: string;
  decidedBy: string;
  actionFailed: string;
}

function exportLabels(locale: 'zh' | 'en'): CraftExportLabels {
  if (locale === 'en') {
    return {
      heading: 'Export restricted derived data',
      version: 'Version',
      digest: 'Manifest digest',
      states: { none: 'No consent needed', awaiting: 'Awaiting owner decision', consented: 'Consented', declined: 'Declined' },
      restrictedBadge: 'restricted derived',
      origins: 'origins',
      needsConsent: 'Some derived files derive from restricted (shared-library) sources. Exporting them needs your explicit decision; the originals themselves are never bundled.',
      noConsentNeeded: 'No derived file derives from restricted sources. Downloads need no consent.',
      approve: 'Approve export',
      safeExport: 'Export without restricted files',
      decidedBy: 'Decided by',
      actionFailed: 'The export decision failed. Try again.',
    };
  }
  return {
    heading: '导出受限派生数据',
    version: '版本',
    digest: '清单摘要',
    states: { none: '无需同意', awaiting: '等待所有者决定', consented: '已同意导出', declined: '已拒绝导出' },
    restrictedBadge: '受限派生',
    origins: '来源',
    needsConsent: '部分派生文件来自受限（共享库）来源。导出这些文件需要你的明确决定；原始来源本身绝不会被打包。',
    noConsentNeeded: '没有派生文件来自受限来源，下载无需同意。',
    approve: '同意导出',
    safeExport: '导出不含受限派生文件',
    decidedBy: '决定人',
    actionFailed: '导出决定操作失败，请重试。',
  };
}

// CraftExportConsentPanel projects the server's export consent summary.
// The owner sees the exact files with their recorded origins and the
// manifest digest they are deciding on; every other member sees the state
// without controls. No URL is ever rendered: origins are opaque
// identities, and opening an original re-authorizes server-side (T10).
export function CraftExportConsentPanel({ locale, role, view, onDecide }: CraftExportConsentPanelProps) {
  const labels = exportLabels(locale);
  const isOwner = role === 'owner';
  // A declined decision is not final: the server keeps one decision row
  // per task+version and a fresh decision upserts over it, so the owner
  // can decide again at any time.
  const canDecide = isOwner && (view.state === 'awaiting' || view.state === 'declined');
  const [actionError, setActionError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);

  // runExportAction awaits the host callback: a rejected decide becomes
  // visible feedback here instead of an unhandled promise rejection that
  // leaves the panel silently unchanged.
  const runExportAction = useCallback(async (action: () => void | Promise<void>) => {
    setActionError(null);
    setBusy(true);
    try {
      await action();
    } catch {
      setActionError(labels.actionFailed);
    } finally {
      setBusy(false);
    }
  }, [labels.actionFailed]);

  return <section aria-label={labels.heading} className="wk-craft-export" data-status={view.state}>
    <h4>{labels.heading}</h4>
    <p className="wk-craft-export-summary">
      <span className="wk-craft-export-version">{labels.version}: <code>{view.versionId}</code></span>
      <span className="wk-craft-export-digest">{labels.digest}: <code>{view.manifestDigest.slice(0, 16)}…</code></span>
      <span className="wk-craft-export-state" data-state={view.state}>{labels.states[view.state]}</span>
    </p>
    <p className="wk-craft-export-notice">
      {view.state === 'none' ? labels.noConsentNeeded : labels.needsConsent}
    </p>
    <ul className="wk-craft-export-files">
      {view.files.map((file) => <li key={file.path} className="wk-craft-export-file" data-restricted-derived={file.restrictedDerived}>
        <code>{file.path}</code>
        {file.restrictedDerived ? <span className="wk-craft-export-badge">{labels.restrictedBadge}</span> : null}
        <span className="wk-craft-export-origins">
          {labels.origins} ({file.origins.length}): {file.origins.map((origin) => origin.ref).join(', ')}
        </span>
      </li>)}
    </ul>
    {view.decision && view.state !== 'awaiting' && <p className="wk-craft-export-decision">
      {labels.decidedBy}: <code>{view.decision.owner_id}</code>
    </p>}
    {actionError && <p role="alert" className="wk-craft-export-error">{actionError}</p>}
    {canDecide && <div className="wk-craft-export-actions">
      <button type="button" className="wk-craft-export-approve" disabled={busy} onClick={() => void runExportAction(() => onDecide('approved'))}>{labels.approve}</button>
      <button type="button" className="wk-craft-export-decline" disabled={busy} onClick={() => void runExportAction(() => onDecide('rejected'))}>{labels.safeExport}</button>
    </div>}
  </section>;
}
