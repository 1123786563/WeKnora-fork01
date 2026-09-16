# /login 视觉与多语言对齐（2026-09-12，Round 4）

## 实施
- 重建 LoginPage 渲染层，结构/样式逐值移植 Vue Login.vue：渐变背景（225deg 九段色标）、12 动画知识节点 + 12 虚线连线（nodePulse/lineFlow 关键帧、prefers-reduced-motion 降级）、左 52%/右 48% 布局、showcase 标题/描述/4 标签 chips、4 帧自动轮播（4s fade、可点分页圆点、圆角16/阴影）、卡片 rgba(255,255,255,.97)/radius 16/40px 内边距、46px 输入与按钮、绿色焦点环 rgba(7,192,95,.1)、首次使用分隔线、功能勾选列表、invite 横幅。
- 页头：logo（移植 weknora.png 资源）、官方网站/GitHub 链接（图标+圆角胶囊样式）、语言切换下拉（5 locale，localStorage 'locale' 持久化，对齐 Vue:523-528）。
- 文案：程序化从 frontend/src/i18n/locales/*.ts 提取 73 键 × 5 locale（platform/auth/inviteRegister/common/language，全部命中零缺失），生成 apps/web/src/auth/locales.ts；页面字符串全量走 t()，无硬编码英文。
- 必填星号（* 邮箱/密码）与注册卡（用户名/确认密码/返回登录链接）同步对齐。

## 测试与证据
- web 131/131 通过（auth 文件无类型错误；App.tsx 现存类型错误来自并行 KB 子代理进行中工作，不属本切片）。
- 截图：login-react-visual-round4c.png vs 基准 login-vue.png（1440x900，zh-CN）—— 布局、配色、字体层级、卡片、按钮、勾选列表一致。

## 仍开放（保持 review，不得视为通过）
- 部分背景 SVG 节点图标渲染为占位方块（需逐一比对 viewBox/描边参数）。
- showcase 区块垂直位置略高于 Vue（padding 校准）。
- 轮播分页圆点位置待校准；桌面展示屏上 OIDC 按钮需后端开启后核对。
- locales.ts 待迁移进 packages/i18n 共享包（当前被并行子代理持有，避免冲突）。
