# open-connector 私网部署运维手册（T16）

> 上游固定版本：`33dd4ad6ee22f9ce5158a1516a11d8b8566b5c8a`（`@oomol-lab/open-connector` v1.5.0）。
> 契约证据：[open-connector-contract.md](./open-connector-contract.md)；部署栈：`docker/compose.open-connector.yaml`；静态门禁：`python3 scripts/open-connector/check_deployment.py <compose-config.json>`（测试：`scripts/open-connector/test_deployment.py`）。
> 本文所有事实来自 T01 运行时探针与 T16 本地演练（见 §8），非 README 推断。

## 1. 拓扑与网络

```
            (Internet, OAuth providers)
                   ^  受控出口（非 internal 网；上游自带 SSRF 守卫 fetch）
                   |
  +----------------oc-net (bridge, 无任何 published port)----------------+
  |  open-connector (runtime, :3000)      connector-control (worker)      |
  |        |                                        |                     |
  +--------|----------------------------------------+---------------------+
           | oc-db-net (bridge, internal: true —— 无出口)
       connector-db (postgres, :5432)
```

- **不发布任何 OC/DB/control 端口**。`check_deployment.validate_compose` 对三个核心服务（`open-connector` / `connector-db` / `connector-control`）出现任何 `ports:` 即报错（部署反例测试 `test_public_runtime_port_rejected` 等钉死该行为）。
- **oc-net 不加 `internal: true`**：OC 需要出网完成 OAuth provider 流程与 provider API 调用；裸 `internal:true` 会断掉 OAuth。出口纪律依赖上游运行时自带的 SSRF 守卫（DNS 解析校验、重定向逐跳校验、云元地址封禁）。
- **oc-db-net 是 internal**：数据库没有任何出口路径。
- **WeKnora API 进程**（本 compose 之外）以第二容器加入 `oc-net`（或经反代到达 `http://open-connector:3000`），只持有受限 runtime token 读回所需的 **sink 加密密钥**与共享 sink 卷，**绝不挂载 admin secret**（validator 强制：`connector-admin-token` 只允许出现在 runtime 与 control worker 上）。
- **Callback ingress（唯一允许的外部入口）**：T01 核实的免鉴权回调路径是 `/oauth/callback*`。如需从外部收 OAuth 回调，在独立反代上**只**放行该路径，其余（`/api/*` 管理 API、`/v1/*`、`/mcp`、控制台 `/`、`/docs`、日志面）一律 deny；默认部署（本 compose）不发布端口，外部探测一律拒绝（§8 演练记录）。
- 保留独立实例选项：另起一套同构 compose（换 `name:` 与网络）即可，不改代码；配置面通过 `open_connector.runtime` 内部 ID 映射扩容（见 §4）。

## 2. 镜像与 provenance（裁决 R9）

- ghcr.io/oomol-lab/open-connector **没有** pinned SHA 的不可变 digest（tag 列表无 `33dd4ad`/`v1.5.0`；`tip` 可变且无法证明由该 commit 构建）。**绝不使用 tip/latest**（validator 拒绝 `:latest`/`:tip`/无 digest 引用）。
- compose 钉的是**本地从 pinned 源构建的 digest**：
  - 源：`.oc-upstream-33dd4ad`（构建前 `git rev-parse HEAD` == `33dd4ad6ee22f9ce5158a1516a11d8b8566b5c8a`，工作树 clean）。
  - 命令：`docker build -f docker/Dockerfile -t oc-t16-open-connector:33dd4ad .`（在该 clone 内执行；`docker/open-connector.Dockerfile` 是该 Dockerfile 的逐字节副本）。
  - 产物：`docker image inspect --format '{{.Id}}'` → `sha256:4de6df4d649e3f99bff31aeba854439b3f6e104434e4087aba0a4b9c2b4c9330`（compose 引用形如 `oc-t16-open-connector:33dd4ad@sha256:4de6df4d…`，`pull_policy: never`）。
  - T01 对同一源曾构建并记录 `sha256:fcd8d2b871360efcfc43ec051f3359b9de69c04be5d93eebd382633a2418d130`；buildx attestations 使逐位复现不可能，两个 digest 均为同一 pinned 源的本地构建，本文件钉 T16 重 build（证据即上述命令与输出）。复验方式：同一 clone 重建并比对 `docker build` 输出的 manifest list digest（允许不同，但不允许引用任何 ghcr 可变 tag）。
