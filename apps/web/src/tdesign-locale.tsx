// TDesign ConfigProvider locale 接线 — Vue App.vue:17-38 平移。
//
// Vue 端事实源：App.vue 从 tdesign-vue-next/esm/locale 静态导入五个 locale
// （en_US/zh_CN/ko_KR/ja_JP/ru_RU），按 vue-i18n 当前语言取值注入
// t-config-provider 的 globalConfig（未知语言回退 en_US）。React 端以同一
// 语义包裹应用根：语言来自 usePreferredLocale（locale.ts，随
// weknora:locale-changed 事件响应式重读），globalConfig 随之切换。
//
// 五个 locale 全量静态导入的理由（与 Vue App.vue 一致）：每个模块是纯数据
// （组件文案 + dayjs 语言包，源码约 12-24KB），五个合计 gzip 后体积可忽略；
// 而懒加载会在语言切换时引入异步 chunk 瀑布，让分页/日期选择器文案闪回
// 默认语言——Vue 端没有接受这种闪变，React 端也不接受。
import type { ReactNode } from 'react';
import { type Locale } from '@weknora/i18n';
import { usePreferredLocale } from './locale.ts';
import ConfigProvider from 'tdesign-react/es/config-provider';
import type { GlobalConfigProvider } from 'tdesign-react/es/config-provider/type';
import enUS from 'tdesign-react/es/locale/en_US';
import zhCN from 'tdesign-react/es/locale/zh_CN';
import koKR from 'tdesign-react/es/locale/ko_KR';
import jaJP from 'tdesign-react/es/locale/ja_JP';
import ruRU from 'tdesign-react/es/locale/ru_RU';

// @weknora/i18n 的 supportedLocales（zh-CN/en-US/ja-JP/ko-KR/ru-RU）与 Vue
// 端五语言集合一一对应，Record<Locale, ...> 在类型层面保证不存在无映射的
// 语言键，也保证不会引入 Vue 端没有的 locale。
const TD_LOCALES: Record<Locale, GlobalConfigProvider> = {
  'zh-CN': zhCN,
  'en-US': enUS,
  'ja-JP': jaJP,
  'ko-KR': koKR,
  'ru-RU': ruRU,
};

/** Vue App.vue t-config-provider 层级的 React 对位：包住页面内容（RouterProvider
 * 渲染的整棵路由树），globalConfig 随当前语言响应式切换。 */
export function TDesignLocaleProvider({ children }: { children: ReactNode }) {
  const locale = usePreferredLocale();
  return <ConfigProvider globalConfig={TD_LOCALES[locale]}>{children}</ConfigProvider>;
}
