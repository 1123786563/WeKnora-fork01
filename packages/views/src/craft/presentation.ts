// W05 craft workbench presentation logic.
//
// Every rule the craft views render lives here as a pure function so the
// workbench components stay dumb: they receive data + callbacks and project
// through these helpers. Two invariants are load-bearing:
//
//   1. statusLabel is the single wording authority for the main-run status
//      (brief Step 3, verbatim): a finished child delegation while the main
//      run is still running reads "主 Agent 正在检查结果".
//   2. projectAssistant marks the assistant message complete ONLY when the
//      main run projection is terminal (CRAFT_TERMINAL_RUN_STATUSES).
//      delegation.finished changes nothing here — a child event can never
//      finish the main message, mirroring the W04 domain reducer.
import {
  CRAFT_TERMINAL_RUN_STATUSES,
  parseCraftEventPayload,
  parseCraftRunEvent,
  type CraftFileVersionView,
  type CraftVersionCheckView,
  type CraftVersionView,
} from '@weknora/contracts';
import type { CraftEventFrame } from '@weknora/core/craft/controller';

// ---------------------------------------------------------------------------
// Locale + strings
// ---------------------------------------------------------------------------

export type CraftLocale = 'zh' | 'en';

export interface CraftStrings {
  craftHomeTitle: string;
  craftHomeSubtitle: string;
  craftNewGoalLabel: string;
  craftNewGoalPlaceholder: string;
  craftKindLabel: string;
  craftKnowledgeScopeLabel: string;
  craftKnowledgeNone: string;
  craftAttachmentsLabel: string;
  craftUploadLabel: string;
  craftUploadHint: string;
  craftAttachmentUploading: string;
  craftAttachmentReady: string;
  craftAttachmentFailed: string;
  craftCreate: string;
  craftCreateBusy: string;
  craftLoginRequired: string;
  craftRecentTitle: string;
  craftRecentEmpty: string;
  craftRecentLoading: string;
  craftNextPage: string;
  craftPrevPage: string;
  craftRetry: string;
  craftLoading: string;
  craftOpen: string;
  craftWorkbenchBack: string;
  craftTabConversation: string;
  craftTabPreview: string;
  craftTabFiles: string;
  craftTabDetails: string;
  craftDownload: string;
  craftHistory: string;
  craftVersionLabel: string;
  craftVersionNone: string;
  craftDownloading: string;
  craftPreviewLoading: string;
  craftPreviewExpired: string;
  craftPreviewFailed: string;
  craftPreviewEmpty: string;
  craftPreviewRefresh: string;
  craftPreviewExpiresAt: string;
  craftFilesPath: string;
  craftFilesSize: string;
  craftFilesDownload: string;
  craftFilesEmpty: string;
  craftHistoryRun: string;
  craftHistoryTime: string;
  craftHistoryChecks: string;
  craftHistoryCurrent: string;
  craftHistoryTimeUnavailable: string;
  craftCheckPassed: string;
  craftCheckFailed: string;
  craftCheckNotRun: string;
  craftDetailsTools: string;
  craftDetailsEvents: string;
  craftDetailsEmpty: string;
  craftDetailsChildStatus: string;
  craftTerminalToggle: string;
  craftTerminalHint: string;
  craftComposerPlaceholder: string;
  craftSend: string;
  craftSendBusy: string;
  craftConversationEmpty: string;
  craftConversationResumed: string;
  craftInteractionQuestion: string;
  craftInteractionPermission: string;
  craftInteractionUnknown: string;
  craftInteractionAnswerPlaceholder: string;
  craftInteractionSubmitAnswer: string;
  craftInteractionApprove: string;
  craftInteractionReject: string;
  craftInteractionResolved: string;
  craftInteractionScope: string;
  craftErrorTitle: string;
  craftStreamReconnecting: string;
  craftLocaleToggle: string;
}

