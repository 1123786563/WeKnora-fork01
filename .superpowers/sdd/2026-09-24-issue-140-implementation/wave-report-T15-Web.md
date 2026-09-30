# T15 Web 子任务报告：材料编辑与 V1/V2 比较（Issue #153）

- BASE：`05122c146`（Wave 4 集成 HEAD，T15 后端已集成）
- HEAD：`11cb36553`（`feat(web): edit materials and compare immutable versions`）
- Worktree：`/Users/wuyongjun/.codex/worktrees/issue-140-t15-web/WeKnora-fork01`（独立、本地提交、未 push）
- 所有权：create `apps/web/src/career/MaterialPage.tsx` + `MaterialPage.test.tsx` + `material.css`；modify `packages/api-client/src/career.ts` + `career.test.ts` + `apps/web/src/career/ApplicationPage.tsx` + `ApplicationPage.test.tsx`（最小入口接线）。未改后端、未改 career-core 合同。

## 1. RED → GREEN 证据

### 1a. api-client materials 七方法（`packages/api-client/src/career.test.ts` 追加 3 个聚焦测试）

- RED：`node --import tsx --test packages/api-client/src/career.test.ts` → `ℹ pass 15 / fail 3 / cancelled 0`，3 个新测试全部 `TypeError: api.editMaterial is not a function`（方法尚不存在）。
- 实现：`career.ts` 新增冻结类型（MaterialKind/MaterialStatus/MaterialRiskCode/MaterialChangeKind 枚举、MaterialBody/Claim/Section/Pin/Risk/Receipt/View/VersionView/VersionList/Comparison/EditInput/ConfirmInput）+ 严格解码器（decodeMaterialReceipt/View/VersionList/VersionView/Comparison，共享 decodeMaterialPin/Body/Claim/Risks）+ `createCareerApi` 七方法：`editMaterial`、`confirmMaterial`、`materialReceipt`、`material`、`materialVersions`、`materialVersion`、`compareMaterialVersions`（路径 `encodeURIComponent`，compare 发 `…/versions/:target/compare?baseline=N`）。
- GREEN：同命令 → `ℹ tests 18 / pass 18 / fail 0 / cancelled 0`。

### 1b. MaterialPage（`apps/web/src/career/MaterialPage.test.tsx` 新建 10 个行为测试）

- RED：`node --import tsx --test apps/web/src/career/MaterialPage.test.tsx` → `ERR_MODULE_NOT_FOUND: MaterialPage.tsx`（组件尚不存在）。
- 实现：`MaterialPage.tsx`（结构化正文编辑：章节/主张/事实编号/待审阅勾选；pinned evidence 展示；审阅风险清单；审阅失败常驻警示；保存草稿/确认发布不可变版本；不可变版本列表 + 只读回看 + V(n-1)↔Vn 并排比较；未确认事实 409 保留草稿；未知结果原 requestId 回执查询/重试；revision 冲突显示当前修订；403 清除/404 明确错误；scope 切换清除）+ `material.css`（TDesign 浅色 token、`#07c05f` 确认发布态、比较双栏 grid、640px 堆叠、overflow-wrap anywhere）。
- 首轮 6/10（4 处为测试 fixture/CSS 选择器组修复：compare 目标正文参数化、只读测试补 URL 参数、conflict 后 head 更新、`.wk-material__compare` 独立块），修后 GREEN：`ℹ tests 10 / pass 10 / fail 0 / cancelled 0`。

### 1c. 入口接线（ApplicationPage → MaterialPage）

- RED：`node --import tsx --test apps/web/src/career/ApplicationPage.test.tsx` → 新接线测试 fail（14 既有 pass）。
- 实现：`ApplicationPage.tsx` 在申请回执存在时渲染 `MaterialPage`（key 含 opportunityId+snapshotId，传 `client/scopeController/opportunityId/snapshotId`）。
- GREEN：`ℹ tests 15 / pass 15 / fail 0`。
- career 全家回归：`node --import tsx --test apps/web/src/career/*.test.tsx packages/api-client/src/career.test.ts` → `ℹ tests 104 / pass 104 / fail 0 / cancelled 0`。

## 2. 全量验证（真实命令与输出）

- `pnpm typecheck:web`：修复 1 处自身测试类型（TS2345 optional factKey）后 **0 错误**（`TYPECHECK_PASS`；`grep -cE "error TS"` = 0）。
- `pnpm build:web`：`✓ built in 24.66s`（仅既有 chunk >500kB 告警）。
- `git diff --check`（对 BASE）：无输出（EXIT=0，无空白错误）。
- `pnpm test:web`（全量 node26，终验口径）：见 §2.1（跑前 `ps` 确认 0 孤儿 runner；输出 /tmp/t15-testweb-full.log）。

### 2.1 全量 test:web 结果

`pnpm test:web`（node26，全量，输出 /tmp/t15-testweb-full.log，全程约 3.5 分钟）：

```
ℹ tests 2400
ℹ pass 2400
ℹ fail 0
ℹ cancelled 0
ℹ skipped 0
ℹ todo 0
```

