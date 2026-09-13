# open-connector 发布门禁与灰度手册（T18）

> 任务：T18（验收链 T17✓）。基线 `219ffe67`（集成分支 HEAD）。
> 门禁工具：`python3 scripts/open-connector/release_gate.py <release-report.json>`（测试：`scripts/open-connector/test_release_gate.py`，29 用例）。
> **当前结论：不可发布（NOT releasable）**。七类证据中 provider_write 与 billing 为 blocked-env（本会话无真实 Provider 写授权、无商业计量环境授权）；逐类状态与缺项见 §5——按规格如实列缺项，不写"生产可用"。任何"通过门禁"的结论只能由 §4 的门禁命令 exit 0 给出，不由本文叙述给出。

## 1. 发布候选身份（identity）

| 项 | 值 |
| --- | --- |
| 上游源码 | `33dd4ad6ee22f9ce5158a1516a11d8b8566b5c8a`（`@oomol-lab/open-connector` v1.5.0，本地 clone `.oc-upstream-33dd4ad`，HEAD 已核验） |
| runtime 镜像 | `oc-t16-open-connector:33dd4ad@sha256:4de6df4d649e3f99bff31aeba854439b3f6e104434e4087aba0a4b9c2b4c9330`（T16 本地构建钉定，`pull_policy: never`；绝不使用 ghcr 可变 tag） |
| connector-control 镜像 | compose `dockerfile_inline` 从本仓库 checkout 构建（构建态 `golang@sha256:9fdc884a…`，运行态 `debian@sha256:d5d3f9c2…`；需不可变部署时 build 后 `docker image inspect` 自行锁定） |
| connector-db 镜像 | `postgres@sha256:7c688148e5e156d0e86df7ba8ae5a05a2366aaec1e2ad8d6d11bdf10504b1fb7`（16.9-alpine） |
| WeKnora 提交 | 发布报告必须绑定一个 40-hex commit；门禁逐类校验所有证据同 commit（见 §4） |

镜像 provenance、secrets 与拓扑详见 [open-connector-operations.md](./open-connector-operations.md) §2–§3；契约详见 [open-connector-contract.md](./open-connector-contract.md)。

## 2. 开关与允许范围（phase one）

- **feature flag**：`open_connector.enabled=false` 默认关（nil/缺省 = 显式拒绝派发，503 fail-closed）；env 覆盖 `WEKNORA_OC_ENABLED` / config 级 `WEKNORA_OPEN_CONNECTOR_ENABLED`（只能使能，不能静默关闭 yaml 显式开启）。`open_connector.runtime` 只接受内部 ID `shared`。
- **Provider Proxy 首期关闭**（`OOMOL_CONNECT_ALLOWED_PROXIES=""`，空清单=拒绝全部代理）；文件/临时 URL 动作不发布；现有 Sync 保留。
- **空间灰度顺序**：先**单个授权测试空间** → 成功后逐步扩大。扩大名单按**当时的用户发布授权**执行，不由本文预置。
- **动作与版本**：只发布冻结目录（T06）中的动作；同 (app, version, action) 的 schema 字节冻结，改动即拒（T17 场景 6）；审批摘要 digest v2，v1 审批遇新 digest 强制重新 Prepare（`action_reprepare_required`）。**价格约束**：仅发布 ≤ ActionService budgetUpper（1 credit/次）的动作，否则结算标 `abnormal_cost`（operations §4 上线清单）。

## 3. 灰度阈值与停止规则（T16 §6.5）

| 信号 | 阈值 / 动作 |
| --- | --- |
| 最老 unknown 年龄 | > 5 分钟 → 暂停扩大范围，先对账 |
| 撤销清理滞后 | > 5 分钟 → 暂停扩大范围 |
| 结算 outbox 最老未处理 | > 10 分钟 → 暂停扩大范围 |
| 串空间 / 凭据泄漏迹象 | **立即停新派发**：`WEKNORA_OC_ENABLED=false` 重启 API（或 config 关闭），数据面不动 |
| 门禁失败（任一类） | 不得开启新空间的派发授权 |

## 4. 门禁：七类证据 + 运行报告格式

发布前在一个**独立 release test namespace** 内重放全部证据，产出 release report（JSON，artifact 路径相对报告文件目录）：

```json
{
  "commit": "<40-hex WeKnora 发布 commit>",
  "image_digest": "sha256:<64-hex>",
  "test_namespace": "oc-release-<id>",
  "open_unknown_count": 0,
  "contract":       {"kind":"runtime","passed":true,"artifact":"artifacts/contract.json","sha256":"<文件 sha256>","commit":"…","image_digest":"sha256:…","test_namespace":"…"},
  "integration":    {…同形状…}, "browser": {…}, "provider_read": {…},
  "provider_write": {…}, "billing": {…}, "recovery": {…}
}
```

```bash
python3 scripts/open-connector/release_gate.py release-report.json   # exit 0 才可进入灰度
```

门禁规则（fail closed，`release_errors` 按计划草图逐字实现）：

