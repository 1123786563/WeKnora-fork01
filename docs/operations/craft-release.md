# Craft 发布能力门禁、演练与回退手册（O05）

本手册是 Craft 程序（R/W/C/D/O 共 26 项任务）的**可审阅发布包**：发布能力门禁的机器判据、发布开关的推荐顺序、故障演练索引、以及停止准入与回退的操作命令。**部署实施是后续的明确动作**——本文件与 `docs/testing/craft/release-evidence.json` 一起构成发布评审材料，评审通过不等于自动向生产发布。

## 1. 发布能力门禁（机器判据）

门禁由 `scripts/check-craft-release.py` 执行，证据为 `docs/testing/craft/release-evidence.json`：

```bash
python3 scripts/check-craft-release.py docs/testing/craft/release-evidence.json
```

判据（缺一即 exit 1）：

1. **head 必须等于待发布 commit**（默认取本仓 `git rev-parse HEAD`，可用 `--commit` 指定）。证据文件里的 head 是证据生成时所在 commit；release commit 变化后必须**重新生成证据**（重跑演练与回归），绝不手改 head 字段。
2. **26 项任务依赖 SHA 齐全**，且每个 SHA 都是待发布 commit 的祖先（`git merge-base --is-ancestor`）。
3. **必需场景全部 passed 且证据文件真实存在**：每个场景的证据文件必须含正向退出码标记（`EXIT=0`/`SPEC_EXIT=0`）与按类型要求的通过标记（go 测试 `--- PASS`；浏览器 `N passed/skipped`），且不含失败标记（`EXIT=<非0>`、`--- FAIL`、`N failed`）。手写摘要、not_run、空文件一律不过。
4. **默认能力只发布已测 PostgreSQL+Docker 组合**；SQLite 为独立 capability 条目（须自带证据方可 enabled）；Cube/E2B 沙箱为独立条目，无 craft 专用证据时只能 disabled/waiting（保守等待）。
5. **已知外部缺口必须留在 limitations**：G4 商业库缺口（commercial_reservations 缺 owner 列 + 4 张商业表仅 AutoMigrate）、O02 受控模型网关未接线（用量账本空）、/metrics 端点未暴露。门禁禁止它们被悄悄挪进 capabilities。
6. **付费发布必须 Billing=true 且 commercial capability enabled**——当前两者都不成立，`CanRelease(f, paid=true)` 拒绝；受控发布（controlled）为默认姿态。

门禁自身的反向验证（篡改副本必须全部被拒）记录于 `docs/testing/craft/o05/release-gate-verification.txt`：wrong-head / 丢依赖 / not_run / 证据文件缺失 / 无证据开 cube / 无 Billing 开付费 / 隐藏 limitations 七种篡改各得 exit 1。

## 2. 发布开关顺序（推荐）

总原则：**默认全关 → 小流量真实使用 → 逐能力开闸 → 商业门禁通过前收费永不开放**。每一步之间留观察期（建议 ≥1 个工作日 + 一次 sweep 周期），期间用排障手册（`docs/operations/craft-observability.md`）三路径巡检。

| 阶段 | 开关 | 说明 |
|---|---|---|
| 0 默认全关 | `WEKNORA_CRAFT_ENABLED` 不设或非 true | 路由不挂载、工具不注册，builtin 行为零变化（fail-closed 已被默认矩阵证明：document 4 skipped、spreadsheet/slides 04 走 503）|
| 1 内部 tenant 网页 | `WEKNORA_CRAFT_ENABLED=true`，`WEKNORA_CRAFT_KINDS=web`（缺省即 web） | 仅 web 报告类；对内部租户开放创建/执行/版本/预览 |
| 2 限定 tenant 协作恢复 | 保持阶段 1，向选定 tenant 演示恢复能力 | 快照恢复/崩溃接管已在演练证明（真 SIGKILL 后持久快照恢复、OC session 原子续历史）；观察 `craft_workspace_restore_seconds` 与恢复工单反馈 |
| 3 逐 kind 开启 | `WEKNORA_CRAFT_KINDS=web,document` → `+spreadsheet` → `+slides` | 每个 kind 的四门禁（生成/修改/预览/导出）证据链在其任务报告与全开矩阵；一次只加一个 kind，回归该 kind 的 4 条浏览器用例 |
| 4 收费开放 | **前置**：G4 侧补齐商业库迁移 + O02 网关接线 + usage 账本有真实物理调用 | 重跑 `check-craft-release.py` 且 commercial capability enabled、Billing=true 之后才可评估；在此之前 `WEKNORA_CRAFT_*` 不引入任何计费开关 |

