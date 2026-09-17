import * as React from 'react';
import { AppState, Pressable, Text, TextInput, View } from 'react-native';
import { SessionView } from '@/-session/SessionView';
import { ConversationViewModelContext } from './context';
import { createRequestID, type ConversationViewModel } from './view-model';
import type { AttachmentEntryActions, SessionUploadRecord, SessionUploadStatus } from '../resources/upload';
import { isRecoveryNotFound, type ExecutionRecovery } from '../executions/recovery';
import { createDictationController, DictationError, type DictationLimits, type DictationPort, type DictationScope } from '../voice/dictation';
import { DictationInput } from '../voice/DictationInput';
export { ProductConversationMessages } from './ProductConversationMessages';

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
export function ConversationScreen({ sessionId, viewModel, sessionRenderer: SessionRenderer = SessionView, attachments, recovery, dictation }: ConversationScreenProps) {
  const [, redraw] = React.useReducer((value: number) => value + 1, 0);
  React.useEffect(() => viewModel.subscribe?.(() => redraw()), [redraw, viewModel]);
  // W12: the product conversation resumes executions when the app returns to
  // the foreground and closes only its subscription when it leaves. Unmount
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
  const recoveryState = recovery?.getState();
  const executionNotice = viewModel.execution?.status === 'unknown'
    ? '连接状态未知，正在等待服务端确认'
    : viewModel.execution?.status === 'pending' || viewModel.execution?.status === 'dispatching'
      ? '任务正在排队'
      : null;
  return (
    <ConversationViewModelContext.Provider value={viewModel}>
      <View style={{ flex: 1 }}>
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
        <SessionRenderer id={sessionId} viewModel={viewModel} />
      </View>
    </ConversationViewModelContext.Provider>
  );
}
