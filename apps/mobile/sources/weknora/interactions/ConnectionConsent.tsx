import React from 'react';
import { StyleSheet, Text, View } from 'react-native';
import { useWeknoraTheme } from '../ui/theme.ts';
import { Card } from '../ui/Card.tsx';
import { Button } from '../ui/Button.tsx';
import { StatusBadge } from '../ui/StatusBadge.tsx';

/**
 * 连接授权同意卡（MX-020）：授权跳转使用**受信系统浏览器**（产品 API 签发的授权 URL）；
 * 回跳后**查询产品连接状态**（authorizing→connected/failed 由服务端状态驱动）；
 * 不在 App 内嵌 Provider 凭据输入；未知交互类型只读展示不 fall back。
 */
export type ConnectionAuthorizationState = 'idle' | 'authorizing' | 'connected' | 'failed' | 'revoked';

export interface ConnectionConsentProps {
  connectionName: string;
  scopeDescription: string;
  state: ConnectionAuthorizationState;
  /** 系统浏览器打开产品 API 签发的授权 URL（非 Provider 直连） */
  onAuthorize?: () => void;
  onRecheck?: () => void;
  testID?: string;
}

const STATE_LABEL: Record<ConnectionAuthorizationState, string> = {
  idle: '未授权', authorizing: '授权中（等待浏览器回跳）', connected: '已连接', failed: '授权失败', revoked: '已撤销',
};

export function ConnectionConsent({ connectionName, scopeDescription, state, onAuthorize, onRecheck, testID }: ConnectionConsentProps) {
  const { theme } = useWeknoraTheme();
  return (
    <Card testID={testID}>
      <View style={styles.row}>
        <StatusBadge tone={state === 'connected' ? 'brand' : state === 'authorizing' ? 'warning' : state === 'failed' || state === 'revoked' ? 'danger' : 'neutral'} label={STATE_LABEL[state]} />
        <Text accessibilityLabel={`连接 ${connectionName}，状态：${STATE_LABEL[state]}。授权范围：${scopeDescription}`} style={{ color: theme.colors.muted, flex: 1, marginLeft: theme.spacing[8], fontSize: theme.typography.caption.fontSize, lineHeight: theme.typography.caption.lineHeight }}>
          {connectionName}
        </Text>
      </View>
      <Text style={{ color: theme.colors.ink, fontSize: theme.typography['body-sm'].fontSize, lineHeight: theme.typography['body-sm'].lineHeight, marginTop: theme.spacing[12] }}>
        授权范围：{scopeDescription}
      </Text>
      <Text style={{ color: theme.colors.subtle, fontSize: theme.typography.caption.fontSize, lineHeight: theme.typography.caption.lineHeight, marginTop: theme.spacing[8] }}>
        授权将通过系统浏览器完成（不在 App 内输入服务商凭据）；回跳后自动查询连接状态。
      </Text>
      <View style={{ marginTop: theme.spacing[12], gap: theme.spacing[8] }}>
        {state === 'idle' || state === 'failed' ? (
          onAuthorize ? <Button label="开始授权" onPress={onAuthorize} accessibilityLabel={`在系统浏览器中开始 ${connectionName} 授权`} /> : null
        ) : null}
        {state === 'authorizing' ? (
          onRecheck ? <Button label="查询授权状态" variant="secondary" onPress={onRecheck} accessibilityLabel="查询连接授权结果" size="compact" /> : null
        ) : null}
      </View>
    </Card>
  );
}

const styles = StyleSheet.create({
  row: { flexDirection: 'row', alignItems: 'center' },
});
