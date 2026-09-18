import React, { useMemo } from 'react';
import { FlatList, StyleSheet, Text, View } from 'react-native';
import { useWeknoraTheme } from '../ui/theme.ts';
import { Card } from '../ui/Card.tsx';
import { StatusBadge } from '../ui/StatusBadge.tsx';
import { StateView } from '../ui/StateView.tsx';
import { Button } from '../ui/Button.tsx';
import {
  knowledgeRefForPrompt,
  revokedKnowledgeProjection,
  scanStatusPresentation,
  toKnowledgeResource,
  type KnowledgeResource,
} from '@weknora/domain/mobile';

/**
 * 知识消费页（MX-022 / M11 知识段 + M12 前身）：
 * - 列表展示扫描/索引状态（文字+tone 双通道）；
 * - 撤权（403）→ 敏感字段全部不可见（revoked 投影，不缓存回填）；
 * - 「用此知识提问」只携带引用（ref），不复制文档；
 * - 收藏按 scope 隔离（favorites 注入）。
 */
export interface KnowledgeScreenProps {
  rows: ReadonlyArray<Record<string, unknown>>;
  access: { revoked: boolean; reason?: string };
  favorites?: { isFavorite(id: string): boolean; toggle(id: string): void };
  onOpenKnowledge?: (id: string) => void;
  onAskWithKnowledge?: (knowledgeId: string) => void;
  loading?: boolean;
  onRetry?: () => void;
  testID?: string;
}

export function KnowledgeScreen({ rows, access, favorites, onOpenKnowledge, onAskWithKnowledge, loading, onRetry, testID }: KnowledgeScreenProps) {
  const { theme } = useWeknoraTheme();
  const resources = useMemo(() => rows.map(toKnowledgeResource).filter((item) => item.id !== ''), [rows]);
  const revoked = access.revoked ? revokedKnowledgeProjection() : null;

  if (loading) {
    return <View style={styles.container} testID={testID}><StateView kind="loading" message="正在加载知识库" /></View>;
  }
  if (revoked) {
    return (
      <View style={styles.container} testID={testID}>
        <StateView kind="noPermission" message="无法访问知识库" detail={access.reason ?? '该空间的访问已被撤销'} actionLabel="重试" onAction={onRetry} />
      </View>
    );
  }
  if (resources.length === 0) {
    return (
      <View style={styles.container} testID={testID}>
        <StateView kind="empty" message="暂无知识库" detail="知识库是可选资源——任务无需预先创建" />
      </View>
    );
  }
  return (
    <FlatList
      testID={testID}
      data={resources}
      keyExtractor={(item) => item.id}
      contentContainerStyle={{ padding: theme.spacing[16], gap: theme.spacing[12] }}
      renderItem={({ item }) => (
        <KnowledgeCard
          resource={item}
          favorite={favorites?.isFavorite(item.id) ?? false}
          onToggleFavorite={() => favorites?.toggle(item.id)}
          onOpen={onOpenKnowledge ? () => onOpenKnowledge(item.id) : undefined}
          onAsk={onAskWithKnowledge}
          access={{ revoked: false }}
        />
      )}
    />
  );
}

export function KnowledgeCard({ resource, favorite, onToggleFavorite, onOpen, onAsk, access }: {
  resource: KnowledgeResource;
  favorite: boolean;
  onToggleFavorite?: () => void;
  onOpen?: () => void;
  onAsk?: (knowledgeId: string) => void;
  access: { revoked: boolean };
}) {
  const { theme } = useWeknoraTheme();
  const scan = scanStatusPresentation(resource.scanStatus);
  const ref = knowledgeRefForPrompt(resource, access);
  return (
    <Card onPress={onOpen} accessibilityLabel={`知识库 ${resource.title}，${scan.label}，${resource.documentCount} 个文档`}>
      <View style={styles.row}>
        <View style={styles.grow}>
          <Text style={{ color: theme.colors.ink, fontSize: theme.typography.body.fontSize, lineHeight: theme.typography.body.lineHeight, fontWeight: '600' }}>{resource.title}</Text>
          <Text style={{ color: theme.colors.muted, fontSize: theme.typography.caption.fontSize, lineHeight: theme.typography.caption.lineHeight, marginTop: theme.spacing[4] }}>
            {resource.documentCount} 个文档 · 更新于 {resource.updatedAt || '未知时间'}
          </Text>
        </View>
        <StatusBadge tone={scan.tone} label={scan.label} />
      </View>
      <View style={[styles.actions, { marginTop: theme.spacing[12], gap: theme.spacing[8] }]}>
        {onAsk ? (
          <Button
            label="用此知识提问"
            size="compact"
            variant="secondary"
            disabled={!ref}
            accessibilityLabel={ref ? `以引用方式使用 ${resource.title} 提问（不复制文档）` : '当前无权使用该知识库提问'}
            onPress={() => { if (ref) onAsk(ref.knowledgeId); }}
          />
        ) : null}
        {onToggleFavorite ? (
          <Button label={favorite ? '取消收藏' : '收藏'} size="compact" variant="secondary" onPress={onToggleFavorite} accessibilityLabel={`${favorite ? '取消收藏' : '收藏'} ${resource.title}（收藏仅保存在当前空间）`} />
        ) : null}
      </View>
    </Card>
  );
}

const styles = StyleSheet.create({
  container: { flex: 1 },
  row: { flexDirection: 'row', alignItems: 'center' },
  grow: { flex: 1 },
  actions: { flexDirection: 'row' },
});
