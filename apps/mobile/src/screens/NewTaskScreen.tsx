import { useEffect, useState } from 'react';
import { Button, Text, TextInput, View } from 'react-native';
import type { NewTaskDraft, TaskAttachmentRef } from '@weknora/domain/mobile';
import type { DictationState } from '@weknora/mobile-core';
import { DICTATION_FAILURE_COPY } from '../dictation-view.ts';
import { OFFLINE_SUBMIT_COPY, type NewTaskViewState } from '../new-task-view.ts';

/**
 * 目标文本的源头长度上限（R1 裁决第三层，主控裁决 2026-09-24，数值为工程默认）：
 * Android SecureStore 单值约 2048 字节；500 个中文字 ≈1500B，knowledgeIds（UUID
 * 数组）与 attachments 逐项计入单条记录——合法组合仍可能越限，此时存储层以
 * TASK_OFFICE_INVALID_INPUT 显式拒绝（B3-F38 双保险：源头 maxLength 控制常规
 * 情形，越限组合得到可行动错误码而非 setItemAsync 底层抛错；不在存储层截断
 * ——截断破坏 goalKeyOf digest，见 adapters/intent-log.ts 的 1536B 预算注释）。
 */
export const GOAL_TEXT_MAX_LENGTH = 500;

export interface NewTaskScreenProps {
  state: NewTaskViewState;
  onUpdate(patch: Partial<Omit<NewTaskDraft, 'attachments' | 'knowledgeIds'>>): void;
  onSetAttachments(attachments: TaskAttachmentRef[]): void;
  onToggleKnowledge(knowledgeId: string): void;
  onSubmit(): void;
  onCancel(): void;
  onRefreshAgents(): void;
  /** 听写状态（组合根无原生捕获 Adapter 时为 undefined——屏不渲染任何听写元素，fail closed）。 */
  dictation?: DictationState;
  onDictationBegin?(): void;
  onDictationFinish?(): void;
  onDictationCancel?(): void;
  onDictationEditTranscript?(text: string): void;
  onDictationRetryTranscription?(): void;
  onDictationConfirmTranscript?(): void;
  onDictationDiscardTranscript?(): void;
}

