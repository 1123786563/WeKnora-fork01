import React, { useMemo, useState } from 'react';
import { FlatList, StyleSheet, Text, View } from 'react-native';
import { useWeknoraTheme } from '../ui/theme.ts';
import { Card } from '../ui/Card.tsx';
import { Button } from '../ui/Button.tsx';
import { StatusBadge, type BadgeTone } from '../ui/StatusBadge.tsx';
import { StateView } from '../ui/StateView.tsx';
import {
  platformTarget,
  selectExecutionTarget,
  targetCapabilityExplanation,
  type ExecutionTargetOption,
} from '@weknora/domain/mobile';

/**
 * 执行目标选择页 M15（MX-026）：create/observe 能力分开裁决；不可观察的远程不可选；
 * 平台为回退目标；每个目标显示能力解释（不只靠禁用态）。
 */
export interface TargetPickerScreenProps {
  remoteTargets: readonly ExecutionTargetOption[];
  initialSelected?: string;
  onConfirm: (targetID: string) => void;
  onCancel: () => void;
  testID?: string;
}

export function TargetPickerScreen({ remoteTargets, initialSelected, onConfirm, onCancel, testID }: TargetPickerScreenProps) {
  const { theme } = useWeknoraTheme();
  const [selected, setSelected] = useState<string>(initialSelected ?? 'platform');
  const selection = useMemo(() => selectExecutionTarget(remoteTargets, selected), [remoteTargets, selected]);
  const all = useMemo(() => [platformTarget(), ...remoteTargets.filter((t) => t.visibility === 'visible')], [remoteTargets]);

  return (
    <View testID={testID} style={[styles.container, { backgroundColor: theme.colors.bg }]}>
      <FlatList
        data={all}
        keyExtractor={(item) => item.id}
        contentContainerStyle={{ padding: theme.spacing[16], gap: theme.spacing[12] }}
        renderItem={({ item }) => {
          const admitted = item.kind === 'platform' || selection.admittedRemoteCount > 0 && selection.selectedTarget === item.id || item.id === 'platform' ? true : !selection.rejected.some((r) => r.id === item.id);
          const isSelected = selection.selectedTarget === item.id;
          const explanation = targetCapabilityExplanation(item);
          const tone: BadgeTone = item.kind === 'platform' ? 'brand' : admitted ? 'brand' : 'warning';
          return (
            <Card
              tone={isSelected ? 'hero' : 'surface'}
              compact
              onPress={admitted ? () => setSelected(item.id) : undefined}
              accessibilityLabel={`${item.kind === 'platform' ? '平台执行' : `远程目标 ${item.name}`}，${admitted ? '可选择' : `不可选，原因：${explanation}`}`}
            >
              <View style={styles.row}>
                <View style={styles.grow}>
                  <Text style={{ color: theme.colors.ink, fontSize: theme.typography.body.fontSize, lineHeight: theme.typography.body.lineHeight, fontWeight: '600' }}>
                    {item.name}
                  </Text>
                  <Text style={{ color: theme.colors.muted, fontSize: theme.typography.caption.fontSize, lineHeight: theme.typography.caption.lineHeight, marginTop: theme.spacing[4] }}>
                    {explanation}
                  </Text>
                </View>
                <StatusBadge tone={tone} label={admitted ? (isSelected ? '已选择' : '可选') : '不可选'} />
              </View>
            </Card>
          );
        }}
        ListEmptyComponent={<StateView kind="empty" message="只有平台执行可用" detail="远程目标需先授权并处于可观察状态" />}
      />
      <View style={{ padding: theme.spacing[16], gap: theme.spacing[12] }}>
        <Button label="确认目标" onPress={() => onConfirm(selection.selectedTarget)} accessibilityLabel={`确认执行目标 ${selection.selectedTarget}`} />
        <Button label="返回" variant="secondary" onPress={onCancel} accessibilityLabel="返回不选择" size="compact" />
      </View>
    </View>
  );
}

const styles = StyleSheet.create({
  container: { flex: 1 },
  row: { flexDirection: 'row', alignItems: 'center' },
  grow: { flex: 1 },
});