const STRINGS_ZH: CraftStrings = {
  craftHomeTitle: 'Craft 创作',
  craftHomeSubtitle: '用一句话描述目标，Agent 生成可预览、可修改、可下载的作品。',
  craftNewGoalLabel: '新建作品目标',
  craftNewGoalPlaceholder: '例如：按月分析销售数据，生成可筛选的网页报告',
  craftKindLabel: '作品类型',
  craftKnowledgeScopeLabel: '知识范围',
  craftKnowledgeNone: '不使用知识库',
  craftAttachmentsLabel: '参考附件',
  craftUploadLabel: '上传附件',
  craftUploadHint: '上传后由 Agent 在沙箱内只读使用',
  craftAttachmentUploading: '上传中…',
  craftAttachmentReady: '已就绪',
  craftAttachmentFailed: '上传失败',
  craftCreate: '创建作品',
  craftCreateBusy: '创建中…',
  craftLoginRequired: '请先登录后再创建作品。',
  craftRecentTitle: '最近作品',
  craftRecentEmpty: '还没有作品，从上面创建第一个吧。',
  craftRecentLoading: '正在加载…',
  craftNextPage: '下一页',
  craftPrevPage: '上一页',
  craftRetry: '重试',
  craftLoading: '加载中…',
  craftOpen: '打开',
  craftWorkbenchBack: '返回作品列表',
  craftTabConversation: '对话',
  craftTabPreview: '预览',
  craftTabFiles: '文件',
  craftTabDetails: '执行详情',
  craftDownload: '下载',
  craftHistory: '版本历史',
  craftVersionLabel: '当前版本',
  craftVersionNone: '暂无版本',
  craftDownloading: '下载中…',
  craftPreviewLoading: '正在签发预览…',
  craftPreviewExpired: '预览已过期，请重新打开预览。',
  craftPreviewFailed: '预览不可用，可重试预览（不会重新执行生成）。',
  craftPreviewEmpty: '还没有可预览的作品产物。',
  craftPreviewRefresh: '重新打开预览',
  craftPreviewExpiresAt: '预览有效期至',
  craftFilesPath: '文件',
  craftFilesSize: '大小',
  craftFilesDownload: '下载此文件',
  craftFilesEmpty: '此版本没有文件。',
  craftHistoryRun: '执行 Run',
  craftHistoryTime: '生成时间',
  craftHistoryChecks: '检查事实',
  craftHistoryCurrent: '当前版本',
  craftHistoryTimeUnavailable: '—（版本时间待后端契约提供）',
  craftCheckPassed: '通过',
  craftCheckFailed: '未通过',
  craftCheckNotRun: '未执行',
  craftDetailsTools: '工具调用',
  craftDetailsEvents: '事件流',
  craftDetailsEmpty: '暂无执行事件。',
  craftDetailsChildStatus: '子执行状态',
  craftTerminalToggle: '打开沙箱终端（可选）',
  craftTerminalHint: '终端复用会话沙箱授权入口，默认收起。',
  craftComposerPlaceholder: '输入修改要求…',
  craftSend: '发送',
  craftSendBusy: '执行中…',
  craftConversationEmpty: '描述你的目标，Agent 会在这里汇报进展。',
  craftConversationResumed: '已恢复当前执行的实时消息流（历史消息不在本次快照中）。',
  craftInteractionQuestion: 'Agent 提问',
  craftInteractionPermission: '权限申请',
  craftInteractionUnknown: '等待处理',
  craftInteractionAnswerPlaceholder: '输入你的回答…',
  craftInteractionSubmitAnswer: '提交回答',
  craftInteractionApprove: '同意此范围',
  craftInteractionReject: '拒绝',
  craftInteractionResolved: '已处理',
  craftInteractionScope: '申请范围',
  craftErrorTitle: '出错了',
  craftStreamReconnecting: '连接中断，正在恢复事件流…',
  craftLocaleToggle: 'EN / 中文',
};

