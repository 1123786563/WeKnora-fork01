export default definePageConfig({
  // OCR r3 ocr3-004：Taro 4 要求页面 config 显式声明该 flag 才挂载 useShareAppMessage
  // 回调——缺失时分享卡片用微信默认标题与当前页路径（不带 jd 参数），分享导入链路不可达。
  enableShareAppMessage: true,
  usingComponents: {
    // D1/T06：引用构建产物绝对路径（config/index.ts 闭包拷贝 miniprogram_dist 到
    // dist/npm/tdesign），不能写 npm 包名（会被重写成含 node_modules 段的路径）。
    't-button': '/npm/tdesign/button/button',
  },
});
