# Task 沿用 Session 作为唯一身份

移动 AI Office 将现有 WeKnora Session 呈现为 Task，并保持 `taskId = sessionId`，不新增第二个 Task 聚合身份。一个 Task 表示一个初始目标及同一目标下的追问，Run 表示其中的一次执行；Lead Agent 的内部委派通过任务内部执行关系表达，而不是把多个内部 Session 聚合成新的用户 Task。这样可以直接复用现有权限、历史、fork、持久 Run 与 Artifact，并避免双重身份和生命周期分歧。
