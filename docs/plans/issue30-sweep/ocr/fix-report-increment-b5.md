# OCR 增量修复批次 5 · 三轮 执行报告（issue30-sweep / B5）

- **执行者**：OCR 修复员-B5（修复执行员）
- **计划**：`docs/plans/issue30-sweep/plans/ocr-fix-increment-b5.md`（13 findings → 8 任务）
- **worktree**：`.worktrees/issue30-sweep`（分支 `codex/issue30-mobile-office`）
- **基线 HEAD**：`eaefabcff`（执行前）；**收尾 HEAD**：`6839023d9`
- **测试命令**：均在 worktree 根实跑（`go test` / `go build` / `go vet`，Apple silicon darwin）

## 结果总览

| # | 任务 | 发现 | Commit | 状态 |
|---|---|---|---|---|
| 1 | Confluence Cloud v2 创建 wire 契约 | R5-F5 (critical) | `f29b67783` | ✅ |
| 2 | HTTP 响应截断检测（跨模块） | R5-F6 + R5-F9 | `a8f817647` | ✅ |
| 3 | Confluence Query storage 归一对账 | R5-F7 | `2e4f6dcf5` | ✅ |
| 4 | GitLab MR target 维度与翻页 | R5-F8；F10 | `6b6b398c7` | ✅ |
| 5 | GitLab EnsureBranch 锚定与可证拒绝 | R5-F11 + R5-F13 | `42c933a88` | ✅ |
| 6 | agent 升级建议 reconcile 生命周期 | R5-F1 + R5-F2；F4 | `bd12bd056` | ✅ |
| 7 | marketplace 许可登记审计保真 | R5-F3 | `96688f9d2` | ✅ |
| 8 | ConfluencePublishService 收敛 ProviderProfile | F12 (lowWorth) | `6839023d9` | ✅ |

10 项主发现全部修复；3 项 lowWorth 全部有归宿（F4→Task 6、F10→Task 4、F12→Task 8）。

---

## Task 1：Confluence Cloud v2 创建 wire 契约（R5-F5）

**改动**：`internal/modules/appconnector/confluence_create.go:88-89` 两个 json tag `space_id`/`parent_id` → `spaceId`/`parentId`；顺带同步两处按 snake_case 解码的测试双打到官方契约（`publish/confluence_bridge_test.go`、Task 8 时发现的 `handler/app_connector_confluence_publish_e2e_test.go`，见差异记录）。

**RED**（工作区现场收编，`go test ./internal/modules/appconnector/ -run "TestConfluenceCreate" -count=1`）：

```
--- FAIL: TestConfluenceCreateCloudHappyPath (0.00s)
    confluence_create_test.go:418: create: state=failed err=confluence_provider_error: status=400
--- FAIL: TestConfluenceCreateCloudWikiContextPath (0.00s)
--- FAIL: TestConfluenceCreateUnknownOnLostCreateReply (0.00s)
--- FAIL: TestConfluenceCreateWireUsesOfficialCamelCaseKeys (0.00s)
FAIL	github.com/Tencent/WeKnora/internal/modules/appconnector	0.100s
```

**GREEN**：同命令 → `ok github.com/Tencent/WeKnora/internal/modules/appconnector 0.410s`；`go test ./internal/modules/appconnector/...` 全 ok（publish 子包双打同步后）。

## Task 2：HTTP 响应截断检测（R5-F6 + R5-F9）

**改动**：
- `confluence_create.go`：新增 `maxConfluenceBodyBytes = 8 << 20`（注释互指 publish/plan.go 的 1MiB artifact → EscapeString ~5x 放大）；`confluenceDo` 读 `cap+1` 字节、超限返回 `ErrConfluenceOutcomeUnknown` 包装错误（含 method/path/cap），绝不返回截断字节。
- `gitlab_client.go`：`callRaw` cap 8MB → `maxRawBodyBytes = 16 << 20`（对齐 `maxBaselineBytes`，service.go 注释互指）；超限报 `ErrCodeTransport`；`Blob()` 返回前以 `GitBlobSHA`（与 CreateBlob 同一内容寻址算法）校验，不符报 `ErrCodeTransport`「content-addressed」错误。

**RED**（编译层——新常量未定义，属 TDD 正常形态；计划预期行为 FAIL ×4）：

```
internal/modules/codedelivery/gitlab_wire_test.go:519:16: undefined: maxRawBodyBytes
internal/modules/appconnector/confluence_create_test.go:608:16: undefined: maxConfluenceBodyBytes
FAIL ... [build failed]
```

**GREEN**：