kind 封闭集：`web/document/spreadsheet/slides`（`craft.KnownKind`）；集外 kind 一律 400/503 拒绝（D01 04 用例证明）。**一个合法 kind ≠ 一个已开 kind**。

## 3. 回退（先停新准入，绝不破坏在跑）

回退顺序与红线（与 Step 6 一致）：

1. **停止新准入**：`WEKNORA_CRAFT_ENABLED=false`（或不设）。新会话创建、新 Run 准入、新委派派发全部关闭（O03 GuardDispatch 以持久 deleting 标记为权威，lifecycle 锁不是权威）。
2. **保留运行中的 worker 与旧镜像**，让已准入的 Run 走完或等到 deadline：恢复程序只观察/落库/等待（C04：无任何自动重发分支）；到 deadline 仍未证明的委派按既有语义 park，不制造失败。
3. **等待存量收敛**：`craft_pending_decisions` 归零、无 `result_json IS NULL` 的委派、sweep 空转（`craft_lifecycle_states` 无 deleting 残留）后，才进入清理。
4. **版本不兼容时暂停并给出恢复选项**：runtime digest 变更使观察/恢复不可比（C04/C05 语义）——此时**不要**删旧 OC 镜像；按快照（若该版本有完整快照）恢复，或等用户显式放弃会话。预览旧版本永远可用（不可变版本 + 授权下载路由不依赖 runtime）。

红线（回退时禁止）：

- **不删表**：craft_* 表全部保留；需要清理时走 O03 lifecycle sweep 的四保护路径（active run/未知结果/pending decision/generation 保护），绝不对表做 DROP/TRUNCATE。
- **不删旧 OC 镜像**：旧版本产物与潜在恢复都依赖锁定 digest（`docker/craft/*.lock`）；镜像清理仅在「无任何引用该 digest 的版本且无待恢复会话」并经人工复核后进行。
- **不把 active 会话换 builtin**：会话引擎一经 tRPC+craft 绑定，回退靠停止新准入而非改引擎；改 `sessions.engine_type` 会使 craft 工具消失但历史 Run 语义断裂，禁止。

## 4. 配置清单

### 4.1 功能开关（服务端，env）

| 变量 | 语义 | 建议值 |
|---|---|---|
| `WEKNORA_CRAFT_ENABLED` | craft 总开关（true/1/yes 才开） | 灰度阶段 true；回退第一步置 false |
| `WEKNORA_CRAFT_KINDS` | 开启的 kind 封闭子集（csv） | 按第 2 节阶段推进；缺省=web |
| `WEKNORA_CRAFT_PREVIEW_ORIGIN` | 预览隔离 origin（票据签发与 CORP/COOP 语义） | `https://craft-preview.<主域>` |
| `CRAFT_LIFECYCLE_SWEEP_DISABLED` | 置 true 关闭 O03 sweep（默认开） | 保持默认开 |

### 4.2 运行时（本地单 serve 部署）

| 变量 | 语义 |
|---|---|
| `CRAFT_OPENCODE_BASE_URL` | 锁定 opencode serve 地址；**缺省= fail-closed executor**（委派一律明确 unsupported 失败，绝不伪造成功）|
| `CRAFT_OPENCODE_RUNTIME_DIGEST` | 运行时摘要；空=不可证明=恢复不兼容（C04/C05 语义）|
| `CRAFT_OPENCODE_WORK_DIR` / `CRAFT_OPENCODE_OUTPUT_DIR` | serve 工作目录 / 产物输出目录（持久卷上）|