const STRINGS_EN: CraftStrings = {
  craftHomeTitle: 'Craft Studio',
  craftHomeSubtitle: 'Describe a goal in one sentence; the agent produces a work you can preview, revise and download.',
  craftNewGoalLabel: 'New work goal',
  craftNewGoalPlaceholder: 'e.g. analyze monthly sales and build a filterable web report',
  craftKindLabel: 'Work kind',
  craftKnowledgeScopeLabel: 'Knowledge scope',
  craftKnowledgeNone: 'No knowledge base',
  craftAttachmentsLabel: 'Attachments',
  craftUploadLabel: 'Upload attachment',
  craftUploadHint: 'Attachments are staged read-only inside the sandbox',
  craftAttachmentUploading: 'Uploading…',
  craftAttachmentReady: 'Ready',
  craftAttachmentFailed: 'Upload failed',
  craftCreate: 'Create work',
  craftCreateBusy: 'Creating…',
  craftLoginRequired: 'Please sign in before creating a work.',
  craftRecentTitle: 'Recent works',
  craftRecentEmpty: 'No works yet — create your first one above.',
  craftRecentLoading: 'Loading…',
  craftNextPage: 'Next page',
  craftPrevPage: 'Previous page',
  craftRetry: 'Retry',
  craftLoading: 'Loading…',
  craftOpen: 'Open',
  craftWorkbenchBack: 'Back to works',
  craftTabConversation: 'Chat',
  craftTabPreview: 'Preview',
  craftTabFiles: 'Files',
  craftTabDetails: 'Details',
  craftDownload: 'Download',
  craftHistory: 'Version history',
  craftVersionLabel: 'Version',
  craftVersionNone: 'No version',
  craftDownloading: 'Downloading…',
  craftPreviewLoading: 'Issuing a preview ticket…',
  craftPreviewExpired: 'The preview ticket expired. Reopen the preview.',
  craftPreviewFailed: 'Preview unavailable. Retry reopens the preview without re-running generation.',
  craftPreviewEmpty: 'No published artifact to preview yet.',
  craftPreviewRefresh: 'Reopen preview',
  craftPreviewExpiresAt: 'Preview valid until',
  craftFilesPath: 'File',
  craftFilesSize: 'Size',
  craftFilesDownload: 'Download this file',
  craftFilesEmpty: 'This version has no files.',
  craftHistoryRun: 'Run',
  craftHistoryTime: 'Generated at',
  craftHistoryChecks: 'Check facts',
  craftHistoryCurrent: 'Current version',
  craftHistoryTimeUnavailable: '— (per-version time pending backend contract)',
  craftCheckPassed: 'passed',
  craftCheckFailed: 'failed',
  craftCheckNotRun: 'not run',
  craftDetailsTools: 'Tool calls',
  craftDetailsEvents: 'Event stream',
  craftDetailsEmpty: 'No execution events yet.',
  craftDetailsChildStatus: 'Child execution',
  craftTerminalToggle: 'Open sandbox terminal (optional)',
  craftTerminalHint: 'The terminal reuses the authorized session sandbox entry and stays collapsed.',
  craftComposerPlaceholder: 'Describe the change you want…',
  craftSend: 'Send',
  craftSendBusy: 'Working…',
  craftConversationEmpty: 'Describe your goal; the agent reports progress here.',
  craftConversationResumed: 'Live stream of the active run restored (past turns are not part of the snapshot).',
  craftInteractionQuestion: 'Agent question',
  craftInteractionPermission: 'Permission request',
  craftInteractionUnknown: 'Waiting',
  craftInteractionAnswerPlaceholder: 'Type your answer…',
  craftInteractionSubmitAnswer: 'Submit answer',
  craftInteractionApprove: 'Approve this scope',
  craftInteractionReject: 'Reject',
  craftInteractionResolved: 'Handled',
  craftInteractionScope: 'Requested scope',
  craftErrorTitle: 'Something went wrong',
  craftStreamReconnecting: 'Stream interrupted, reconnecting…',
  craftLocaleToggle: '中文 / EN',
};

export function craftStrings(locale: CraftLocale): CraftStrings {
  return locale === 'zh' ? STRINGS_ZH : STRINGS_EN;
}

// ---------------------------------------------------------------------------
// Main-run status wording (brief Step 3 — verbatim)
// ---------------------------------------------------------------------------

export function statusLabel(main: string, child: string, hasCurrentVersion = false): string {
  if (main === 'stopping') return '正在停止';
  if (main === 'waiting_user') return '需要你的处理';
  if (main === 'running' && child === 'finished') return '主 Agent 正在检查结果';
  // W05/C03 review: after a terminal run the refreshed snapshot releases the
  // active run and the client honestly projects idle — with a published
  // current_version that state is "completed, viewable", not "unclear".
  if (main === 'idle' && hasCurrentVersion) return '已完成，可查看版本';
  return ({ queued: '等待执行', running: '正在生成', succeeded: '已完成', failed: '执行失败', canceled: '已停止' } as Record<string, string>)[main] ?? '状态待核对';
}

