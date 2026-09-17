import * as React from 'react';
import { AppState, Pressable, Text, TextInput, View } from 'react-native';
import { SessionView } from '@/-session/SessionView';
import { ConversationViewModelContext } from './context';
import { createRequestID, type ConversationViewModel } from './view-model';
import type { AttachmentEntryActions, SessionUploadRecord, SessionUploadStatus } from '../resources/upload';
import { isRecoveryNotFound, isTerminalExecutionStatus, type ExecutionRecovery } from '../executions/recovery';
import { createDictationController, DictationError, type DictationLimits, type DictationPort, type DictationScope } from '../voice/dictation';
import { DictationInput } from '../voice/DictationInput';
import { createVoiceControls, createVoiceUtteranceHandler, type RealtimeVoiceSession, type VoiceUtteranceOutcome } from '../voice/realtime';
import { VoicePanel } from '../voice/VoicePanel';
import type { RunProgressPresenter } from '../notifications/live-progress';
import { ConversationResultResourcesContext, type ConversationResultResources } from './ProductConversationMessages';
export { ProductConversationMessages } from './ProductConversationMessages';
export { selectRenderer } from '../renderers/registry';
export type { ConversationResultResources } from './ProductConversationMessages';

export interface ConversationScreenProps {
  sessionId: string;
  viewModel: ConversationViewModel;
  /** Injectable native renderer seam used by the RN interaction tests. */
  sessionRenderer?: React.ComponentType<{ id: string; viewModel: ConversationViewModel }>;
  /**
   * W25 session attachment controls. `entries.chooseFromLibrary` backs the
   * input-box picker, `entries.takePhoto` the camera button, and
   * `entries.acceptSharedFile` is invoked by the app's system-share entry
   * (native intent wiring; device evidence is blocked-env).
   */
  attachments?: ConversationAttachments;
  /**
   * W12 lifecycle recovery handle. Foreground transitions rerun the recovery
   * pass (status -> history -> stream-when-active); background only closes
   * the subscription. Unmounting removes the AppState subscription and
   * disposes the handle, so the assembler passes a per-mount instance.
   */
  recovery?: ExecutionRecovery;
  /**
   * W29 hold-to-talk dictation surface: the native audio port plus optional
   * capability limits and the product scope seam. The transcription itself
   * is the port's W30 consumption point; until it lands the voice flow
   * surfaces a typed failure while the text input stays usable.
   */
  dictation?: ConversationDictation;
  /**
   * W31 realtime voice surface: the W30-grant-backed session (admission,
   * bounded renewal, system-disconnect handling). The screen derives the
   * three separated controls (stop playback / end voice / explicit product
   * cancel) and the utterance policy handler from it; high-risk approvals
   * stay on the pending-interaction card below — spoken answers never
   * approve.
   */
  voice?: ConversationVoice;
  /**
   * W31 background run progress presenter (F-3): the screen feeds it Run
   * status changes from the view-model — status-only hints, the presenter
   * owns no worker and never renders conversation content. Live Activity is
   * the preferred surface when the port reports it; plain notifications are
   * the always-available fallback.
   */
  progress?: RunProgressPresenter;
  /**
   * W28 authorized resource seam for structured results: citations open and
   * oversized analysis tables / artifact files download by re-requesting
   * authorization through the product knowledge/attachment interfaces on
   * every click. Renderers are never the authorization layer; without this
   * seam the structured cards explain the product entry instead of faking
   * an authorized open.
   */
  resultResources?: ConversationResultResources;
}

/** Attachment surface the conversation input box consumes. */
export interface ConversationAttachments {
  entries: AttachmentEntryActions;
  records(): SessionUploadRecord[];
  cancel(id: string): void;
  remove(id: string): void;
  subscribe?(listener: () => void): () => void;
}

/** Voice surface the conversation input box consumes (W29). */
export interface ConversationDictation {
  port: DictationPort;
  limits?: Partial<DictationLimits>;
  scope?: DictationScope;
}

