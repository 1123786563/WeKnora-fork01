import React from 'react';
import { StyleSheet, Text, View } from 'react-native';
import { useWeknoraTheme } from './theme.ts';

/**
 * 状态徽章（MX-008）：tone 决定 soft 底色 + 深色文字——状态文字与颜色同时存在，
 * 不只靠颜色区分；文案由调用方给定（业务语义不在此层发明）。
 */
export type BadgeTone = 'neutral' | 'brand' | 'warning' | 'danger' | 'info' | 'purple';

export interface StatusBadgeProps {
  tone: BadgeTone;
  label: string;
  testID?: string;
}

export function StatusBadge({ tone, label, testID }: StatusBadgeProps) {
  const { theme } = useWeknoraTheme();
  const map: Record<BadgeTone, { fg: string; bg: string }> = {
    neutral: { fg: theme.colors.muted, bg: theme.colors['surface-alt'] },
    brand: { fg: theme.colors.brand, bg: theme.colors['brand-soft'] },
    warning: { fg: theme.colors.warning, bg: theme.colors['warning-soft'] },
    danger: { fg: theme.colors.danger, bg: theme.colors['danger-soft'] },
    info: { fg: theme.colors.info, bg: theme.colors['info-soft'] },
    purple: { fg: theme.colors.purple, bg: theme.colors['purple-soft'] },
  };
  const { fg, bg } = map[tone];
  return (
    <View testID={testID} style={[styles.badge, { backgroundColor: bg, borderRadius: theme.radius.pill }]}>
      <Text style={{ color: fg, fontSize: theme.typography.caption.fontSize, lineHeight: theme.typography.caption.lineHeight, fontWeight: '600' }}>
        {label}
      </Text>
    </View>
  );
}

const styles = StyleSheet.create({
  badge: { paddingHorizontal: 8, paddingVertical: 2, alignSelf: 'flex-start' },
});