锁定二进制：opencode 1.18.4，sha256 `9449af91…398`（`internal/agent/opencode/testdata/protocol-lock.json`、`docker/craft/opencode.lock.json`）。升级=新 digest + 全量演练重跑（见第 10 节）。

## 5. 预览域名与 TLS

- 预览走**独立隔离 origin**（`WEKNORA_CRAFT_PREVIEW_ORIGIN`），票据短时效、单版本绑定、路径穿越拒绝、跨租户 404（C03；O05 演练 drill-10 复证）。
- TLS：主站与预览站均需有效证书；自签仅限验收栈（craft-stack.sh 的 nginx + self-signed 即此用途）。生产建议同主域通配符或独立证书 + HSTS，与主站同级别管理。
- web kind 的预览是 iframe 票据面；document/spreadsheet/slides 是**组件面**（服务端转换数据 + 不可变文件授权路由），不进 iframe——预览票据只为 web 面签发（D01 归一后语义）。

## 6. 持久卷

最少三处必须落在持久卷：

1. **业务数据库**（PostgreSQL 数据目录 / SQLite db 文件所在卷）——craft_workspaces/versions/snapshots/delegations 全在此。
2. **受控存储**（`storage` 卷：版本对象、快照对象，内容寻址 sha256）。
3. **opencode serve 数据与工作目录**（XDG data + `CRAFT_OPENCODE_WORK_DIR`/`OUTPUT_DIR`）——OC 会话链的持久层；快照恢复要求会话在 serve 持久数据中仍存在（会话真丢失=按设计 ErrUnsupported，fail-closed 不伪造）。

## 7. 备份与恢复

- **DB**：PostgreSQL 常规 pg_dump/PITR；恢复后 craft_* 表随库一致。
- **受控存储**：对象为内容寻址且带 sha256，可直接随存储卷快照/异地复制；恢复时 C05 会逐对象重算摘要校验。
- **快照即应用层恢复点**：每轮发布后 best-effort 捕获（静默门：无 pending 委派 + OC idle）；人工恢复入口 `POST /api/v1/sessions/:session_id/craft/restore`（body: request_id/snapshot_id/revision；幂等键 + revision CAS，败者 generation 自动清理，旧绑定不动）。
- 演练基线：真 SIGKILL serve → 同数据目录重启 → 恢复成功（文件逐字节、OC session 不变、旧版本不变）→ 第二轮真实执行无重复 prompt（`docs/testing/craft/o05/drill-02-oc-crash-snapshot-restore.txt`）。
- **备份频率建议**：DB 每日全量+PITR；存储卷随 DB 同窗口；两处必须同窗口（版本对象的 ref 存 DB、内容在存储，跨窗口会导致恢复校验失败——C05 会诚实拒绝而非半恢复）。

## 8. 模型凭据轮换

- 主模型与子模型凭据都在租户模型配置（`models` 表/受控配置）中，**绝不进** craft 日志/指标/诊断导出（O04 测试断言 payload 无 credential/secret/api_key）。
- 轮换流程：新增凭据 → 灰度新凭据 → 观察一个 sweep 周期 → 撤销旧凭据。委派在跑时轮换：已在跑的 OC 子执行用旧凭据走完（不影响结论落库）；新委派取新凭据。
- O02 网关接线（后续动作）后，物理调用统一过网关落账，轮换收敛到网关侧一处。

## 9. 保留策略

- **版本**：不可变、不清理（用户显式删除会话走 O03 三路删除 + tombstone + sweep；版本随会话级联）。
- **快照**：随工作区（FK CASCADE）；每版本至多一份内容恒等快照（幂等 id）。空间紧张时优先清「无对应版本可恢复」的孤儿对象（受控存储内容寻址去重后实际增量很小）。
- **OC 会话数据**：serve 持久层自管理；craft 侧不主动清（快照恢复依赖它）。
- **日志**：结构化日志含五类 ID + prompt 哈希（不可逆），按平台日志保留策略；指标为低基数封闭词表，可长期留存。
- **恢复点选择 UI 只列可恢复版本**（无完整快照/执行中/未选版本时按钮禁用并给原因）。

