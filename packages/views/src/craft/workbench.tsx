// W05 craft Workbench: the left conversation + right artifact panel layout.
//
// assistant-ui decision (documented in the task report): G2 has not installed
// or version-locked @assistant-ui/react in apps/web, and the master plan says
// Craft must not take over framework migration. This module therefore
// implements the SAME ExternalStoreRuntime semantics with plain React 19:
//
//   * createCraftMessageLog is an external store fed exclusively by the SAME
//     backend run-event frames the W04 controller consumes (the assembly
//     tees every frame into it). Components never fetch or invent messages.
//   * Message completeness is set ONLY by the main Run projection
//     (projectAssistant + CRAFT_TERMINAL_RUN_STATUSES). A delegation.finished
//     child event changes the child status alone — a child event can never
//     complete the main message, mirroring the W04 reducer invariant.
//   * Every view is a pure projection (presentation.ts) of that store plus
//     the controller state; components hold UI state only.
//
// R06 interaction semantics are enforced in the card rendering: a question
// offers answer/reject, a permission offers approve-with-scope/reject, and an
// unknown kind only reject — there is deliberately no generic Approve.
import {
  useCallback,
  useEffect,
  useMemo,
  useRef,
  useState,
  useSyncExternalStore,
  type CSSProperties,
} from 'react';
import type { CraftPreviewTicketView, CraftVersionView } from '@weknora/contracts';
import type { CraftWorkbenchController } from '@weknora/core/craft/controller';
import { Button } from '@weknora/ui';
import {
  archiveLiveTurn,
  craftStrings,
  downloadFileName,
  entryFileOf,
  formatBytes,
  isMainRunTerminal,
  projectAssistant,
  projectKnowledgeSources,
  statusLabelLocalized,
  type CraftAssistantProjection,
  type CraftInteractionAction,
  type CraftInteractionCard,
  type CraftLocale,
  type CraftMessageLog,
  type CraftTurnRecord,
} from './presentation.ts';
import { CraftPreview } from './preview.tsx';
import { CraftFiles } from './files.tsx';
import { CraftSources } from './sources.tsx';
import { CraftDocument, CRAFT_DOCUMENT_MD_PATH } from './document.tsx';
import { CraftSpreadsheet, type CraftSpreadsheetPreviewView } from './spreadsheet.tsx';
import { CraftSlides, type CraftSlidesPreviewView } from './slides.tsx';
import './craft.css';
// The message log store (createCraftMessageLog) lives in presentation.ts so
// its reset/dedupe semantics stay node-testable without importing CSS; it is
// re-exported here because the assembly imports it from this module.
export { createCraftMessageLog } from './presentation.ts';
export type { CraftMessageSnapshot } from './presentation.ts';

// ---------------------------------------------------------------------------
// Workbench component
// ---------------------------------------------------------------------------

export interface CraftPendingAttachment {
  id: string;
  name: string;
  state: 'pending' | 'uploading' | 'ready' | 'error';
}

export interface CraftInteractionActionInput {
  id: string;
  kind: CraftInteractionCard['kind'];
  prompt: string;
  action: CraftInteractionAction;
  answer: string;
}

export interface CraftWorkbenchProps {
  locale: CraftLocale;
  sessionId: string;
  title: string;
  kind: string;
  canWrite: boolean;
  /** W04 workbench controller: state, stream and plain-submit engine. */
  controller: CraftWorkbenchController;
  messageLog: CraftMessageLog;
  /** Prefills the composer (the Home creation goal, not yet submitted). */
  initialPrompt: string | null;
  resumedRun: boolean;
  pendingAttachments: CraftPendingAttachment[];
  /** True while a send must carry attachments or the creation knowledge scope. */
  enrichedPending: boolean;
  onPickAttachments(files: File[]): void;
  onRemoveAttachment(id: string): void;
  /**
   * Sends a prompt that carries attachments (and, for the creation run, the
   * knowledge scope): the assembly uploads through the existing session
   * attachment entry, associates via POST /craft/inputs and submits the run,
   * then reloads the controller from the snapshot.
   */
  onEnrichedSend(prompt: string): Promise<void>;
  versions: CraftVersionView[];
  sessionUpdatedAt: string;
  snapshotVersionId: string | null;
  onRefreshVersions(): void;
  onIssuePreview(versionId: string): Promise<CraftPreviewTicketView>;
  onDownload(versionId: string, path: string): void | Promise<void>;
  /**
   * C05 restore entrance (optional until the assembly wires the POST
   * /craft/restore call): when provided and the caller may write, the
   * version bar offers "continue editing from this version" for versions
   * with a complete recovery snapshot. The assembly owns the request id,
   * the workspace revision and the reload; the workbench only reflects
   * restorability and reports errors.
   */
  restorableVersionIds?: string[];
  onRestoreVersion?(versionId: string): Promise<void>;
  /**
   * Interaction decisions stay props callbacks: the backend interaction HTTP
   * routes are not wired yet (W04 report §6), so the assembly decides what a
   * decision currently does — never a silent auto-approval.
   */
  onInteractionAction(input: CraftInteractionActionInput): void;
  /**
   * Opens one knowledge source by its durable ref (C01 sources panel): the
   * ASSEMBLY re-resolves the ref through the existing resource permission
   * chain on every click — it must never cache or pre-sign a URL into the
   * component. Optional until the assembly wires it; the panel renders the
   * resolved notice (or permission error) it returns.
   */
  onOpenSource?(citationId: string, ref: string): Promise<string | null>;
  /**
   * Fetches one immutable member's TEXT through the assembly's authorized
   * version-files route (D01 wiring): the document view reads report.md and
   * manifest.json, the spreadsheet/slides views read preview.json. Optional
   * for backwards compatibility — when absent the workbench keeps the
   * isolated-origin iframe surface for every kind.
   */
  onFetchVersionFile?(versionId: string, path: string): Promise<string>;
  /**
   * Resolves one immutable member into an object URL the preview surface may
   * load (slides page images). The assembly owns auth + blob plumbing and
   * the URL's lifetime; the views never assemble raw file paths.
   */
  onResolveVersionFileUrl?(versionId: string, path: string): Promise<string>;
  /** Reuses the existing authorized sandbox terminal entry (ticket-minted WS URL). */
  onMintTerminalUrl(): Promise<string>;
  onBack(): void;
  syncError: string | null;
}

