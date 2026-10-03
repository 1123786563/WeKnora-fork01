# WB-GRAPH 执行集成活体验证（Android）— 2026-10-03

主链 main @ 5909d58da（freezer=c6f373e4b + 容器装配修复 e0d63bae1 + 409 对齐 5909d58da）；
后端从 HEAD 构建（`/tmp/t40r/server-e0d63bae1`，二进制含 5909d58da 前工作树，最终提交同内容），
`/tmp/t40r/start-backend-wbgraph.sh`（#31 轮 OIDC 配方 + `WEKNORA_AGENT_RECOVERY_ENABLED=true`）。
AVD test36（emulator-5554，T40 轮 Release APK 沿用）；nginx :8443 nip.io；Casdoor t01live；租户 10043。

## 目标 1：提交 → Start 准入 → 执行（events>0）→ SSE → detail 结果 —— evidenced

- OIDC 登录（00→02）：SSO → Casdoor t01live 真实认证 → `POST /auth/mobile/exchange` → 首页授权态。
- 提交（04→06→07）：NEW TASK → 快速问答 → `POST /workbench/executions` **202 admitted**。
- **执行（D8 时代 signature=0，本轮 >0）**：
  - run2 `8eb37fe9`：succeeded，events=2（run_started 104B + run_completed 942B），助手消息=枫叶颜色回答；
  - run3 `bb05811c`：succeeded，events=2；
  - run5 `324a41d7`：succeeded，events=2，run_completed 载荷 5259B，助手消息 4852 字符（造纸史 essay，glm-5.3 真实模型回合）。
- **SSE 流内容**（nginx `logs/nginx-sse-hits.log`）：`GET /executions/324a41d7/events?version=2` → **200, 5484 bytes**（okhttp/Android app）；对照 D8 轮（iOS a6bbded9）437 字节空转。
- detail（09/17）：「执行：succeeded · 结算：settled · **已接收事件：2**」+ 时间线 2 条活动 + 「已同步」。
- 首次尝试的中间失败（如实）：run1 `22e48da6` executor_failed——租户未配 rerank 模型，generic 空 base_url 回落 api.openai.com 被 SSRF 拦（`logs/backend-execution.log`）；provisioning 后通过。

## 目标 2：材料 + 分享 —— 部分 evidenced（面未暴露，如实记录）

- 材料页可达（11/12）：「研究与批注/只读终端/证据引用」分区渲染，刷新无崩溃；
  `GET /artifacts` 200 items=[]（quick-answer 文本回合不产工件行）。
- 分享面板：分享动作需 materialId（`materials-view.ts` share(materialId)），无工件行 → 分享面未呈现——与 #69 flow7 / T40 W4 同口径，本轮 executor 已通，剩余边界=该 agent 模式无工件产物，非执行集成缺陷。
- `GET /delivery` 500 `code_delivery_failed`：**#31 轮已知残留**（workbench_notifications 表迁移未跑），非本轮面。

## 目标 3：解析失败短码 + 无 Run 行 —— evidenced

- 提交时选 `WBGRAPH-NOMODEL`（无 model_id fixture）→ **409**
  `graph resolution failed: model_unresolved: chat model is not configured: please set model_id on agent wb-graph-nomodel-agent`（13 截图 + `logs/failcase-evidence.txt`）；
- 拒绝前后 `agent_runs`（tenant 10043）计数 4/4 不变——**无 Run 行**；
- app 面：「上次提交已被服务端拒绝；修改目标后重新提交将使用新的请求标识」。
- 活体还顺带实证 `rerank_unresolved` 短码（provisioning 前的 builtin-quick-answer 提交）。

## 活体轮暴露并修复的缺陷（均属本轮 owned surface）

1. **容器装配顺序**（e0d63bae1）：`wireWorkbenchAdmissionGraphFreezer` Invoke 在 `provideAgentSecurity` 之前解析 coordinator → 生产 boot panic（missing AgentSecurityService）。测试位未覆盖生产调用序。修=Invoke 后移。
2. **HTTP 状态对齐**（5909d58da）：freezer 失败首轮 500 vs 重放 409。修=`ErrGraphResolutionFailed`→409 + handler 集成测试。

## 环境侧 provisioning（非代码，`logs/provisioning.sql`）

- 租户 10043 默认 KnowledgeQA 模型（克隆 10001 可用的 glm-5.3/zhipu 行）+ rerank 模型行；
- builtin-quick-answer 定制行挂 model_id/rerank_model_id（freezer 链要求 per-agent 配置——tenant 默认救不了 builtin，watch-item 实测确认并收窄为「agent 自带 model_id」）；
- WBGRAPH-NOMODEL 失败 fixture agent。

## 截图索引

00 登录前 / 01 Casdoor custom tab / 02 登录后首页 / 03 表单初始（agent 目录未就绪）/ 04 表单填好 /
05 首次提交被拒（500 时代）/ 06-07 二次提交 / 08 任务列表（succeeded）/ 09-10 detail（2 事件+settled）/
11-12 材料页 / 13 失败用例拒绝文案 / 14-15 竞速中间帧 / 16 列表 running / 17 run5 detail。
