# open-connector 集成进度（2026-09-12）

> 计划：[2026-09-12-open-connector-integration.md](./2026-09-12-open-connector-integration.md)
> 固定上游：`33dd4ad6ee22f9ce5158a1516a11d8b8566b5c8a`（@oomol-lab/open-connector v1.5.0）。契约与证据：[docs/integrations/open-connector-contract.md](../../integrations/open-connector-contract.md)、`scripts/open-connector/`（门禁 + 场景 + 脱敏 fixtures）。
> 状态机：pending / in_progress / review / passed / blocked-env / blocked-contract。协调者同步各任务状态。

## 任务状态表

| 任务 | 状态 | 备注 |
| --- | --- | --- |
| T01 固定版本契约探针与证据门禁 | in_progress | 实现与证据完成，待协调者 review |
| T02 受限 Token HTTP 执行客户端 | pending | 消费 T01 fixtures 与 errorCode/meta 契约 |
| T03 连接绑定与授权模型 | pending | |
| T04 Token 生命周期管理 | pending | NewGrant 拒空 grant（T01 已证 upstream 空=放行） |
| T05 审批与预算接入 | pending | |
| T06 OAuth 关联流程 | pending | 消费 T01 oauth_correlation（state/connectionRequestId） |
| T07 执行快照与幂等 | pending | 24h 窗口/指纹含 runtimeTokenId（T01 已证） |
| T08 审计与补偿 | pending | auditPersisted 语义（T01 已证） |
| T09 计量与计费 | pending | |
| T10 产品流程 UI | pending | |
| T11 前端连接管理 | pending | |
| T12 运维与部署 | pending | |
| T13 集成测试 | pending | |
| T14 文档与示例 | pending | |
| T15 监控告警 | pending | |
| T16 共享实例部署 | pending | 必须配置 ADMIN_TOKEN（T01 已证无认证=完全开放）；ghcr 无 pinned SHA 镜像（R9） |
| T17 租户开放验收 | pending | |
| T18 发布验收 | pending | |

## T01 证据

- 工作区：`.worktrees/oc-t01`（分支 `codex/oc-t01`，base `83e2ef5c`）。
- RED：`python3 -m unittest discover -s scripts/open-connector -p 'test_*.py' -v` → **FAILED（errors=1，ModuleNotFoundError: contract_gate），exit 1**。
- GREEN（最小实现 `contract_gate.py` 后）：同命令 → **OK，15 tests，exit 0**（覆盖：doc-only 拒绝、缺 case、非 pinned SHA、缺 digest、非 sha256 digest、部分 case 集、passed!=true、空 artifact、完整报告通过、CLI exit 0/2、坏 JSON、缺文件）。
- 门禁 CLI：`python3 scripts/open-connector/contract_gate.py scripts/open-connector/fixtures/contract-report.json` → `GATE OK`，**exit 0**。
- 镜像 digest（R9）：`sha256:fcd8d2b871360efcfc43ec051f3359b9de69c04be5d93eebd382633a2418d130` — 本地从 pinned commit 构建（`docker build -f docker/Dockerfile -t oc-t01-open-connector:33dd4ad .`，构建前 clone HEAD==pinned SHA，取 `docker image inspect --format '{{.Id}}'`）；ghcr 无该 SHA 标签（详见契约文档 §2）。镜像验证后已删除（digest 已记录，可复现）。
- 每案例证据（kind=runtime, passed=true, artifact 于 `scripts/open-connector/fixtures/`）：

| case | 方法 | 结果 | artifact |
| --- | --- | --- | --- |
| cross_connection | live HTTP | pass（403 connection_not_allowed） | fixtures/cross_connection.json |
| empty_grant | live HTTP | pass（空 grant 放行，provider 拒假 key） | fixtures/empty_grant.json |
| default_alias | live HTTP | pass（default 无回退；未知别名 404） | fixtures/default_alias.json |
| no_auth | live HTTP | pass（虚拟连接豁免 grant） | fixtures/no_auth.json |
| admin_denied | live HTTP | pass（401；对照：无认证=开放 200） | fixtures/admin_denied.json |
| proxy_denied | live HTTP | pass（403 proxy_not_allowed） | fixtures/proxy_denied.json |
| oauth_correlation | live HTTP | pass（state 关联全链路；connected 态 blocked-env） | fixtures/oauth_correlation.json |
| key_replay | live HTTP | pass（同 meta.executionId） | fixtures/key_replay.json |
| key_conflict | live HTTP | pass（409 idempotency_key_conflict） | fixtures/key_conflict.json |
| in_progress | live 并发 + 上游测试 | pass（409 idempotency_request_in_progress） | fixtures/in_progress.json |
| expired_key | 上游可控时钟测试 | pass（24h 过期） | fixtures/expired_key.json |
| audit_failure | 上游注入失败测试 | pass（auditPersisted=false 结果不变） | fixtures/audit_failure.json |

- 环境备注：探针实例 127.0.0.1:31701（一次性 sqlite /tmp/oc-t01-data-a）+ 对照实例 31702（无认证）；host Node v26.7.0 原生 TS 运行 pinned 源码；上游 vitest 4.1.9 于隔离 clone；host DNS 将部分公网 API 解析至私网 → 一次性实例加 EGRESS_TRUSTED_HOSTS（export.arxiv.org, api.crossref.org）。dune/postmark/hasdata 无 credentialValidators，假 key 可存（cross_connection/empty_grant 用 dune）。
- 偏差与事实修正：package.json 无 engines/packageManager（Node 24 由 Dockerfile/CI 佐证）；错误码字段为 errorCode（非 code）、executionId/auditPersisted 在 meta；幂等指纹含 runtimeTokenId（未文档化）；OAuth clientConfig 字段在请求体顶层；ghcr 无 pinned SHA 镜像（R9：本地构建 digest 为准）。
- 清理：2 个 token 已 revoke、4 个存储连接 + github client config 已删除（残留检查 0/0）、实例 A/B 已停止（31701/31702 down）、docker 镜像已删、/tmp 数据目录已删；上游 clone 留在 detached pinned HEAD（仅 node_modules/生成物，可复验）。
- 遗留阻塞：无（oauth happy-path 为 blocked-env 子项，已在契约文档与 fixture 中如实标注，机制证据完整）。
