# R249 Agents 模型就绪语义验证（2026-09-15）

当前工作树中的 Agents 改动将创建就绪判定从 `llm` 类型收敛到 Vue `useTenantModelReadiness` 使用的 `KnowledgeQA` 类型，并保留 Embedding 等非对话模型的排除规则。

验证（针对用户未提交工作树改动）：
- Agents 聚焦测试：17/17 通过
- `pnpm test:web`：901/901 通过，0 失败、0 取消、0 跳过
- `pnpm run typecheck:web`：通过
- `git diff --check`：通过

代码改动仍保留在工作树，未由本任务提交。
