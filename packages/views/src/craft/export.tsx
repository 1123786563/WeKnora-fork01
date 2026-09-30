import React, { useCallback, useState } from 'react';
import {
  parseCraftExportConsentView,
  type CraftExportConsentState,
  type CraftExportDecision,
  type CraftExportOriginRef,
} from '@weknora/contracts';

// T13 (#133): exporting restricted derived data is server authority this
// panel only PROJECTS. The server classifies every derived file from its
// recorded origins and reduces the owner's decision to a state bound to
// the immutable manifest digest; the panel never derives authority
// client-side and never renders a URL — origins render as the opaque
// identity they are, and opening an original is a separate, freshly
// authorized act.

export type CraftExportConsentStatus = CraftExportConsentState;
export type CraftExportRole = 'owner' | 'collaborator' | 'viewer';

// The decision shape IS the frozen @weknora/contracts export — 'unknown'
// is already inside the frozen CRAFT_DECISIONS vocabulary, so no local
// residual is added on top of it.

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

// projectExportConsentView normalizes the wire payload into the typed
// view. The SHAPE and the closed vocabularies (state, decision, origin
// kinds) validate through the ONE frozen contract parser — the same
// authority the server's wire is pinned against — so the contract and
// this projection can never drift apart. On top of the parsed shape this
// layer only adds the projection rules: authority is never inferred — a
// decision that does not bind the CURRENT version and manifest digest is
// stale, a consented claim without a binding approved decision downgrades
// to awaiting, and a state that contradicts the recorded origins
// (restricted derived files claimed as "none") resolves closed.
export function projectExportConsentView(raw: unknown): CraftExportConsentView | null {
  let parsed;
  try {
    parsed = parseCraftExportConsentView(raw);
  } catch {
    // The frozen parser rejects a digest-mismatched decision outright — a
    // stale decision is history, never a reason to hide the whole panel:
    // retry once with the decision dropped and let the state-downgrade
    // rules below speak. Every OTHER malformation still fails the retry
    // closed (null), so this only rescues the stale-history case.
    try {
      parsed = parseCraftExportConsentView(
        typeof raw === 'object' && raw !== null ? { ...(raw as Record<string, unknown>), decision: null } : raw,
      );
    } catch {
      return null;
    }
  }

  const files: CraftExportConsentFile[] = parsed.files.map((file) => ({
    path: file.path,
    sha256: file.sha256,
    origins: file.origins,
    restrictedDerived: file.origins.some((origin) => origin.restricted),
  }));

  // The binding guard mirrors the server rule: a decision of another
  // version or another manifest digest is history, never authority. (The
  // frozen parser already REJECTS a digest-mismatched decision outright;
  // this keeps the same discipline for a decision of another VERSION,
  // which the wire can legally carry as history.)
  const decision = parsed.decision !== null && parsed.decision.version_id === parsed.version_id
    ? parsed.decision
    : null;

  // Authority is never inferred: a consented claim needs the binding
  // approved decision, and a "none" claim cannot stand over restricted
  // derived members the recorded origins themselves prove.
  let effectiveStatus = parsed.state;
  if (effectiveStatus === 'consented' && decision?.decision !== 'approved') effectiveStatus = 'awaiting';
  if (effectiveStatus === 'none' && files.some((file) => file.restrictedDerived)) effectiveStatus = 'awaiting';

  return {
    versionId: parsed.version_id,
    manifestDigest: parsed.manifest_digest,
    state: effectiveStatus,
    restrictedDerived: files.filter((file) => file.restrictedDerived).map((file) => file.path),
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
  withdraw: string;
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
      withdraw: 'Withdraw consent, export safe files',
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
    withdraw: '撤回同意，导出安全文件',
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
  // The owner can decide again at any time: the server keeps ONE decision
  // row per task+version and a fresh decision upserts over it — a declined
  // decision is not final, and a standing APPROVAL can be withdrawn by a
  // fresh rejection (the only long-lived way to end a consent, exactly as
  // the service and the migration docs pin). Hiding the controls after
  // consented would make the approval irrevocable from this panel.
  const canDecide = isOwner && view.state !== 'none';
  const withdrawing = view.state === 'consented';
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
      {withdrawing ? null : <button type="button" className="wk-craft-export-approve" disabled={busy} onClick={() => void runExportAction(() => onDecide('approved'))}>{labels.approve}</button>}
      <button type="button" className="wk-craft-export-decline" data-withdrawing={withdrawing} disabled={busy} onClick={() => void runExportAction(() => onDecide('rejected'))}>{withdrawing ? labels.withdraw : labels.safeExport}</button>
    </div>}
  </section>;
}
