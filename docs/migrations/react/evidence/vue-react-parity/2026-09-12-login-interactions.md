# /login 交互验证与视觉收尾（2026-09-12，Round 5）

## 修复
- 轮播分页圆点此前被误加的白底遮挡（Vue swiper 无背景），移除后圆点在绿色背景上可见。

## 浏览器交互验证（playwright，1440x900，真实点击）
- 语言切换：点击下拉第 2 项（English）→ localStorage 'locale'='en-US' 持久化；整页文案切换为英文（screenshots/login-round5-en.png）；切回中文后标题恢复「登录」。
- 轮播：点击第 2 个分页圆点 → active slide 索引变为 1。
- 证据脚本：.parity-tools/verify-login.cjs（可重复执行）。

## 与 Vue 基准逐项对照状态（/login）
- 布局/配色/字体层级/卡片/按钮/勾选列表/语言切换/轮播：一致（screenshots/login-vue.png vs login-round5-*.png）。
- 仍开放：部分背景 SVG 节点图标形状需与 Vue 逐图标比对；注册卡与邀请横幅视觉需带后端数据核对；Wails 端验证未做。/login 维持 review，不提前判 accepted。