- `connector-control` 镜像在 compose 内 `dockerfile_inline` 从**本仓库 checkout**（二进制的 pinned 源）构建：`golang@sha256:9fdc884aacc3bec89b20ffc69f4bb369c78210e3e4f600387b5128b12c199f81`（1.26-bookworm）构建态 + `debian@sha256:d5d3f9c23164ea16f31852f95bd5959aad1c5e854332fe00f7b3a20fcc9f635c`（12.12-slim）运行态。**cgo 必需**：pg_query（经 internal/types 引入）构建 libpg_query 需要 gcc，bookworm 自带；早先的 alpine 方案因构建网络不可达 alpine CDN 且缺 cgo 工具链而弃用。gojieba 词典随镜像发布（`JIEBA_DICT_DIR=/usr/local/share/jieba-dict`）。需不可变部署时，build 后 `docker image inspect` 取 digest 并自行锁定引用。
- `connector-db`：`postgres@sha256:7c688148e5e156d0e86df7ba8ae5a05a2386aaec1e2ad8e6d11bdf10504b1fb7`（16.9-alpine）。
- `oc-volume-init`：`alpine@sha256:1435830…`，一次性把 `oc-data`（uid 10001）/`oc-secrets`（uid 10002, 0700）卷属主修正后退出——两个应用容器都以非 root 运行。

## 3. Secrets（fail closed by construction）

Operator 在 `${OC_SECRETS_DIR:-./docker/secrets}` 下置备三个文件；**文件缺失时 `docker compose config` 直接渲染失败**，即不配 secrets 无法到达部署一步：

| 文件 | 用途 | 挂载方 | 权限 |
| --- | --- | --- | --- |
| `connector-admin-token` | runtime ADMIN token（**强制**，T01：无认证 = 完全开放） | open-connector、connector-control | 0444 → worker 侧复制为 0400 |
| `connector-db-password` | connector-db 的 postgres 口令 | open-connector、connector-db | 0444（经 `_FILE`/wrapper） |
| `connector-secret-key` | secret sink 静态加密密钥（64 hex / base64(32B) / 32 原始字节） | connector-control（写入侧；API 容器另挂同一文件用于读回） | 0444 → 复制为 0400 |

- ADMIN_TOKEN 的强制性有三层：validator（缺挂载即 invalid）、compose 渲染（secret 文件缺失即失败）、运行时行为（T01 实测未带 token 打 `/api/*` → 401；§8 演练复测）。Provider Proxy 首期保持关闭（`OOMOL_CONNECT_ALLOWED_PROXIES=""`；上游语义：空清单 = 拒绝全部代理）。
- worker 容器对两个 0400 语义敏感的 secret 做 tmpfs 内 `install -m 0400` 复制（compose 文件 secret 到达时可能是 root:0444；`FileAdminSecretSource`/`FileSecretKeySource` 强制 group/world 位为 0）。

## 4. WeKnora 侧接线（config + env）

- `config/config.yaml`：
  ```yaml
  open_connector:
    enabled: false   # 默认关：nil/缺省 = 显式拒绝派发（503 fail-closed）
    runtime: "shared" # 内部 ID，经 config.OpenConnectorRuntimeAddr 映射到 http://open-connector:3000
  ```
  **runtime 只接受内部 ID**（`shared`），原始 URL 在 `ValidateConfig` 被拒——配置文件永远无法把派发器指向任意主机。新增部署拓扑 = 在 `internal/config` 的映射表加一行（代码变更），而非改配置。