```
--- PASS: TestGitLabRawRejectsBodyOverCap (0.01s)
--- PASS: TestGitLabBlobVerifiesContentAddressedSHA (0.00s)
--- PASS: TestConfluenceDoRejectsBodyOverCap (0.01s)
--- PASS: TestConfluenceDoAcceptsMultiMegabyteStorageEcho (0.02s)   # ~3MiB 合法回显不误伤
ok github.com/Tencent/WeKnora/internal/modules/codedelivery	1.425s
ok github.com/Tencent/WeKnora/internal/modules/appconnector	0.418s
```

回归 `go test ./internal/modules/codedelivery/ ./internal/modules/appconnector/...` → 全 ok。

## Task 3：Confluence Query storage 归一对账（R5-F7）

**改动**：`confluence_update.go` 新增 `normalizeStorageXHTML`（`html.UnescapeString` + CRLF→LF + `TrimSpace`，双侧同施）；:287 比较改为归一后比较（title 同步 TrimSpace）；版本号先决检查不动。fake 的 `reserialize` 旋钮补建模服务端 LF 归一。

**RED**（与计划预期逐字一致：第一项 FAIL、第二项 PASS 钉住基线语义）：

```
--- FAIL: TestConfluenceQueryConvergesAfterServerEntityNormalization (0.00s)
    confluence_update_test.go:403: entity/whitespace-only reserialization must converge, got state=unknown err=confluence_query_unverifiable: remote content drifted from the approved snapshot
--- PASS: TestConfluenceQueryStillRejectsRealContentDrift (0.00s)
```

**GREEN**：两测试 PASS；`go test ./internal/modules/appconnector/ -run 'Confluence'` 与 `./internal/modules/appconnector/...` 全 ok。

## Task 4：GitLab MR target 维度与翻页（R5-F8；顺手 F10）

**改动**：
- `github.go:92` 接口签名 `PullRequestForHead(ctx, head, base string)`；GitHub 客户端适配（head 在 GitHub 唯一，base 参数 identity-neutral，行为不变）；GitLab 实现：`target_branch` 服务端过滤 + 响应字段本地双保险比对 + `per_page=100` 翻页（短页终止，上限 50 对齐 Tree 先例）；`DraftPullRequest` 两处调用点传 `input.Base`。
- `dispatcher.go` QueryProvider：经 `client.Repository()` 解析默认分支为 base（与派发半程 PR leg 同源取值）。
- F10：`call()` 2xx 解码失败 `ErrCodeRequestInvalid` → `ErrCodeTransport`（选型与理由记入 commit message：2xx 非法 JSON 是「已到达响应的不可信交换」，后者是「从未出网」，语义最近族）。
- emulator 扩展：MR 列表响应携带 `target_branch`、支持 `target_branch` 过滤与 `per_page/page` 分页（默认 20 对齐真实 GitLab）；新增 `addMR`/`deleteBranch`。

**RED**：编译层（签名未变，`too many arguments in call to client.PullRequestForHead`）。**行为 RED 补验**：临时恢复旧语义（去掉过滤）跑 dispatcher 级测试：

```
--- FAIL: TestQueryProviderIgnoresForeignTargetMR (0.01s)
    Error: Expected error with "dispatch_unknown" in chain but got nil.   # 旧语义下无关 MR 误判 delivered
```

恢复修复后四测试全 PASS；`go test ./internal/modules/codedelivery/` 全 ok（含 GitHub 链既有用例）。

## Task 5：GitLab EnsureBranch 基线锚定与可证拒绝（R5-F11 + R5-F13）

**改动**：
- 锚定：新分支 `startRef = c.stagedBase`（实读确认 `stagedBase` 即 BaselineSHA 形态——dispatcher `CommitTree(material.BaselineSHA)` → GitLab 侧恒等映射），`start_branch` 传基线 ref（SHA）；actions = baseline↔intended 差集（仅 staged 变更）；新分支路径 Tree 拉取合并为一次；已有分支路径（head 锚定）不动；无用的 `baseBranch()` 删除（EnsureBranch 不再依赖默认分支）。
- 可证拒绝：空收敛+分支不存在（原裸 `ErrInvalidMaterial`）、超 cap（链上保留 `ErrBaselineTooLarge`）、Tree 翻页超限（`localCapError` 判定 Status==0 合成错误），统一包 `appconnectorsvc.ErrDispatchNotStarted` → settle 落 `ActionFailed`（契约 action.go:576-582 未放宽）。

**RED ×3**（行为级）：

```
--- FAIL: TestEnsureBranchAnchorsNewBranchAtBaseline      # start_branch 是默认分支名而非基线 ref
--- FAIL: TestEnsureBranchEmptyConvergenceIsDispatchNotStarted  # 错误链无 ErrDispatchNotStarted
--- FAIL: TestDispatchSettlesFailedOnEnsureBranchLocalRejection  # settle 落 unknown（resolve 零错误）
```