1. 七类证据 contract / integration / browser / provider_read / provider_write / billing / recovery 缺一不可；每类必须 `kind=runtime` 且 `passed=true` 且有非空 `artifact`。**mock/doc 证据永不合格**（`test_mock_write_never_qualifies` 钉死——门禁存在的目的就是拒绝伪造/模拟写证据）。
2. `open_unknown_count` 必须显式为 0（缺省按 -1 处理 → 报错"unresolved executions in release test namespace"）。
3. CLI **绝不只信 passed=true**：逐类核验 artifact 文件存在、记录的 sha256 与磁盘文件一致、且每类的 commit / image_digest / test_namespace 与报告头**完全一致**（同一 commit、同一镜像、同一测试命名空间）。
4. 暂无法取证的类用 `{"kind":"blocked-env","blocked_env":"missing: <具体缺项>"}` 记录：门禁对该类仍然 FAIL（这正是目的），CLI 会回显 `blocked-env: <类>: missing: …`，让缺项显式出现在输出里，而不是摘要式宣称。

## 5. 七类证据现状（截至 T18，2026-09-13）

| 类 | 状态 | 证据落点（真实 artifact + 指纹） | 缺项（如实） |
| --- | --- | --- | --- |
| contract | runtime 证据已在库（T01） | `scripts/open-connector/fixtures/contract-report.json`（sha256 `3155ef5d…e3160`，12 case fixtures 同目录） | 产出时镜像为 T01 本地构建 `sha256:fcd8d2b8…`（同一 pinned 源的另一次本地构建，buildx attestation 使逐位复现不可能）。**发布时须在候选镜像 `4de6df4d…` 上重放探针**以通过 §4 第 3 条同镜像校验 |
| integration | runtime 证据已在库（T17） | `scripts/open-connector/acceptance-cases.json`（sha256 `08136aca…1944`；14 场景全绿、-race 0 竞争、`OC17-EVIDENCE` 38 条证据行记于 SDD ledger task-T17-report.md） | fake Provider（操作计数+信封），真实 Provider 链路的集成证据归 provider_read/provider_write 类；发布时须在候选 commit/namespace 重跑矩阵 |
| browser | 会话内已做，库内 artifact 缺 | T15 浏览器 26/26 + R18 复验 5/5 记录于 SDD ledger task-T15-report.md（截图路径同处）；库内自动面：`frontend/src/views/apps/actionState.test.ts`（sha256 `559be3df…d474`）、`pollBackoff.test.ts`（`2b78b156…5d674`）、`apps/web/src/appconnector/action-state.test.ts`（`313c59b2…3dbf`）、`connection-state.test.ts`（`11071434…75d9`），T17 §6 复验 7/7 | **无库内浏览器运行产物 JSON**（当时证据为 ledger + 截图）。发布时须重跑四视图浏览器验证并以文件 artifact 落库 |
| provider_read | **本会话已在候选镜像上真实验证（只读）**，见 §6；证据已按 R21 落库 | §6 记录 + **库内工件** `scripts/open-connector/fixtures/provider_read_runtime.json`（sha256 `6a6a48755a997b4af67e300b5cb5f3db0079527db1e92a3ad7b46232c7d2fd97`；200/400/403 观测 + 清理记录，脱敏无 token） | (a) ~~证据 fixture 未入库~~ 已按 R21 落库；(b) 经 WeKnora 全链（Prepare→审批→Execute→预算结算）的真实 Provider 读未做：blocked-env，缺授权测试空间；(c) OAuth connected 态真实流程：blocked-env，无授权 OAuth app/账号 |
| provider_write | **blocked-env** | 无（不得伪造；门禁拒绝 mock 写证据） | 缺：本会话无任何真实 Provider 测试账号/目标/内容授权（GitHub 写目标仓库、Notion 测试页面、消息测试联系人）。授权后按计划执行 Prepare→审批→Execute→Provider 查询确认→结算，且绝不向真实联系人发测试消息 |
| billing | **blocked-env**（本地结算面证据存在） | 本地：T17 场景 7/9/15（恰 1 条用量事实、结算恢复单事实 Revision=1、期末未决结算 0，`acceptance-cases.json`） | 缺：真实商业计量环境授权（余额/预占/使用/结算的真实 API 证据，不用本地 DB 行代替计量方接收证据） |
| recovery | runtime 证据已在库（T17） | `acceptance-cases.json` 场景 8/9/10/11（unknown 不重发、崩溃恢复 key/runtime 保持、结算重投恰一次、ReplayAllowed 边界）+ 收尾扫描（未解释 unknown 0、未决结算 0） | 发布时须在候选 commit/namespace 重跑（同 integration 类） |

## 6. provider_read 会话内真实验证记录（T18，只读）

边界遵守：仅只读验证（协调者裁决 2："GitHub get_current_user with any available account"）；**未发生任何 Provider 写、未向任何联系人发消息、未伪造证据**。凭据来自宿主机 gh CLI keyring（账号 `1123786563`），全程未回显 token。