**全绿 0 cancelled**。基线 2389 + 本任务新增 11 个 web 测试（MaterialPage 10 + ApplicationPage 接线 1）= 2400。api-client 的 3 个 materials 聚焦测试属于 `test:shared` 口径，单跑 GREEN（§1a 18/18）。跑前 `ps` 确认孤儿 runner 数为 0。

## 3. 浏览器 E2E（T15 verified 关键验收，后端端口 57810）

- 环境：Lite SQLite（一次性 `/tmp/weknora-t15-browser.01lb2L` 库+本地存储+内存流；`AUTO_MIGRATE` 默认开，迁移 0→121 含 T15 三表），`127.0.0.1:57810`，`DISABLE_REGISTRATION=false`，随机 `JWT_SECRET`，`go build` 产物；未认证 `GET /api/v1/career/open` → 401。前端 vite dev `127.0.0.1:57813`（`VITE_DEV_PROXY_TARGET=http://127.0.0.1:57810`；57811 被其他会话占用故用 57813）。浏览器：仓库 `@playwright/test@1.63` 真实 Chromium headless。第一轮发现 server 必须以 worktree 为 CWD 启动（迁移文件按 CWD 加载；误从主仓库 CWD 启动会得到 7/19 表的旧迁移集并 FATAL，已换库重启）。
- 结果：**13/13 相位 OK**（register-A → 建档 → 导岗 → 评估 → 创建申请 → 材料编辑 → V1 → V2 → 比较 → 旧版只读 → 失败保留草稿 → reload 恢复 → 重复 heading 观测 → 跨租户拒绝）。截图 11 张 + 逐步 evidence.log 已归档至集成 worktree `.superpowers/sdd/2026-09-24-issue-140-implementation/t15-web-shots/`。

关键观测（摘自 evidence.log）：

1. 材料编辑器随申请回执出现（`material:editorAppearedFromApplication`）；保存草稿后 pinned evidence 四字段可见（岗位/快照/快照摘要 SHA-256/档案修订 1）。
2. 占位主张显示 `缺失/待补充（needs_review）：不得补造`；事实主张显示 `已链接确认事实：毕业时间`；界面无补造值。
3. 确认发布 V1（`POST /materials/confirm` 200）→ 版本列表 `不可变版本（1）V1`；修改正文再存再确认 → V2；`GET …/versions/2/compare?baseline=1` 200 → 双栏并排（pane1=V1 正文、pane2=V2 正文）+ 差异 `章节「教育经历」变更：计算机科学与技术本科，2026 年毕业 → …校奖学金两次`。
4. 只读查看 V1：`版本 V1（只读，不可修改）`，`readonly:editableFields=0`。
5. 失败路径：主张链接未确认事实 `学历`（档案只确认了 `毕业时间`）→ `POST /materials` **409** → UI `草稿保留未发布`，版本仍为 2（未发布 V3），本地草稿保留（`failure:localDraftKept=本科学历`），dirty 期间确认按钮 disabled。
6. reload 恢复：URL `…&material=1e3edf50-…` → 草稿正文与版本列表完整恢复；被拒主张未持久化（服务端保留的是上一份完好草稿，符合"失败不覆盖草稿"）。
7. 跨租户：账户 B（另一 Tenant）打开同一材料 URL → `当前空间不可访问`，无岗位/草稿/版本内容泄露。

## 4. 观测项结论：后端 compare 重复 heading 误报（评审遗留 1 medium）——**已复现**

- 复现步骤（浏览器实测，见 t15-web-shots/10-observe-compare-duplicate-heading.png 与 evidence.log `observe:` 行）：
  1. 在既有材料上新增两个同名章节：`项目经历`（正文 `项目 A：内部知识库工具`）与 `项目经历`（正文 `项目 B：数据看板平台`）；
  2. 保存草稿并确认 → V3；
  3. 不做任何修改，再次确认 → V4（V3 与 V4 结构化正文逐字一致）；
  4. 点击 `与 V3 比较` → `GET /api/v1/career/materials/1e3edf50-…/versions/4/compare?baseline=3` → 200。
- 期望：changes = []（UI 应显示 `两个版本正文一致，无差异。`）。
- 实际：`章节「项目经历」变更：项目 B：数据看板平台 → 项目 A：内部知识库工具`（伪差异）。
- 根因（读码）：`internal/modules/career/material.go:923` `diffMaterialBodies` 以 `map[heading]section` 索引，重复 heading 时后写覆盖先写；第一个「项目经历」（项目 A）被拿去与 map 中保留的「项目 B」比较 → 误报 section_changed。`diffMaterialClaims`（claimId 索引）无此问题。
- 影响：读路径级展示伪差异；不写库、不损版本数据、不阻塞发布。建议后续修复轮将 heading 索引改为有序对齐（如按 heading 分组保持次序的 LCS 或索引对）。

## 5. 自查与已知局限

