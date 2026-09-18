// CFT-S01-T011 (P06): the versions drawer content — version rows with the
// view/restore separation. Viewing is read-only; "continue from this
// version" is an EXPLICIT confirmation dialog that names the restore source
// and states that no old version is overwritten. Downloadable is never
// conflated with restorable: the eligibility projection (domain rules)
// disables restore with a readable reason while download stays available.
import React, { useState } from 'react';
import { Button, Dialog } from '@weknora/ui';
import {
  restoreEligibility,
  type CraftVersionFact,
  type CraftVersionSelectionState,
} from '@weknora/domain/craft/version-selection';
import { formatDateTime, type CraftLocale } from './presentation.ts';
import './craft.css';

export interface CraftVersionsDrawerProps {
  locale: CraftLocale;
  /** The selection facts the drawer renders from. */
  selection: CraftVersionSelectionState;
  /** Per-version display rows (time/checks come from the version views). */
  rows: readonly {
    versionId: string;
    runId: string;
    updatedAt: string;
    checksPassed: number;
    checksTotal: number;
  }[];
  canWrite: boolean;
  hasActiveRun: boolean;
  onView(versionId: string): void;
  onDownload(versionId: string): void;
  onRestore(versionId: string): Promise<void> | void;
}

export function CraftVersionsDrawer(props: CraftVersionsDrawerProps) {
  const [confirming, setConfirming] = useState<CraftVersionFact | null>(null);
  const zh = props.locale === 'zh';

  const eligibilityOf = (version: CraftVersionFact) =>
    restoreEligibility(props.selection, version.id, { canWrite: props.canWrite, hasActiveRun: props.hasActiveRun });

  const rowOf = (versionId: string) => props.rows.find((row) => row.versionId === versionId);

  return (
    <div className="wk-craft-versions" data-testid="craft-versions">
      <ul className="wk-craft-list">
        {props.selection.versions.map((version) => {
          const eligibility = eligibilityOf(version);
          const row = rowOf(version.id);
          const isViewing = props.selection.viewVersionId === version.id;
          const isBase = props.selection.baseVersionId === version.id;
          return (
            <li key={version.id} data-version={version.id} data-viewing={isViewing}>
              <span className="wk-craft-item-title">
                {version.id}
                {isBase ? <span className="wk-craft-current-badge"> {zh ? '编辑基线' : 'baseline'}</span> : null}
              </span>
              <span className="wk-craft-item-meta">
                {row ? formatDateTime(row.updatedAt, props.locale) : ''} · run {row?.runId ?? '—'}
              </span>
              <span className="wk-craft-item-meta">
                {row ? `${row.checksPassed}/${row.checksTotal} ${zh ? '检查通过' : 'checks'}` : ''}
              </span>
              <span className="wk-craft-actions">
                <Button type="button" size="small" onClick={() => props.onView(version.id)}>
                  {zh ? '查看此版本' : 'View'}
                </Button>
                <Button
                  type="button"
                  size="small"
                  disabled={!eligibility.downloadable}
                  onClick={() => props.onDownload(version.id)}
                >
                  {zh ? '下载' : 'Download'}
                </Button>
                <Button
                  type="button"
                  size="small"
                  disabled={!eligibility.restorable}
                  title={eligibility.blockReason ?? undefined}
                  onClick={() => setConfirming(version)}
                >
                  {zh ? '从此版本继续' : 'Continue from'}
                </Button>
              </span>
              {!eligibility.restorable && eligibility.blockReason !== null ? (
                <span className="wk-craft-muted">{eligibility.blockReason}</span>
              ) : null}
            </li>
          );
        })}
      </ul>

      <Dialog
        open={confirming !== null}
        title={zh ? '从此版本继续？' : 'Continue from this version?'}
        onClose={() => setConfirming(null)}
      >
        <p className="wk-craft-muted">
          {zh
            ? `将以 ${confirming?.id ?? ''} 的完整恢复快照重建工作区；已发布的历史版本不会被覆盖；恢复成功后下一轮交付才会发布新版本。`
            : `The workspace will be rebuilt from ${confirming?.id ?? ''}'s complete snapshot; published versions are never overwritten; the next successful delivery publishes the new version.`}
        </p>
        <div className="wk-craft-actions">
          <Button type="button" onClick={() => setConfirming(null)}>{zh ? '取消' : 'Cancel'}</Button>
          <Button
            type="button"
            data-testid="craft-restore-confirm"
            onClick={() => {
              const target = confirming;
              setConfirming(null);
              if (target !== null) void props.onRestore(target.id);
            }}
          >
            {zh ? '确认恢复' : 'Restore'}
          </Button>
        </div>
      </Dialog>
    </div>
  );
}
