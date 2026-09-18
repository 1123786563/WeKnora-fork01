// i18n 骨架（RW-029）：zh 为主文案源；t() 支持参数插值与缺失键回退。
// 切换语言通过 I18nProvider + useI18n；当前仅 zh 全量（其他语言表按需增补）。
import React, { createContext, useContext, useMemo, useState } from "react";
import { zh, type I18nKey, type TextTable } from "./zh";

export type Locale = "zh";

const TABLES: Record<Locale, TextTable> = { zh: zh as unknown as TextTable };

/** 取文案；params 以 {name} 插值；缺失键返回 key 本身（可发现，不静默错文案） */
export function t(key: I18nKey | string, params?: Record<string, string | number>, locale: Locale = "zh"): string {
  const table = TABLES[locale];
  let text = table[key as string];
  if (text === undefined) return key as string;
  if (params) {
    for (const [k, v] of Object.entries(params)) {
      text = text.split(`{${k}}`).join(String(v));
    }
  }
  return text;
}

interface I18nContextValue {
  locale: Locale;
  t: (key: I18nKey | string, params?: Record<string, string | number>) => string;
}

const I18nContext = createContext<I18nContextValue | null>(null);

export function I18nProvider({ children, locale: initial = "zh" }: { children: React.ReactNode; locale?: Locale }) {
  const [locale] = useState<Locale>(initial);
  const value = useMemo<I18nContextValue>(
    () => ({ locale, t: (key, params) => t(key, params, locale) }),
    [locale],
  );
  return <I18nContext.Provider value={value}>{children}</I18nContext.Provider>;
}

export function useI18n(): I18nContextValue {
  const ctx = useContext(I18nContext);
  if (ctx) return ctx;
  // 组件树未包 Provider 时仍可用（默认 zh），便于渐进迁移
  return { locale: "zh", t: (key, params) => t(key, params, "zh") };
}

export { zh };
export type { I18nKey };