- 简报行为要求逐条覆盖：冻结证据可见 ✓（pinned evidence 四字段 + 快照 SHA-256）；主张链接确认事实 ✓；缺失不补造 + needs_review 占位与审阅风险清单 ✓（409 实测）；确认 → 不可变版本且版本号递增 ✓（V1→V4）；V1/V2 并排比较且差异可见 ✓；旧版本只读 ✓（0 可编辑字段）；失败保留草稿与原因、不发布 ✓（409 实测 + 服务端保留完好草稿）；未知回执原 requestId 恢复 ✓（组件级测试；E2E 中网络层未注入超时）；revision 冲突显示当前修订 ✓（组件级）；跨租户 403/404 ✓（E2E 403 + 组件级 404）。
- 已知局限：
  1. 比较入口仅提供相邻版本（V(n-1)↔Vn）按钮，非相邻版本比较走同一 API 但 UI 未提供选择器（后端支持任意 baseline/target）。
  2. 未知写入恢复（查询回执/原编号重试）在组件测试中以 TIMEOUT stub 验证；浏览器 E2E 未注入网关超时（Lite 下无现成注入点），失败路径 E2E 覆盖的是确定型 409。
  3. E2E 中申请走了 ineligible+显式继续路径（JD `仅限2027届` 冻结硬规则），不影响材料面行为。
  4. compare 重复 heading 误报为后端既有问题（本任务只记录不修，符合简报"不改后端"）。
- 文件清单：新增 `apps/web/src/career/MaterialPage.tsx`、`MaterialPage.test.tsx`、`material.css`；修改 `packages/api-client/src/career.ts`、`packages/api-client/src/career.test.ts`、`apps/web/src/career/ApplicationPage.tsx`、`apps/web/src/career/ApplicationPage.test.tsx`。
- 服务器与端口已停止并确认释放；临时目录 `/tmp/weknora-t15-browser.01lb2L`（库+日志+原始截图）保留至集成评审。

## 6. 第 1 轮评审回应（R1，HEAD 4bdbf19d1）

### F1（high，已修复）：恢复材料后新增占位主张 claimId 冲突

- 根因确认：`MaterialPage.tsx` 的 `claimCounter = useRef(0)` 每次挂载从 0 起算；restore 载入含 `claim-1` 的服务端正产后，"添加缺失占位主张"再次生成 `claim-1` → React key 重复 + 后端 `validateMaterialShape` seenClaims 去重以 `invalid_request`(400) 拒绝保存（`internal/modules/career/material.go:836-840`）。
- RED：`MaterialPage.test.tsx` 新增 `adding a placeholder after restoring a stored draft never collides with existing claim IDs`（恢复含 claim-1/claim-2 的正本 → 新增占位 → 断言 3 个 claimId 唯一、新 ID 为 claim-3、保存 body 的 claimId 集合为 [claim-1, claim-2, claim-3]）。修复前 `node --import tsx --test apps/web/src/career/MaterialPage.test.tsx` → `ℹ tests 11 / pass 10 / fail 1`。
- 修复（双保险）：① `syncClaimCounter(body)` 在恢复（acceptView）、保存成功（saveDraft）、回执恢复（lookupReceipt）三处把计数器同步为正文中 `claim-N` 模式的最大 N；② `nextClaimId(existing)` 生成时跳过与当前任何章节任何主张的 claimId 冲突（防御非 `claim-N` 模式或同步遗漏）。
- GREEN：同命令 → `ℹ tests 11 / pass 11 / fail 0 / cancelled 0`。

### R1 重跑验证（真实命令与输出）

- `node --import tsx --test apps/web/src/career/*.test.tsx packages/api-client/src/career.test.ts` → `ℹ tests 105 / pass 105 / fail 0 / cancelled 0`（104+新增 1）。
- `pnpm typecheck:web` → `TYPECHECK_PASS`（0 错误）。
- `pnpm build:web` → `✓ built in 31.71s`（仅既有 chunk 告警）。
- `pnpm test:web`（全量，/tmp/t15-testweb-r1.log，EXIT=0）→ `ℹ tests 2401 / pass 2401 / fail 0 / cancelled 0`（R0 全量 2400 + F1 新测试 1）。
- `git diff --check` → EXIT=0 无输出。
- 提交：`4bdbf19d1 fix(web): keep new material claim IDs unique after draft restore`（2 files, +49/−3）；评审包 `review-05122c146..4bdbf19d1.diff`（2 commits）。

### low 项处置（记录不修）

- **F2（low）**：同一申请上下文重入且 URL 无 `material` 参数时会新建第二份草稿、既有草稿仅能靠 URL 找回——后端七路由无"按岗位/申请列材料"端点，属合同级限制（前端已用尽现有合同面）；如需主动发现需后端新增列表端点，留待后续轮。
- **F3（low）**：未知写入恢复仅有组件级 TIMEOUT stub 覆盖，E2E 未注入网关超时——维持 §5 已知局限 2。
- **F4（low）**：evidence.log 跨租户相位 `leakCheck` 值记录为 `[object Object]` 未序列化——复核途径：同相位 `crossTenant:materialDenied=true` 已单独断言，且截图 `11-cross-tenant-material-denied.png` 与该相位页面文本（无 教育经历/校奖学金/不可变版本 内容）可直接目验；E2E 服务器已停，不为此重跑脚本。
- **F5（low）**：比较 UI 仅相邻版本入口——维持 §5 已知局限 1。