/**
 * W31 realtime voice surface: only the session is injected — the screen owns
 * the composition so the separated controls and the utterance policy always
 * ride the live view-model (runID/revision, pending approvals, the W37-gated
 * cancel capability).
 */
export interface ConversationVoice {
  session: RealtimeVoiceSession;
}

const uploadStatusText: Record<SessionUploadStatus, string> = {
  uploading: '上传中',
  uploaded: '已附加',
  failed: '上传失败',
};

export function ConversationControlPanel({ viewModel, attachments, dictation }: { viewModel: ConversationViewModel; attachments?: ConversationAttachments; dictation?: ConversationDictation }) {
  const [draft, setDraftState] = React.useState('');
  // The dictation controller writes and reads the draft outside React's
  // render cycle, so the state travels through a synchronous ref mirror.
  const draftRef = React.useRef('');
  const applyDraft = React.useCallback((text: string) => {
    draftRef.current = text;
    setDraftState(text);
  }, []);
  const [busy, setBusy] = React.useState(false);
  const [, redrawAttachments] = React.useReducer((value: number) => value + 1, 0);
  const [, redrawDictation] = React.useReducer((value: number) => value + 1, 0);
  React.useEffect(() => attachments?.subscribe?.(() => redrawAttachments()), [attachments, redrawAttachments]);
  // The send seam the dictation confirm rides: the same single-flight W10
  // SendController as typed input, reached through a live ref so the
  // once-created controller never holds a stale submit closure. A skipped
  // (busy/empty) send rejects instead of resolving so confirm keeps the draft.
  const sendRef = React.useRef<(text: string) => Promise<void>>(async () => undefined);
  const sendOnce = async (text: string): Promise<void> => {
    if (!viewModel.send || text.trim() === '') throw new DictationError('TRANSCRIPT_REQUIRED');
    if (busy) throw new Error('SEND_IN_PROGRESS');
    setBusy(true);
    try {
      await viewModel.send.submit(text, createRequestID());
      applyDraft('');
    } finally {
      setBusy(false);
    }
  };
  sendRef.current = sendOnce;
  const submit = async () => {
    if (!viewModel.send || !draftRef.current.trim() || busy) return;
    await sendOnce(draftRef.current);
  };
  const canVoice = viewModel.capabilities.canVoice && Boolean(dictation);
  const dictationController = React.useMemo(() => {
    if (!canVoice || !dictation) return null;
    return createDictationController(
      // Portless transcription is unused in production: the native port owns
      // the flow and its `transcribe` is the W30 consumption point.
      async () => { throw new DictationError('TRANSCRIBE_FAILED', 'DICTATION_PORT_REQUIRED'); },
      applyDraft,
      () => draftRef.current,
      (text) => sendRef.current(text),
      { port: dictation.port, limits: dictation.limits, scope: dictation.scope, onStateChange: () => redrawDictation() },
    );
  }, [applyDraft, canVoice, dictation, redrawDictation]);
  const canAttach = viewModel.capabilities.canAttach && Boolean(attachments);
  return (
    <View accessibilityLabel="conversation-controls">
      <TextInput accessibilityLabel="conversation-draft" value={draft} onChangeText={applyDraft} />
      <Pressable accessibilityRole="button" accessibilityLabel="发送" disabled={busy} onPress={() => void (dictationController ? dictationController.confirm().catch(() => undefined) : submit())}>
        <Text>{busy ? '发送中' : '发送'}</Text>
      </Pressable>
      {canVoice && dictationController ? <DictationInput controller={dictationController} /> : null}
      {canAttach && attachments ? (
        <View style={{ flexDirection: 'row', gap: 8 }}>
          <Pressable accessibilityRole="button" accessibilityLabel="附件" onPress={() => void attachments.entries.chooseFromLibrary()}>
            <Text>附件</Text>
          </Pressable>
          <Pressable accessibilityRole="button" accessibilityLabel="拍照" onPress={() => void attachments.entries.takePhoto()}>
            <Text>拍照</Text>
          </Pressable>
        </View>
      ) : null}
      {canAttach && attachments ? attachments.records().map((record) => (
        <View key={record.id} accessibilityLabel={`attachment-${record.id}`} style={{ flexDirection: 'row', gap: 8 }}>
          <Text>{record.input.name}</Text>
          <Text>{uploadStatusText[record.status]}</Text>
          {record.error ? <Text accessibilityRole="alert">{record.error}</Text> : null}
          {record.status === 'uploading' ? (
            <Pressable accessibilityRole="button" accessibilityLabel={`取消上传 ${record.input.name}`} onPress={() => attachments.cancel(record.id)}>
              <Text>取消</Text>
            </Pressable>
          ) : null}
          {record.status === 'failed' ? (
            <Pressable accessibilityRole="button" accessibilityLabel={`移除 ${record.input.name}`} onPress={() => attachments.remove(record.id)}>
              <Text>移除</Text>
            </Pressable>
          ) : null}
        </View>
      )) : null}
      {viewModel.pendingInteractions.filter((item) => item.status === 'pending').map((item) => (
        <View key={item.id} accessibilityLabel={`pending-interaction-${item.id}`} style={{ paddingVertical: 6 }}>
          <Text>{item.label}</Text>
          {item.reason ? <Text>{item.reason}</Text> : null}
          {item.error ? <Text accessibilityRole="alert">{item.error}</Text> : null}
          <View style={{ flexDirection: 'row', gap: 8 }}>
            <Pressable accessibilityRole="button" accessibilityLabel={`批准 ${item.label}`} onPress={() => void Promise.resolve(viewModel.commands.approve?.(item.id, item.revision ?? 0)).catch(() => undefined)}>
              <Text>批准</Text>
            </Pressable>
            <Pressable accessibilityRole="button" accessibilityLabel={`拒绝 ${item.label}`} onPress={() => void Promise.resolve(viewModel.commands.reject?.(item.id, item.revision ?? 0)).catch(() => undefined)}>
              <Text>拒绝</Text>
            </Pressable>
            <Pressable accessibilityRole="button" accessibilityLabel={`刷新 ${item.label}`} onPress={() => void viewModel.commands.refreshPending?.(item.id)}>
              <Text>刷新</Text>
            </Pressable>
          </View>
        </View>
      ))}
    </View>
  );
}

