import React, { useMemo, useState } from 'react';
import { Pressable, ScrollView, StyleSheet, Text, View } from 'react-native';
import { useWeknoraTheme } from '../ui/theme.ts';
import { Card } from '../ui/Card.tsx';
import { StatusBadge } from '../ui/StatusBadge.tsx';
import { StateView } from '../ui/StateView.tsx';
import { nativeTokens } from '@weknora/design-tokens/mobile';

/**
 * 资源页 M11（MX-022）：分「知识 / 连接 / 成果」段——不复制全后台菜单。
 * 每段由宿主注入 loader；撤权后敏感内容不可见（KnowledgeScreen 承载知识交互）。
 */
export interface ResourcesScreenProps {
  knowledge?: React.ReactNode;
  connections?: React.ReactNode;
  artifacts?: React.ReactNode;
  loading?: boolean;
  error?: string | null;
  onRetry?: () => void;
  testID?: string;
}

const SECTIONS = [
  { id: 'knowledge', label: '知识库' },
  { id: 'connections', label: '连接' },
  { id: 'artifacts', label: '任务成果' },
] as const;

export function ResourcesScreen({ knowledge, connections, artifacts, loading, error, onRetry, testID }: ResourcesScreenProps) {
  const { theme } = useWeknoraTheme();
  const [section, setSection] = useState<(typeof SECTIONS)[number]['id']>('knowledge');
  if (loading) {
    return <View style={[styles.container, { backgroundColor: theme.colors.bg }]} testID={testID}><StateView kind="loading" message="正在加载资源" /></View>;
  }
  if (error) {
    return (
      <View style={[styles.container, { backgroundColor: theme.colors.bg }]} testID={testID}>
        <StateView kind="error" message="资源加载失败" detail={error} actionLabel="重试" onAction={onRetry} />
      </View>
    );
  }
  return (
    <View testID={testID} style={[styles.container, { backgroundColor: theme.colors.bg }]}>
      <View accessibilityRole="tablist" style={[styles.tabs, { backgroundColor: theme.colors.surface, borderBottomColor: theme.colors.line }]}>
        {SECTIONS.map((item) => {
          const selected = section === item.id;
          return (
            <Pressable
              key={item.id}
              accessibilityRole="tab"
              accessibilityLabel={item.label}
              accessibilityState={{ selected }}
              onPress={() => setSection(item.id)}
              style={[styles.tab, { borderColor: selected ? theme.colors.brand : 'transparent' }]}
            >
              <Text style={{ color: selected ? theme.colors.brand : theme.colors.muted, fontSize: theme.typography['body-sm'].fontSize, lineHeight: theme.typography['body-sm'].lineHeight, fontWeight: selected ? '600' : '400' }}>
                {item.label}
              </Text>
            </Pressable>
          );
        })}
      </View>
      <ScrollView contentContainerStyle={{ padding: theme.spacing[16], gap: theme.spacing[12] }}>
        {section === 'knowledge' ? (knowledge ?? <StateView kind="empty" message="暂无知识库" detail="知识库是可选资源，不是任务的前置条件" />) : null}
        {section === 'connections' ? (connections ?? <StateView kind="empty" message="暂无连接" />) : null}
        {section === 'artifacts' ? (artifacts ?? <StateView kind="empty" message="暂无任务成果" />) : null}
      </ScrollView>
    </View>
  );
}

const styles = StyleSheet.create({
  container: { flex: 1 },
  tabs: { flexDirection: 'row', borderBottomWidth: StyleSheet.hairlineWidth },
  tab: { flex: 1, alignItems: 'center', paddingVertical: nativeTokens.spacing[12], borderBottomWidth: 2 },
});

export { Card as ResourceCard, StatusBadge as ResourceBadge, StateView as ResourceStateView };
