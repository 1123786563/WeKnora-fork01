import { Button, Text, TextInput, View } from 'react-native';
import type { NewTaskDraft, TaskAttachmentRef } from '@weknora/domain/mobile';
import type { NewTaskViewState } from '../new-task-view.ts';

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
      <TextInput value={state.draft.text} onChangeText={(text) => { onUpdate({ text }); }} placeholder="今天想完成什么？" multiline />
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