/** Product-owned seam around the retained Happy renderer. */
export function ConversationScreen({ sessionId, viewModel, sessionRenderer: SessionRenderer = SessionView, attachments, recovery, dictation, voice, progress, resultResources }: ConversationScreenProps) {
  const [, redraw] = React.useReducer((value: number) => value + 1, 0);
  React.useEffect(() => viewModel.subscribe?.(() => redraw()), [redraw, viewModel]);
  // W12: the product conversation resumes executions when the app returns to the
  // foreground and closes only its subscription when it leaves. Unmount
  // removes the AppState listener and disposes the per-mount recovery handle.
  React.useEffect(() => {
    if (!recovery) return;
    const unsubscribeState = recovery.subscribe(() => redraw());
    const subscription = AppState.addEventListener('change', (next) => recovery.appStateChange(next));
    return () => {
      subscription.remove();
      unsubscribeState();
      recovery.dispose();
    };
  }, [recovery, redraw]);
  // W31: the realtime voice session follows the app lifecycle. Backgrounding
  // closes the provider session and settles billing — the paid session is
  // never reopened automatically; returning to the foreground only renews an
  // expired grant through fresh admission while the W12 controller above
  // re-attaches to the run. Unmounting (navigating away) ends the session
  // the same way so a mid-call navigation never dangles the W30 hold (F-2).
  React.useEffect(() => {
    if (!voice) return;
    const subscription = AppState.addEventListener('change', (next) => {
      if (next === 'background') void voice.session.interruptedBySystem('background');
      else if (next === 'active') void voice.session.renewIfExpired();
    });
    return () => {
      subscription.remove();
      voice.session.dispose();
    };
  }, [voice]);
  // W31 F-3: run status changes ride the progress presenter — status-only
  // hints (the presenter's fixed label map), no worker, no content. The
  // presenter never owns navigation; foreground recovery stays W12's.
  React.useEffect(() => {
    if (!progress) return;
    const report = () => {
      const execution = viewModel.execution;
      if (execution?.runID) void progress.onRunStatus(execution.runID, execution.status).catch(() => undefined);
    };
    report();
    return viewModel.subscribe?.(report);
  }, [progress, viewModel]);
  // The separated W31 controls: stop playback and end voice map onto the
  // realtime session; cancelTask is the explicit product command riding the
  // view-model boundary (the W37 protocol gate stops it there).
  const voiceControls = React.useMemo(() => (voice ? createVoiceControls({
    stopAudio: () => voice.session.interruptPlayback(),
    closeVoice: () => voice.session.end(),
    cancelRun: async () => {
      const execution = viewModel.execution;
      if (!execution?.runID) return;
      await viewModel.commands.cancel(execution.runID, execution.revision ?? 0);
    },
  }) : null), [voice, viewModel]);
  // Utterance policy: audio intents ride the controls above; approval
  // answers — ambiguous or not — only surface the W05 card. There is no
  // approve path anywhere on the voice surface.
  const [voiceNotice, setVoiceNotice] = React.useState<string | null>(null);
  const voiceUtterance = React.useMemo(() => ((voice && voiceControls) ? createVoiceUtteranceHandler({
    controls: voiceControls,
    hasPendingApproval: () => viewModel.pendingInteractions.some((item) => item.status === 'pending'),
    surfaceApprovalCard: () => setVoiceNotice('高风险操作请在审批卡上手动确认'),
  }) : undefined), [voice, voiceControls, viewModel]);
  const recoveryState = recovery?.getState();
  // W37 protocol gate: present exactly when the compatibility verdict is not
  // 'full'. The safe surface — login, reads and this explanation — stays
  // available while cancel/steer are stopped at the view-model boundary.
  const protocolNotice = viewModel.protocolNotice;
  const executionNotice = viewModel.execution?.status === 'unknown'
    ? '连接状态未知，正在等待服务端确认'
    : viewModel.execution?.status === 'pending' || viewModel.execution?.status === 'dispatching'
      ? '任务正在排队'
      : null;
  return (
    <ConversationViewModelContext.Provider value={viewModel}>
      {/* W28: the authorized resource seam rides a context so the message
          surface mounted inside the Happy renderer reaches it without
          prop-drilling through SessionView. */}
      <ConversationResultResourcesContext.Provider value={resultResources ?? null}>
      <View style={{ flex: 1 }}>
        {protocolNotice && (
          <View accessibilityLabel="protocol-compatibility-notice" accessibilityRole="alert" style={{ paddingHorizontal: 16, paddingVertical: 8 }}>
            <Text>{protocolNotice}</Text>
          </View>
        )}
        {executionNotice && (
          <View accessibilityRole="alert" style={{ paddingHorizontal: 16, paddingVertical: 8 }}>
            <Text>{executionNotice}</Text>
          </View>
        )}
        {recoveryState?.state === 'failed' && (
          <View accessibilityLabel="recovery-failure" accessibilityRole="alert" style={{ paddingHorizontal: 16, paddingVertical: 8 }}>
            <Text>{isRecoveryNotFound(recoveryState.error) ? '执行不存在或已被清理，未自动新建会话' : '恢复执行失败'}</Text>
            <Pressable accessibilityRole="button" accessibilityLabel="重试恢复" onPress={() => void recovery?.recover()}>
              <Text>重试恢复</Text>
            </Pressable>
          </View>
        )}
        <ConversationControlPanel viewModel={viewModel} attachments={attachments} dictation={dictation} />
        {voice && voiceControls ? (
          <VoicePanel
            session={voice.session}
            controls={voiceControls}
            canCancel={viewModel.capabilities.canCancel}
            runActive={viewModel.execution !== null && !isTerminalExecutionStatus(viewModel.execution.status)}
            utterance={voiceUtterance}
            notice={voiceNotice}
          />
        ) : null}
        <SessionRenderer id={sessionId} viewModel={viewModel} />
      </View>
      </ConversationResultResourcesContext.Provider>
    </ConversationViewModelContext.Provider>
  );
}
