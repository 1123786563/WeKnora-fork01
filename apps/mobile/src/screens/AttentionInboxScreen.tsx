import { Button, ScrollView, Text, View } from 'react-native';
import { INTERACTION_ACTIONS } from '@weknora/mobile-core';
import type { InboxItem, InteractionActionValue } from '@weknora/mobile-core';
import type { AttentionInboxViewState } from '../attention-inbox-view.ts';

const ACTION_LABELS: Record<InteractionActionValue, string> = {
  approve: '批准',
  reject: '拒绝',
  extend: '扩展预算',
  retry: '重试',
  provide_result: '提供结果',
  terminate: '终止',
};

const KIND_LABELS: Record<InboxItem['kind'], string> = {
  tool_approval: '工具审批',
  budget: '预算扩展',
  recovery: '恢复请求',
};

export interface AttentionInboxScreenProps {
  state: AttentionInboxViewState;
  onRefresh(): void;
  onDecide(item: InboxItem, action: InteractionActionValue): void;
}

/** Attention Inbox（T08）：同一 Interaction 身份的类型化决定面。动作按冻结矩阵渲染，
 *  receipt 文案如实区分 recorded / delivery-unknown / superseded / gone——绝不把决定
 *  ACK 显示成「外部派发完成」。 */
export function AttentionInboxScreen({ state, onRefresh, onDecide }: AttentionInboxScreenProps) {
  return (
    <ScrollView>
      <Text>Attention Inbox</Text>
      {state.loading && <Text>Loading</Text>}
      {state.error !== undefined && <Text>{state.error}</Text>}
      {state.items !== undefined && state.items.map((item) => (
        <View key={item.interactionId}>
          <Text>{`${KIND_LABELS[item.kind]} · run ${item.runId}`}</Text>
          {INTERACTION_ACTIONS[item.kind].map((action) => (
            <Button key={action} title={ACTION_LABELS[action]} onPress={() => onDecide(item, action)} />
          ))}
        </View>
      ))}
      {state.items !== undefined && state.items.length === 0 && <Text>Nothing needs you</Text>}
      {state.receipts.map((receipt) => (
        <Text key={receipt.key}>{receipt.copy}</Text>
      ))}
      <Button title="Refresh" onPress={onRefresh} disabled={state.loading} />
    </ScrollView>
  );
}