- 运行时：一次性容器 `oc-t18-probe`，镜像**逐 digest 钉定候选镜像** `oc-t16-open-connector:33dd4ad@sha256:4de6df4d…9330`，sqlite 数据目录，`-p 127.0.0.1:31818:3000`（仅回环）。
- 环境：宿主 fake-IP VPN 将 `api.github.com` 解析为 `198.18.0.18`（198.18.0.0/15 基准保留段）→ 上游 SSRF 守卫按设计拒绝；按守卫自带提示与 T01 先例加 `OOMOL_CONNECT_EGRESS_TRUSTED_HOSTS=api.github.com` 后恢复（curl 直连 200 佐证出网可达真实 GitHub）。
- 步骤与结果（全部真实 HTTP）：
  1. `PUT /api/connections/github`（admin bearer，api_key=gh token，alias `t18-read-probe`）→ **200**；GitHub 连接器凭据校验器真实调用 GitHub 成功，返回 `accountId=1123786563`（连接 id `184c4379-fb34-4d2b-a279-c3f1fb5484a6`）。
  2. `POST /api/runtime-tokens`（非空 grant：`allowedActions=["github.get_current_user"]`、`allowedConnections=[<连接id>]`、`allowedProxies=[]`（空=拒绝全部代理））→ 200，受限 token `oct_…`（id `6e540308-9083-4b25-aeca-8b7d2f8d42be`）。
  3. `POST /v1/actions/github.get_current_user`（受限 token + `x-oo-connector-alias: t18-read-probe` + `Idempotency-Key: t18-probe-getcuser-1`）→ **200** `success=true`，`data.login=1123786563`、`type=User`，`meta.executionId=79c4571e-23c6-4c6f-8378-6021ea04af2b`、`auditPersisted=true`。响应文件 sha256 `48a91fe2fc10c4689ebfe110b42b7bd161e1918cc59f4d6f280e9efbe4fdbc5f`（脱敏后已随一次性目录销毁；指纹在此留档）。
  4. 负向 1：未授权动作 `github.list_my_repositories` → **400 `action_not_allowed`**（"not included in the local action allowlist"，执行前拒绝，0 次 Provider 调用）。响应 sha256 `6322b90a…674a6`。
  5. 负向 2：无 alias（解析 default）→ **403 `connection_not_allowed`**（"not granted to this runtime token"，凭据使用前拒绝，0 次 Provider 调用）。响应 sha256 `3cd34f67…5d751`。与 T01 冻结契约的跨连接拒绝语义一致。
- **清理（全部核验）**：DELETE runtime token → 200 `revoked=true`；删除后原 token 调用 → **403**；DELETE 连接 → 200；`GET /api/connections` github 行 **0**、`GET /api/runtime-tokens` **0** 条；容器已删、`/tmp/oc-t18-probe`（含 token 文件）已删。**零残留**。
- **工件落盘（R21）**：本节全部观测（含上述 4 枚响应指纹、脱敏规则与范围限制）已固化为库内证据 fixture：`scripts/open-connector/fixtures/provider_read_runtime.json`（sha256 `6a6a48755a997b4af67e300b5cb5f3db0079527db1e92a3ad7b46232c7d2fd97`）——release 门禁 provider_read 类的库内工件。
- 契约观察（真实 Provider 语境，已记入契约文档 §3.3 注，不改写契约）：token 动作允许层拒绝 = 400 `action_not_allowed`（§3.3 "400 default" 家族）；github api_key 连接在 connect 时即做真实凭据校验。

## 7. 回退命令（rollback）

与 operations §6.4 一致，摘录关键序（先关开关，数据面不动）：

```bash
# 1) 立即停新派发（fail-closed 503）
#    config: open_connector.enabled=false 或 env: WEKNORA_OC_ENABLED=false，重启 WeKnora API
# 2) 停 OC 栈（30s 宽限排空 in-flight）
docker compose -f docker/compose.open-connector.yaml stop -t 30
# 3) 镜像/库回滚到备份版本；绝不删除 connector_* 审计表
# 4) 已铸 token 的撤销经 control worker 正常走（DELETE），绝不写 allowedConnections=[]
# 5) 回滚后重新放量前：重跑 §4 门禁（exit 0）+ §3 阈值观察
```

## 8. 已知一期限制（终局记录，非延期）

1. **T01 OAuth 真实 Provider 子项**：blocked-env 成为**永久记录的限制**（本会话无授权 OAuth app/账号做 connected 态流程）；机制证据完整（state 关联/回调/失败路径 fixtures 齐全）。
2. **stale 授权 attempt 清理**：无 sweeper；T17 期末观测 non_terminal_attempts=2。操作面以 §3 阈值监控兜底。
3. **Retry-After 透传**：opportunistic-unimplemented——dispatcher 对 429 记固定 30s 冷却（`ocThrottleCooldown`），不解析 Provider Retry-After 头。
4. **T17-F2 列表可见性**：连接列表返回同租户全部连接元数据投影（无凭据材料）；服务端 owner 过滤列规格积压。
5. **T13-F-3 auth URL**：一期不设授权 URL 端点；T15 指引文案（`noUrlGuidance`，无链接、仅状态轮询）为终局决定。

详细裁决记录见 SDD ledger task-T18-report.md（六项 CARRY 逐项结论）。