## 10. 数据库升级顺序

1. **先备份**（第 7 节窗口）。
2. `WEKNORA_CRAFT_ENABLED=false` 停新准入（回退第 1 步语义）。
3. 等 Run 收敛（回退第 3 步判据）。
4. 应用迁移（`scripts/migrate.sh` / 部署管线）：craft 迁移链 PG 000125–000132、SQLite 000045–000052（2026-09-13 重编号：main 先合入的 open-connector 占用 000121–000124/000041–000044，craft 侧整体 +4 让路，内容与 down 逆序不变），全部带 down 逆序。
5. 起新版本服务，`WEKNORA_CRAFT_ENABLED` 按原阶段恢复。
6. 重跑门禁：`python3 scripts/check-craft-release.py …`（head 已变 → 重新生成证据）+ 抽一条浏览器 spec 冒烟。

**runtime（opencode/镜像）升级**同构：新 digest 先在演练环境跑 drill-01/02/05 全套（真 SIGKILL 矩阵 + 快照恢复），再灰度切换 `CRAFT_OPENCODE_RUNTIME_DIGEST`；旧镜像保留至「无引用版本且无待恢复会话」。

## 11. 停止准入与回退命令（速查）

```bash
# 1) 停止新准入（滚动重启所有实例使 env 生效）
WEKNORA_CRAFT_ENABLED=false

# 2) 观察存量收敛（O04 排障三路径）
#    - craft_lifecycle_states 无 deleting 残留（sweep 空转）
#    - craft_pending_decisions 归零
#    - 无 craft_delegations.result_json IS NULL 的行（或已 park 等待人工决断）

# 3) 回退到阶段 0（全关）
systemctl restart weknora-server   # 或部署管线的等价动作

# 4) 发布门禁复验（head 变化则先重生成证据）
python3 scripts/check-craft-release.py docs/testing/craft/release-evidence.json
```

**绝不执行**：`DROP/TRUNCATE craft_*`；`docker rmi` 旧 craft 运行时镜像（未复核引用前）；`UPDATE sessions SET engine_type='builtin'`（对 active craft 会话）。

## 12. 故障演练索引（O05 实测）

| # | 场景 | 证据 |
|---|---|---|
| 1 | 主服务 SIGKILL（四 barrier） | `docs/testing/craft/o05/drill-01-main-service-sigkill.txt` |
| 2 | OC 崩溃 + 持久快照恢复（真 SIGKILL） | `docs/testing/craft/o05/drill-02-oc-crash-snapshot-restore.txt` |
| 3 | sandbox 删除 / lifecycle 回收 | `docs/testing/craft/o05/drill-03-sandbox-deletion-lifecycle.txt` |
| 4 | 存储失败零发布 | `docs/testing/craft/o05/drill-04-storage-failure.txt` |
| 5 | 两 worker 抢占（SQLite / PostgreSQL） | `drill-05-…sqlite.txt` / `drill-05b-…postgresql.txt` |
| 6 | 浏览器断网重连 | `docs/testing/craft/o05/drill-06-browser-offline-reconnect.txt` |
| 7 | decision 竞争（杀死不重复投递） | `docs/testing/craft/o05/drill-07-decision-contention.txt` |
| 8 | usage 迟到修正 | `docs/testing/craft/o05/drill-08-late-usage.txt` |
| 9 | 预算不足 | `docs/testing/craft/o05/drill-09-budget-exhausted.txt` |
| 10 | 预览越权 | `docs/testing/craft/o05/drill-10-preview-privilege-escalation.txt` |

最终回归：legacy builtin / 后端包 / 共享 TS+web build / E2E 全开与默认矩阵见同目录 `regression-*.txt`；门禁验证与七项反向篡改见 `release-gate-verification.txt`。
