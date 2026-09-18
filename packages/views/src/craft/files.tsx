// W05 craft Files: the versioned file table and the version history.
//
// Downloads always name a FIXED (versionId, path) pair — the immutable object
// the backend resolves — so an old version never silently redirects to the
// latest file. The history table states facts only: the Run that produced the
// version, the generation time where the wire contract carries one (the
// session's updated_at belongs to the current version; older rows stay
// explicit "—"), and every Check with its recorded status. "Continue from this
// version" is deliberately absent: restore lands with C05.
import React from 'react';
import type { CraftVersionView } from '@weknora/contracts';
import { Button } from '@weknora/ui';
import { craftStrings, downloadFileName, formatBytes, historyRows, type CraftLocale } from './presentation.ts';

export interface CraftFilesProps {
  locale: CraftLocale;
  /** The selected version whose files are listed. */
  version: CraftVersionView | null;
  /** The version history page (newest first from the backend cursor). */
  versions: CraftVersionView[];
  currentVersionId: string | null;
  selectedVersionId: string | null;
  sessionUpdatedAt: string;
  downloadingPath: string | null;
  onSelectVersion(versionId: string): void;
  /** Downloads one immutable member; the assembly owns auth + blob plumbing. */
  onDownload(versionId: string, path: string): void;
}

export function CraftFiles(props: CraftFilesProps) {
  const strings = craftStrings(props.locale);
  const rows = historyRows(props.versions, props.currentVersionId, props.sessionUpdatedAt, props.locale);

  return (
    <div className="wk-craft-panel-body">
      <h3>{strings.craftFilesPath}</h3>
      {props.version === null || props.version.files.length === 0 ? (
        <p className="wk-craft-muted">{strings.craftFilesEmpty}</p>
      ) : (
        <table className="wk-craft-table">
          <thead>
            <tr>
              <th scope="col">{strings.craftFilesPath}</th>
              <th scope="col">{strings.craftFilesSize}</th>
              <th scope="col">{strings.craftFilesDownload}</th>
            </tr>
          </thead>
          <tbody>
            {props.version.files.map((file) => (
              <tr key={file.path}>
                <td><code>{file.path}</code></td>
                <td>{formatBytes(file.bytes)}</td>
                <td>
                  <Button
                    type="button"
                    disabled={props.downloadingPath === file.path}
                    onClick={() => props.onDownload(props.version?.id ?? '', file.path)}
                  >
                    {props.downloadingPath === file.path ? strings.craftDownloading : strings.craftFilesDownload}
                  </Button>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      )}

      <h3>{strings.craftHistory}</h3>
      {rows.length === 0 ? (
        <p className="wk-craft-muted">{strings.craftPreviewEmpty}</p>
      ) : (
        <table className="wk-craft-table">
          <thead>
            <tr>
              <th scope="col">{strings.craftVersionLabel}</th>
              <th scope="col">{strings.craftHistoryRun}</th>
              <th scope="col">{strings.craftHistoryTime}</th>
              <th scope="col">{strings.craftHistoryChecks}</th>
              <th scope="col">{strings.craftOpen}</th>
            </tr>
          </thead>
          <tbody>
            {rows.map((row) => (
              <tr key={row.versionId}>
                <td>
                  <code>{row.versionId}</code>
                  {row.isCurrent ? <span className="wk-craft-current-badge"> {strings.craftHistoryCurrent}</span> : null}
                  <div>{row.fileCount} · {formatBytes(row.totalBytes)}</div>
                </td>
                <td><code>{row.runId}</code></td>
                <td>{row.timeLabel ?? strings.craftHistoryTimeUnavailable}</td>
                <td>
                  {row.checks.length === 0
                    ? strings.craftCheckNotRun
                    : row.checks.map((check) => (
                        <div key={check.name}>
                          <span className="wk-craft-check" data-status={check.status}>{check.name}</span>{' '}
                          <span className="wk-craft-hint">{check.detail === '' ? check.status : check.detail}</span>
                        </div>
                      ))}
                </td>
                <td>
                  {props.selectedVersionId === row.versionId ? (
                    <span className="wk-craft-hint">{strings.craftTabPreview}</span>
                  ) : (
                    <Button type="button" onClick={() => props.onSelectVersion(row.versionId)}>{strings.craftOpen}</Button>
                  )}
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      )}
    </div>
  );
}