/** English wording with identical branching (i18n companion of statusLabel). */
export function statusLabelEn(main: string, child: string, hasCurrentVersion = false): string {
  if (main === 'stopping') return 'Stopping';
  if (main === 'waiting_user') return 'Needs your input';
  if (main === 'running' && child === 'finished') return 'Main agent is verifying the result';
  if (main === 'idle' && hasCurrentVersion) return 'Completed — view the version';
  return ({ queued: 'Queued', running: 'Generating', succeeded: 'Completed', failed: 'Failed', canceled: 'Stopped' } as Record<string, string>)[main] ?? 'Status unknown';
}

export function statusLabelLocalized(locale: CraftLocale, main: string, child: string, hasCurrentVersion = false): string {
  return locale === 'zh' ? statusLabel(main, child, hasCurrentVersion) : statusLabelEn(main, child, hasCurrentVersion);
}

// ---------------------------------------------------------------------------
// Conversation projection (the assistant-ui ExternalStoreRuntime semantics)
// ---------------------------------------------------------------------------

/** One craft event payload the message log captured from the run stream. */
export interface CraftLoggedEvent {
  seq: number;
  kind: string;
  data: Record<string, unknown>;
}

export type CraftInteractionKind = 'question' | 'permission' | 'unknown';
export type CraftInteractionAction = 'answer' | 'approve' | 'reject';

export interface CraftInteractionCard {
  id: string;
  seq: number;
  kind: CraftInteractionKind;
  prompt: string;
  scope: string;
  resolved: boolean;
  /** Fail-closed action set: a question is never approvable, an unknown kind only rejectable. */
  allowedActions: CraftInteractionAction[];
}

function optionalText(data: Record<string, unknown>, keys: string[]): string {
  for (const key of keys) {
    const value = data[key];
    if (typeof value === 'string' && value.trim() !== '') return value;
  }
  return '';
}

export function interactionCardFrom(data: Record<string, unknown>, resolved: boolean, seq: number): CraftInteractionCard {
  const rawKind = data['kind'];
  const kind: CraftInteractionKind = rawKind === 'question' || rawKind === 'permission' ? rawKind : 'unknown';
  const id = optionalText(data, ['interaction_id', 'pending_id', 'part_id', 'id']) || 'seq-' + seq;
  return {
    id,
    seq,
    kind,
    prompt: optionalText(data, ['prompt', 'text', 'question', 'message']),
    scope: optionalText(data, ['scope', 'command', 'tool', 'operation']),
    resolved,
    allowedActions: kind === 'question' ? ['answer', 'reject'] : kind === 'permission' ? ['approve', 'reject'] : ['reject'],
  };
}

export interface CraftToolFact {
  seq: number;
  tool: string;
  status: string;
  callId: string;
}

export interface CraftAssistantProjection {
  text: string;
  tools: CraftToolFact[];
  interactions: CraftInteractionCard[];
  artifactVersionIds: string[];
  childStatus: string;
  workspaceUnavailable: boolean;
  /**
   * True ONLY when the main run projection is terminal. delegation.finished
   * contributes to childStatus alone — a child event can never complete the
   * main message (mirrors the W04 reducer invariant).
   */
  complete: boolean;
}

export function isMainRunTerminal(mainStatus: string): boolean {
  return (CRAFT_TERMINAL_RUN_STATUSES as readonly string[]).includes(mainStatus);
}

