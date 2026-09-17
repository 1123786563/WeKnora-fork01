import React, { forwardRef, useCallback, useRef, useState } from 'react';
import { ActivityIndicator, Pressable, StyleSheet, Text, View, type ViewStyle } from 'react-native';
import { useWeknoraTheme, touchSizes } from './theme.ts';
import type { WeknoraTheme } from './theme.ts';

/**
 * 基础按钮（MX-008）。视觉合同来自设计令牌（theme.ts，唯一源）：
 * - 变体 primary/secondary/danger + disabled/loading；
 * - 主触控 48 / 紧凑 44（minHeight，不固定 height——大字体不裁切）；
 * - busy（loading）时不重复触发（onPress 被抑制）；
 * - 危险操作不只靠颜色：accessibilityLabel 由调用方给动词，danger 变体附加图标位；
 * - focus 态用 focus 色边框（键盘/读屏导航可见），无鼠标悬浮假设。
 */

export type ButtonVariant = 'primary' | 'secondary' | 'danger';
export type ButtonSize = 'primary' | 'compact';

export interface ButtonProps {
  label: string;
  onPress: () => void;
  variant?: ButtonVariant;
  size?: ButtonSize;
  disabled?: boolean;
  loading?: boolean;
  /** 危险操作的明确动词（如「删除知识库」）；读屏与视觉共用 */
  accessibilityLabel?: string;
  /** danger 变体的图标位（组件不自行选择图标，避免语义漂移） */
  LeadingIcon?: React.ComponentType<{ size: number; color: string }>;
  testID?: string;
}

function variantStyle(theme: WeknoraTheme, variant: ButtonVariant): { container: ViewStyle; text: { color: string } } {
  switch (variant) {
    case 'secondary':
      return {
        container: { backgroundColor: theme.colors['surface-alt'], borderWidth: StyleSheet.hairlineWidth, borderColor: theme.colors['control-line'] },
        text: { color: theme.colors.ink },
      };
    case 'danger':
      return {
        container: { backgroundColor: theme.colors.danger, borderWidth: StyleSheet.hairlineWidth, borderColor: theme.colors.danger },
        text: { color: theme.colors['danger-soft'] },
      };
    case 'primary':
    default:
      return {
        container: { backgroundColor: theme.colors.brand, borderWidth: StyleSheet.hairlineWidth, borderColor: theme.colors.brand },
        text: { color: theme.colors['on-brand'] },
      };
  }
}

export const Button = forwardRef<View, ButtonProps>(function Button(
  { label, onPress, variant = 'primary', size = 'primary', disabled = false, loading = false, accessibilityLabel, LeadingIcon, testID },
  ref,
) {
  const { theme } = useWeknoraTheme();
  const [focused, setFocused] = useState(false);
  const busy = loading || disabled;
  const handlePress = useCallback(() => {
    // busy 不重复触发：loading 中与禁用中一律不派发
    if (loading || disabled) return;
    onPress();
  }, [loading, disabled, onPress]);
  const vs = variantStyle(theme, variant);
  const minHeight = size === 'compact' ? touchSizes.compact : touchSizes.primary;
  const typography = size === 'compact' ? theme.typography['body-sm'] : theme.typography.body;
  const iconColor = variant === 'primary' ? theme.colors['on-brand'] : variant === 'danger' ? theme.colors['danger-soft'] : theme.colors.ink;
  return (
    <Pressable
      ref={ref}
      testID={testID}
      accessibilityRole="button"
      accessibilityLabel={accessibilityLabel ?? label}
      accessibilityState={{ disabled: busy, busy: loading }}
      disabled={busy}
      onPress={handlePress}
      onFocus={() => setFocused(true)}
      onBlur={() => setFocused(false)}
      style={({ pressed }) => [
        styles.base,
        { minHeight, borderRadius: theme.radius.control, paddingHorizontal: theme.spacing[16] },
        vs.container,
        focused && { borderColor: theme.colors.focus, borderWidth: 2 },
        pressed && !busy && { opacity: 0.85 },
        disabled && { backgroundColor: theme.colors['disabled-bg'], borderColor: theme.colors['disabled-bg'] },
      ]}
    >
      {loading ? (
        <ActivityIndicator color={vs.text.color} />
      ) : (
        <View style={styles.row}>
          {LeadingIcon ? <LeadingIcon size={theme.size.icon} color={iconColor} /> : null}
          <Text style={[{ color: disabled ? theme.colors.disabled : vs.text.color, fontWeight: typography.fontWeight }, styles.label]}>{label}</Text>
        </View>
      )}
    </Pressable>
  );
});

const styles = StyleSheet.create({
  base: { alignItems: 'center', justifyContent: 'center', gap: 8 },
  row: { flexDirection: 'row', alignItems: 'center', gap: 8 },
  label: { fontSize: 16 },
});
