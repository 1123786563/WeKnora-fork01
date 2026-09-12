# embed 冷恢复缺陷根因分析（2026-09-12，Round 30）

现象：embed 小组件初始 token 交换/建会话成功，但后续 load-messages 渲染 "Failed to fetch"。

根因（网络层证据）：后端 CORS 响应同时包含
  Access-Control-Allow-Credentials: true
  Access-Control-Allow-Origin: *
该组合无效（带凭证时禁止通配 origin），浏览器直接拦截带 Authorization 头的响应 → ERR_FAILED。
此前 OPTIONS 预检可通过，故缺陷仅在真实 GET/POST 响应上暴露。

处理建议（后端侧，非前端）：
- 对 /api/v1/embed/* 公共面回显具体 Origin（或显式白名单渠道 allowed_origins），与 Access-Control-Allow-Credentials 组合使用；或去掉 allow-credentials。
- Vue 基准同样受此影响（其请求亦带 Authorization），属后端共性缺陷——按验收规则记录，不在前端掩盖。

embed 前端逻辑（含已存储会话失效回退创建新会话）本身正确；恢复验证待后端 CORS 修复后重测。
