# /register UI 级 live E2E（2026-09-12，Round 20）

真实浏览器（playwright，:5181 → :8080 隔离后端）验证 round-2 校验与注册流转：
- 无效提交（用户名 x!、短密码、确认不一致）：网络层 0 次 /auth/register 请求；4 条内联校验错误渲染——客户端校验在先，不打到服务端。
- 有效提交：恰好 1 次 register 请求；成功后切回登录卡且邮箱预填（Vue Login.vue:744-746 对齐）。
- 新账号 parity-ui@local.dev 已在隔离库创建（后续可作邀请/权限测试账号）。
- 截图：register-flow-live.png；脚本 .parity-tools/register-flow.cjs（可重复）。
