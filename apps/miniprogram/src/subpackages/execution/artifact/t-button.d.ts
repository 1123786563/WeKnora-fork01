import type { ReactNode } from 'react';

// TDesign Miniprogram 原生自定义组件（dist/npm/tdesign，经页面 usingComponents 注册）。
// D2 根因备注：Taro 构建期的三方组件属性/事件收集器（webpack5-runner
// TaroNormalModulesPlugin）只识别 JSX 编译产物 `_jsx("t-button", …)` 与
// `React.createElement("t-button", …)` 两种调用形态；裸标识符
// `createElement("t-button", …)` 不被收集，导致 base.wxml 的 t-button 模板缺
// bindtap 与全部属性绑定。因此这里必须以 JSX 书写，不能用 createElement 字符串标签。
declare global {
  namespace JSX {
    interface IntrinsicElements {
      't-button': {
        block?: boolean;
        disabled?: boolean;
        loading?: boolean;
        theme?: 'default' | 'primary' | 'danger' | 'light' | 'warning';
        size?: 'extra-small' | 'small' | 'medium' | 'large';
        ariaLabel?: string;
        customStyle?: string;
        onTap?: (event: unknown) => void;
        children?: ReactNode;
      };
    }
  }
}

export {};
