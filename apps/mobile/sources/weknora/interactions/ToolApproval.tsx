import React from 'react';
import { StyleSheet, Text, View } from 'react-native';
import { useWeknoraTheme } from '../ui/theme.ts';
import { Card } from '../ui/Card.tsx';
import { StatusBadge } from '../ui/StatusBadge.tsx';
import type { InteractionRecord } from '@weknora/contracts';

/**
 * 工具审批详情卡（MX-019 / M09）：连接账号、精确目标、内容摘要、风险、
 * 版本（revision）与有效期——确认前信息完备；风险不只靠颜色（文字+badge 双通道）。
 */
export interface ToolApprovalProps {
  record: InteractionRecord;
  /** 服务端冻结摘要（args 摘要展示；密钥/敏感内容不进入客户端） */
  targetSummary: { tool: string; account?: string; argumentDigest: string; risk: 'low' | 'medium' | 'high' };
  fetchedAt: string;
  testID?: string;
}

const RISK_LABEL = { low: '低风险', medium: '中风险', high: '高风险' } as const;

export function ToolApproval({ record, targetSummary, fetchedAt, testID }: ToolApprovalProps) {
  const { theme } = useWeknoraTheme();
  return (
    <Card testID={testID}>
      <View style={styles.row}>
        <StatusBadge tone={targetSummary.risk === 'high' ? 'danger' : targetSummary.risk === 'medium' ? 'warning' : 'brand'} label={RISK_LABEL[targetSummary.risk]} />
        <Text accessibilityLabel={`工具 ${targetSummary.tool}${targetSummary.account ? `，账号 ${targetSummary.account}` : ''}，修订 ${record.expected_revision}，详情刷新于 ${fetchedAt}`} style={{ color: theme.colors.muted, flex: 1, marginLeft: theme.spacing[8], fontSize: theme.typography.caption.fontSize, lineHeight: theme.typography.caption.lineHeight }}>
          修订 {record.expected_revision} · 刷新于 {fetchedAt}
        </Text>
      </View>
      <Text style={{ color: theme.colors.ink, fontSize: theme.typography.subtitle.fontSize, lineHeight: theme.typography.subtitle.lineHeight, fontWeight: '600', marginTop: theme.spacing[12] }}>
        工具调用审批
      </Text>
      {([
        ['工具', targetSummary.tool],
        ['连接账号', targetSummary.account ?? '（未指定）'],
        ['内容摘要', targetSummary.argumentDigest],
        ['风险等级', RISK_LABEL[targetSummary.risk]],
        ['版本 (revision)', String(record.expected_revision)],
      ] as const).map(([label, value]) => (
        <View key={label} style={[styles.row, { marginTop: theme.spacing[8] }]}>
          <Text style={{ color: theme.colors.muted, flex: 1, fontSize: theme.typography['body-sm'].fontSize, lineHeight: theme.typography['body-sm'].lineHeight }}>{label}</Text>
          <Text style={{ color: theme.colors.ink, fontSize: theme.typography['body-sm'].fontSize, lineHeight: theme.typography['body-sm'].lineHeight, fontWeight: '600', flexShrink: 1, textAlign: 'right' }}>{value}</Text>
        </View>
      ))}
    </Card>
  );
}

const styles = StyleSheet.create({
  row: { flexDirection: 'row', alignItems: 'center' },
});
