# 上传流程 / AgentSelector / embed CORS 修复集成（2026-09-12，Round 36）

- KB 上传流程 parity：多文件 + 拖拽 + 确认 Dialog（文件清单/大小/tag_ids/分块配置提示）+ 逐文件状态列表 + 取消清空（upload-pipeline.ts 5 测试：逐文件调用、错误隔离、取消不上传、批次中止、汇总）。documents.css 新增。
- 聊天 AgentSelector + 建议问题：picker（已有 listWithState=GET /api/v1/agents）+ per-message agent_id（已有）；新增 agents.suggestedQuestions()（GET /api/v1/agents/:id/suggested-questions，routes_agent.go 核实）+ starter-questions fail-open 模块（5 测试）+ 新会话视图 chips；后端缺口如实记录：POST /sessions 不接受 agent_id（CreateSessionRequest 仅 title/description），agent_id 走首条 chat 请求。
- **embed CORS 修复**（backend）：AllowHeaders 增补 X-Embed-Visitor（embed 组件访客头，types/principal.go:22），修复预检拒绝导致的 ERR_FAILED。修复后 live 实测：config 200、建会话成功、embed 小组件完整渲染（embed-live-fixed 路径见 embed-live-token3.png 最新版）。
- 门禁：shared 276/276、web 194/194（+21 新测试）、typecheck×2、build:web 全绿。提交 40a5d86。
- Round 30 的 CORS 根因分析修正为最终结论（X-Embed-Visitor 缺失），embed-resume-defect.md 已补记。
