# SP2-b Connectors 治理下半（targeted reindex + 凭据动态续期）验收证据 — 2026-09-20/21

> Spec：`docs/specs/2026-09-19-sp2-connectors-governance-design.md` §5+§6。SP2-b 七实现任务 + Task 8 回归收官，SP2（SP2-a+b）整体完成。

## 1. 交付清单（提交号 + 收编说明）

| # | 提交 | 任务 | 内容 |
|---|------|------|------|
| 1 | `ef2ccb24`（收编） | Task 1 | 基建三合一：`SyncItemError.ExternalID`（spec §5.1，失败样本可定位→UI 单条重试）+ `DataSourceSyncPayload.Scope.ExternalIDs`（§5.3 scoped 载荷）+ ManualSync `force_full` handler 放开（§5.3）。净效果被并行会话大提交卷入，报告核对逐字节一致 |
| 2 | `848f19d2` | Task 2 | `TargetedFetcher` 接口（`connector.go`，FetchByExternalID）+ 高复用四连装：feishu wiki（obj_token）/drive（ListDriveFilesAllPages+filter，Ruling P-2 放弃三路径轮询）/notion（page id）/yuque（doc slug）——四实装全部复用既有构造路径零新写解析 |
| 3 | `d292e17e` | Task 3 | TargetedFetcher 五连装：gitlab（file path@ref）/rss（feed GUID 重拉）/confluence（page id，cloud/server 双 edition；title 从 KB metadata 恢复，Ruling P-3）/ima（降级：`ErrTargetedRefetchUnsupported` 前置条件错误——external_id 不可逆 hash，Ruling P-1）/dingtalk（知识库文档 id）。9 实现=11 type 全覆盖（lark/lark_drive 区域化复用） |
| 4 | `2d0e3dcb`（+`7e820411`/`aab41d5c` 收编） | Task 4 | scoped 重跑 service+API：`ReindexItems`（request_id 幂等→确定性 asynq TaskID，排队期重复 409）+ `runScopedReindex`（每项 FetchByExternalID→ingestItem，SubtreeKeep 契约）+ 独立 SyncLog 收敛（Ruling P-4 自写收敛，不复用 updateSyncRunResult——scoped 对 ds.Status/LastSyncResult/审计零写，豁免=两条源定义级 prologue 失败不触 LastSyncResult）+ `POST /datasource/:id/reindex`（202 异步） |
| 5 | `d86d244c` | Task 5 | 凭据存储协议+机器回写通道（§6.1/§6.2/§6.4）：`RefreshDataSourceCredential` 单 key 回写——三子句防覆盖守卫（parsed==nil / 无已配置凭据 / storedCredentialDecryptFailed 密钥轮换后密文不可读→拒写防空值覆盖）+写入集 {key, last_refreshed_at}+`credential_auto_refreshed` 独立审计；`expires_at` 宽松解析（缺失/坏值=长效）；credentials 子资源字段级元数据（expires_at/last_refreshed_at/needs_reauthorization，omitempty 缺省即 false） |
| 6 | `56ad6032` | Task 6 | 续期触发协议（§6.3/§6.4）：`CredentialsRefresher` 可选接口 + 同步启动临期检查（<5min，位于 scoped 分支后——凭据轮换不在 scoped run 发生，OAuth refresh 单次性+双写收敛双向隔离）+ AuthVersion 递增→旧 cursor 失效→下次自动全量对账 + 过期未续成功→SyncPausedError(permission) 语义暂停 + mock OAuth 验收载体（测试装配注册，构造性假 token） |
| 7 | `d63c2881`（+`de6ddd2d` 收编） | Task 7 | React 失败样本渲染+重试：api-client `reindexItems`/`DataSourceSyncLog.result.errors`（ExternalID）+ DataSourcesPage 日志抽屉失败项渲染（`datasource.syncError.<code>` 本地化+title—reason）+失败项复选框+「重试选中项」+duplicate 409 toast + i18n `dataSource.syncError.*` 连接器码（React 侧 generated/dataSource.ts）。api-client 两文件+i18n 测试被并行 `de6ddd2d` 提前收编，核对逐字节一致 |
| 8 | `1f693ea4`（收编）+ 本笔 | Task 8 | 回归+台账收官：Vue 五语言 `datasource.syncError` 连接器码 18×5（feishu 6+confluence 6+dingtalk 2+targeted_unsupported/not_found/fetch_failed+sync_failed 兜底码；zh 中文/en 英文/ja/ko/ru 英文同 React 基线，值照 React `dataSource.syncError.*` 逐字节抄写）+ spec 勘误两处（§6.2 守卫三子句+写入集 {key,last_refreshed_at}；§5.3 scoped 触发器位置补注）——**locale 五文件与 spec 已被并行会话 sweep 提交 `1f693ea4` 收编（含其自身 FAQ/message-list 改动），本任务核对提交版本与本地最终版一致**；本笔提交 evidence/ROADMAP/GAP-MATRIX |

