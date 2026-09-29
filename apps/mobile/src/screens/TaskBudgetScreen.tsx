import { Button, Switch, Text, TextInput, View } from 'react-native';
import type { TaskBudgetViewState } from '../task-budget-view.ts';

export interface TaskBudgetScreenProps {
  taskId: string;
  state: TaskBudgetViewState;
  /** 独立确认（默认不勾选）：预算追加与外部写操作审批相互独立，永不自动勾选。 */
  confirmed: boolean;
  credits: string;
  onToggleConfirmed: (value: boolean) => void;
  onChangeCredits: (value: string) => void;
  onRefresh: () => void;
  onExtend: () => void;
}

/** 任务预算屏（T09 #39）：预计/已用/预占/剩余 四数分立（Story 58），委派与达限暂停
 * 如实投影；扩额入口在 canExtend=false 时禁用（AC1 的 UI 投影——权限权威在服务端）。 */
export function TaskBudgetScreen(props: TaskBudgetScreenProps) {
  const { state } = props;
  const facts = state.facts;
  return (
    <View>
      <Text accessibilityRole="header">任务预算</Text>
      <Text>预算追加与外部写操作审批相互独立：追加预算不会、也不能授权任何外部发送或删除操作。</Text>
      {state.message !== undefined ? <Text>{state.message}</Text> : null}
      {state.error !== undefined ? <Text accessibilityRole="alert">{state.error}</Text> : null}
      {state.loading && facts === undefined ? <Text>正在读取预算…</Text> : null}
      {facts !== undefined ? (
        <View>
          <Text testID="budget-paused">
            {facts.pausedRunIds.length > 0 ? '预算已达限，运行已持久暂停；授权扩额后将恢复同一运行。' : '没有因预算暂停的运行。'}
          </Text>
          <Text testID="budget-limit">预计（批准上限）：{facts.limitCredits} 额度</Text>
          <Text testID="budget-used">已用：{facts.usedCredits} 额度</Text>
          <Text testID="budget-held">预占：{facts.heldCredits} 额度</Text>
          <Text testID="budget-remaining">剩余：{facts.remainingCredits} 额度</Text>
          <Text testID="budget-delegated">委派运行：{facts.delegatedRunIds.length > 0 ? facts.delegatedRunIds.join(', ') : '无'}</Text>
          <Button title="刷新" onPress={props.onRefresh} />
          <Text>追加额度（正整数，含委派运行的合计上限）</Text>
          <TextInput
            testID="budget-credits"
            keyboardType="numeric"
            value={props.credits}
            onChangeText={props.onChangeCredits}
          />
          <Switch
            testID="budget-confirm"
            value={props.confirmed}
            onValueChange={props.onToggleConfirmed}
          />
          <Text>我确认追加此任务预算（仅预算决定，不包含任何外部发送授权）</Text>
        </View>
      ) : null}
      {!state.loading && facts === undefined ? <Text>暂无预算数据。</Text> : null}
      <Button
        testID="budget-extend"
        title={state.extending ? '正在追加…' : '追加预算'}
        disabled={!facts?.canExtend || state.extending || !props.confirmed}
        onPress={props.onExtend}
      />
      {!facts?.canExtend && facts !== undefined ? <Text>当前身份不能追加预算：只有任务所有者或获授权的账单管理员可以增加上限。</Text> : null}
    </View>
  );
}
