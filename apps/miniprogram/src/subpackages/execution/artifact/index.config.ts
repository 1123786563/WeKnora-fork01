export default definePageConfig({
  usingComponents: {
    // D1：不能写 npm 包名（'tdesign-miniprogram/button/button'）——Taro 会重写成含
    // node_modules 段的 /npm/.pnpm/... 路径，DevTools 拒绝加载且 dist/npm 从不产出。
    // 这里引用构建产物内的绝对路径：config/index.ts 的 mini.copy 会把 tdesign 的
    // miniprogram_dist 拷到 dist/npm/tdesign。
    't-button': '/npm/tdesign/button/button',
  },
});