净效果 8+6 提交（含 4 笔并行收编）。收编均经"git diff HEAD 为空 / 提交内容逐字节核对"确认无分叉。

## 2. 回归结果（Task 8）

### 2.1 后端

- `go build ./internal/... ./cmd/...`：**通过**（仅预存 ld 重复库警告 `-lc++`）。
- 本线涉及包显式复跑全绿：`go test ./internal/datasource/...`（12 包：core+wiki/drive/gitlab/ima/rss/confluence/dingtalk/notion/yuque/moauth+根）+ `./internal/application/service`（72s）+ `./internal/handler/...` + `./internal/application/repository`（120s）+ `./internal/types` + `./internal/router` 全部 **ok**。
- **全量 `go test ./...`：零失败**（预存失败清单中的 Go 侧——execution registration 族 SQLite 迁移缺列——本轮全量未复现失败；判归属项全部落在前端 pnpm 套件，见 2.2）。

### 2.2 前端（本线触达=Vue 五 locale 文件）

| 套件 | 结果 |
|---|---|
| `cd frontend && npm run check-i18n`（localeKeyAudit 11 子测：五 locale 键齐/编译/引用闭包） | **11/11 通过** |
| `npx tsx --test src/i18n/locales/workspaceTerminology.test.ts` | 1/1 通过 |
| `npm run type-check`（vue-tsc --build） | 通过（0 错误） |
| pnpm web 全量预存失败（KnowledgeDocumentDetailPage 38 败 createPortal / AppsPages 1 败 / documents 域） | **预存判归属**：本任务未触达任何 React 组件文件（Task 7 已在其线内复跑 data-sources/api-client/i18n 绿）；本轮不重复执行 |

## 3. 冒烟（真实运行栈，非欠账——已完成）

**环境**：共享 dev 栈 8084 后端二进制为 `412c2192`（2026-09-20 22:51 构建，**早于 SP2-b 全部连接器提交**——首个 TargetedFetcher 提交 848f19d2 为 23:01），且并行会话正活跃使用（vite 5174/5175 代理其上），重启会打断并行工作，未动。照 SP13/SP14 配方起**专用栈**：`go run ./cmd/server` 于 **8085**（当前 HEAD 代码；同 postgres/redis/docreader 容器 127.0.0.1；`REDIS_DB=2` 隔离 asynq；`SSRF_WHITELIST=127.0.0.1,192.168.3.30`；`LOCAL_STORAGE_BASE_DIR` 本机可写目录）。冒烟账号 `sp2b-smoke@local.dev`（tenant 10042 owner，保留于 dev 库）。

### 3.1 scoped reindex 一轮 ✅（API 全链）

- 造数：KB `sp2b-smoke-kb` + RSS 数据源 `sp2b-rss-smoke`（feed_urls 指向本机 `127.0.0.1:8899/feed.xml`，2 条目 GUID smoke-item-001/002）。
- 整源同步 `POST /:id/sync` → success，result `{total:2, created:2, failed:0}`。
- **reindex 一轮**：`POST /api/v1/datasource/:id/reindex` body `{"external_ids":["http://127.0.0.1:8899/feed.xml:smoke-item-001"],"request_id":"sp2b-smoke-req-1"}` → **202** `{sync_log_id}`；scoped run → **success，result `{total:1, updated:1, failed:0}`**（单条重抓+SubtreeKeep 复用 ingestItem）。
- **双写收敛实证**：scoped run 后 `GET /datasource/:id` 的 `last_sync_result` 仍为整源结果 `{total:2, created:2}`——scoped 未污染 ds（Ruling P-4 live 验证）；sync_logs 表三行独立（整源 1 + scoped 2）。
- request_id 复用：首跑**已完成**后同 request_id 再发 → 新 202 新 run（设计即排队期才 409，asynq TaskID 冲突窗口内拒重）。

