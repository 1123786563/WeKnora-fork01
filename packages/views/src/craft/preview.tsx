// W05 craft Preview: the isolated-origin controlled preview panel (W02).
//
// The iframe src is ALWAYS the ticket URL the backend returned — never a
// client-joined path — and the sandbox allows scripts only (no same-origin):
// the preview origin is untrusted by construction. Switching the version
// destroys the previous iframe node AND its one-time ticket: the parent clears
// the ticket on version change and remounts this panel, so an old capability
// URL can never survive into the next version's frame.
import React from 'react';
import { useEffect, useState } from 'react';
import type { CraftPreviewTicketView } from '@weknora/contracts';
import { Button } from '@weknora/ui';
import { classifyPreview, craftStrings, formatDateTime, type CraftLocale, type CraftPreviewState } from './presentation.ts';

export interface CraftPreviewProps {
  locale: CraftLocale;
  /** The selected version (null when nothing is published yet). */
  versionId: string | null;
  /** Whether the selected version manifest actually holds files. */
  hasArtifact: boolean;
  ticket: CraftPreviewTicketView | null;
  ticketIssuing: boolean;
  ticketError: string | null;
  /** Re-issues the ticket for the selected version — preview only, never re-runs generation. */
  onIssueTicket(versionId: string): void;
}

export function CraftPreview(props: CraftPreviewProps) {
  const strings = craftStrings(props.locale);
  // Local clock so an expired ticket flips the panel without user action.
  const [nowMs, setNowMs] = useState(() => Date.now());
  useEffect(() => {
    const timer = window.setInterval(() => setNowMs(Date.now()), 15_000);
    return () => window.clearInterval(timer);
  }, []);
  // A frame the browser refused to load is a failed preview, not a failed run.
  const [frameFailed, setFrameFailed] = useState(false);
  useEffect(() => { setFrameFailed(false); }, [props.ticket?.url, props.versionId]);

  const state: CraftPreviewState = classifyPreview({
    hasArtifact: props.versionId !== null && props.hasArtifact,
    ticketUrl: props.ticket?.url ?? null,
    ticketExpiresAt: props.ticket?.expires_at ?? null,
    ticketError: props.ticketError,
    frameFailed,
    nowMs,
  });

  if (state === 'active' && props.ticket !== null) {
    return (
      <div className="wk-craft-frame-wrap" data-testid="craft-preview" data-state={state}>
        <iframe
          // key destroys the old frame whenever the version or ticket changes.
          key={props.versionId + ':' + props.ticket.url}
          src={props.ticket.url}
          sandbox="allow-scripts"
          title={strings.craftTabPreview}
          onError={() => setFrameFailed(true)}
        />
        <p className="wk-craft-preview-meta">
          {strings.craftPreviewExpiresAt} {formatDateTime(props.ticket.expires_at, props.locale)}
        </p>
      </div>
    );
  }

  return (
    <div className="wk-craft-preview-state" data-testid="craft-preview" data-state={state} role="status">
      {state === 'empty' ? <p>{strings.craftPreviewEmpty}</p> : null}
      {state === 'loading' ? <p>{props.ticketIssuing ? strings.craftPreviewLoading : strings.craftLoading}</p> : null}
      {state === 'expired' ? <p>{strings.craftPreviewExpired}</p> : null}
      {state === 'failed' ? <p>{strings.craftPreviewFailed}{props.ticketError !== null ? ' — ' + props.ticketError : ''}</p> : null}
      {state !== 'loading' && props.versionId !== null ? (
        // Retry reopens the PREVIEW only; it never resubmits the run.
        <Button type="button" onClick={() => props.onIssueTicket(props.versionId ?? '')}>{strings.craftPreviewRefresh}</Button>
      ) : null}
    </div>
  );
}
