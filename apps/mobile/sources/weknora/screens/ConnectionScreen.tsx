import React, { useCallback, useEffect, useMemo, useState } from 'react';
import { ScrollView, StyleSheet, Text, View } from 'react-native';
import { useWeknoraTheme } from '../ui/theme.ts';
import { Card } from '../ui/Card.tsx';
import { Button } from '../ui/Button.tsx';
import { Sheet } from '../ui/Sheet.tsx';
import { StatusBadge, type BadgeTone } from '../ui/StatusBadge.tsx';
import { StateView } from '../ui/StateView.tsx';
import { ConnectionConsent } from '../interactions/ConnectionConsent.tsx';
import {
  createConnectionController,
  type ConnectionState,
  type ConnectionPorts,
} from '../resources/connection-controller.ts';

/**
 * 连接详情页 M13（MX-023）：个人/空间连接显式区分；授权经产品 API+系统浏览器；
 * 撤销带当前授权版本（确认 Sheet）→撤销后权限立即失效；不暴露 Provider 凭据。
 */
export interface ConnectionScreenProps {
  connectionID: string;
  ports: ConnectionPorts;
  onAuthorizeInBrowser?: (connectionID: string) => void;
  testID?: string;
}

const OWNERSHIP_LABEL = { personal: '个人连接（仅你可见可用）', tenant: '空间连接（空间成员可用，凭据由服务端保管）' } as const;
const STATUS_TONE: Record<ConnectionState['status'], BadgeTone> = {
  connected: 'brand', disconnected: 'neutral', revoked: 'danger',
};

export function ConnectionScreen({ connectionID, ports, onAuthorizeInBrowser, testID }: ConnectionScreenProps) {
  const { theme } = useWeknoraTheme();
  const controller = useMemo(() => createConnectionController(ports), [ports]);
  const [connection, setConnection] = useState<ConnectionState | null>(null);
  const [phase, setPhase] = useState<'loading' | 'ready' | 'error'>('loading');
  const [confirmRevoke, setConfirmRevoke] = useState(false);
  const [errorMessage, setErrorMessage] = useState('');

  const refresh = useCallback(async () => {
    setPhase('loading');
    try {
      const next = await controller.open(connectionID);
      if (!next) {
        setPhase('error');
        setErrorMessage('连接不存在或已不可访问');
        return;
      }
      setConnection(next);
      setPhase('ready');
    } catch (error) {
      setPhase('error');
      setErrorMessage(error instanceof Error ? error.message : '查询失败');
    }
  }, [connectionID, controller]);

  useEffect(() => {
    void refresh();
  }, [refresh]);

  const doRevoke = async () => {
    setConfirmRevoke(false);
    if (!connection) return;
    try {
      const next = await controller.revoke(connection);
      setConnection(next); // 撤销成功：立即显示 revoked（正在显示的权限失效）
    } catch {
      setErrorMessage('撤销失败（版本可能已变化，重新查询后重试）');
      void refresh();
    }
  };

  return (
    <ScrollView testID={testID} style={{ backgroundColor: theme.colors.bg }} contentContainerStyle={{ padding: theme.spacing[16], gap: theme.spacing[16] }}>
      {phase === 'loading' ? <StateView kind="loading" message="正在查询连接状态" /> : null}
      {phase === 'error' ? <StateView kind="error" message="连接加载失败" detail={errorMessage} actionLabel="重试" onAction={() => void refresh()} /> : null}
      {phase === 'ready' && connection ? (
        <>
          <Card>
            <View style={styles.row}>
              <StatusBadge tone={STATUS_TONE[connection.status]} label={connection.status === 'connected' ? '已连接' : connection.status === 'revoked' ? '已撤销' : '未连接'} />
              <Text accessibilityLabel={`连接 ${connection.name}，${OWNERSHIP_LABEL[connection.ownership]}，授权版本 ${connection.authVersion}`} style={{ color: theme.colors.muted, flex: 1, marginLeft: theme.spacing[8], fontSize: theme.typography.caption.fontSize, lineHeight: theme.typography.caption.lineHeight }}>
                授权版本 v{connection.authVersion}
              </Text>
            </View>
            <Text style={{ color: theme.colors.ink, fontSize: theme.typography.subtitle.fontSize, lineHeight: theme.typography.subtitle.lineHeight, fontWeight: '600', marginTop: theme.spacing[8] }}>
              {connection.name}
            </Text>
            <Text style={{ color: theme.colors.muted, fontSize: theme.typography['body-sm'].fontSize, lineHeight: theme.typography['body-sm'].lineHeight, marginTop: theme.spacing[8] }}>
              {OWNERSHIP_LABEL[connection.ownership]}
            </Text>
            <Text style={{ color: theme.colors.subtle, fontSize: theme.typography.caption.fontSize, lineHeight: theme.typography.caption.lineHeight, marginTop: theme.spacing[8] }}>
              服务商凭据由服务端保管——本设备不保存、不显示任何密钥。
            </Text>
          </Card>

          <ConnectionConsent
            connectionName={connection.name}
            scopeDescription={OWNERSHIP_LABEL[connection.ownership]}
            state={connection.status === 'connected' ? 'connected' : connection.status === 'revoked' ? 'revoked' : 'idle'}
            onAuthorize={onAuthorizeInBrowser ? () => onAuthorizeInBrowser(connectionID) : undefined}
            onRecheck={() => void refresh()}
          />

          {connection.status === 'connected' ? (
            <Button
              label="撤销连接"
              variant="danger"
              onPress={() => setConfirmRevoke(true)}
              accessibilityLabel={`撤销 ${connection.name}（授权版本 ${connection.authVersion}，撤销后正在显示的权限立即失效）`}
            />
          ) : null}
        </>
      ) : null}

      <Sheet
        visible={confirmRevoke}
        title="确认撤销连接"
        onClose={() => setConfirmRevoke(false)}
        footer={
          <View style={{ gap: theme.spacing[8] }}>
            <Button label="确认撤销（v{版本}）" variant="danger" accessibilityLabel={`确认撤销连接，携带授权版本 ${connection?.authVersion}`} onPress={() => void doRevoke()} />
            <Button label="返回" variant="secondary" accessibilityLabel="返回不撤销" onPress={() => setConfirmRevoke(false)} size="compact" />
          </View>
        }
      >
        <Text style={{ color: theme.colors.ink, fontSize: theme.typography['body-sm'].fontSize, lineHeight: theme.typography['body-sm'].lineHeight }}>
          将以授权版本 v{connection?.authVersion} 撤销「{connection?.name}」。撤销后：正在显示的权限立即失效；使用该连接的任务需要重新授权才能继续。
        </Text>
      </Sheet>
    </ScrollView>
  );
}

const styles = StyleSheet.create({
  row: { flexDirection: 'row', alignItems: 'center' },
});