### 3.2 凭据元数据端点 ✅（API）

- `PUT /:id/credentials`（future `expires_at` 2026-09-28）→ `fields.credentials = {configured:true, expires_at:"2026-09-28T00:00:00Z"}`（needs_reauthorization 缺省省略=false）。
- `PUT`（past `expires_at` 2026-09-19）→ `{configured:true, expires_at, **needs_reauthorization:true**}`——临期/过期→UI 提示重授权语义成立。
- `GET /datasource/:id` 携带同一 `credentials` 元数据块（值不出端，只有元数据）。
- 机器回写通道（RefreshDataSourceCredential 守卫/审计/AuthVersion 递增/旧 cursor 失效）无 API 面——由 Task 5/6 mock OAuth 验收测试覆盖（`datasource_service` 包内），冒烟不重复。

### 3.3 冒烟观察（记录，非缺陷）

- `sync_logs` 行/日志 DTO 不透出 trigger 字段（Trigger 在 payload 上，`types/datasource.go:575`）；scoped 与整源 run 在 UI 以独立日志行+逐项计数区分。
- 共享 8084 栈二进制落后（412c2192 < HEAD），SP2-b 端点在其上不可达——**栈重启欠账**（与 SP13/SP14 同款登记；本证据以专用 8085 栈等价覆盖，同一 DB/容器、同一工作树代码）。SP2-a 遗留 5 项端到端冒烟同欠账清单。

## 4. 遗留预填（下阶段输入）

- **真实 OAuth 连接器留 SP8**（spec §6.4）：mock OAuth 验收载体就位，SP8 接 Google Drive 时换真实 CredentialsRefresher 实现。
- **ima 降级前置条件**（Ruling P-1）：FetchByExternalID 返回"需先跑一次增量同步"前置条件错误（external_id 不可逆 hash，KB metadata 无 external_id 时单条重试不可用）——UI 呈现 `targeted_unsupported` 码。
- **confluence 单页 title 依赖 metadata**（Ruling P-3）：重抓 title 从 KB metadata 恢复，不做单页 GET API；metadata 缺失时 title 为空（可忍受）。
- **失败分类不对称**（Task 2 输出）：yuque 所有非凭据错误映射 ErrItemNotFound（含 429/5xx，cause 文本保留在 Message）；wiki 全 space 试探失败终错不含原因；drive walkErr 透传——重试标注失败时前端以 cause 文本区分瞬态。
- **rss 回环守卫死代码**（Task 3 minor）：守卫条件永真，无行为影响，待清理。
- **7 天窗口常量**（Task 5 minor）：`credentialReauthorizationWindow` 待前端消费时提升为命名配置。
- **scoped run 级失败审计 action 用 SyncCompleted 非 SyncFailed**（Task 4 minor，罕见路径）；豁免路径（prologue 双失败不触 LastSyncResult）无专属测试。
- **Vue/React ja/ko/ru 连接器码为英文值**（Task 7/8）：待统一翻译 pass。
- **React 面板日志无自动轮询 / retrySelected 无全局互斥**（Task 7 minor）：toast+一次性重载后收敛。

## 5. 收官状态

- **SP2 整体 ✅（SP2-a 2026-09-20 + SP2-b 2026-09-21）**：K-11/K-29（SP2-a）+ K-12/K-3（SP2-b）四差距行全部 ✅；ROADMAP SP2 行与变更记录、GAP-MATRIX K-3/K-12 行及 P1 清单已更新。
- 治理线剩余差距转移至：连接器广度（P2/SP7-9）、文档级权限（EE 三件套）、真实 OAuth（SP8）。