**GREEN**：三测试 PASS；`go test ./internal/modules/codedelivery/` 全 ok。settle 级测试置于 `service_gitlab_test.go`（GitLab 链 fixture 就近；计划写 service_dispatch_test.go，属文件宿主就近偏差）。

## Task 6：agent 升级建议 reconcile 生命周期（R5-F1 + R5-F2；顺手 F4）

**改动**（`internal/application/service/agent_upgrade.go`）：
- F1：`reconcileProposals` 前置 `ListProposals` 拉全，建 `adoptionID+"\x00"+toReleaseID` 索引；已物化组合（任意状态）在两次 GetRelease + 两份 Bundle 解码之前 `continue` 短路。
- F2 上半：循环内对该 adoption 的 open 行查 `row.FromReleaseID != adoption.AcceptedReleaseID` → `TransitionProposal(open → dismissed, resolved_by=AgentUpgradeSystemResolvedBy)`，系统标记 `"system:adoption-advanced"`（varchar(255) 内）。
- F2 下半：`AcceptUpgradeProposal` 在 adoption active 校验后追加 from_release 校验 → `ErrAgentUpgradeStateConflict`。
- F4：移除死注入 `now func() time.Time` 与 `time` import；构造签名不变（`go build ./...` exit 0 为编译级证明，container 调用零改动）。

**RED**：编译层（`AgentUpgradeSystemResolvedBy` 未定义）→ 实现后三测试 PASS。计数断言主项达成：第二次 List 的 `releaseReads == 0`（listing 读保留每 active adoption 一次——当前 release 指针只能从 listing 行获知，跳过它在逻辑上不可实现；计划「或 GetMarketplaceListing 仅被未物化组合触达」的弱变体不可实现，主断言达成）。

**回归**：`go test ./internal/application/service/` ok（110.8s）；`go build ./...` exit 0；`go test ./internal/application/service/ ./internal/application/repository/` → service ok、repository 仅 3 项预存在失败（见终局验收）。

## Task 7：marketplace 许可登记审计保真（R5-F3）

**改动**：`agent_marketplace_lineage.go` `UpsertLicense`：`DoUpdates` 去除 `"created_by"`（保留 name/allows_redistribution/updated_at）；成功后 `return r.GetLicense(ctx, license.ID)` 重读库中行。Create 前设输入时间戳逻辑保留（insert 路径需要）。

**RED**：

```
--- FAIL: TestUpsertLicensePreservesFirstRegistrar (0.00s)
    Error: Not equal: expected: "first-admin"  actual  : "second-admin"
```

**GREEN**：该测试 PASS；`-run 'License|Lineage'`（repository）与 service 侧 License/Marketplace 面 ok。

## Task 8：ConfluencePublishService 收敛 ProviderProfile（R5-F12）

**改动**：
- `provider.go` 新增 `ConfluenceProfile(remote ConfluenceRemoteReader)`：storage XHTML 经 `BlocksOf` 单元素 RawMessage 携带（仅 profile 的 Create/UpdateArgs 解包为 `"storage"` 字段——快照字节与收敛前逐字一致，digest 兼容）；`ReadRemoteVersion` 包装空版本串为错误（保住 confluence 两侧 fail-closed 的预读语义——共享体对 create 容忍空基线是 feishu 文件夹语义）；`ParseReceipt`/`ConflictResultPrefix`/`ActionVersion`/`Provider` 对齐。
- `confluence.go`（234 行 → 66 行）：服务体五方法全部删除；`ConfluencePublishService = NotionPublishService` 别名（handler/container/测试调用面零改动）；`confluenceScopeAdapter` 把 ConfluenceScopeSource 适配到共享 NotionScopeSource 端口（计划面只消费 AppID/AuthVersion/ApprovedParents 三字段，完全重叠——无需扩展 ProviderProfile，共享体零 provider 分支）。
- `confluence_test.go` 一处 `svc.scopes` 直接赋值改经适配器包装（测试意图不变）。
- **顺带发现并修复**：handler e2e 自带 wire 双打仍按 snake_case 解码（Task 1 只同步了 appconnector 与 publish 包的双打），R5-F5 camelCase 修复在其面暴露为 400——已同步官方契约（并入本 commit，属 R5-F5 掩盖面残留）。

**RED**（守护测试）：`TestConfluencePublishServiceIsProviderProfileThin` FAIL（源断言：`func (s *ConfluencePublishService) FormPlan` 在位）。**GREEN**：该测试 PASS；`go test ./internal/modules/appconnector/publish/` ok（既有 confluence 套件即行为回归网）；`go build ./...` exit 0；`go test ./internal/modules/appconnector/... ./internal/handler/ -run 'Publish|Confluence'` 全 ok。

---

## 终局验收（全批完成后，实跑输出）

