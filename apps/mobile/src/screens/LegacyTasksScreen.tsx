import { useEffect, useState } from 'react';
import { Button, Text, TextInput, View } from 'react-native';
import type { LegacyMessage, LegacyTaskCard, TaskOffice } from '@weknora/mobile-core';
import { activeTaskOffice } from '../composition.ts';
import { createLegacyTasksController, type LegacyTasksController, type LegacyTasksViewState } from '../legacy-tasks-view.ts';

const INTENT_LABELS: Array<{ intent: 'run-command' | 'decision' | 'budget' | 'agent-version'; label: string }> = [
  { intent: 'run-command', label: 'Run commands' },
  { intent: 'decision', label: 'Approvals' },
  { intent: 'budget', label: 'Budget' },
  { intent: 'agent-version', label: 'Agent version' },
];

/** Legacy Task 屏：facts-only 投影 + 显式门禁文案 + 历史与普通追问。Screen 只消费 Task Office Interface（seams §10）。 */
export function LegacyTasksScreen({ onStartNewRun }: { onStartNewRun?: () => void } = {}) {
  const [controller, setController] = useState<LegacyTasksController | undefined>(undefined);
  const [view, setView] = useState<LegacyTasksViewState>({ loading: true, items: [], hasMore: false, duplicateTaskIds: [], followUpState: 'idle' });
  const [question, setQuestion] = useState('');

  useEffect(() => {
    const office = activeTaskOffice();
    if (!office) return;
    const next = createLegacyTasksController(office as Pick<TaskOffice, 'legacyTasks' | 'moreLegacyTasks' | 'legacyHistory' | 'followUp'>);
    setController(next);
    setView(next.state());
    const unsubscribe = next.subscribe(setView);
    return () => { unsubscribe(); next.dispose(); };
  }, []);

  return (
    <View>
      <Text>Legacy tasks</Text>
      <Text>Old chat sessions appear here with the same task identity. Only history-provable facts are shown.</Text>
      {view.loading && <Text>Loading</Text>}
      {view.error !== undefined && <Text>{view.error}</Text>}
      {view.items.length === 0 && !view.loading && view.error === undefined && <Text>No legacy tasks</Text>}
      {view.items.map((card: LegacyTaskCard) => (
        <View key={card.taskId}>
          <Text>{`${card.title || card.taskId} · legacy${card.archivedAt !== undefined ? ' · archived' : ''}`}</Text>
          {INTENT_LABELS.map(({ intent, label }) => (
            <Text key={intent}>{`${label}: ${card.gates[intent].state}${card.gates[intent].state === 'unavailable' ? ' — new Run required' : ''}`}</Text>
          ))}
          <Button title="History" onPress={() => { void controller?.openHistory(card.taskId); }} />
          {view.history?.taskId === card.taskId && (
            <View>
              {(view.history.messages as LegacyMessage[]).map((message) => (
                <Text key={message.messageId}>{`${message.role}: ${message.content}`}</Text>
              ))}
            </View>
          )}
          <TextInput value={question} onChangeText={setQuestion} placeholder="Continue with an ordinary follow-up" />
          <Button title="Send follow-up" disabled={view.followUpState === 'sending' || question.trim() === ''} onPress={() => { void controller?.submitFollowUp(card.taskId, question); setQuestion(''); }} />
          {view.followUpState === 'sent' && <Text>Follow-up sent</Text>}
          {view.followUpError !== undefined && <Text>{view.followUpError}</Text>}
        </View>
      ))}
      {view.hasMore && <Button title="Load more" onPress={() => { void controller?.loadMore(); }} />}
      <Button title="Reload" onPress={() => { void controller?.reload(); }} />
      {onStartNewRun !== undefined && <Button title="Start a new Run for new capabilities" onPress={onStartNewRun} />}
    </View>
  );
}
