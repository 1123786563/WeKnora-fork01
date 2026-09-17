# 本交付验证范围

核验对象是本轮生成的HTML、设计令牌、类型提案和任务文档，不是WeKnora生产系统。

| 检查 | 结果文件 | 范围 |
|---|---|---|
| 浏览器布局 | browser-layout.json | 18页×2主题×6宽度=216组；390宽度125%字体再36组，共252组；单页入口18个与UI Kit默认入口另核对 |
| 浏览器交互 | browser-interactions.json | 31个真实DOM交互用例，包含表单、审批、隔离、未知请求、语音草稿、下载与信息层次 |
| 原型状态机 | model-green.log / model.test.cjs | 8个Node单元测试；只验证模拟状态机 |
| 静态结构与依赖 | static-report.json | 85项，包括32种激活profile组合下的36节点无环检查、页面/缺口映射、路径与令牌 |
| 对比度 | token-contrast.json | 26个选定语义组合；不等于全页或全WCAG认证 |
| 类型 | typecheck.log | 原生令牌与提案类型，未编译Expo项目 |
| 视觉 | contact-sheet-light.png / contact-sheet-dark.png | 明暗18页截图拼版人工查看；关键页面与确认状态独立查看 |

本环境阻止file://和localhost导航，使用Chromium set_content注入同一自包含HTML。每份独立页面在新Page中注入以避免脚本词法上下文复用。页面外部HTTP请求和JS错误均记录在报告，未启动真实服务。原型可尝试sessionStorage，但opaque origin下无法验证刷新持久化；本轮只验证同一浏览器页面会话内的scope草稿隔离。

检查中修正了列表标题与说明未分行、模拟附件迟到回调缺少generation守卫的问题。检测脚本还修正了重复set_content词法上下文、模态关闭按钮选择器和gallery嵌套article计数这三项测试适配问题；这些不是生产系统故障。

没有验证：Expo真机构建/动态文字200%、VoiceOver/TalkBack、原生键盘/旋转/通知、真实Go/PG/SQLite/Provider/付款、部署或应用商店。截图里任务、账号、内容和Credits都是模拟。所有MX工程任务仍pending。