type SideTab = 'preview' | 'files' | 'details';
type NarrowTab = 'conversation' | SideTab;

// C05 restore wording lives here (not presentation.ts) so the entry ships
// with its own feature strings; locale follows the workbench prop.
const RESTORE_STRINGS = {
  zh: {
    action: '从此版本继续编辑',
    busy: '恢复中…',
    reasonNoSnapshot: '此版本没有完整恢复快照（仅下载可用）',
    reasonRunActive: '执行完成后再从此版本继续编辑',
    reasonUnselected: '选择一个版本后可继续编辑',
    failed: '恢复失败',
  },
  en: {
    action: 'Continue editing from this version',
    busy: 'Restoring…',
    reasonNoSnapshot: 'This version has no complete recovery snapshot (download only)',
    reasonRunActive: 'Wait for the run to finish before continuing from a version',
    reasonUnselected: 'Select a version to continue from',
    failed: 'Restore failed',
  },
} as const;

/** Stable-callback hook so effects never loop on parent re-renders. */
function useEventCallback<A extends unknown[], R>(fn: (...args: A) => R): (...args: A) => R {
  const ref = useRef(fn);
  useEffect(() => {
    ref.current = fn;
  });
  return useCallback((...args: A) => ref.current(...args), []);
}

export function CraftWorkbench(props: CraftWorkbenchProps) {
  const strings = craftStrings(props.locale);

  // --- controller state (the Run projection authority) ---------------------
  const [controllerState, setControllerState] = useState(props.controller.state());
  useEffect(() => props.controller.onChange(setControllerState), [props.controller]);
  const mainStatus = controllerState?.mainStatus ?? 'idle';
  const runActive = controllerState !== null && controllerState.runId !== null && !isMainRunTerminal(mainStatus);

  // --- message log (backend event projection) -------------------------------
  // The same getter serves as the server snapshot: the log is a synchronous
  // in-memory store, so SSR reads the current projection.
  const snapshot = useSyncExternalStore(props.messageLog.subscribe, props.messageLog.getSnapshot, props.messageLog.getSnapshot);
  const projection = useMemo(() => projectAssistant(snapshot.events, mainStatus), [snapshot, mainStatus]);

  // --- conversation turns ----------------------------------------------------
  // B1 fix: the finished turn is archived AT SEND TIME, inside the sender's
  // own event handler, while the message log still holds that run's events
  // and BEFORE the new run's subscription resets the log. (The previous
  // effect-based archive was dead code: an earlier-declared every-render ref
  // write had already overwritten the old runId by the time the [runId]
  // effect read it, so from the second send on the previous conversation
  // silently vanished.)
  const [turns, setTurns] = useState<CraftTurnRecord[]>([]);
  const [livePrompt, setLivePrompt] = useState<string | null>(null);
  // The run whose turn already lives in `turns`: its projection stays hidden
  // from the live area until the NEXT run becomes current.
  const [archivedRunId, setArchivedRunId] = useState<string | null>(null);
  const runId = controllerState?.runId ?? null;
  const liveArchived = runId !== null && runId === archivedRunId;
  const liveProjection: CraftAssistantProjection = liveArchived
    ? { text: '', tools: [], interactions: [], artifactVersionIds: [], childStatus: 'idle', workspaceUnavailable: false, complete: false }
    : projection;
  useEffect(() => {
    // A new run takes over: un-hide the live area and follow the workspace version again.
    if (archivedRunId !== null && runId !== null && runId !== archivedRunId) setArchivedRunId(null);
    if (runId !== null) setPinnedVersion(false);
  }, [runId, archivedRunId]);
  useEffect(() => {
    // Scope reset/dispose: the conversation belonged to the dead scope.
    if (controllerState === null) {
      setTurns([]);
      setLivePrompt(null);
      setArchivedRunId(null);
    }
  }, [controllerState]);

  // --- versions + selection ---------------------------------------------------
  const versions = props.versions;
  // The live authority: the controller's versionId tracks artifact.published
  // events AND refreshed snapshots, so a newly published version auto-selects
  // unless the user pinned an older one for inspection/download.
  const effectiveVersionId = controllerState?.versionId ?? props.snapshotVersionId;
  const [selectedVersionId, setSelectedVersionId] = useState<string | null>(props.snapshotVersionId);
  const [pinnedVersion, setPinnedVersion] = useState(false);
  useEffect(() => {
    if (!pinnedVersion && effectiveVersionId !== null) setSelectedVersionId(effectiveVersionId);
  }, [effectiveVersionId, pinnedVersion]);
  const selectedVersion = useMemo(
    () => versions.find((version) => version.id === selectedVersionId) ?? null,
    [versions, selectedVersionId],
  );
  const entryFile = entryFileOf(selectedVersion);

  const refreshVersions = useEventCallback(props.onRefreshVersions);
  const terminalRunRef = useRef<string | null>(null);
  useEffect(() => {
    if (controllerState === null || controllerState.runId === null) return;
    if (!isMainRunTerminal(controllerState.mainStatus)) return;
    if (terminalRunRef.current === controllerState.runId) return;
    terminalRunRef.current = controllerState.runId;
    refreshVersions();
  }, [controllerState, refreshVersions]);

  // --- preview ticket (web-kind iframe surface only) --------------------------
  // D01 wiring: document/spreadsheet/slides versions render through their
  // workbench components (server-converted preview data + the immutable
  // files route), so the isolated-origin ticket is issued for the WEB
  // surface only — the skill's static index.html stays the web kind's
  // preview, never the other kinds'.
  const [ticket, setTicket] = useState<CraftPreviewTicketView | null>(null);
  const [ticketIssuing, setTicketIssuing] = useState(false);
  const [ticketError, setTicketError] = useState<string | null>(null);
  const [ticketNonce, setTicketNonce] = useState(0);
  const issuePreview = useEventCallback(props.onIssuePreview);
  const hasArtifact = selectedVersion !== null && selectedVersion.files.length > 0;
  // Hooks are called UNCONDITIONALLY (D01 review N1): the stable callbacks
  // are always created; absence of the prop is expressed as null at the
  // consumption boundary, which keeps the child prop types ((...) => ...) | null.
  const fetchVersionFileCallback = useEventCallback(props.onFetchVersionFile ?? (async () => ''));
  const resolveVersionFileUrlCallback = useEventCallback(props.onResolveVersionFileUrl ?? (async () => ''));
  const fetchVersionFile = props.onFetchVersionFile === undefined ? null : fetchVersionFileCallback;
  const resolveVersionFileUrl = props.onResolveVersionFileUrl === undefined ? null : resolveVersionFileUrlCallback;
  const previewIsFrame = fetchVersionFile === null || selectedVersion === null || selectedVersion.kind === 'web';
  useEffect(() => {
    // Switching versions (or retrying) destroys the previous ticket and frame;
    // a retry re-issues the PREVIEW ticket only — it never re-runs execution.
    let cancelled = false;
    setTicket(null);
    setTicketError(null);
    if (selectedVersionId === null || !hasArtifact || !previewIsFrame) {
      setTicketIssuing(false);
      return () => { cancelled = true; };
    }
    setTicketIssuing(true);
    void issuePreview(selectedVersionId)
      .then((next) => {
        if (cancelled) return;
        setTicket(next);
        setTicketIssuing(false);
      })
      .catch((error: unknown) => {
        if (cancelled) return;
        setTicketError(error instanceof Error ? error.message : String(error));
        setTicketIssuing(false);
      });
    return () => { cancelled = true; };
  }, [selectedVersionId, hasArtifact, ticketNonce, issuePreview, previewIsFrame]);

  // --- composer ----------------------------------------------------------------
  const [draft, setDraft] = useState(props.initialPrompt ?? '');
  const [sending, setSending] = useState(false);
  const [sendError, setSendError] = useState<string | null>(null);
  const composable = !sending && (!runActive) && mainStatus !== 'waiting_user';
  const handleSend = async (): Promise<void> => {
    const text = draft.trim();
    if (text === '' || sending) return;
    // Archive the finished live turn NOW (B1): the composer only opens when
    // the live run is terminal, so this projection is final, and the log is
    // still populated — the reset that accompanies the new run's subscription
    // can no longer erase the turn.
    if (!liveArchived) {
      const archive = archiveLiveTurn(turns, { runId, prompt: livePrompt, projection });
      if (archive.archived !== null) {
        setTurns(archive.turns);
        setArchivedRunId(runId);
      }
    }
    setSending(true);
    setSendError(null);
    setDraft('');
    setLivePrompt(text);
    try {
      if (props.enrichedPending) {
        await props.onEnrichedSend(text);
      } else {
        await props.controller.submit(text);
      }
    } catch (error) {
      // The send failed: the archived turn stays (it is real history); the
      // failed prompt returns to the composer for an explicit retry.
      setLivePrompt(null);
      setSendError(error instanceof Error ? error.message : String(error));
      setDraft(text);
    } finally {
      setSending(false);
    }
  };

  // --- knowledge sources (C01 panel; data plane is the message log) -------------
  const knowledgePackage = useMemo(() => projectKnowledgeSources(snapshot.events), [snapshot]);
  const [openingCitation, setOpenCitation] = useState<string | null>(null);
  const [sourceNotice, setSourceNotice] = useState<string | null>(null);
  const onOpenSourceProp = useEventCallback((citationId: string, ref: string): Promise<string | null> => {
    if (props.onOpenSource === undefined) return Promise.resolve(null);
    return props.onOpenSource(citationId, ref);
  });
  const openSource = (citationId: string, ref: string): void => {
    setOpenCitation(citationId);
    setSourceNotice(null);
    void onOpenSourceProp(citationId, ref)
      .then((notice) => { setSourceNotice(notice); })
      .catch((error: unknown) => {
        setSourceNotice(error instanceof Error ? error.message : String(error));
      })
      .finally(() => { setOpenCitation(null); });
  };

  // D01: a document citation click resolves the id against the run's
  // knowledge package and reuses the SAME authorized opener — never a
  // cached or pre-signed URL, and an id the package does not carry answers
  // the honest not-visible notice.
  const openDocumentCitation = (citationId: string): void => {
    const hit = knowledgePackage.sources.find((source) => source.citationId === citationId);
    if (hit === undefined) {
      setOpenCitation(citationId);
      setSourceNotice(props.locale === 'zh' ? '引用来源不可见（无权限或已删除）' : 'Cited source is not visible (no permission or deleted)');
      return;
    }
    openSource(citationId, hit.ref);
  };

  // --- interaction answers -------------------------------------------------------
  const [answers, setAnswers] = useState<Record<string, string>>({});
  const onInteractionAction = useEventCallback(props.onInteractionAction);
  const submitInteraction = (card: CraftInteractionCard, action: CraftInteractionAction): void => {
    onInteractionAction({
      id: card.id,
      kind: card.kind,
      prompt: card.prompt,
      action,
      answer: answers[card.id] ?? '',
    });
  };

  // --- downloads -------------------------------------------------------------------
  const [downloadingPath, setDownloadingPath] = useState<string | null>(null);
  const download = useEventCallback(props.onDownload);
  const handleDownload = async (versionId: string, path: string): Promise<void> => {
    setDownloadingPath(path);
    try {
      await download(versionId, path);
    } finally {
      setDownloadingPath(null);
    }
  };

  // --- C05 restore (continue editing from a version) ------------------------------
  const restoreStrings = props.locale === 'zh' ? RESTORE_STRINGS.zh : RESTORE_STRINGS.en;
  const [restoring, setRestoring] = useState(false);
  const [restoreError, setRestoreError] = useState<string | null>(null);
  const restoreVersion = useEventCallback(props.onRestoreVersion ?? (async () => undefined));
  const restoreEntryVisible = props.onRestoreVersion !== undefined && props.canWrite;
  const restorableIds = props.restorableVersionIds ?? [];
  const selectedRestorable = selectedVersionId !== null && restorableIds.includes(selectedVersionId);
  const restoreBlockedReason = selectedVersionId === null
    ? restoreStrings.reasonUnselected
    : runActive || mainStatus === 'waiting_user'
      ? restoreStrings.reasonRunActive
      : selectedRestorable
        ? null
        : restoreStrings.reasonNoSnapshot;
  const handleRestore = async (): Promise<void> => {
    if (selectedVersionId === null || restoring || restoreBlockedReason !== null) return;
    setRestoring(true);
    setRestoreError(null);
    try {
      await restoreVersion(selectedVersionId);
    } catch (error) {
      setRestoreError(error instanceof Error ? error.message : String(error));
    } finally {
      setRestoring(false);
    }
  };

  // --- terminal (optional, collapsed, owners only) -----------------------------------
  const [terminalOpen, setTerminalOpen] = useState(false);
  const [terminalUrl, setTerminalUrl] = useState<string | null>(null);
  const [terminalError, setTerminalError] = useState<string | null>(null);
  const mintTerminal = useEventCallback(props.onMintTerminalUrl);
  useEffect(() => {
    if (!terminalOpen || !props.canWrite || terminalUrl !== null || terminalError !== null) return;
    let cancelled = false;
    void mintTerminal()
      .then((url) => { if (!cancelled) setTerminalUrl(url); })
      .catch((error: unknown) => { if (!cancelled) setTerminalError(error instanceof Error ? error.message : String(error)); });
    return () => { cancelled = true; };
  }, [terminalOpen, props.canWrite, terminalUrl, terminalError, mintTerminal]);

  // --- layout: tabs, resize, narrow ------------------------------------------------
  const [sideTab, setSideTab] = useState<SideTab>('preview');
  const [narrowTab, setNarrowTab] = useState<NarrowTab>('conversation');
  const [leftWidth, setLeftWidth] = useState(44);
  const [narrow, setNarrow] = useState(() => typeof window !== 'undefined' && window.matchMedia('(max-width: 960px)').matches);
  useEffect(() => {
    const query = window.matchMedia('(max-width: 960px)');
    const onChange = () => setNarrow(query.matches);
    query.addEventListener('change', onChange);
    return () => query.removeEventListener('change', onChange);
  }, []);

  const splitterRef = useRef<HTMLDivElement | null>(null);
  const onSplitterKeyDown = (event: React.KeyboardEvent<HTMLDivElement>): void => {
    if (event.key === 'ArrowLeft') { event.preventDefault(); setLeftWidth((w) => Math.max(25, w - 4)); }
    else if (event.key === 'ArrowRight') { event.preventDefault(); setLeftWidth((w) => Math.min(75, w + 4)); }
    else if (event.key === 'Home') { event.preventDefault(); setLeftWidth(25); }
    else if (event.key === 'End') { event.preventDefault(); setLeftWidth(75); }
  };
  const onSplitterPointerDown = (event: React.PointerEvent<HTMLDivElement>): void => {
    event.preventDefault();
    const node = splitterRef.current;
    if (node === null) return;
    node.setPointerCapture(event.pointerId);
  };
  const onSplitterPointerMove = (event: React.PointerEvent<HTMLDivElement>): void => {
    const node = splitterRef.current;
    if (node === null || !node.hasPointerCapture(event.pointerId)) return;
    const bounds = node.parentElement?.getBoundingClientRect();
    if (bounds === undefined || bounds.width === 0) return;
    const ratio = ((event.clientX - bounds.left) / bounds.width) * 100;
    setLeftWidth(Math.min(75, Math.max(25, ratio)));
  };

  const selectVersion = (versionId: string): void => {
    setSelectedVersionId(versionId);
    setPinnedVersion(true);
  };

  const statusLabel = statusLabelLocalized(props.locale, mainStatus, liveProjection.childStatus, props.snapshotVersionId !== null);
  const liveTurnEmpty = livePrompt === null && liveProjection.text === '' && liveProjection.tools.length === 0 && liveProjection.interactions.length === 0 && props.versions.length === 0 && turns.length === 0;

  const conversation = (
    <>
      <div className="wk-craft-msgs" aria-label={strings.craftTabConversation}>
        {turns.map((turn, index) => (
          <div key={'turn-' + index}>
            {turn.prompt !== null ? <div className="wk-craft-msg wk-craft-msg-user">{turn.prompt}</div> : null}
            <div className="wk-craft-msg wk-craft-msg-assistant">{turn.assistant.text}</div>
          </div>
        ))}
        {props.resumedRun && turns.length === 0 && livePrompt === null ? (
          <p className="wk-craft-hint">{strings.craftConversationResumed}</p>
        ) : null}
        {livePrompt !== null ? <div className="wk-craft-msg wk-craft-msg-user">{livePrompt}</div> : null}
        <div className="wk-craft-msg wk-craft-msg-assistant">
          {liveProjection.text !== '' ? liveProjection.text : liveTurnEmpty ? strings.craftConversationEmpty : null}
          {!liveProjection.complete && liveProjection.text !== '' ? <span className="wk-craft-msg-cursor" aria-hidden="true"> ▍</span> : null}
        </div>
        {liveProjection.tools.length > 0 || liveProjection.childStatus !== 'idle' ? (
          <details className="wk-craft-card">
            <summary>{strings.craftDetailsChildStatus}: {liveProjection.childStatus}</summary>
            <ul>
              {liveProjection.tools.map((tool) => (
                <li key={tool.seq}><code>{tool.tool}</code> — {tool.status}</li>
              ))}
            </ul>
          </details>
        ) : null}
        {liveProjection.artifactVersionIds.map((versionId) => (
          <div key={versionId} className="wk-craft-card">
            <p style={{ margin: 0 }}>{strings.craftVersionLabel}: <code>{versionId}</code></p>
          </div>
        ))}
        {liveProjection.workspaceUnavailable ? <p className="wk-craft-error" role="alert">{strings.craftErrorTitle}: workspace unavailable</p> : null}
        {liveProjection.interactions.map((card) => (
          <InteractionCardView
            key={card.id}
            card={card}
            strings={strings}
            answer={answers[card.id] ?? ''}
            onAnswerChange={(text) => setAnswers((prev) => ({ ...prev, [card.id]: text }))}
            onAction={(action) => submitInteraction(card, action)}
          />
        ))}
      </div>
      <form
        className="wk-craft-composer"
        onSubmit={(event) => {
          event.preventDefault();
          void handleSend();
        }}
      >
        {props.pendingAttachments.length > 0 ? (
          <ul className="wk-craft-chips">
            {props.pendingAttachments.map((attachment) => (
              <li key={attachment.id} className="wk-craft-chip" data-state={attachment.state}>
                <span>{attachment.name}</span>
                <button type="button" onClick={() => props.onRemoveAttachment(attachment.id)} aria-label={strings.craftRetry + ': ' + attachment.name}>×</button>
              </li>
            ))}
          </ul>
        ) : null}
        <label className="wk-craft-hint" htmlFor="craft-composer-prompt">{strings.craftComposerPlaceholder}</label>
        <textarea
          id="craft-composer-prompt"
          data-testid="craft-prompt"
          value={draft}
          placeholder={strings.craftComposerPlaceholder}
          onChange={(event) => setDraft(event.target.value)}
          disabled={sending}
        />
        <div className="wk-craft-actions">
          <input
            data-testid="craft-upload"
            type="file"
            multiple
            aria-label={strings.craftUploadLabel}
            onChange={(event) => {
              props.onPickAttachments(Array.from(event.target.files ?? []));
              event.target.value = '';
            }}
          />
          <Button type="submit" data-testid="craft-send" disabled={!composable || draft.trim() === ''}>
            {sending ? strings.craftSendBusy : strings.craftSend}
          </Button>
        </div>
        {sendError !== null ? <p className="wk-craft-error" role="alert">{sendError}</p> : null}
        {runActive || mainStatus === 'waiting_user' ? (
          <p className="wk-craft-hint">{statusLabel}</p>
        ) : null}
      </form>
    </>
  );

  const sidePanel = sideTab === 'preview' ? (
    <CraftKindPreview
      locale={props.locale}
      selectedVersion={selectedVersion}
      selectedVersionId={selectedVersionId}
      fetchVersionFile={fetchVersionFile}
      resolveVersionFileUrl={resolveVersionFileUrl}
      onDownload={(versionId, path) => void handleDownload(versionId, path)}
      downloading={downloadingPath !== null}
      onOpenCitation={openDocumentCitation}
      onRequestSlidesChange={(page, versionId) => {
        setDraft(props.locale === 'zh'
          ? '请只修改演示稿第 ' + page + ' 页，其余页面保持不变（当前版本 ' + versionId + '）。'
          : 'Modify only page ' + page + ' of the deck and keep every other page unchanged (current version ' + versionId + ').');
      }}
      webSurface={(
        <CraftPreview
          locale={props.locale}
          versionId={selectedVersionId}
          hasArtifact={hasArtifact}
          ticket={ticket}
          ticketIssuing={ticketIssuing}
          ticketError={ticketError}
          onIssueTicket={() => setTicketNonce((nonce) => nonce + 1)}
        />
      )}
    />
  ) : sideTab === 'files' ? (
    <CraftFiles
      locale={props.locale}
      version={selectedVersion}
      versions={versions}
      currentVersionId={effectiveVersionId}
      selectedVersionId={selectedVersionId}
      sessionUpdatedAt={props.sessionUpdatedAt}
      downloadingPath={downloadingPath}
      onSelectVersion={selectVersion}
      onDownload={(versionId, path) => void handleDownload(versionId, path)}
    />
  ) : (
    <div className="wk-craft-panel-body">
      <h3>{strings.craftDetailsEvents}</h3>
      {snapshot.events.length === 0 ? (
        <p className="wk-craft-muted">{strings.craftDetailsEmpty}</p>
      ) : (
        <ul className="wk-craft-list">
          {snapshot.events.map((event) => (
            <li key={event.seq}>
              <span className="wk-craft-item-meta">#{event.seq} {event.kind}</span>
            </li>
          ))}
        </ul>
      )}
      <CraftSources
        locale={props.locale}
        sources={knowledgePackage.sources}
        truncated={knowledgePackage.truncated}
        openingCitation={openingCitation}
        onOpenSource={openSource}
      />
      {sourceNotice !== null ? <p className="wk-craft-hint" data-testid="craft-source-notice" role="status">{sourceNotice}</p> : null}
      <h3>{strings.craftDetailsTools}</h3>
      {projection.tools.length === 0 ? (
        <p className="wk-craft-muted">{strings.craftDetailsEmpty}</p>
      ) : (
        <table className="wk-craft-table">
          <thead><tr><th scope="col">#</th><th scope="col">tool</th><th scope="col">status</th></tr></thead>
          <tbody>
            {projection.tools.map((tool) => (
              <tr key={tool.seq}><td>{tool.seq}</td><td><code>{tool.tool}</code></td><td>{tool.status}</td></tr>
            ))}
          </tbody>
        </table>
      )}
      {selectedVersion !== null && selectedVersion.checks.length > 0 ? (
        <>
          <h3>{strings.craftHistoryChecks}</h3>
          <ul className="wk-craft-list">
            {selectedVersion.checks.map((check) => (
              <li key={check.name}>
                <span className="wk-craft-check" data-status={check.status}>{check.name}</span>
                <span className="wk-craft-hint">{check.detail === '' ? check.status : check.detail}</span>
              </li>
            ))}
          </ul>
        </>
      ) : null}
      {props.canWrite ? (
        <div className="wk-craft-card">
          <button type="button" className="wk-craft-tab" aria-expanded={terminalOpen} onClick={() => setTerminalOpen((open) => !open)}>
            {strings.craftTerminalToggle}
          </button>
          <p className="wk-craft-hint">{strings.craftTerminalHint}</p>
          {terminalOpen ? (
            terminalUrl !== null ? (
              <TerminalView url={terminalUrl} />
            ) : terminalError !== null ? (
              <p className="wk-craft-error" role="alert">{terminalError}</p>
            ) : (
              <p className="wk-craft-muted">{strings.craftLoading}</p>
            )
          ) : null}
        </div>
      ) : null}
    </div>
  );

  const tabButton = (tab: NarrowTab, label: string): React.JSX.Element => {
    const selected = narrow ? narrowTab === tab : sideTab === tab;
    return (
      <button
        type="button"
        role="tab"
        className="wk-craft-tab"
        aria-selected={selected}
        onClick={() => (narrow ? setNarrowTab(tab) : setSideTab(tab as SideTab))}
      >
        {label}
      </button>
    );
  };

  return (
    <main className="wk-craft wk-craft-page" aria-label={props.title || strings.craftHomeTitle}>
      <div className="wk-craft-head">
        <div>
          <h1>{props.title === '' ? props.sessionId : props.title}</h1>
          <p className="wk-craft-muted"><span className="wk-craft-kind">{props.kind}</span> <code>{props.sessionId}</code></p>
        </div>
        <div className="wk-craft-actions">
          <Button type="button" onClick={props.onBack}>{strings.craftWorkbenchBack}</Button>
        </div>
      </div>
      {props.syncError !== null ? <p className="wk-craft-error" role="alert">{strings.craftStreamReconnecting} ({props.syncError})</p> : null}
      {restoreEntryVisible && restoreBlockedReason !== null && selectedVersionId !== null && !runActive && mainStatus !== 'waiting_user' ? (
        <p className="wk-craft-hint" data-testid="craft-restore-reason">{restoreBlockedReason}</p>
      ) : null}
      {restoreError !== null ? <p className="wk-craft-error" role="alert">{restoreStrings.failed}: {restoreError}</p> : null}
      <div className="wk-craft-bench">
        <div className="wk-craft-topbar">
          <span className="wk-craft-title">{props.title === '' ? props.sessionId : props.title}</span>
          <span
            className="wk-craft-status"
            data-testid="craft-main-status"
            data-terminal={isMainRunTerminal(mainStatus) && mainStatus !== 'failed'}
            data-failed={mainStatus === 'failed'}
            aria-live="polite"
          >
            {statusLabel}
          </span>
          <select
            className="wk-craft-version-select"
            data-testid="craft-version"
            aria-label={strings.craftVersionLabel}
            value={selectedVersionId ?? ''}
            onChange={(event) => selectVersion(event.target.value)}
          >
            {versions.length === 0 ? <option value="">{strings.craftVersionNone}</option> : null}
            {versions.map((version) => (
              <option key={version.id} value={version.id}>{version.id === effectiveVersionId ? version.id + ' · ' + strings.craftHistoryCurrent : version.id}</option>
            ))}
          </select>
          <Button
            type="button"
            data-testid="craft-download"
            disabled={entryFile === null || selectedVersionId === null || downloadingPath !== null}
            onClick={() => {
              if (entryFile !== null && selectedVersionId !== null) void handleDownload(selectedVersionId, entryFile.path);
            }}
          >
            {downloadingPath !== null ? strings.craftDownloading : strings.craftDownload + (entryFile !== null ? ' · ' + downloadFileName(entryFile.path) + ' · ' + formatBytes(entryFile.bytes) : '')}
          </Button>
          {restoreEntryVisible ? (
            <Button
              type="button"
              data-testid="craft-restore"
              disabled={restoring || restoreBlockedReason !== null}
              title={restoreBlockedReason ?? restoreStrings.action}
              onClick={() => {
                void handleRestore();
              }}
            >
              {restoring ? restoreStrings.busy : restoreStrings.action}
            </Button>
          ) : null}
          <Button type="button" onClick={() => (narrow ? setNarrowTab('files') : setSideTab('files'))}>{strings.craftHistory}</Button>
        </div>
        {narrow ? (
          <div className="wk-craft-narrow-tabs" role="tablist" aria-label={strings.craftHomeTitle}>
            {tabButton('conversation', strings.craftTabConversation)}
            {tabButton('preview', strings.craftTabPreview)}
            {tabButton('files', strings.craftTabFiles)}
            {tabButton('details', strings.craftTabDetails)}
          </div>
        ) : null}
        <div className="wk-craft-split" style={{ '--wk-craft-left': leftWidth + '%' } as CSSProperties}>
          <section
            className="wk-craft-pane wk-craft-pane-left"
            aria-label={strings.craftTabConversation}
            data-narrow-hidden={narrow && narrowTab !== 'conversation'}
          >
            {conversation}
          </section>
          {!narrow ? (
            <div
              ref={splitterRef}
              className="wk-craft-splitter"
              role="separator"
              aria-orientation="vertical"
              aria-label={strings.craftTabConversation + ' / ' + strings.craftTabPreview}
              aria-valuenow={Math.round(leftWidth)}
              aria-valuemin={25}
              aria-valuemax={75}
              tabIndex={0}
              onKeyDown={onSplitterKeyDown}
              onPointerDown={onSplitterPointerDown}
              onPointerMove={onSplitterPointerMove}
            />
          ) : null}
          <section
            className="wk-craft-pane"
            aria-label={strings.craftTabPreview + ' / ' + strings.craftTabFiles + ' / ' + strings.craftTabDetails}
            data-narrow-hidden={narrow && narrowTab === 'conversation'}
          >
            {!narrow ? (
              <div className="wk-craft-tabs wk-craft-side-tabs" role="tablist">
                {tabButton('preview', strings.craftTabPreview)}
                {tabButton('files', strings.craftTabFiles)}
                {tabButton('details', strings.craftTabDetails)}
              </div>
            ) : null}
            <div className="wk-craft-panel">{sidePanel}</div>
          </section>
        </div>
      </div>
    </main>
  );
}