- env 契约（`WEKNORA_OC_*`，T13 沿用；优先级高于 yaml）：`WEKNORA_OC_ENABLED`、`WEKNORA_OC_RUNTIME_ADDR`、`WEKNORA_OC_TOKEN_DIR`、`WEKNORA_OC_SECRET_KEY_FILE`（置则读侧用加密 sink）、`WEKNORA_OC_SLOT_OWNER`、`WEKNORA_OC_RECOVERY_INTERVAL`。config 级 env 覆盖：`WEKNORA_OPEN_CONNECTOR_ENABLED` / `WEKNORA_OPEN_CONNECTOR_RUNTIME`（只能使能，不能静默关闭 yaml 的显式开启）。
- control worker env：`CONNECTOR_CONTROL_SECRET_KEY_FILE` **必填**（无密钥拒绝启动——生产永不落明文 sink）；`CONNECTOR_CONTROL_SECRET_DIR`、`CONNECTOR_CONTROL_ADMIN_SECRET_FILE`、`CONNECTOR_CONTROL_RUNTIME_BASE_URL`/`_ALLOWLIST`（钉 `http://open-connector:3000`）、`DB_*` 指向 **WeKnora 库**（outbox 在 WeKnora 库，不在 connector-db）。
- **budgetUpper 计费要求（T12/T13 备注，部署前必读）**：ActionService 以 Upper=1 credit/次预占（冻结面，无 setter）。任何实际价格 >1 credit 的动作会以 `abnormal_cost` 结算（结算仍持久化，不丢单但产生异常标记）。上线清单：只发布价格 ≤ 默认上限的动作，或先给 ActionService 增加可配置 upper（additive 变更，另行任务）。

## 5. 观测：指标、标签纪律与审计

数据面（WeKnora 库表 + 运行日志）承载以下指标定义；Prometheus 化时按此映射，**标签纪律先行**：

| 指标 | 定义/来源 |
| --- | --- |
| 排队深度 | `connector_dispatch_records WHERE state='dispatched'` 计数（在途未决） |
| claim 冲突 | dispatch claim 竞争失败计数（`PRIMARY KEY (tenant_id, action_id)` 单发 + 429 边缘拒绝日志） |
| unknown 数量/年龄 | `state='unknown'` 计数与 `max(now()-updated_at)`（T12 恢复循环每 15s 对账） |
| outbox 重试 | `connector_operations_outbox` 重试计数（worker backoff 上限 5m，带抖动） |
| 撤销延迟 | 授权撤销（DELETE token + 绑定状态翻转）端到端耗时；清理滞后看 `state='revoked'` 但 token 记录未删的残留 |
| 429 | `connector_provider_retry_state`（每 provider 的 Retry-After 生效窗口）+ 边缘 429 日志 |
| 结算积压 | 派发已终态但 settlement 未落账的记录数与最老时间戳 |

- **标签只用有界 provider/status**；tenant id、action id、secret 一律不得进高基数 label——审计里用 tenant/action/execution ID 做**关联**（日志与表字段），不做指标维度。
- **Provider 数据脱敏按白名单**：落日志/审计的 provider 载荷只保留白名单字段（provider、action、状态、耗时、错误码）；凭据、token、原始请求体永不落盘（T01/T05 契约：DB 只存 secret 引用）。
- 上游健康：`GET /health`（免鉴权）→ compose healthcheck（镜像自带 `node scripts/healthcheck.ts`）。

## 6. 备份 / 恢复 / 升级 / 回滚 playbook

### 6.1 备份（一致性集合）
1. WeKnora 库（含 `connector_connection_bindings`、`connector_authorization_attempts`、`connector_operations_outbox`、`connector_dispatch_records`、`connector_dispatch_leases`、`connector_provider_retry_state`、商业结算表）——`pg_dump --serializable-deferrable` 或等价快照。
2. OC runtime 库（connector-db：连接、runtime token 记录、审计）——同上快照。
3. **密钥版本**：`connector-admin-token`、`connector-db-password`、`connector-secret-key`（版本化保存；sink 密钥丢失 = 已封存 token 材料不可读，只能全量重铸）。
4. compose 引用 digest 清单（`docker compose config` 渲染件）留档。