export function projectAssistant(events: CraftLoggedEvent[], mainStatus: string): CraftAssistantProjection {
  const ordered = [...events].sort((a, b) => a.seq - b.seq);
  let text = '';
  const tools: CraftToolFact[] = [];
  const interactions: CraftInteractionCard[] = [];
  const artifactVersionIds: string[] = [];
  let childStatus = 'idle';
  let workspaceUnavailable = false;

  for (const event of ordered) {
    switch (event.kind) {
      case 'delegation.text': {
        const chunk = event.data['text'];
        if (typeof chunk === 'string' && chunk !== '') text += chunk;
        break;
      }
      case 'delegation.tool': {
        tools.push({
          seq: event.seq,
          tool: optionalText(event.data, ['tool']) || 'tool',
          status: optionalText(event.data, ['status']) || 'unknown',
          callId: optionalText(event.data, ['call_id', 'part_id']),
        });
        break;
      }
      case 'delegation.started': {
        childStatus = 'running';
        break;
      }
      case 'attempt_replaced': {
        // C03: the tRPC runtime replaced the main model's unfinished attempt
        // (graph.go emits attempt_replaced with previous_attempt_id); the
        // abandoned partial text is dropped — the new attempt's text NEVER
        // appends after the old one.
        text = '';
        break;
      }
      case 'delegation.finished': {
        // Child outcome only — never the main message outcome.
        childStatus = optionalText(event.data, ['status']) || 'finished';
        break;
      }
      case 'artifact.published': {
        const versionId = optionalText(event.data, ['version_id']);
        if (versionId !== '' && !artifactVersionIds.includes(versionId)) artifactVersionIds.push(versionId);
        break;
      }
      case 'interaction.pending': {
        interactions.push(interactionCardFrom(event.data, false, event.seq));
        break;
      }
      case 'interaction.resolved': {
        const resolvedId = optionalText(event.data, ['interaction_id', 'pending_id', 'part_id', 'id']);
        for (let index = interactions.length - 1; index >= 0; index -= 1) {
          if (resolvedId === '' || interactions[index].id === resolvedId) {
            interactions[index] = { ...interactions[index], resolved: true };
          }
        }
        break;
      }
      case 'workspace.unavailable': {
        workspaceUnavailable = true;
        break;
      }
      default:
        break;
    }
  }

  return {
    text,
    tools,
    interactions,
    artifactVersionIds,
    childStatus,
    workspaceUnavailable,
    complete: isMainRunTerminal(mainStatus),
  };
}


// ---------------------------------------------------------------------------
// Conversation turns (send-time archive) + the backend-fed message log
// ---------------------------------------------------------------------------

/** One archived conversation turn: the prompt and the FINAL assistant projection of its run. */
export interface CraftTurnRecord {
  prompt: string | null;
  assistant: CraftAssistantProjection;
}

export interface CraftArchiveLiveTurnInput {
  runId: string | null;
  prompt: string | null;
  projection: CraftAssistantProjection;
}

/**
 * Archives the finished live turn when the user sends a new prompt.
 *
 * This is the B1 fix as a pure rule: archiving happens AT SEND TIME, on the
 * sender's own event, BEFORE the new run's subscription resets the message
 * log — never inside a render/effect that could observe the runId after it
 * already flipped (that ordering silently dropped every previous turn).
 *
 * Guards: a run that never existed (first send of a fresh session) or a live
 * turn without any content (no prompt, no streamed text, no tool facts)
 * archives nothing.
 */
export function archiveLiveTurn(
  turns: CraftTurnRecord[],
  live: CraftArchiveLiveTurnInput,
): { turns: CraftTurnRecord[]; archived: CraftTurnRecord | null } {
  const hasContent = live.prompt !== null || live.projection.text !== '' || live.projection.tools.length > 0;
  if (live.runId === null || !hasContent) return { turns, archived: null };
  const record: CraftTurnRecord = { prompt: live.prompt, assistant: live.projection };
  return { turns: [...turns, record], archived: record };
}

// ---------------------------------------------------------------------------
// Knowledge sources projection (C01 sources panel data plane)
// ---------------------------------------------------------------------------

/** One projected knowledge source row (shape of the C01 material manifest row). */
export interface CraftKnowledgeSourceFact {
  citationId: string;
  ref: string;
  title: string;
  digest: string;
  excerptBytes: number;
  tenantId: number;
}

export interface CraftKnowledgeSourcesProjection {
  sources: CraftKnowledgeSourceFact[];
  truncated: boolean;
}

