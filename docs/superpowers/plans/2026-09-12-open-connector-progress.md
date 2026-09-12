# open-connector 集成进度（2026-09-12）

> 计划：[2026-09-12-open-connector-integration.md](./2026-09-12-open-connector-integration.md)
> 固定上游：`33dd4ad6ee22f9ce5158a1516a11d8b8566b5c8a`（@oomol-lab/open-connector v1.5.0）。契约与证据：[docs/integrations/open-connector-contract.md](../../integrations/open-connector-contract.md)、`scripts/open-connector/`（门禁 + 场景 + 脱敏 fixtures）。
> 状态机：pending / in_progress / review / passed / blocked-env / blocked-contract。协调者同步各任务状态。

## 任务状态表

| 任务 | 状态 | 备注 |
| --- | --- | --- |
| T01 固定上游契约与失败关闭的证据门禁 | passed | commit e12a73ce（集成分支 HEAD）；规格审查 PASS + 质量审查 PASS（非阻塞 findings 记录在 SDD ledger）；oauth 真实 Provider 完成子项 blocked-env（R10，转 T18 关闭） |
| T02 只持受限 Token 的 HTTP 执行客户端 | passed | commit b37da57b（含 F-01 修复，初版 144ecb8b）；spec 审查 F-01 FAIL→修复→复审 PASS；质量审查 PASS（观察级 findings 记录在 SDD ledger）；go test -race 17/17、gofmt/vet 过；errorCode/meta.executionId/meta.auditPersisted 映射已钉死 |
| T03 持久绑定、版本与租户作用域仓储 | passed | commit db971251（集成分支 fast-forward）；8 仓储测试行为级 RED→GREEN（含计划 TestOCBindingRejectsZeroTenant）；PG 000121 真实 up/down/up 独立容器实测（down 拒绝非空绑定、6 项约束负向全拒）；sqlite 000041 twin 由测试直接执行真实迁移 SQL；spec 审查 PASS + 质量审查 PASS（3 MINOR + 4 INFO 观察项记录在 SDD ledger；F-02 严格版本比对要求转入 T04） |
| T04 授权与原始凭据解耦 | passed | commit b5f7b19b（集成分支 fast-forward）；A02Guard→Check(subject) 全量改造（含 DI :496 一行）；F-02 双重严格版本比对失效关闭；QR-F1/F2 调用方错误契约；Check 全路径零 LoadCredential；RED 15 项行为级失败→GREEN；-race 全绿；spec PASS + 质量 PASS（QF-1 handler 500 映射与 QF-6 nil dispatcher 为基线遗留，转 T13/T09/T10；残余 TOCTOU 归 T10 fence） |
| T05 隔离管理凭据的控制 worker | passed | commit 98bcd5a2（cherry-pick 到集成分支）；三段行为级 RED→GREEN（29 PASS+3 PG-skip，-race 过）；真实 PG 双 claim/排空/过期接管 -count=3 全过（实现者与质量审查者各自独立容器复跑）；next_at 兼任租约（冻结 schema 适配）；admin 客户端 allowlist 精确匹配+禁 redirect+错误无 secret；spec PASS + 质量 PASS（Q-01/Q-02 MINOR 与 F-03 加密归属转 T16/T08） |
| T06 审核目录与版本固定 | passed | commit 0bbd9249（集成分支 fast-forward）；17 项目录测试 RED→GREEN（含计划逐字 TestCatalogRejectsUnknownRisk 与 10 项矩阵）；jsonschema compile+metaschema 真校验（R2 santhosh-tekuri v6 偏差已记录）；SHA-256 常量独立逐位复核；冻结面（install.go 12 函数、T03/T05 仓储方法）逐字未动；spec PASS（A–D 全 ACCEPT）+ 质量 PASS（16 路并发首发 PK 竞态探针、published/Get 两态互斥、-race 全绿；8 INFO 归 T13/T07） |
| T07 可关联的 OAuth 与 API key 授权 | passed | commit a6819af5 + fix 54ab2277（集成分支 fast-forward）；R14 方案 A：admin authorizations 钉 UUID alias + 精确 alias 列表过滤相关 + api-key connect/api-key（源码钉死，T17/T18 复验）；R11 完整守卫换装落地；一次性消费条件更新经 16-goroutine 真并发探针；质量审查 Q-1 时区缺陷（BLOCKING）→修复→双审复审 PASS；真实 OAuth e2e blocked-env → T18；Q-2/Q-3（api-key 双提交/verifying 孤儿）→ T08 |
| T08 撤销、权限变化和远端清理 | passed | commit 62396d31（集成分支 fast-forward）；25 项新测试（含计划逐字 TestOCRevokeVersionGuard）；撤销单事务+幂等+行数守卫+FOR UPDATE 锁契约（文档化给 T10）；裁决 1 四项孤儿/重绑修复各有测试；PG 8 并发撤销恰一次（实现者+质量审查者独立容器各复跑一遍，-count=3 稳定）；spec PASS + 质量 PASS（Q-1 MAJOR-latent mint 守卫缺失 → T09 硬前置；Q-2/Q-3 sweeper → T17/T18） |
| T09 可信 Prepare 与完整审批快照 | passed | commit b61389a7（集成分支 fast-forward）；digest v2 全字段结构化材料（含 OC/AuthVersion/DigestVersion，无文本拼接）；v1 审批强制重新 Prepare（行级双门禁）；HARD PREREQ：T08 mint 对账 404 守卫 + 双测试落地；PG 000122 v1 回填/守卫/再 up 实测（sqlite 000042 twin 全序列复现）；spec PASS（4 项解释全 ACCEPT，13 findings 非阻塞）+ 质量 PASS（7 项 -race digest 决定论探针、原子性/安全面/守卫分类全核验；MINOR-1 handler 映射 → T13） |
| T10 原子 claim、全局幂等键与分布式限流 | passed | commit 764e0287（集成分支 fast-forward）；27 项新测试；PG 20 并发单胜者 + Check 后撤销拒绝（T04-QF-2 关闭）+ 45 对死锁锤零死锁 + 崩溃 lease 回收不重发（实现者+质量审查者各自独立容器）；nil dispatcher fail-closed（T04-QF-6 关闭）；ReplayUntil=FirstSentAt+23h50m 不随重试推进；spec PASS（F-3 接线次序转 T13 强制）+ 质量 PASS（Q-1 孤立预占释放 → T11/T13） |
| T11 HTTP dispatcher 与保守结果分类 | passed | commit ee8c6788（集成分支 fast-forward）；settleOutcome 计划草图逐字（unknown-first）；预发送 16+ 拒绝全 failed 零 HTTP；单次 POST + 持久 claim key 纪律；httptest 矩阵全过（质量审查者独立重搭 8 场景 16 子测试含 TCP hijack 断连）；CARRY T10-Q-1 零用量释放（比裁决更严）+ T10-Q-5 保留值语义；spec PASS + 质量 PASS（QF-5 释放后重占窗口 → T12 Reconcile 双 Finish / T13 不变式注释；QF-1 failed 白名单封闭性 → T18 契约复核） |
| T12 崩溃、unknown 与结算恢复 | pending | |
| T13 产品 API、租户 DTO 与容器注入 | pending | |
| T14 Agent 共用 Action 生命周期 | pending | |
| T15 Vue 目录、连接和审批界面 | pending | |
| T16 私网部署、观测、备份与升级 | pending | 必须配置 ADMIN_TOKEN（T01 已证无认证=完全开放）；ghcr 无 pinned SHA 镜像（R9 本地构建 digest 为准） |
| T17 多空间与故障注入集成验收 | pending | OC_TEST_DATABASE_URL 专用 PostgreSQL |
| T18 真实 Provider、商业链路和灰度门禁 | pending | 关闭 T01 oauth blocked-env 子项；需真实账号/目标/内容授权 |

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