// ---------------------------------------------------------------------------
// Interaction card (R06: question ≠ permission; no generic Approve)
// ---------------------------------------------------------------------------

function InteractionCardView(props: {
  card: CraftInteractionCard;
  strings: ReturnType<typeof craftStrings>;
  answer: string;
  onAnswerChange(text: string): void;
  onAction(action: CraftInteractionAction): void;
}) {
  const { card, strings } = props;
  const title = card.kind === 'question'
    ? strings.craftInteractionQuestion
    : card.kind === 'permission'
      ? strings.craftInteractionPermission
      : strings.craftInteractionUnknown;
  return (
    <div className="wk-craft-interaction" data-resolved={card.resolved}>
      <h4>{title}</h4>
      {card.prompt !== '' ? <p style={{ margin: '0.2rem 0' }}>{card.prompt}</p> : null}
      {card.kind === 'permission' && card.scope !== '' ? (
        <>
          <p className="wk-craft-hint" style={{ margin: '0.2rem 0' }}>{strings.craftInteractionScope}</p>
          <pre className="wk-craft-scope" style={{ margin: '0.2rem 0' }}>{card.scope}</pre>
        </>
      ) : null}
      {card.resolved ? (
        <p className="wk-craft-hint" style={{ margin: '0.2rem 0' }}>{strings.craftInteractionResolved}</p>
      ) : (
        <>
          {card.allowedActions.includes('answer') ? (
            <textarea
              aria-label={strings.craftInteractionAnswerPlaceholder}
              placeholder={strings.craftInteractionAnswerPlaceholder}
              value={props.answer}
              onChange={(event) => props.onAnswerChange(event.target.value)}
            />
          ) : null}
          <div className="wk-craft-actions">
            {card.allowedActions.includes('answer') ? (
              <Button type="button" disabled={props.answer.trim() === ''} onClick={() => props.onAction('answer')}>
                {strings.craftInteractionSubmitAnswer}
              </Button>
            ) : null}
            {card.allowedActions.includes('approve') ? (
              <Button type="button" onClick={() => props.onAction('approve')}>{strings.craftInteractionApprove}</Button>
            ) : null}
            {card.allowedActions.includes('reject') ? (
              <Button type="button" onClick={() => props.onAction('reject')}>{strings.craftInteractionReject}</Button>
            ) : null}
          </div>
        </>
      )}
    </div>
  );
}