/**
 * Projects the run's knowledge material package from the SAME backend craft
 * event frames every other workbench detail consumes: the latest
 * knowledge.built event wins (a rebuilt package replaces the earlier one).
 * Absent events answer the empty package — the panel's honest empty state.
 */
export function projectKnowledgeSources(events: CraftLoggedEvent[]): CraftKnowledgeSourcesProjection {
  let latest: Record<string, unknown> | null = null;
  for (const event of events) {
    if (event.kind !== 'knowledge.built') continue;
    if (event.data !== null && typeof event.data === 'object' && !Array.isArray(event.data)) {
      latest = event.data as Record<string, unknown>;
    }
  }
  if (latest === null) return { sources: [], truncated: false };
  const rawSources = Array.isArray(latest['sources']) ? latest['sources'] : [];
  const sources: CraftKnowledgeSourceFact[] = [];
  for (const raw of rawSources) {
    if (raw === null || typeof raw !== 'object' || Array.isArray(raw)) continue;
    const row = raw as Record<string, unknown>;
    const citationId = typeof row['citation_id'] === 'string' ? row['citation_id'] : '';
    const ref = typeof row['ref'] === 'string' ? row['ref'] : '';
    if (citationId === '' || ref === '') continue;
    sources.push({
      citationId,
      ref,
      title: typeof row['title'] === 'string' ? row['title'] : '',
      digest: typeof row['digest'] === 'string' ? row['digest'] : '',
      excerptBytes: typeof row['excerpt_bytes'] === 'number' ? row['excerpt_bytes'] : 0,
      tenantId: typeof row['tenant_id'] === 'number' ? row['tenant_id'] : 0,
    });
  }
  return { sources, truncated: latest['truncated'] === true };
}

export interface CraftMessageSnapshot {
  runId: string | null;
  events: CraftLoggedEvent[];
}

export interface CraftMessageLog {
  subscribe(listener: () => void): () => void;
  getSnapshot(): CraftMessageSnapshot;
  /** Feeds one raw SSE frame through the SAME contracts parsers the controller uses. */
  ingest(frame: CraftEventFrame): void;
  /** A new run replaces the log; reconnects of the same run keep it (seq dedupe). */
  resetForRun(runId: string): void;
}

const EMPTY_MESSAGE_SNAPSHOT: CraftMessageSnapshot = { runId: null, events: [] };

/**
 * The ExternalStoreRuntime-equivalent message store: an in-memory projection
 * fed exclusively by the backend run-event frames the assembly tees into it
 * (the same frames the W04 controller consumes). Views read it through
 * useSyncExternalStore; they never fetch or invent messages.
 */
export function createCraftMessageLog(): CraftMessageLog {
  const listeners = new Set<() => void>();
  let snapshot: CraftMessageSnapshot = EMPTY_MESSAGE_SNAPSHOT;

  function publish(next: CraftMessageSnapshot): void {
    snapshot = next;
    for (const listener of listeners) listener();
  }

  return {
    subscribe(listener: () => void): () => void {
      listeners.add(listener);
      return () => listeners.delete(listener);
    },
    getSnapshot(): CraftMessageSnapshot {
      return snapshot;
    },
    ingest(frame: CraftEventFrame): void {
      // The controller owns keepalive/error/run frames; the log keeps only
      // numbered run events (the persisted craft payloads) plus the tRPC
      // attempt_replaced marker, which the projection needs to drop the
      // abandoned partial text of a replaced main-model attempt (C03).
      if (frame.event !== undefined && !/^[0-9]+$/.test(frame.event)) return;
      try {
        const wire = parseCraftRunEvent(JSON.parse(frame.data));
        if (wire.type === 'attempt_replaced') {
          if (snapshot.events.some((event) => event.seq === wire.seq)) return; // reconnect replay dedupe
          const marker = typeof wire.payload === 'object' && wire.payload !== null && !Array.isArray(wire.payload)
            ? (wire.payload as Record<string, unknown>)
            : {};
          publish({
            runId: snapshot.runId,
            events: [...snapshot.events, { seq: wire.seq, kind: 'attempt_replaced', data: marker }].sort((a, b) => a.seq - b.seq),
          });
          return;
        }
        const payload = parseCraftEventPayload(wire.payload);
        if (payload === null) return; // foreign run events are not craft messages
        if (snapshot.events.some((event) => event.seq === wire.seq)) return; // reconnect replay dedupe
        publish({
          runId: snapshot.runId,
          events: [...snapshot.events, { seq: wire.seq, kind: payload.kind, data: payload.data }].sort((a, b) => a.seq - b.seq),
        });
      } catch {
        // Contract violations surface through the controller's error path.
      }
    },
    resetForRun(runId: string): void {
      if (snapshot.runId === runId) return;
      publish({ runId, events: [] });
    },
  };
}

