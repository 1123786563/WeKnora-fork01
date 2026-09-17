import React from 'react';
import { ActivityIndicator, StyleSheet, Text, View } from 'react-native';
import { useWeknoraTheme } from './theme.ts';
import { StatusBadge, type BadgeTone } from './StatusBadge.tsx';
import { Button } from './Button.tsx';

/**
 * 状态视图（MX-008）：加载/空/失败/离线/无权限/能力不可用，全部 props 驱动、
 * 无业务网络副作用。状态文字与颜色同时存在；动态文案提供读屏标签；
 * 每态给出动作位（重试/回退），由调用方接真实命令——本组件不发请求。
 */
export type StateViewKind = 'loading' | 'empty' | 'error' | 'offline' | 'noPermission' | 'capability';

export interface StateViewProps {
  kind: StateViewKind;
  /** 主体说明（动态内容时调用方拼好完整句子） */
  message: string;
  /** 可选补充行（如能力不可用的 reason） */
  detail?: string;
  actionLabel?: string;
  onAction?: () => void;
  testID?: string;
}

const KIND_TONE: Record<StateViewKind, BadgeTone> = {
  loading: 'brand',
  empty: 'neutral',
  error: 'danger',
  offline: 'warning',
  noPermission: 'info',
  capability: 'purple',
};

const KIND_LABEL: Record<StateViewKind, string> = {
  loading: '加载中',
  empty: '暂无内容',
  error: '出错了',
  offline: '离线',
  noPermission: '无权限',
  capability: '能力不可用',
};

export function StateView({ kind, message, detail, actionLabel, onAction, testID }: StateViewProps) {
  const { theme } = useWeknoraTheme();
  return (
    <View testID={testID} style={[styles.container, { padding: theme.spacing[24], gap: theme.spacing[4] }]} accessibilityLiveRegion={kind === 'error' ? 'assertive' : undefined}>
      {kind === 'loading' ? (
        <ActivityIndicator color={theme.colors.brand} />
      ) : (
        <StatusBadge tone={KIND_TONE[kind]} label={KIND_LABEL[kind]} />
      )}
      <Text
        accessibilityLabel={`${KIND_LABEL[kind]}。${message}${detail ? `，${detail}` : ''}`}
        style={{ color: theme.colors.ink, fontSize: theme.typography.body.fontSize, lineHeight: theme.typography.body.lineHeight, textAlign: 'center', marginTop: theme.spacing[12] }}
      >
        {message}
      </Text>
      {detail ? (
        <Text style={{ color: theme.colors.muted, fontSize: theme.typography['body-sm'].fontSize, lineHeight: theme.typography['body-sm'].lineHeight, textAlign: 'center', marginTop: theme.spacing[8] }}>
          {detail}
        </Text>
      ) : null}
      {actionLabel && onAction ? (
        <View style={{ marginTop: theme.spacing[16], alignSelf: 'stretch' }}>
          <Button label={actionLabel} variant="secondary" onPress={onAction} accessibilityLabel={actionLabel} />
        </View>
      ) : null}
    </View>
  );
}

const styles = StyleSheet.create({
  container: { alignItems: 'center', justifyContent: 'flex-start' },
});
