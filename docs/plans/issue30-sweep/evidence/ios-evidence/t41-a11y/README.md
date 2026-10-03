# T41 #71 可访问性矩阵补齐 — iOS（iPhone 18 Pro + iPhone 17e 模拟器，iOS 27.0，2026-10-03）

环境：iPhone 18 Pro（0A38DB71，402pt，app 0.1.0 已登录态，T39 轮缓存）+ iPhone 17e（0D4ED257，390pt，本轮新装）。后端 :8084（HEAD）via nginx :8443。macOS 用户级 CoreTunnel 代理（127.0.0.1:17890）仍在 → 模拟器网络对外 hostname 受限（T01 round-2 已知阻塞），故 iOS 侧未走线上登录，18 Pro 用既有登录态。

## 逐项 disposition

| 项 | 文件 | 判定 |
|---|---|---|
| Leg1 390 宽 | leg1-a-login-390-iphone17e.png（1170x2532px @3x = 390x844pt） | evidenced（login 面）：iPhone 17e 新装首启 login 屏完整渲染（Sign in to WeKnora / Email / Password / SSO 按钮）。**320pt iOS 无设备**（iOS 27 runtime 无 SE 家族）→ 320 口径由 Android 覆盖，如实登记。home/taskdetail 390 未采（17e 无登录态 + 代理阻塞 SSO）→ 由 18 Pro 402pt 近似（leg2/4/5 各图）+ Android 320 全覆盖，口径已注明 |
| Leg2 动态字体 | leg2-a-taskdetail-content-xl.png / leg2-b-taskdetail-content-axl.png | evidenced：`content_size extra-large` 与 `accessibility-extra-large`（回执见 settings-proof-ios.txt，已恢复 large）。axl 档任务标题自动折行、中文文案无破坏性截断、操作区完整。注意：brief 中 `content-size accessibility-xl` 非有效 simctl token，实际为 `content_size accessibility-extra-large` |
| Leg3 VoiceOver | leg3-vo-01-app-initial-focus.png / leg3-vo-04-tutorial-dismissed.png / axtree-vo-app2.json | evidenced：defaults write 不激活 VO 运行时（probe1/2 在 settings-proof 记录）；经 idb 驱动 Settings UI 真激活（VoiceOverTouch launchctl PID 在案）；VO 手势教学卡出现；交互语义实测（单击=聚焦、紧凑双击=激活）；VO 客户端下 idb describe-all 暴露 app AX 树（Button "Sign out" 等带完整 AXLabel）= labels present 运行时证据。元素级朗读序列（focus order 逐步推进的 caption 日志）模拟器无采集通道——已用「交互语义 + AX 树 + 18 Pro app 屏在版本门屏」组合替代，如实登记。iOS 端 app 停在 Update required 屏（T39 轮 IPv6 origin 缓存触发版本门，环境残留非产品回归），VO 证据采于该屏与 Settings |
| Leg4 主题 | leg4-0-taskdetail-light.png（luma 239）/ leg4-a-taskdetail-dark.png（luma 214） | **部分 evidenced + 产品发现**：`appearance dark` 回执生效、系统 chrome 变暗，但 app 界面未跟随（亮度仅 239→214，主体仍白底）→ **iOS 端 app 不随系统 dark 模式**（Android 端跟随，见 android-evidence/t41-a11y）。判定权在 owner/Task 3 |
| Leg5 减动效 | leg5-a-taskdetail-reducemotion.png | evidenced（settings 口径）：`ReduceMotionEnabled` defaults write 1 回执 + 截图（已恢复 0）；app 无 reduce-motion 消费点（Task-1 审计），OS 级生效、无 UI 破坏 |
| Leg6 回滚 | （Android 侧执行） | 见 ../android-evidence/t41-a11y/（pragmatic 口径：app 重装冒烟 + 迁移 down.sql 三处引用），待 Task 3 裁定 |

## 坑位记录
- 双模拟器 booted 时 `simctl ui booted` 会静默命中错误设备 → 全部命令须 UDID 显式。
- content_size 切换不杀 app；本机曾因 booted 漂移误判 app 消失。
- VO 激活唯一可靠路径 = Settings UI 驱动（idb tap/swipe，像素→逻辑点 ÷3）；开关 swipe 可切，tap 不可（3 次坐标尝试记录在案）。