// ---------------------------------------------------------------------------
// Preview states
// ---------------------------------------------------------------------------

export type CraftPreviewState = 'empty' | 'loading' | 'active' | 'expired' | 'failed';

export interface CraftPreviewClassificationInput {
  hasArtifact: boolean;
  ticketUrl: string | null;
  ticketExpiresAt: string | null;
  ticketError: string | null;
  frameFailed: boolean;
  nowMs: number;
}

export function classifyPreview(input: CraftPreviewClassificationInput): CraftPreviewState {
  if (!input.hasArtifact) return 'empty';
  if (input.ticketError !== null || input.frameFailed) return 'failed';
  if (input.ticketUrl === null || input.ticketExpiresAt === null) return 'loading';
  const expiry = Date.parse(input.ticketExpiresAt);
  if (Number.isNaN(expiry) || expiry <= input.nowMs) return 'expired';
  return 'active';
}

// ---------------------------------------------------------------------------
// Files / history facts
// ---------------------------------------------------------------------------

export interface CraftHistoryFact {
  name: string;
  status: string;
  detail: string;
}

export interface CraftHistoryRow {
  versionId: string;
  runId: string;
  isCurrent: boolean;
  /**
   * The wire contract carries no per-version timestamp: only the session's
   * updated_at is real, and it belongs to the current version's update. Older
   * rows honestly render null until the contract gains created_at.
   */
  timeLabel: string | null;
  fileCount: number;
  totalBytes: number;
  checks: CraftHistoryFact[];
}

export function historyRows(
  versions: CraftVersionView[],
  currentVersionId: string | null,
  sessionUpdatedAt: string,
  locale: CraftLocale,
): CraftHistoryRow[] {
  return versions.map((version) => ({
    versionId: version.id,
    runId: version.run_id,
    isCurrent: version.id === currentVersionId,
    timeLabel:
      version.id === currentVersionId && sessionUpdatedAt.trim() !== ''
        ? formatDateTime(sessionUpdatedAt, locale)
        : null,
    fileCount: version.files.length,
    totalBytes: version.files.reduce((sum, file) => sum + file.bytes, 0),
    checks: version.checks.map((check: CraftVersionCheckView) => ({ name: check.name, status: check.status, detail: check.detail })),
  }));
}

export function entryFileOf(version: CraftVersionView | null): CraftFileVersionView | null {
  if (version === null || version.files.length === 0) return null;
  const html = version.files.find((file) => file.path === 'index.html') ?? version.files.find((file) => file.path.endsWith('.html'));
  return html ?? version.files[0];
}

export function downloadFileName(path: string): string {
  const segments = path.split('/').filter((segment) => segment !== '');
  return segments[segments.length - 1] ?? 'craft-file';
}

// ---------------------------------------------------------------------------
// Formatting
// ---------------------------------------------------------------------------

export function formatBytes(bytes: number): string {
  if (!Number.isFinite(bytes) || bytes <= 0) return '0 B';
  const units = ['B', 'KB', 'MB', 'GB', 'TB'];
  let value = bytes;
  let unit = 0;
  while (value >= 1024 && unit < units.length - 1) {
    value /= 1024;
    unit += 1;
  }
  return unit === 0 ? value.toFixed(0) + ' ' + units[unit] : value.toFixed(1) + ' ' + units[unit];
}

export function formatDateTime(iso: string, locale: CraftLocale): string {
  const parsed = Date.parse(iso);
  if (Number.isNaN(parsed)) return iso;
  return new Intl.DateTimeFormat(locale === 'zh' ? 'zh-CN' : 'en-US', {
    year: 'numeric',
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
  }).format(new Date(parsed));
}