// ---------------------------------------------------------------------------
// Minimal sandbox terminal view (reuses the authorized ticketed WS entry)
// ---------------------------------------------------------------------------

function TerminalView(props: { url: string }) {
  const [output, setOutput] = useState('');
  const [status, setStatus] = useState<'connecting' | 'open' | 'closed' | 'error'>('connecting');
  const [input, setInput] = useState('');
  const socketRef = useRef<WebSocket | null>(null);
  useEffect(() => {
    const socket = new WebSocket(props.url);
    socket.binaryType = 'arraybuffer';
    socketRef.current = socket;
    setStatus('connecting');
    socket.onopen = () => setStatus('open');
    socket.onmessage = (event: MessageEvent) => {
      if (typeof event.data === 'string') {
        setOutput((prev) => (prev + event.data + '\n').slice(-65536));
      } else {
        const text = new TextDecoder().decode(event.data as ArrayBuffer);
        setOutput((prev) => (prev + text).slice(-65536));
      }
    };
    socket.onclose = () => setStatus('closed');
    socket.onerror = () => setStatus('error');
    return () => {
      socket.close();
      socketRef.current = null;
    };
  }, [props.url]);
  const send = (): void => {
    const socket = socketRef.current;
    if (socket !== null && socket.readyState === WebSocket.OPEN && input !== '') {
      socket.send(new TextEncoder().encode(input + '\r'));
      setInput('');
    }
  };
  return (
    <div>
      <div className="wk-craft-terminal" role="log" aria-label="sandbox terminal">{output === '' ? status : output}</div>
      <div className="wk-craft-terminal-input">
        <input
          value={input}
          aria-label="terminal input"
          onChange={(event) => setInput(event.target.value)}
          onKeyDown={(event) => {
            if (event.key === 'Enter') {
              event.preventDefault();
              send();
            }
          }}
        />
        <Button type="button" onClick={send}>↵</Button>
      </div>
    </div>
  );
}
// ---------------------------------------------------------------------------
// Kind preview surface (D01 wiring): the preview tab renders the kind's OWN
// component from server-verified data — the document view reads the
// editable Markdown source, the spreadsheet/slides views read the skill's
// preview.json — while the isolated-origin iframe stays the WEB kind's
// surface (the skill's static index.html). One preview surface per kind,
// decided by the selected version's kind, never by the session's.
// ---------------------------------------------------------------------------