/** 统一 New 入口屏（受控组件）：目标输入 + 推荐/改选主理 Agent + 附加知识 + 预算 + 就绪裁决（module-seams §10：Screen 不见 wire）。 */
export function NewTaskScreen({ state, onUpdate, onSetAttachments, onToggleKnowledge, onSubmit, onCancel, onRefreshAgents, dictation, onDictationBegin, onDictationFinish, onDictationCancel, onDictationEditTranscript, onDictationRetryTranscription, onDictationConfirmTranscript, onDictationDiscardTranscript }: NewTaskScreenProps) {
  // 预算输入的本地中间态（B3-F46）：''（未设置）与 0 是两个状态，双向折叠会
  // 把非纯数字的编辑中间态静默清零并渲染为空——本地保留原样，仅在合法数字时提交。
  const [budgetText, setBudgetText] = useState(state.draft.budgetUpper === 0 ? '' : String(state.draft.budgetUpper));
  useEffect(() => { if (state.draft.budgetUpper === 0) setBudgetText(''); }, [state.draft.budgetUpper]); // bound 清空草稿后同步
  return (
    <View>
      <Text>{state.loading ? 'Loading' : 'New task'}</Text>
      <Text>{state.recommendation.agent ? `Lead Agent: ${state.recommendation.agent.name}` : 'No supported lead agent'}</Text>
      {state.agents.map((agent) => (
        <Button
          key={agent.id}
          title={`${agent.name}${state.draft.agentId === agent.id ? ' ✓' : ''}`}
          disabled={state.loading}
          onPress={() => { onUpdate({ agentId: agent.id }); }}
        />
      ))}
      <Button title="Refresh agents" disabled={state.loading} onPress={() => { void onRefreshAgents(); }} />
      <TextInput value={state.draft.text} onChangeText={(text) => { onUpdate({ text }); }} placeholder="今天想完成什么？" multiline maxLength={GOAL_TEXT_MAX_LENGTH} editable={!state.loading} />
      {state.draft.text.length >= GOAL_TEXT_MAX_LENGTH && <Text>{`目标文本已达 ${GOAL_TEXT_MAX_LENGTH} 字上限，超出部分不会保存`}</Text>}
      {dictation !== undefined && dictation.phase === 'idle' && (
        <Button title="Dictate" disabled={state.loading || state.submitting} onPress={() => { onDictationBegin?.(); }} />
      )}
      {dictation !== undefined && dictation.phase === 'recording' && (
        <View>
          <Text>Recording…</Text>
          <Button title="Stop dictation" onPress={() => { onDictationFinish?.(); }} />
          <Button title="Cancel recording" onPress={() => { onDictationCancel?.(); }} />
        </View>
      )}
      {dictation !== undefined && dictation.phase === 'transcribing' && (
        <View>
          <Text>Transcribing…</Text>
          <Button title="Cancel transcription" onPress={() => { onDictationCancel?.(); }} />
        </View>
      )}
      {dictation !== undefined && dictation.phase === 'review' && (
        <View>
          <Text>转写草稿（确认前可编辑）</Text>
          <TextInput
            value={dictation.transcript ?? ''}
            onChangeText={(text) => { onDictationEditTranscript?.(text); }}
            placeholder="转写草稿（确认前可编辑）"
            multiline
            maxLength={GOAL_TEXT_MAX_LENGTH}
            editable={!state.loading && !state.submitting}
          />
          <Button title="Use transcript" disabled={state.submitting} onPress={() => { onDictationConfirmTranscript?.(); }} />
          <Button title="Discard transcript" onPress={() => { onDictationDiscardTranscript?.(); }} />
        </View>
      )}
      {dictation !== undefined && dictation.phase === 'denied' && (
        <View>
          <Text>{DICTATION_FAILURE_COPY.denied}</Text>
          <Button title="Retry dictation" disabled={state.loading || state.submitting} onPress={() => { onDictationBegin?.(); }} />
        </View>
      )}
      {dictation !== undefined && dictation.phase === 'failed' && (
        <View>
          <Text>{dictation.failure !== undefined ? DICTATION_FAILURE_COPY[dictation.failure] : ''}</Text>
          {dictation.failure === 'transcription-failed' && (
            <Button title="Retry transcription" onPress={() => { onDictationRetryTranscription?.(); }} />
          )}
          <Button title="Dismiss" onPress={() => { onDictationCancel?.(); }} />
        </View>
      )}
      <TextInput
        value={budgetText}
        onChangeText={(text) => { setBudgetText(text); onUpdate({ budgetUpper: /^\d+$/.test(text) ? Number(text) : 0 }); }}
        placeholder="预算上限（Credits，可留空）"
        keyboardType="numeric"
        editable={!state.loading}
      />
      {state.knowledge.map((item) => (
        <Button
          key={item.id}
          title={`${state.draft.knowledgeIds.includes(item.id) ? '✓ ' : ''}${item.title}`}
          disabled={state.loading}
          onPress={() => { onToggleKnowledge(item.id); }}
        />
      ))}
      {state.draft.attachments.map((attachment) => (
        <Text key={attachment.id}>{`${attachment.name} · ${attachment.readiness}`}</Text>
      ))}
      {!state.readiness.ready && state.readiness.reason !== undefined && <Text>{state.readiness.reason}</Text>}
      {state.inFlight !== undefined && state.inFlight.phase !== 'rejected' && <Text>{`Unresolved submission ${state.inFlight.requestId} (${state.inFlight.phase}) — retrying keeps the same request id`}</Text>}
      {state.offline && <Text>{OFFLINE_SUBMIT_COPY}</Text>}
      {state.error !== undefined && <Text>{state.error}</Text>}
      <Button title="Submit task" disabled={state.submitting || !state.readiness.ready || state.loading} onPress={() => { void onSubmit(); }} />
      <Button title="Keep draft" onPress={() => { void onCancel(); }} />
    </View>
  );
}
