import React, { useMemo } from 'react';
import { ScrollView, StyleSheet, Text, View } from 'react-native';
import { useWeknoraTheme } from '../ui/theme.ts';
import { Card } from '../ui/Card.tsx';
import { StatusBadge } from '../ui/StatusBadge.tsx';
import { StateView } from '../ui/StateView.tsx';
import { presentUsage, type TenantUsageSnapshot } from '../commercial/usage-presenter.ts';

/**
 * 空间用量页 M18（MX-031）：只读商业视图——三桶分离（预留/未最终/已结算）、
 * as_of+单位+结算最终性可见、账单账户作用域、BYOK 与平台费分列、分权说明；
 * 不含任何购买/退款入口。
 */
export interface UsageScreenProps {
  snapshot: TenantUsageSnapshot | null;
  loading?: boolean;
  error?: string | null;
  onRetry?: () => void;
  testID?: string;
}

export function UsageScreen({ snapshot, loading, error, onRetry, testID }: UsageScreenProps) {
  const { theme } = useWeknoraTheme();
  const presentation = useMemo(() => (snapshot ? presentUsage(snapshot) : null), [snapshot]);

  if (loading) {
    return <ScrollView testID={testID} style={{ backgroundColor: theme.colors.bg }} contentContainerStyle={{ padding: theme.spacing[20] }}><StateView kind="loading" message="正在加载空间用量" /></ScrollView>;
  }
  if (error || !snapshot || !presentation) {
    return (
      <ScrollView testID={testID} style={{ backgroundColor: theme.colors.bg }} contentContainerStyle={{ padding: theme.spacing[20] }}>
        <StateView kind="error" message="用量数据加载失败" detail={error ?? '暂无可用数据'} actionLabel="重试" onAction={onRetry} />
      </ScrollView>
    );
  }

  return (
    <ScrollView testID={testID} style={{ backgroundColor: theme.colors.bg }} contentContainerStyle={{ padding: theme.spacing[16], gap: theme.spacing[16] }}>
      <Card tone="hero">
        <Text style={{ color: theme.colors['hero-ink'], fontSize: theme.typography.title.fontSize, lineHeight: theme.typography.title.lineHeight, fontWeight: '600' }}>空间用量</Text>
        <Text accessibilityLabel={presentation.asOfLabel} style={{ color: theme.colors.muted, fontSize: theme.typography.caption.fontSize, lineHeight: theme.typography.caption.lineHeight, marginTop: theme.spacing[4] }}>
          {presentation.asOfLabel}
        </Text>
        <Text accessibilityLabel={presentation.scopeLabel} style={{ color: theme.colors.subtle, fontSize: theme.typography.caption.fontSize, lineHeight: theme.typography.caption.lineHeight, marginTop: theme.spacing[4] }}>
          {presentation.scopeLabel}
        </Text>
      </Card>

      <Card>
        <Text style={{ color: theme.colors.ink, fontSize: theme.typography.body.fontSize, lineHeight: theme.typography.body.lineHeight, fontWeight: '600' }}>用量汇总</Text>
        {([
          ['已预留（进行中任务）', presentation.reserved, snapshot.reserved.currency, snapshot.reserved.unitLabel, 'neutral'],
          ['未最终（结算中）', presentation.pending, snapshot.pending.currency, snapshot.pending.unitLabel, 'warning'],
          ['已结算（最终）', presentation.settled, snapshot.settled.currency, snapshot.settled.unitLabel, 'brand'],
        ] as const).map(([label, value, currency, unit, tone]) => (
          <View key={label} style={[styles.row, { marginTop: theme.spacing[12] }]}>
            <Text style={{ color: theme.colors.muted, flex: 1, fontSize: theme.typography['body-sm'].fontSize, lineHeight: theme.typography['body-sm'].lineHeight }}>{label}</Text>
            <Text style={{ color: theme.colors.ink, fontSize: theme.typography.body.fontSize, lineHeight: theme.typography.body.lineHeight, fontWeight: '700' }}>
              {value.toLocaleString()} {currency}
            </Text>
            <View style={{ marginLeft: theme.spacing[8] }}><StatusBadge tone={tone} label={label.includes('未最终') ? '未最终' : label.includes('预留') ? '预占' : '最终'} /></View>
          </View>
        ))}
        <Text style={{ color: theme.colors.subtle, fontSize: theme.typography.caption.fontSize, lineHeight: theme.typography.caption.lineHeight, marginTop: theme.spacing[12] }}>
          未最终金额仍在结算中，最终以已结算口径为准；预占随任务结束释放或转入结算。
        </Text>
      </Card>

      <Card>
        <Text style={{ color: theme.colors.ink, fontSize: theme.typography.body.fontSize, lineHeight: theme.typography.body.lineHeight, fontWeight: '600' }}>费用构成</Text>
        <View style={[styles.row, { marginTop: theme.spacing[12] }]}>
          <Text style={{ color: theme.colors.muted, flex: 1, fontSize: theme.typography['body-sm'].fontSize, lineHeight: theme.typography['body-sm'].lineHeight }}>平台服务费</Text>
          <Text style={{ color: theme.colors.ink, fontSize: theme.typography['body-sm'].fontSize, fontWeight: '600' }}>
            {snapshot.breakdown.platformServiceFees.value.toLocaleString()} {snapshot.breakdown.platformServiceFees.currency}
          </Text>
        </View>
        <View style={[styles.row, { marginTop: theme.spacing[8] }]}>
          <Text style={{ color: theme.colors.muted, flex: 1, fontSize: theme.typography['body-sm'].fontSize, lineHeight: theme.typography['body-sm'].lineHeight }}>BYOK 模型费（自带密钥）</Text>
          <Text style={{ color: theme.colors.ink, fontSize: theme.typography['body-sm'].fontSize, fontWeight: '600' }}>
            {snapshot.breakdown.byokModelFees ? `${snapshot.breakdown.byokModelFees.value.toLocaleString()} ${snapshot.breakdown.byokModelFees.currency}` : '未启用'}
          </Text>
        </View>
        <Text style={{ color: theme.colors.subtle, fontSize: theme.typography.caption.fontSize, lineHeight: theme.typography.caption.lineHeight, marginTop: theme.spacing[12] }}>
          BYOK 模型费用独立于平台服务费；并非所有任务都免费。
        </Text>
      </Card>

      <Card>
        <Text style={{ color: theme.colors.ink, fontSize: theme.typography['body-sm'].fontSize, lineHeight: theme.typography['body-sm'].lineHeight }}>
          {presentation.canManageBilling
            ? '你是空间所有者：账单管理操作在 Web 控制台完成；本页为只读视图，不提供购买或退款入口。'
            : '用量为只读视图；账单管理由空间所有者/账单管理员在 Web 控制台操作。'}
        </Text>
      </Card>
    </ScrollView>
  );
}

const styles = StyleSheet.create({
  row: { flexDirection: 'row', alignItems: 'center' },
});
