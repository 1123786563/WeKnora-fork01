import React, { useMemo, useState } from 'react';
import { FlatList, StyleSheet, Text, View } from 'react-native';
import { Card } from '../ui/Card.tsx';
import { StatusBadge } from '../ui/StatusBadge.tsx';
import { StateView } from '../ui/StateView.tsx';
import { Button } from '../ui/Button.tsx';
import { useWeknoraTheme } from '../ui/theme.ts';
import type { MembershipSummary } from '../auth/bootstrap.ts';

/**
 * 空间选择页 M02（MX-011）。数据来自身份引导的真实 memberships；
 * 选择即 scope.switchTo（generation 前进→旧请求中止、旧流关闭、旧可见数据清理）；
 * 当前空间标记；退出后台/断流不影响服务端 Run（客户端隔离语义）。
 */
export interface SpacePickerScreenProps {
  memberships: MembershipSummary[];
  currentTenantId: string | null;
  onSelect: (tenantId: string) => void;
  onSignOut: () => void;
  testID?: string;
}

export function SpacePickerScreen({ memberships, currentTenantId, onSelect, onSignOut, testID }: SpacePickerScreenProps) {
  const { theme } = useWeknoraTheme();
  const [busyTenant, setBusyTenant] = useState<string | null>(null);
  const active = useMemo(() => memberships.filter((row) => !row.status || row.status === 'active'), [memberships]);

  const pick = (tenantId: string) => {
    if (busyTenant || tenantId === currentTenantId) return;
    setBusyTenant(tenantId);
    try {
      onSelect(tenantId);
    } finally {
      setBusyTenant(null);
    }
  };

  return (
    <View testID={testID} style={[styles.container, { backgroundColor: theme.colors.bg }]}>
      <Text style={{ color: theme.colors.ink, fontSize: theme.typography.title.fontSize, lineHeight: theme.typography.title.lineHeight, fontWeight: '600', paddingHorizontal: theme.spacing[20], paddingTop: theme.spacing[20] }}>
        选择空间
      </Text>
      <Text style={{ color: theme.colors.muted, fontSize: theme.typography['body-sm'].fontSize, lineHeight: theme.typography['body-sm'].lineHeight, paddingHorizontal: theme.spacing[20], paddingVertical: theme.spacing[8] }}>
        空间是独立的购买与授权单位；切换会中止当前空间的请求并清理本地可见数据。
      </Text>
      {active.length === 0 ? (
        <StateView kind="empty" message="没有可用的空间" detail="请先接受空间邀请或联系空间管理员" actionLabel="退出登录" onAction={onSignOut} />
      ) : (
        <FlatList
          data={active}
          keyExtractor={(row) => row.tenantId}
          contentContainerStyle={{ padding: theme.spacing[20], gap: theme.spacing[12] }}
          renderItem={({ item }) => (
            <Card
              onPress={() => pick(item.tenantId)}
              accessibilityLabel={`进入空间 ${item.tenantName ?? item.tenantId}`}
              tone={item.tenantId === currentTenantId ? 'hero' : 'surface'}
            >
              <View style={styles.row}>
                <View style={styles.grow}>
                  <Text style={{ color: theme.colors.ink, fontSize: theme.typography.body.fontSize, lineHeight: theme.typography.body.lineHeight, fontWeight: '600' }}>
                    {item.tenantName ?? `空间 ${item.tenantId}`}
                  </Text>
                  <Text style={{ color: theme.colors.muted, fontSize: theme.typography.caption.fontSize, lineHeight: theme.typography.caption.lineHeight }}>
                    {item.role ? `角色：${item.role}` : ''}
                  </Text>
                </View>
                {item.tenantId === currentTenantId ? <StatusBadge tone="brand" label="当前空间" /> : null}
              </View>
            </Card>
          )}
        />
      )}
      <View style={{ padding: theme.spacing[20] }}>
        <Button label="退出登录" variant="secondary" onPress={onSignOut} accessibilityLabel="退出当前账号" size="compact" />
      </View>
    </View>
  );
}

const styles = StyleSheet.create({
  container: { flex: 1 },
  row: { flexDirection: 'row', alignItems: 'center' },
  grow: { flex: 1 },
});
