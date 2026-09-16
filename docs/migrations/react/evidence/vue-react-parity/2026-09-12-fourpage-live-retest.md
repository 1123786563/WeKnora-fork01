# 四页 live 复测（2026-09-12）

playwright 真实登录后逐页复测（:5181 → :8080 隔离后端，owner 账号）：
- Integrations API tab：im-channels 解析错误消失（双层修复生效）、MIGRATION SEAM 眉题移除、Create API key 表单渲染（live-integrations-api-after.png）。API-key 创建/复制/吊销留待带真实 key 的深度测试。
- Agents：分组列表（builtin/mine/shared）渲染（live-agents-after.png）。
- Organizations：页面正常（live-organizations-after.png）。
- 脚本：.parity-tools/fourpage-after.cjs（可重复）。