### 6.2 恢复（restore）
1. **先停新派发**：`WEKNORA_OC_ENABLED=false`（或 config 关闭）重启 API；control worker 停止（`docker compose stop connector-control`）。
2. 恢复 WeKnora 库 + OC 库 + 密钥（版本必须与备份时一致）。
3. **对账未决 key**：`connector_dispatch_records` 中 `state IN ('dispatched','unknown')` 的记录，逐条与 OC runtime 的 execution 结果核对（admin 查询经 control worker）；`unknown` 且无可靠 Provider 查询依据的**保持 unknown**——不制造失败、不自动重发（规格约束）。
4. **绝不切新 runtime 重发**：`UNIQUE (runtime_id, key)` 使旧 runtime 的幂等键无法迁移；正确路径是原 runtime 恢复后对账，或按 §6.4 回滚后重新发起。
5. 恢复完成后灰度放量（先 1 个空间）。

### 6.3 升级（upgrade）
1. **停授权与 claim**：停止新的授权开始（control worker 停止 = outbox 不再消费，新授权入队不执行）；`OCShutdownGate` 关闭后新 claim 429 拒绝（`docker stop` API 触发 30s in-flight 排空，残余交恢复循环）。
2. **排空或冻结在途**：等待 `state='dispatched'` 归零或超时转 unknown；快照备份（§6.1）。
3. **回归**：`python3 -m unittest discover -s scripts/open-connector -p 'test_deployment.py' -v`、`go test ./internal/config/... ./internal/container/... ./internal/connectorcontrol/...`、`check_deployment` 对新 compose 渲染件跑通。
4. **发布门禁（T18）**：在新 digest + 候选 commit 上重放七类证据并产出 release report，`python3 scripts/open-connector/release_gate.py <report.json>` 必须 exit 0（见 [open-connector-release.md](./open-connector-release.md) §4–§5；失败按缺项补证，不得跳过）。
5. **灰度**：替换 open-connector 镜像 digest（新 digest 附同源构建证明）→ 单实例起 → 1 个空间验证 prepare/approve/execute/撤销 → 按 §6.5 阈值放量。

### 6.4 回滚（rollback）
1. **先关 feature flag**：`open_connector.enabled=false`（或 `WEKNORA_OC_ENABLED=false`）——派发面立即 fail-closed（503），数据面不动。
2. 镜像/库回滚到备份版本；**绝不删除审计表**（`connector_*` 表与审计日志是补偿与合规的依据；down 迁移只在整特性下线评审后执行）。
3. 已铸 token 的撤销经 control worker 正常走（DELETE，绝不写 `allowedConnections=[]`——空清单 = 全放开，T01 运行时证实）。

### 6.5 默认告警（T17 负载验证后调整阈值）

| 告警 | 默认阈值 |
| --- | --- |
| 最老 unknown 年龄 | > 5 分钟 |
| 撤销清理滞后 | > 5 分钟 |
| 结算 outbox 最老未处理 | > 10 分钟 |

## 7. 部署步骤（摘要）

```bash
# 0) secrets（§3）
export OC_SECRETS_DIR=/secure/path   # 三个文件就位
# 1) runtime 镜像（R9，一次性；见 §2 命令与 digest）
docker build -f docker/Dockerfile -t oc-t16-open-connector:33dd4ad /path/to/.oc-upstream-33dd4ad
# 2) control worker 镜像（compose 内联构建）
docker compose -f docker/compose.open-connector.yaml build connector-control
# 3) 渲染校验（隔离临时文件；绝不回显解析出的 secrets）
docker compose -f docker/compose.open-connector.yaml config --format json > /tmp/oc-compose.json
python3 scripts/open-connector/check_deployment.py /tmp/oc-compose.json   # exit 0
# 4) 起栈（WeKnora DB 连接变量按环境给）
WEKNORA_DB_HOST=… WEKNORA_DB_USER=… WEKNORA_DB_PASSWORD=… \
  docker compose -f docker/compose.open-connector.yaml up -d
# 5) WeKnora API：open_connector.enabled=true + WEKNORA_OC_TOKEN_DIR/WEKNORA_OC_SECRET_KEY_FILE 指到共享卷与密钥
```

