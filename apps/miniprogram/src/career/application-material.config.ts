export default definePageConfig({
  usingComponents: {
    // D1/T06：引用构建产物绝对路径（config/index.ts 闭包拷贝 miniprogram_dist 到
    // dist/npm/tdesign），不能写 npm 包名（会被重写成含 node_modules 段的路径）。
    't-button': '/npm/tdesign/button/button',
  },
});
