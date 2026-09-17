import React from 'react';

/**
 * react-native 测试基底 mock（MX-008 挂载验证层）。
 * 仅在 vitest.mobile-v2.config.ts 的 alias 中生效——产品代码 import 'react-native'
 * 在 node 测试环境解析到这里；真实设备链路仍用 RN 本体（MX-034 native-e2e）。
 * 职责：提供可挂载的最小原语 + 可观测的焦点/返回 API；不实现任何业务。
 */

export const focusCalls: Array<string | number> = [];
export const backHandlers: Array<() => boolean> = [];

const el = React.createElement;

export const View = (props: React.PropsWithChildren<Record<string, unknown>>) => el('view', props, props.children);
export const Text = (props: React.PropsWithChildren<Record<string, unknown>>) => el('text', props, props.children);
export const ScrollView = (props: React.PropsWithChildren<Record<string, unknown>>) => el('scrollview', props, props.children);
export const KeyboardAvoidingView = (props: React.PropsWithChildren<Record<string, unknown>>) => el('keyboardavoidingview', props, props.children);
export const TextInput = (props: React.PropsWithChildren<Record<string, unknown>>) => el('textinput', props, props.children);
export const ActivityIndicator = (props: React.PropsWithChildren<Record<string, unknown>>) => el('activityindicator', props, props.children);

export const Pressable = (props: React.PropsWithChildren<Record<string, unknown>>) => {
  const { onPressIn, onPressOut, ...rest } = props as Record<string, unknown> & { onPressIn?: unknown; onPressOut?: unknown };
  void onPressIn; void onPressOut;
  return el('pressable', rest, props.children);
};

export const Modal = (props: React.PropsWithChildren<Record<string, unknown>>) => {
  const { onRequestClose, transparent, animationType, ...rest } = props as Record<string, unknown> & { onRequestClose?: unknown; transparent?: unknown; animationType?: unknown };
  void onRequestClose; void transparent; void animationType;
  return el('modal', rest, props.children);
};

export const StyleSheet = {
  create: <T,>(styles: T): T => styles,
  hairlineWidth: 1 as number,
};

export const AccessibilityInfo = {
  setAccessibilityFocus: (handle: string | number): void => {
    focusCalls.push(handle);
  },
  isScreenReaderEnabled: async (): Promise<boolean> => false,
};

export const BackHandler = {
  addEventListener: (_eventName: string, handler: () => boolean): { remove: () => void } => {
    backHandlers.push(handler);
    return { remove: () => { const at = backHandlers.indexOf(handler); if (at >= 0) backHandlers.splice(at, 1); } };
  },
  exitApp: (): void => { /* 测试基底无操作 */ },
};

export const Appearance = {
  getColorScheme: (): 'light' | 'dark' | null => 'light',
  addChangeListener: (): { remove: () => void } => ({ remove: () => undefined }),
};

export function findNodeHandle(component: unknown): string | number | null {
  if (component && typeof component === 'object' && '__testTag' in (component as Record<string, unknown>)) {
    return (component as { __testTag: string | number }).__testTag;
  }
  return null;
}

export const useColorScheme = (): 'light' | 'dark' | null => 'light';

export const Share = {
  sharedAction: 'sharedAction' as const,
  dismissedAction: 'dismissedAction' as const,
  share: async (_content: { title?: string; message?: string }) => ({ action: 'sharedAction' as const }),
};

// 其余 RN 导出（未被产品 UI 层消费）按需抛错，防止静默假实现
const unavailable = (name: string) => (): never => {
  throw new Error(`rn-mock: ${name} is not implemented in the node test substrate`);
};
export const Platform = { OS: 'test' as const, select: (obj: Record<string, unknown>) => obj.test };
export const Keyboard = { addListener: unavailable('Keyboard.addListener') };
