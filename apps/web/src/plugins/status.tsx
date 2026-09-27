// plugins 域（#106 React-only 功能）Status 组件。原面板 `@weknora/ui` 引用
// 悬空——该包不在 package.json / pnpm workspace / packages/ 下，main 上 React
// 构建因此断裂；按 configuration / commercial / data-sources 三域既有的"域内
// 本地 Status"惯例（见 data-sources/ui.tsx 注释）补齐本域副本，Button/Input
// 由调用方直接改用 tdesign-react。不并入 ui.ts（该文件是纯常量模块且为 .ts，
// 不承载 JSX）。
import type { ReactNode } from 'react';

/** 内联状态文本：视觉 = data-sources 域 .wk-dsui-status（13px / #506078 + tone 变体）。 */
export function Status({ tone = 'neutral', children }: { tone?: 'neutral' | 'error' | 'success' | 'warning'; children: ReactNode }) {
  const color = tone === 'error' ? '#b42318' : tone === 'success' ? '#137333' : tone === 'warning' ? '#9a6700' : '#506078';
  return <p role={tone === 'error' ? 'alert' : 'status'} style={{ margin: '0.25rem 0', fontSize: 13, color }}>{children}</p>;
}
