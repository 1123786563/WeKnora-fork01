import React, { useMemo, useState } from 'react';
import { FlatList, Pressable, StyleSheet, Text, View } from 'react-native';
import {
  defaultAgent,
  filterAgents,
  selectAgent,
  toAgentOptions,
  type AgentOption,
} from '@weknora/domain/mobile';
import { useWeknoraTheme } from '../ui/theme.ts';
import { Card } from '../ui/Card.tsx';
import { StatusBadge, type BadgeTone } from '../ui/StatusBadge.tsx';
import { StateView } from '../ui/StateView.tsx';
import { Field } from '../ui/Field.tsx';
import { Button } from '../ui/Button.tsx';

/**
 * Agent 选择页 M06（MX-016）：名称/能力摘要展示（无 Prompt 配置）；
 * 类型+关键词筛选；不可用显示原因（禁止隐藏入口/静默 fallback）；
 * 编码 Agent 只在授权 target 存在时可选；选中返回原表单。
 */
export interface AuthorizedTarget {
  id: string;
  revoked?: boolean;
}

export interface AgentPickerScreenProps {
  /** 原始目录行（含可能的敏感字段由 toAgentOptions 投影过滤） */
  rows: ReadonlyArray<Record<string, unknown>>;
  authorizedTargets: readonly AuthorizedTarget[];
  initialSelectedId?: string;
  onConfirm: (agentId: string) => void;
  onCancel: () => void;
  testID?: string;
}

const KIND_LABEL: Record<AgentOption['kind'], string> = {
  general: '通用', coding: '编码', analysis: '分析', custom: '自定义',
};

const CAPABILITY_TONE: Record<string, BadgeTone> = {
  supported: 'brand', unavailable: 'warning', forbidden: 'danger',
};

export function AgentPickerScreen({ rows, authorizedTargets, initialSelectedId, onConfirm, onCancel, testID }: AgentPickerScreenProps) {
  const { theme } = useWeknoraTheme();
  const directory = useMemo(() => ({ agents: toAgentOptions(rows) }), [rows]);
  const [selectedId, setSelectedId] = useState<string | undefined>(undefined);
  const [keyword, setKeyword] = useState('');
  const [kindFilter, setKindFilter] = useState<AgentOption['kind'] | undefined>(undefined);

  const effectiveSelected = useMemo(() => selectedId ?? initialSelectedId ?? defaultAgent(directory, authorizedTargets)?.id, [selectedId, initialSelectedId, directory, authorizedTargets]);
  const selection = useMemo(() => (effectiveSelected ? selectAgent(directory, effectiveSelected, authorizedTargets) : undefined), [directory, effectiveSelected, authorizedTargets]);
  const visible = useMemo(() => filterAgents(directory, { kind: kindFilter, keyword }), [directory, kindFilter, keyword]);

  const kinds: ReadonlyArray<AgentOption['kind']> = ['general', 'coding', 'analysis', 'custom'];

  return (
    <View testID={testID} style={[styles.container, { backgroundColor: theme.colors.bg }]}>
      <View style={{ padding: theme.spacing[16], gap: theme.spacing[12] }}>
        <Field label="关键词" value={keyword} onChangeText={setKeyword} placeholder="按名称或能力摘要筛选" />
        <View style={styles.filters} accessibilityRole="tablist">
          {kinds.map((kind) => {
            const selected = kindFilter === kind;
            return (
              <Pressable
                key={kind}
                accessibilityRole="tab"
                accessibilityLabel={KIND_LABEL[kind]}
                accessibilityState={{ selected }}
                onPress={() => setKindFilter(selected ? undefined : kind)}
                style={[styles.chip, { borderColor: selected ? theme.colors.brand : theme.colors['control-line'], backgroundColor: selected ? theme.colors['brand-soft'] : theme.colors.surface, borderRadius: theme.radius.pill, paddingHorizontal: theme.spacing[12], paddingVertical: theme.spacing[6] }]}
              >
                <Text style={{ color: selected ? theme.colors.brand : theme.colors.muted, fontSize: theme.typography.caption.fontSize, lineHeight: theme.typography.caption.lineHeight }}>{KIND_LABEL[kind]}</Text>
              </Pressable>
            );
          })}
        </View>
      </View>
      {visible.length === 0 ? (
        <StateView kind="empty" message="没有匹配的 Agent" detail="调整筛选条件" />
      ) : (
        <FlatList
          data={visible}
          keyExtractor={(item) => item.id}
          contentContainerStyle={{ padding: theme.spacing[16], gap: theme.spacing[12] }}
          renderItem={({ item }) => {
            const resolved = selectAgent({ agents: [item] }, item.id, authorizedTargets);
            const selectable = resolved.agent !== undefined;
            const isSelected = effectiveSelected === item.id;
            return (
              <Card
                tone={isSelected ? 'hero' : 'surface'}
                compact
                onPress={selectable ? () => setSelectedId(item.id) : undefined}
                accessibilityLabel={`${KIND_LABEL[item.kind]} Agent ${item.name}${selectable ? '，可选择' : `，不可用，原因 ${resolved.unavailableReason}`}`}
              >
                <View style={styles.row}>
                  <View style={styles.grow}>
                    <Text style={{ color: theme.colors.ink, fontSize: theme.typography.body.fontSize, lineHeight: theme.typography.body.lineHeight, fontWeight: '600' }}>{item.name}</Text>
                    <Text style={{ color: theme.colors.muted, fontSize: theme.typography.caption.fontSize, lineHeight: theme.typography.caption.lineHeight, marginTop: theme.spacing[4] }}>{item.summary || KIND_LABEL[item.kind]}</Text>
                  </View>
                  <StatusBadge tone={CAPABILITY_TONE[item.capability.state]} label={selectable ? '可用' : (resolved.unavailableReason ?? item.capability.state)} />
                </View>
              </Card>
            );
          }}
        />
      )}
      <View style={{ padding: theme.spacing[16], gap: theme.spacing[12] }}>
        {selection && !selection.agent ? (
          <Text accessibilityRole="alert" style={{ color: theme.colors.warning, fontSize: theme.typography['body-sm'].fontSize, lineHeight: theme.typography['body-sm'].lineHeight }}>
            当前选择不可用：{selection.unavailableReason}
          </Text>
        ) : null}
        <Button label="确认选择" onPress={() => { if (selection?.agent) onConfirm(selection.agent.id); }} disabled={!selection?.agent} accessibilityLabel={selection?.agent ? `确认选择 ${selection.agent.name}` : '没有可用的选择'} />
        <Button label="返回" variant="secondary" onPress={onCancel} accessibilityLabel="返回原表单" size="compact" />
      </View>
    </View>
  );
}

const styles = StyleSheet.create({
  container: { flex: 1 },
  filters: { flexDirection: 'row', gap: 8, flexWrap: 'wrap' },
  chip: { borderWidth: StyleSheet.hairlineWidth },
  row: { flexDirection: 'row', alignItems: 'center' },
  grow: { flex: 1 },
});
