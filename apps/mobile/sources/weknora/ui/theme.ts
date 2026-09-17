import { useMemo } from 'react';
import { Appearance } from 'react-native';
import { resolveNativeTheme, type ThemeMode } from '@weknora/design-tokens/mobile';

/**
 * WeKnora 产品主题（MX-007）。
 * 令牌唯一源：packages/design-tokens/src/mobile/tokens.json（版本化自设计包）。
 * - 产品组件不散落颜色/间距/圆角常量，一律消费本模块；
 * - 数值主题（dp/可缩放字号）不携带 CSS px、box-shadow 字符串；elevation 由组件按平台适配；
 * - 默认跟随系统外观；用户显式偏好（外观设置）由 MX-030 接入，此处留受控入口；
 * - 不替换 Happy 全局 Unistyles 主题（壳层既有主题保持不动），产品 UI 经本模块取值。
 */

export type WeknoraTheme = ReturnType<typeof resolveNativeTheme>;

export const weknoraThemes: Record<ThemeMode, WeknoraTheme> = {
  light: resolveNativeTheme('light'),
  dark: resolveNativeTheme('dark'),
};

/** 受控模式源：默认跟随系统；偏好层（MX-030）可覆盖。 */
export function resolveThemeMode(explicit?: ThemeMode): ThemeMode {
  if (explicit === 'light' || explicit === 'dark') return explicit;
  return Appearance.getColorScheme() === 'dark' ? 'dark' : 'light';
}

export function useWeknoraTheme(explicit?: ThemeMode): { mode: ThemeMode; theme: WeknoraTheme } {
  const mode = resolveThemeMode(explicit);
  return useMemo(() => ({ mode, theme: weknoraThemes[mode] }), [mode]);
}

/** 触控尺寸合同（设计令牌 size）：紧凑触控 44、主触控 48。 */
export const touchSizes = { compact: 44, primary: 48 } as const;
