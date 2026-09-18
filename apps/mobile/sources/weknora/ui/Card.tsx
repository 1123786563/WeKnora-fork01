import React from 'react';
import { Pressable, StyleSheet, View, type ViewStyle } from 'react-native';
import { useWeknoraTheme } from './theme.ts';

/**
 * 卡片容器（MX-008）：surface + radius.card + 令牌间距。
 * 可点击卡片整卡单一点击区（禁止嵌套点击区域）；不可点击卡片纯展示。
 */
export interface CardProps {
  children: React.ReactNode;
  onPress?: () => void;
  accessibilityLabel?: string;
  /** 紧凑内边距（列表项） */
  compact?: boolean;
  /** 强调底色（hero 区） */
  tone?: 'surface' | 'alt' | 'hero';
  testID?: string;
  style?: ViewStyle;
}

export function Card({ children, onPress, accessibilityLabel, compact = false, tone = 'surface', testID, style }: CardProps) {
  const { theme } = useWeknoraTheme();
  const background = tone === 'hero' ? theme.colors.hero : tone === 'alt' ? theme.colors['surface-alt'] : theme.colors.surface;
  const padding = compact ? theme.spacing[12] : theme.spacing[16];
  const body = (
    <View testID={testID} style={[styles.card, { backgroundColor: background, borderRadius: theme.radius.card, padding, borderColor: theme.colors.line }, style]}>
      {children}
    </View>
  );
  if (!onPress) return body;
  return (
    <Pressable
      accessibilityRole="button"
      accessibilityLabel={accessibilityLabel}
      onPress={onPress}
      style={({ pressed }) => [pressed && { opacity: 0.9 }]}
    >
      {body}
    </Pressable>
  );
}

const styles = StyleSheet.create({
  card: { borderWidth: StyleSheet.hairlineWidth },
});
