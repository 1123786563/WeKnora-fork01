import React from 'react';
import { StyleSheet, Text, View } from 'react-native';
import { useWeknoraTheme } from '../ui/theme.ts';
import { Card } from '../ui/Card.tsx';
import { Button } from '../ui/Button.tsx';
import { StatusBadge } from '../ui/StatusBadge.tsx';

/**
 * 预算审批卡（MX-020）：显示旧上限/新申请/差额三值；budget 授权角色校验；
 * 预算决定是 extend（D-012 矩阵）——与工具审批 approve/reject 语义互斥。
 */
export interface BudgetApprovalProps {
  currentUpper: number;
  requestedUpper: number;
  currency: string;
  canAuthorize: boolean;
  onExtend?: (requestedUpper: number) => void;
  testID?: string;
}

export function BudgetApproval({ currentUpper, requestedUpper, currency, canAuthorize, onExtend, testID }: BudgetApprovalProps) {
  const { theme } = useWeknoraTheme();
  const delta = requestedUpper - currentUpper;
  return (
    <Card testID={testID}>
      <View style={styles.row}>
        <StatusBadge tone="warning" label="预算增加申请" />
        <Text accessibilityLabel={`当前上限 ${currentUpper} ${currency}，申请新上限 ${requestedUpper} ${currency}，增加 ${delta} ${currency}`} style={{ color: theme.colors.muted, flex: 1, marginLeft: theme.spacing[8], fontSize: theme.typography.caption.fontSize, lineHeight: theme.typography.caption.lineHeight }}>
          需要预算授权（extend）
        </Text>
      </View>
      {([
        ['当前上限', currentUpper],
        ['申请新上限', requestedUpper],
        ['增加差额', delta],
      ] as const).map(([label, value]) => (
        <View key={label} style={[styles.row, { marginTop: theme.spacing[8] }]}>
          <Text style={{ color: theme.colors.muted, flex: 1, fontSize: theme.typography['body-sm'].fontSize, lineHeight: theme.typography['body-sm'].lineHeight }}>{label}</Text>
          <Text style={{ color: theme.colors.ink, fontSize: theme.typography.body.fontSize, lineHeight: theme.typography.body.lineHeight, fontWeight: '700' }}>
            {value.toLocaleString()} {currency}
          </Text>
        </View>
      ))}
      <Text style={{ color: theme.colors.subtle, fontSize: theme.typography.caption.fontSize, lineHeight: theme.typography.caption.lineHeight, marginTop: theme.spacing[12] }}>
        {canAuthorize ? '你有预算授权权限。批准仅提升上限，不直接执行任何任务。' : '预算授权需要空间所有者/账单管理员权限——请联系有权限的成员处理。'}
      </Text>
      {canAuthorize && onExtend ? (
        <View style={{ marginTop: theme.spacing[12] }}>
          <Button label={`批准提升至 ${requestedUpper.toLocaleString()}`} onPress={() => onExtend(requestedUpper)} accessibilityLabel={`批准将预算上限提升到 ${requestedUpper} ${currency}`} />
        </View>
      ) : null}
    </Card>
  );
}

const styles = StyleSheet.create({
  row: { flexDirection: 'row', alignItems: 'center' },
});