## 8. T16 演练记录（evidence）

以下在本任务实际执行（环境：docker 29.4.0 / compose v5.1.2 / OrbStack；完整命令输出见 task-T16-report.md）：

1. **compose 渲染门禁**：`docker compose -f docker/compose.open-connector.yaml config --format json` 写入隔离临时目录 → `python3 scripts/open-connector/check_deployment.py <file>` → `DEPLOYMENT OK`，exit 0；解析件从未回显（只打印 secret source 键名验证）。测试 `test_shipped_compose_passes_validation` 常驻回归。
2. **外网探测私有路径必须拒绝**：本栈 `docker port` 输出为空、`NetworkSettings.Ports` 为 `{"3000/tcp":null}`（零发布）；host 直连容器 IP → 不可达（curl exit 6/000）。注意：宿主机 127.0.0.1:3000 的 200 响应来自 OrbStack 为**另一栈**转发的已发布端口（lsof 证实监听者是 OrbStack，非本栈容器）——本栈自身无任何入口。5432 → connection refused。
3. **网内鉴权探测**（运行时容器内）：`GET /health` → 200（免鉴权路径）；`GET /api/runtime-tokens`（无 token）→ **401**（ADMIN_TOKEN 强制生效的活体证据）；控制台壳 `GET /` → 200（网内可见，管理操作仍 401）；`POST /v1/actions/github.search`（匿名）→ 404 unknown_action —— 上游"虚拟连接豁免"模型：网内匿名 /v1 调用受空连接存储约束，真实动作必经 WeKnora 铸造的受限 oct_ token；`GET /mcp` → 405（POST-only 路由）。外部不可达由无发布端口结构性保证。
4. **worker 断网演练**：`docker network disconnect oc-db-net <connector-control>` → 容器内 DNS 立即失效（getent 失败）；在途 claim UPDATE 停滞 12.8s 后以 rows:0 返回；worker 进程保持 running（RestartCount 0，无崩溃、日志零 secret 泄漏），恢复连接后轮询继续 —— fail-closed 且可恢复。
5. **退出信号演练**：`docker compose stop -t 30` 对 connector-control 与 open-connector → 两者均在宽限期内**干净退出**（exit code 0，无 OOM、无 SIGKILL 强杀）。
6. **运行时身份与 sink 硬证**：两应用容器分别以 uid 10001/10002 运行；`/tmp/copy` 下 secret 副本 10002:10002、0400；sink 目录 `drwx------ 10002:10002`（EncryptedFileSecretSink 构造期的目录权限强制通过）。

## 9. 边界与后续

- 首期 Provider Proxy 关闭；文件/临时 URL 动作不发布；现有 Sync 保留。
- 指标Prometheus 化与告警接线随 T17 负载验证落地（阈值先按 §6.5）。
- 独立实例（非共享）部署复用本 compose 模式 + `open_connector.runtime` 新内部 ID。
- 发布门禁与灰度/回退序见 [open-connector-release.md](./open-connector-release.md)（T18）：七类证据齐全 + 同 commit/镜像/命名空间校验通过才可放量。
- 一期已知限制（T18 终局记录）：过期授权 attempt 无 sweeper（T17 期末计数 2）；dispatcher 对 Provider 429 记固定 30s 冷却，不解析 Retry-After 头（opportunistic-unimplemented）；连接列表可见性（T17-F2）列规格积压。
