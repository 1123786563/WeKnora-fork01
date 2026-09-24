import { Button, Text, TextInput, View } from 'react-native';
import type { NewTaskDraft, TaskAttachmentRef } from '@weknora/domain/mobile';
import type { NewTaskViewState } from '../new-task-view.ts';

/**
 * 目标文本的源头长度上限（R1 裁决第三层，主控裁决 2026-09-24，数值为工程默认）：
 * Android SecureStore 单值约 2048 字节，最坏情形（500 个中文字 × 3B UTF-8 +
 * UUID 形态 requestId/sessionId 与 scope 固定开销）单条意图记录序列化约
 * 1.8KB，仍在信封内；无上限时约 560+ 中文字即超限，intent log 的
 * setItemAsync 抛错会让提交永久失败（见 adapters/intent-log.ts 的 1536B
 * 预算注释：单条超限在源头拦截，不在存储层截断——截断破坏 goalKeyOf digest）。
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
}

/** 统一 New 入口屏（受控组件）：目标输入 + 推荐/改选主理 Agent + 附加知识 + 预算 + 就绪裁决（module-seams §10：Screen 不见 wire）。 */
export function NewTaskScreen({ state, onUpdate, onSetAttachments, onToggleKnowledge, onSubmit, onCancel, onRefreshAgents }: NewTaskScreenProps) {
  return (
    <View>
      <Text>{state.loading ? 'Loading' : 'New task'}</Text>
      <Text>{state.recommendation.agent ? `Lead Agent: ${state.recommendation.agent.name}` : 'No supported lead agent'}</Text>
      {state.agents.map((agent) => (
        <Button
          key={agent.id}
          title={`${agent.name}${state.draft.agentId === agent.id ? ' ✓' : ''}`}
          onPress={() => { onUpdate({ agentId: agent.id }); }}
        />
      ))}
      <Button title="Refresh agents" onPress={() => { void onRefreshAgents(); }} />
      <TextInput value={state.draft.text} onChangeText={(text) => { onUpdate({ text }); }} placeholder="今天想完成什么？" multiline maxLength={GOAL_TEXT_MAX_LENGTH} />
      {state.draft.text.length >= GOAL_TEXT_MAX_LENGTH && <Text>{`目标文本已达 ${GOAL_TEXT_MAX_LENGTH} 字上限，超出部分不会保存`}</Text>}
      <TextInput
        value={state.draft.budgetUpper === 0 ? '' : String(state.draft.budgetUpper)}
        onChangeText={(text) => { onUpdate({ budgetUpper: /^\d+$/.test(text) ? Number(text) : 0 }); }}
        placeholder="预算上限（Credits，可留空）"
        keyboardType="numeric"
      />
      {state.knowledge.map((item) => (
        <Button
          key={item.id}
          title={`${state.draft.knowledgeIds.includes(item.id) ? '✓ ' : ''}${item.title}`}
          onPress={() => { onToggleKnowledge(item.id); }}
        />
      ))}
      {state.draft.attachments.map((attachment) => (
        <Text key={attachment.id}>{`${attachment.name} · ${attachment.readiness}`}</Text>
      ))}
      {!state.readiness.ready && state.readiness.reason !== undefined && <Text>{state.readiness.reason}</Text>}
      {state.inFlight !== undefined && <Text>{`Unresolved submission ${state.inFlight.requestId} (${state.inFlight.phase}) — retrying keeps the same request id`}</Text>}
      {state.error !== undefined && <Text>{state.error}</Text>}
      <Button title="Submit task" disabled={state.submitting || !state.readiness.ready} onPress={() => { void onSubmit(); }} />
      <Button title="Keep draft" onPress={() => { void onCancel(); }} />
    </View>
  );
}
