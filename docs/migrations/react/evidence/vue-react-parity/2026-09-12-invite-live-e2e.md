# 邀请/共享链接 live API E2E（2026-09-12，Round 11）

使用隔离后端（:8080，本地库）实测 round-2/3 接线的 api-client 面：
- owner（parity-test@local.dev）创建邀请 POST /tenants/10000/invitations → id=1（contributor）。
- 被邀请人登录 → GET /me/invitations/pending-count = 1 → POST /me/invitations/1/accept → membership{tenant 10000, contributor}；/auth/me memberships=2。
- 共享链接：POST /tenants/10000/invite-links（viewer）→ 从 invite_url 提取 token → POST /me/invitations/accept-by-token {token} → 成功返回 membership + tenant_name（与 Vue acceptInvitationByToken 响应 shape 一致，验证 api-client parse 正确）。
- 全部使用本地隔离库测试账号，未触生产。