1. `go build ./...` → **exit 0**，仅 `ld: warning: ignoring duplicate libraries: '-lc++'`（cmd/server、cmd/desktop）——与计划 Expected 一致。
2. `go test ./internal/modules/appconnector/... ./internal/modules/codedelivery/` → **8 包全 ok**（基线 4 项 TestConfluenceCreate RED 已转绿）：
   ```
   ok .../appconnector  1.229s
   ok .../appconnector/connectorcontrol  1.911s
   ok .../appconnector/openconnector  1.295s
   ok .../appconnector/plan  1.661s
   ok .../appconnector/publish  1.725s
   ok .../appconnector/repository/appconnector  3.157s
   ok .../appconnector/service/appconnector  2.895s
   ok .../codedelivery  1.278s
   ```
3. `go test ./internal/application/...` → **仅 `internal/application/repository` FAIL**，失败集合为差异记录 7 的 3 项预存在 delivery e2e（未新增、未修复——域外）：
   ```
   --- FAIL: TestDeliveryCollaborationEndToEndAC1PersonalLoopAndAttribution (0.58s)
   --- FAIL: TestDeliveryCollaborationEndToEndAC2CollaboratorCannotInheritPersonalConnection (1.52s)
   --- FAIL: TestDeliveryCollaborationEndToEndCrossTenantIsolated (2.62s)
   ok  github.com/Tencent/WeKnora/internal/application/service  309.870s
   ok  github.com/Tencent/WeKnora/internal/application/service/file  1.476s
   ```
4. `go vet ./internal/modules/appconnector/... ./internal/modules/codedelivery/ ./internal/application/...` → **exit 0，无输出**（无新增告警）。

**发现对账**：F5→T1、F6+F9→T2、F7→T3、F8+F10→T4、F11+F13→T5、F1+F2+F4→T6、F3→T7、F12→T8——10 项主发现 + 3 项 lowWorth 全覆盖，均有失败测试钉住。

## 差异与如实声明

1. **RED 形态偏差（Task 2/4/6）**：引用新常量/新签名的测试在实现前呈编译失败（`undefined: maxRawBodyBytes` 等）而非计划预期的行为 FAIL——TDD 正常形态；Task 4/5 另以「临时恢复旧语义」的方式补充拿到了行为级 RED 证据（输出已贴）。
2. **Task 1 双打掩盖面的第三处残留**：Task 1 同步了 appconnector 根包与 publish 桥接双打；handler e2e 的独立 wire 双打副本在 Task 8 的回归面才首次跑到（其 400 失败暴露了它），已随 Task 8 修复并注明归属 R5-F5。
3. **Task 5 start_branch 传 SHA 未对真实 GitLab 实测**：本环境无外网/无真实实例；选型依据为 GitLab commits API 的 ref 解析语义（branch/tag/commit SHA 皆可）+ emulator 以同语义解析（`resolveRef` 同实现）。已记入 commit message；如生产出现 SHA 形态不被接受的情况，需回退为基线分支名锚定。
4. **Task 6 listing 读保留**：计划 Produces 的弱变体（「GetMarketplaceListing 仅被未物化组合触达」）逻辑上不可实现——toReleaseID 只能从 listing 行获知；主断言（GetRelease/diff 重算为 0）达成，昂贵面（两次 release 读 + 双 Bundle 解码）已消除。
5. **Task 6 repository 回归耗时**：`internal/application/repository` 全量 252-412s（与基线记录一致）。
6. **Mimosa 安全扫描**：每次 commit 时 hook 报 `scanner_enobufs`（未得到完整扫描结论，按兼容策略放行）。本报告不宣称项目安全；完整审计需另行重跑。
7. **计划勾选框**：计划文件全部 Step 与终局验收项已勾选（45 处）。

## 提交清单（8 个 fix/refactor commit）

```
f29b67783 fix(confluence): use official camelCase keys on the Cloud v2 create wire (R5-F5)
a8f817647 fix(http): detect cap-exceeding responses and verify blob content addressing ... (R5-F6, R5-F9)
2e4f6dcf5 fix(confluence): normalize server-reserialized storage before query comparison (R5-F7)
6b6b398c7 fix(gitlab): resolve MRs by source+target with pagination, classify 2xx decode failures ... (R5-F8; F10)
42c933a88 fix(gitlab): anchor new task branches at the baseline and settle local rejections ... (R5-F11, R5-F13)
bd12bd056 fix(agent-upgrade): short-circuit materialized pairs, dismiss stale opens, guard accept (R5-F1, R5-F2; F4)
96688f9d2 fix(marketplace): preserve the first license registrar and return the stored row on upsert (R5-F3)
6839023d9 refactor(publish): collapse ConfluencePublishService onto the provider profile (R5-F12)
```
