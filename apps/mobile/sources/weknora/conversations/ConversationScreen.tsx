import * as React from 'react';
import { Pressable, Text, TextInput, View } from 'react-native';
import { SessionView } from '@/-session/SessionView';
import { ConversationViewModelContext } from './context';
import { createRequestID, type ConversationViewModel } from './view-model';
import type { AttachmentEntryActions, SessionUploadRecord, SessionUploadStatus } from '../resources/upload';
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
}

/** Attachment surface the conversation input box consumes. */
export interface ConversationAttachments {
  entries: AttachmentEntryActions;
  records(): SessionUploadRecord[];
  cancel(id: string): void;
  remove(id: string): void;
  subscribe?(listener: () => void): () => void;
}

const uploadStatusText: Record<SessionUploadStatus, string> = {
  uploading: '上传中',
  uploaded: '已附加',
  failed: '上传失败',
};

export function ConversationControlPanel({ viewModel, attachments }: { viewModel: ConversationViewModel; attachments?: ConversationAttachments }) {
  const [draft, setDraft] = React.useState('');
  const [busy, setBusy] = React.useState(false);
  const [, redrawAttachments] = React.useReducer((value: number) => value + 1, 0);
  React.useEffect(() => attachments?.subscribe?.(() => redrawAttachments()), [attachments, redrawAttachments]);
  const submit = async () => {
    if (!viewModel.send || !draft.trim() || busy) return;
    setBusy(true);
    try {
      await viewModel.send.submit(draft, createRequestID());
      setDraft('');
    } finally {
      setBusy(false);
    }
  };
  const canAttach = viewModel.capabilities.canAttach && Boolean(attachments);
  return (
    <View accessibilityLabel="conversation-controls">
      <TextInput accessibilityLabel="conversation-draft" value={draft} onChangeText={setDraft} />
      <Pressable accessibilityRole="button" accessibilityLabel="发送" disabled={busy} onPress={() => void submit()}>
        <Text>{busy ? '发送中' : '发送'}</Text>
      </Pressable>
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
export function ConversationScreen({ sessionId, viewModel, sessionRenderer: SessionRenderer = SessionView, attachments }: ConversationScreenProps) {
  const [, redraw] = React.useReducer((value: number) => value + 1, 0);
  React.useEffect(() => viewModel.subscribe?.(() => redraw()), [redraw, viewModel]);
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
        <ConversationControlPanel viewModel={viewModel} attachments={attachments} />
        <SessionRenderer id={sessionId} viewModel={viewModel} />
      </View>
    </ConversationViewModelContext.Provider>
  );
}
