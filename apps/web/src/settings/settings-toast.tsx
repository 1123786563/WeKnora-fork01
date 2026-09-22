import { useEffect, useState } from 'react';

/*
 * Settings 域错误 Toast（R472 A2，对齐 Vue MessagePlugin 基本语义）。
 *
 * Vue 各 settings 分区加载失败走 `MessagePlugin.error(...)`（右上角浮动、
 * 3s 自动消失、命令式调用）。React 侧此前只有 R445 语言保存的内联成功
 * notice，没有可复用的命令式 toast，这里提供最小等价物：
 * - 命令式 `pushSettingsToast(message, tone)`（模块级总线，任意面板可调）
 * - `<SettingsToastHost />` 渲染一次，固定右上角、role="status" +
 *   aria-live="polite"、SETTINGS_TOAST_DURATION_MS(3000ms) 自动消失
 *   —— 与 tdesign-vue-next MessagePlugin 默认时长一致。
 * 差异记录：Vue MessagePlugin 支持手动关闭与多条堆叠动画，本 host 只保留
 * 基本语义（浮动、自动消失、可堆叠），不做关闭按钮/动画。
 */

export const SETTINGS_TOAST_DURATION_MS = 3000;

export interface SettingsToast {
  id: number;
  tone: 'error' | 'success' | 'warning';
  message: string;
}

type SettingsToastListener = (toast: SettingsToast) => void;

const listeners = new Set<SettingsToastListener>();
let nextToastId = 1;

/** 命令式推送一条 toast（对齐 MessagePlugin.error / .success 的调用形态）。 */
export function pushSettingsToast(message: string, tone: SettingsToast['tone'] = 'error'): void {
  const toast: SettingsToast = { id: nextToastId++, tone, message };
  for (const listener of listeners) listener(toast);
}

export function SettingsToastHost() {
  const [toasts, setToasts] = useState<SettingsToast[]>([]);
  useEffect(() => {
    const listener: SettingsToastListener = (toast) => {
      setToasts((current) => [...current, toast]);
      window.setTimeout(() => {
        setToasts((current) => current.filter((item) => item.id !== toast.id));
      }, SETTINGS_TOAST_DURATION_MS);
    };
    listeners.add(listener);
    return () => { listeners.delete(listener); };
  }, []);
  if (toasts.length === 0) return null;
  return (
    <div
      data-testid="settings-toast-region"
      role="status"
      aria-live="polite"
      className="pointer-events-none fixed right-4 top-4 z-[1200] flex w-[320px] flex-col gap-2"
    >
      {toasts.map((toast) => (
        <div
          key={toast.id}
          data-testid="settings-toast"
          data-tone={toast.tone}
          className={
            toast.tone === 'error'
              ? 'rounded-card border border-danger-line bg-danger-wash px-3 py-2.5 text-[13px] leading-relaxed text-danger shadow-lg'
              : toast.tone === 'warning'
                // MessagePlugin.warning 琥珀色语义（WeKnoraCloud 部分成功/fillRequired，T12b）。
                ? 'rounded-card border border-warning-line bg-warning-wash px-3 py-2.5 text-[13px] leading-relaxed text-warning shadow-lg'
                : 'rounded-card border border-success-line bg-accent-wash px-3 py-2.5 text-[13px] leading-relaxed text-accent-strong shadow-lg'
          }
        >
          {toast.message}
        </div>
      ))}
    </div>
  );
}