/** One immutable version member's text, fetched through the assembly's
 * authorized files route. Reset on version/path change; reload is
 * preview-only and never re-runs generation. */
function useVersionMemberText(
  versionId: string | null,
  path: string,
  fetcher: ((versionId: string, path: string) => Promise<string>) | null,
): { text: string | null; error: string | null; reload(): void } {
  const [text, setText] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [nonce, setNonce] = useState(0);
  useEffect(() => {
    let cancelled = false;
    setText(null);
    setError(null);
    if (versionId === null || fetcher === null) {
      return () => { cancelled = true; };
    }
    void fetcher(versionId, path)
      .then((value) => { if (!cancelled) setText(value); })
      .catch((err: unknown) => { if (!cancelled) setError(err instanceof Error ? err.message : String(err)); });
    return () => { cancelled = true; };
  }, [versionId, path, nonce, fetcher]);
  return { text, error, reload: () => setNonce((n) => n + 1) };
}

const SLIDES_IMAGE_PLACEHOLDER =
  'data:image/svg+xml;utf8,' + encodeURIComponent('<svg xmlns="http://www.w3.org/2000/svg" width="4" height="3"/>');

function CraftKindPreview(props: {
  locale: CraftLocale;
  selectedVersion: CraftVersionView | null;
  selectedVersionId: string | null;
  fetchVersionFile: ((versionId: string, path: string) => Promise<string>) | null;
  resolveVersionFileUrl: ((versionId: string, path: string) => Promise<string>) | null;
  onDownload(versionId: string, path: string): void;
  downloading: boolean;
  onOpenCitation(citationId: string): void;
  onRequestSlidesChange(page: number, versionId: string): void;
  webSurface: React.JSX.Element;
}) {
  const kind = props.selectedVersion?.kind ?? 'web';
  const member = useVersionMemberText(
    props.selectedVersionId,
    kind === 'document' ? CRAFT_DOCUMENT_MD_PATH : 'preview.json',
    props.fetchVersionFile,
  );
  const manifestMember = useVersionMemberText(props.selectedVersionId, 'manifest.json', kind === 'document' ? props.fetchVersionFile : null);
  const [imageUrls, setImageUrls] = useState<Record<string, string>>({});

  // Parse the shared preview member per kind (pure; invalid JSON becomes a
  // preview error the component renders as failed, never as success).
  const previewJsonError = member.text !== null && member.error === null
    ? (() => { try { JSON.parse(member.text ?? ''); return null; } catch { return 'preview.json is not valid JSON'; } })()
    : null;
  const spreadsheetPreview = useMemo<CraftSpreadsheetPreviewView | null>(() => {
    if (kind !== 'spreadsheet' || member.text === null) return null;
    try {
      const parsed = JSON.parse(member.text) as CraftSpreadsheetPreviewView;
      return parsed.kind === 'spreadsheet' ? parsed : null;
    } catch { return null; }
  }, [kind, member.text]);
  const slidesPreview = useMemo<CraftSlidesPreviewView | null>(() => {
    if (kind !== 'slides' || member.text === null) return null;
    try {
      const parsed = JSON.parse(member.text) as CraftSlidesPreviewView;
      return parsed.kind === 'slides' ? parsed : null;
    } catch { return null; }
  }, [kind, member.text]);

  // Slides page images resolve through the assembly's authorized route into
  // object URLs; until one arrives the thumbnail renders a placeholder.
  useEffect(() => {
    let cancelled = false;
    setImageUrls({});
    const pages = slidesPreview?.pages ?? [];
    if (props.resolveVersionFileUrl === null || props.selectedVersionId === null || pages.length === 0) {
      return () => { cancelled = true; };
    }
    for (const page of pages) {
      void props.resolveVersionFileUrl(props.selectedVersionId, page.image)
        .then((url) => { if (!cancelled) setImageUrls((prev) => ({ ...prev, [page.image]: url })); })
        .catch(() => { /* the placeholder stays; the alt text carries the title */ });
    }
    return () => { cancelled = true; };
  }, [slidesPreview, props.selectedVersionId, props.resolveVersionFileUrl]);

  if (props.fetchVersionFile === null) {
    // Assembly without the authorized member fetch: the web iframe remains
    // the only preview surface (the pre-D01 behavior).
    return props.webSurface;
  }
  if (kind === 'document') {
    let manifestCitations: string[] = [];
    if (manifestMember.text !== null) {
      try {
        const parsed = JSON.parse(manifestMember.text) as { citation_ids?: unknown };
        if (Array.isArray(parsed.citation_ids)) {
          manifestCitations = parsed.citation_ids.filter((id): id is string => typeof id === 'string');
        }
      } catch { /* the markdown scan still collects the citations */ }
    }
    return (
      <CraftDocument
        locale={props.locale}
        versionId={props.selectedVersionId}
        markdown={member.text}
        markdownLoading={member.error === null && member.text === null}
        markdownError={member.error}
        manifestCitations={manifestCitations}
        onFetchDocument={() => member.reload()}
        onDownload={props.onDownload}
        downloading={props.downloading}
        onOpenCitation={props.onOpenCitation}
      />
    );
  }
  const previewError = member.error ?? previewJsonError;
  if (kind === 'spreadsheet') {
    return (
      <CraftSpreadsheet
        locale={props.locale}
        versionId={props.selectedVersionId}
        preview={spreadsheetPreview}
        previewLoading={member.error === null && member.text === null}
        previewError={previewError}
        onFetchPreview={() => member.reload()}
        onDownload={props.onDownload}
        downloading={props.downloading}
      />
    );
  }
  if (kind === 'slides') {
    return (
      <CraftSlides
        locale={props.locale}
        versionId={props.selectedVersionId}
        preview={slidesPreview}
        previewLoading={member.error === null && member.text === null}
        previewError={previewError}
        onFetchPreview={() => member.reload()}
        onResolvePageImage={(_versionId, path) => imageUrls[path] ?? SLIDES_IMAGE_PLACEHOLDER}
        onDownload={props.onDownload}
        downloading={props.downloading}
        onRequestChange={props.onRequestSlidesChange}
      />
    );
  }
  return props.webSurface;
}
