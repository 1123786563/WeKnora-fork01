import { Button, ScrollView, Text, TextInput, View } from 'react-native';
import { EVIDENCE_KIND_LABEL, type KnowledgeQAEvidenceCitation, type KnowledgeQATurn } from '@weknora/mobile-core';
import type { KnowledgeResource } from '@weknora/domain/mobile';
import { evidenceCitationLine, type KnowledgeQAViewState } from '../knowledge-qa-view.ts';

/** T15：知识问答屏（presentation Adapter——只消费控制器状态与意图回调）。 */
export function KnowledgeQAScreen(props: {
  state: KnowledgeQAViewState;
  onUpdate(patch: { question?: string }): void;
  onToggleKnowledge(knowledgeId: string): void;
  onAsk(): void;
  onRetry(): void;
  onRefreshKnowledge(): void;
}) {
  const { state } = props;
  return (
    <ScrollView>
      <Text>知识问答</Text>
      <TextInput
        value={state.question}
        placeholder="提出知识问题（答案将携带可核对的来源、版本与时间证据）"
        multiline
        onChangeText={(text) => { props.onUpdate({ question: text }); }}
      />
      <Text>检索范围（不选＝会话默认可发现知识）</Text>
      {state.knowledge.map((resource: KnowledgeResource) => (
        <Button
          key={resource.id}
          title={`${state.selectedKnowledgeIds.includes(resource.id) ? '✓ ' : ''}${resource.title}`}
          disabled={state.asking}
          onPress={() => { props.onToggleKnowledge(resource.id); }}
        />
      ))}
      <Button title="刷新知识列表" disabled={state.asking} onPress={() => { props.onRefreshKnowledge(); }} />
      <Button title="提问" disabled={state.asking || state.phase === 'loading' || state.question.trim() === ''} onPress={() => { props.onAsk(); }} />
      {state.error !== undefined && <Text>{state.error}</Text>}
      {state.turn !== undefined && <TurnView turn={state.turn} />}
      {state.statusLine !== undefined && <Text>{state.statusLine}</Text>}
      {state.retryAvailable && <Button title="重试" disabled={state.asking} onPress={() => { props.onRetry(); }} />}
    </ScrollView>
  );
}

function TurnView({ turn }: { turn: KnowledgeQATurn }) {
  return (
    <View>
      <Text>{`问：${turn.sessionId}`}</Text>
      <Text>{turn.answer === '' ? '（无回答内容）' : turn.answer}</Text>
      <Text>证据</Text>
      {turn.evidence.citations.map((citation: KnowledgeQAEvidenceCitation) => (
        <Text key={citation.citationId}>{evidenceCitationLine(citation)}</Text>
      ))}
      <Text>结论类别</Text>
      {turn.evidence.conclusions.map((conclusion, index) => (
        <Text key={index}>{`${EVIDENCE_KIND_LABEL[conclusion.kind]}${conclusion.modelId === undefined ? '' : ` · ${conclusion.modelId}`}`}</Text>
      ))}
      {!turn.evidence.semanticGraphUsed && <Text>未使用语义图谱（本地检索）</Text>}
    </View>
  );
}
