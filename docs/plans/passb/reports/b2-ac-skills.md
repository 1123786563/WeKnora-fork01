# Pass B 实施报告 — b2-ac-skills（25b 租户 Skill 目录/安装/运行时验证/reaper）

> 节点：`b2-ac-skills`；计划：`docs/plans/passb/25b-skill-catalog-install.md`；BASE=`8592f2aacbe49e43583bf433e045b98cefd44c6f`（派发登记，= worktree 分支起点）。
> 差分证据：`docs/architecture/evidence/passb/b2-ac-skills.md`；Integration Brief：`docs/architecture/passb/briefs/b2-ac-skills.md`（T5 定稿）。

## T1（25b.1）— 特征化基线与证据骨架 ✅

**执行内容**：

1. 前置条件核验（实测）：派发事实源 DAG（`.worktrees/passb-int/docs/plans/passb/execution-dag.json`）python 读值——`b0: done/approved (head d57a2fa70)`、`ib1: done/approved (head 8c45a8815)`、`b2-ac-skills: in_progress, base_sha=8c45a8815`、notes 尾条「恢复。」；worktree HEAD=`8592f2aac`、工作树干净。计划 §前置条件 1/2 的 b0 阻塞与 ib1 派发 gate 均已由协调者收口。
2. 基线复跑：§前置条件 4 全部 7 条命令 + reaper `-v` 摘录命令，共 8 条，全部退出码 0，数值与计划记录一致（唯一差异：service 全量耗时 415.098s vs 撰写时 154.512s，同为 `ok`，机器负载差异）。逐条命令与输出摘录见 evidence §1。
3. 建 evidence 骨架：`docs/architecture/evidence/passb/b2-ac-skills.md`（§基线已填，§2–§4 差分占位）。
4. 建本报告骨架。

**T1 无红灯项，未触发 conventions §5 停工上报。**

## T2（25b.2）— repository 层搬迁 + 残差 ✅

`97b037ccf refactor(agentcatalog): move tenant skill repository into module`：`git mv` repository/tenant_skill.go（签名零变更）+ §5.3 同名重写残差；测试随迁（rename 100%）。本会话（T3 派发）开工时 BASE=97b037ccf 即 T2 完成点，工作树干净。

## T3（25b.3）— service 层原子搬迁 + HostAdapters + 导出化 + 全量残差 ✅（2026-09-24）

**执行内容**（对应计划 §6 T3 全部步骤）：

1. 建 `host_adapters.go`：HostAdapters 11 位 + SkillManifestView + `validate()` fail-fast（SessionUserID 外全必填）+ 小写访问器 + manifestParsers/transcriptSanitizer 组装视图。
2. 原子搬迁 17 生产文件 + 17 测试文件 + `tenant_skill_verify.py`（embed 依赖）至 `internal/modules/agentcatalog/service`；package 名不变（`service`）。
3. 导出化：`MatchSnapshotByName`/`SkillSnapshotNamePrefix`/`SnapshotsNotFromOtherConfig`/`ValidateUserEnvName`/`SkillsForRun`（首参 `PinnedConfigReader` 化，pinned==nil 或空白 sessionID → agentConfigID，行为锚点与原 `sandboxConfigForExistingSandbox` 一致）/`EffectiveTenantSkills`/`SkillImageConfigReader`/`InstalledSkillLister`/`SkillSnapshotLister`；包内调用点同步（reaper:316/567/572、install:1673、admin:18）。
4. §2.3/§2.4 调用点改写：install:1123（installExecutor 升方法）、install:1461（ResolveConfigManager 适配）、install:1473（SessionUserID 适配，nil 缺省 types.SessionOwnerIDFromContext）、install:1976（installerAgentConfig 增 installerTools 参数）、install:1849 链（buildInstallPrompt→formatOnDemandInstallers→bundleOnDemandInstallers 透传 adapters；buildRepairPrompt 无 agentruntime 消费、未加参——计划 §2.4 链描述与实测不符，evidence §5.3 登记）、install:708（newInstallTranscript 收 transcriptSanitizer）、bundle:309/425（skillManifestParsers 注入）、catalog:213（s.adapters.uniqueNonEmptyStrings，**未**用本面 uniqueStrings）。
5. env_declare 删 init()、增 `RegisterReservedEnvNames`；因宿主禁改测试经残差调用 `ParseSkillBundle`，按 §4.4-1 同类先例增 `RegisterBundleParsers` 接缝（remove_at: ib2）；新包测试 init 显式注册（`host_capability_test.go`，§4.4-2）。
6. chat 类型消解：`resolveInstallerModel` 拆 `installerModelID`（返回模型 ID）+ 单点 `GetChatModel`（类型推断），agent 配置模型路径两次读取（校验+终取），选定模型不变。
7. 残差：`tenant_skill_service.go`（同名重写：类型别名族 + 构造器旧 12 参转发 + 4 符号 1:1 委托 + init 注册 + §2.5 未列消费点承接）与 `tenant_skill_effective.go`（skillsForRun 包装）。
8. parity 差分装置 `tenant_skill_export_parity_test.go`（5 测试 12 例，evidence §3）。
9. 门禁与治理：见 evidence §4/§5——四 commit 序列（迁移 bded9ec9b / TEST-SUPPORT-SHIM 0f554a269 / IMPORT-EXCEPTION-REGISTRY 8978183b3 / legacy 占位与契约路径同步 3a7395d37），两项协调者裁定全文与偏差清单见 evidence §5。

**T3 门禁终态（全部退出码 0）**：`go build ./...`；`go test ./internal/modules/agentcatalog/...`（repository ok / service ok 6.125s）；`go test ./internal/application/service`（ok 80.077s）；handler/router Skill 面 ok；`make check-backend-architecture` OK(0 violations, 633/23+23/58/16)；`make verify-module-moves` OK(16 manifests)；`make check-passb-readiness` legacy=396 aliases=99 exceptions=113 contracts=125 events=29 overlaps=0 missing=0。

## T4（25b.4）— handler 层搬迁 + 残差 ✅（2026-09-24）

**执行内容**（对应计划 §6 T4 全部步骤；任务起点 BASE=`7c76e7cf1`，工作树干净）：

1. 基线复跑（改动前）：`go build ./...`（退出码 0，仅 cmd/desktop、cmd/server 既有 `ld: warning: ignoring duplicate libraries`）+ T4 三条 GREEN 命令（handler Skill / router ApiKey|Skill / agentcatalog 全部）全绿。
2. 4 文件随迁至 `internal/modules/agentcatalog/handler`（package 名不变）：`skill_handler.go`、`skill_catalog.go` + `skill_handler_test.go`、`skill_catalog_test.go`。`git mv` 与 sed 均被写入安全 Hook 拦截（Mimosa PreToolUse 仅放行 Write/Edit），按 T3「严格使用 Edit/Write」纪律改用：Write 落新路径 4 文件 + 重写旧路径 2 生产文件为残差 + `rm` 删除旧路径 2 测试文件，终态与 git mv 等价。
3. 生产文件改写：`service.` 前缀 → 同包 `service`（agentcatalog/service，导入别名免 `acatsvc.` 因包名即 service）；`sandboxConfigTenantID(c)` → 包内新 helper `skillTenantID(c)`（与 sandbox_config.go:92 同一公共键表达式，计划 §4.6）；新增测试本地装置 `testSkillTenantID`（对照宿主 sandbox_skill_test.go:26）与 `oversizedSkillSourceJSON`（对照宿主 upload_limit_test.go:153，溢出本包同名常量 `skillSourceJSONMaxBytes`）。
4. 宿主残差落位（§5.4）：`skill_handler.go` 重写为「同形未导出接口 usableSkillLister/skillCatalogService（返回类型指 acatsvc）+ `type SkillHandler = acathandler.SkillHandler` + `NewSkillHandler` 1:1 转发」；`skill_catalog.go` 重写为纯注释占位 stub（T3 manifest legacy 占位同模式）。routes_agent.go:76、container.go:801-803、router_api_key_capabilities_test.go:448、routes_skill_market_test.go:31 零改动编译（方法集经别名整体可达；`&handler.SkillHandler{}` 复合字面量别名兼容）。
5. GREEN 门禁（全部退出码 0，终态 HEAD=`7695a1c9e` 复跑）：`go build ./...`；`go test ./internal/modules/agentcatalog/... -count=1`（handler ok 0.523s / repository ok / service ok）；`go test ./internal/handler -run 'Skill' -count=1`（ok 0.991s）；`go test ./internal/router -run 'ApiKey|Skill' -count=1`（ok 1.052s）；`make check-backend-architecture`（OK 0 violations，633/23+23/58/16 不变）；`make verify-module-moves`（OK 16 manifests）。gofmt 本任务文件零 diff（`gofmt -l` 干净；宿主 5 个既有测试文件 BASE 起即未格式化，禁改不动）。

**T4 偏差登记**（详见 evidence §5.7）：

1. 【计划 §4.6 未列 helper 清单】搬迁的 skill_catalog.go 实际消费 11 个宿主 handler helper（respondSkillServiceError、skillSourceRequest、skillTooLargeError、skillSourceRequestTooLargeError、skillJSONRequestTooLargeError、skillSourceJSONMaxBytes、uploadEnvelopeSlack、limitUploadBody、limitJSONBody、limitSkillUploadBody、isRequestBodyTooLarge）；其定义留驻宿主服务其他禁改端点（sandbox_skill/skill_market/tenant_expert_market/tenant_skill_market/knowledge/model/initialization），新包禁 import 宿主 handler（§4.4-3）→ 按 §4.6 skillTenantID 同一律在模块包 1:1 同名再声明（含原始 doc 注释与真源路径标注），宿主原件不动。
2. 【§5.4「8 方法委托」形态不可实现】Go 不允许经类型别名在宿主包重声明方法；SkillHandler 别名已整体承载方法集，routes/container 调用零改动可达 → skill_catalog.go 残差改为纯注释占位（T3 manifest legacy 占位先例），别名与转发集中在 skill_handler.go 残差。
3. 【搬迁测试装置本地化】新包测试二进制不可引用宿主测试文件 → `testSkillTenantID` 常量与 `oversizedSkillSourceJSON` fixture 在新包测试内本地再声明（宿主原件继续服务禁改测试）；两常量（skillSourceJSONMaxBytes/uploadEnvelopeSlack）与宿主同名同值。
4. 【git rename 识别形态】测试文件 rename 92%/93% 识别；2 个生产文件因旧路径原位重写为残差呈 M+A 形态（与 T2/T3 已审 commit 同形态，`git log --follow` 可追溯）。
5. 【传递依赖说明】`go list -deps ./internal/modules/agentcatalog/handler` 命中 5 个 agentruntime 包——链路为新包 → acatsvc（同模块）→ 宿主 `application/repository` 与 `types/interfaces`（既有依赖）；与 T3 已评审 service 包计数一致（同 5），非本任务引入；§10.10 直接 import 判据（`go list -f '{{.Imports}}'` + 生产文件 grep）为零违规。

**T4 无红灯项，未触发 conventions §5 停工上报。**

## T5（25b.5）— 差分证据收口 + Integration Brief + 节点门禁 ✅（2026-09-24）

**任务起点**：BASE=`4f8ff1de0`（T4 完成点），工作树干净（`git status --short` 空）。

**执行内容**（对应计划 §6 T5 全部步骤）：

1. **双跑差分**（evidence §2 定稿）：
   - 逐包双跑：基线 8 条命令终态复跑，7 条逐包 `ok/FAIL` 一致（build/make 数值逐项不变；耗时差异为机器负载）——详见 evidence §2.1 表。
   - reaper 面逐例：终态新包 `go test ./internal/modules/agentcatalog/service -run 'TestReap|TestPrune|TestReconcile' -count=1 -v` → 27 RUN/27 PASS/0 FAIL；宿主字面复跑 `[no tests to run]`（27 例已迁出）；第 28 例 `TestPruneEmptyFolderChainsDeletesOnlyEmptyCandidateAncestors`（wiki 面 `wiki_page_test.go:17`，BASE 起未动、不属本面）宿主单跑 PASS。**28 = 27 + 1 逐例 1:1，双跑零 FAIL。**
   - 【如实登记】evidence §1 命令 #8 字面含 `\|`（单引号），T5 探针实证该引写下 Go regexp 解析为字面竖线、模式 no-op（`no tests to run`）；基线 28 例 -v 日志实为纯竖线 OR 语义产出；T5 双跑统一纯竖线形式（evidence §2.2）。
2. **公约 §1.2 命令全跑**：`PASSB_BASE_SHA=$(git merge-base origin/main HEAD)` → `b1a3d6dd8`；`git diff --stat b1a3d6dd8...HEAD` → 129 文件（29763+/9439−，含集成线上 b0/ib1 及其他节点产物，非本节点单独可归属）；节点口径以派发基线 `8592f2aac...HEAD` → **73 文件**（时点说明：T5 会话提交本任务 docs commit 前实测 HEAD=`4f8ff1de0` = 72 files/11000+/9372−，此时 Brief 尚未落盘；含本任务新建 Brief 后终态 HEAD=`3df7b97c7` 复测 = `73 files changed, 11137 insertions(+), 9372 deletions(-)`，第 73 文件即 `docs/architecture/passb/briefs/b2-ac-skills.md`，计划 §1 节点自身产出白名单第 7 项；修复轮审查 R1/R2 实证并修正本处，见遗留第 6 条）；`go build ./...` → 0（重复 §2.1 #1）。
3. **节点 gates 全跑**（DAG gates 4 条，全部退出码 0）：`go build ./...`；`go test ./internal/modules/agentcatalog/... -count=1`（handler `ok 0.763s` / repository `ok 1.264s` / service `ok 5.506s`）；`make check-backend-architecture`（`OK (0 violations)`，**633/23+23/58/16 逐项不变**）；`make verify-module-moves`（`OK (16 manifests verified)`）。
4. **附加验收判据实测**：parity 终态 5/5 PASS；`go test ./internal/handler -count=1` 全量 `ok 1.197s`、`go test ./internal/router -count=1` 全量 `ok 1.565s`（§10.4 字面）；§10.9 导出名命中 4、旧名仅存残差；§10.10 新包生产文件零违禁 import（grep + 三包 `go list -f '{{.Imports}}'`）；§10.8 禁改目录 diff 零出现。
5. **白名单差集核对**（§10.1）：**73 文件**（节点终态 HEAD=`3df7b97c7` 实测 `git diff --stat 8592f2aac...HEAD` = `73 files changed, 11137 insertions(+), 9372 deletions(-)`；T5 会话原报「72 文件」系提交本任务 docs commit 前 HEAD=`4f8ff1de0` 的实测，未含当时尚未落盘的本任务 Brief——审查 R1/R2 实证后修正）= 20 生产搬迁行（17 service + 1 repo + 2 handler，每行旧路径 M 残差/占位 + 新路径 A）+ 随迁测试 20 个（R 80–100%，**计划列 19 实际 20——`repository/tenant_skill_test.go` 为 framework:29 义务随迁、计划 §1 glob 口径漏列，evidence §6 登记**）+ 节点产出 6（`host_adapters.go`、`host_capability_test.go`、`tenant_skill_export_parity_test.go`、evidence/passb/b2-ac-skills.md、reports/b2-ac-skills.md、**briefs/b2-ac-skills.md（本任务 T5 新建）**）+ 治理裁定登记 6（exception-ledger.yaml、pass-a-acceptance.md、execution-ledger.md、tools/architectureguard/check.go、tools/passbguard/check.go——IMPORT-EXCEPTION-REGISTRY 裁定；tenant_skill_testsupport_test.go——TEST-SUPPORT-SHIM 裁定）。**无第三类文件，未登记白名单外改动为零。**
6. **定稿 Brief**：`docs/architecture/passb/briefs/b2-ac-skills.md`（(a) 残差全删 20+2 清单、(b) 六组调用点切换表、(c) container 四行接线、(d) routes 切换、(e) contracts 回写 + check-passb-readiness 10 条 IB2 收口清单、(f) DAG 回写确认 2 项、(g) 例外台账如实口径——8 条裁定新增 exc-0106..0113 由 IB2 删除）。全部行号锚点终态实测。
7. **定稿本报告与 evidence**：evidence §2（搬迁等价差分）/§4（T5 行）/§6（计数登记），本节 + 收尾核对。

**T5 无红灯项，未触发 conventions §5 停工上报。**

## 节点收尾核对（T5 定稿逐条核验，2026-09-24）

- [x] `git diff --name-only 8592f2aac...HEAD | sort` vs 计划 §1 白名单差集为空（**73 文件**逐类核对，见 T5 第 5 条；白名单外 12 文件 = 节点产出 6 + 治理裁定登记 6，均有 evidence §5 立条；「第三类」为零。审查 R1/R2 修正：T5 原报 72 文件系 HEAD=`4f8ff1de0` 时点实测，未含本任务新建 Brief）
- [x] `go build ./...` → 退出码 0（仅既有 ld warning）
- [x] `go test ./internal/modules/agentcatalog/... -count=1` → ok（handler 0.763s / repository 1.264s / service 5.506s）
- [x] `go test ./internal/application/service -count=1` → `ok 75.289s`；`go test ./internal/handler -count=1` → `ok 1.197s`；`go test ./internal/router -count=1` → `ok 1.565s`（§10.4 字面全量；与 T1 基线同集合拆布于宿主+新包两处，比对见 evidence §2.3）
- [x] `make check-backend-architecture` → `OK (0 violations)`，633/23+23/58/16 与基线逐项一致；`make verify-module-moves` → `OK (16 manifests verified)`
- [x] parity 测试通过（5/5 PASS，终态复跑；覆盖 §7.2 全部符号与边界用例）
- [x] 差分证据落盘（evidence §2 双跑比对 + §4 T5 行 + §2.2 引写语义登记）
- [x] owned_files 逐条核对（DAG owned_files = manifest 25b 行 + `internal/modules/agentcatalog/**`；73 文件核对结论见 T5 第 5 条）
- [x] 未完成项如实列出（见下）

**未完成项 / 遗留（如实）**：

1. `make check-passb-readiness` 回滚后红（10 条诊断）——非本节点 gate（公约 §2 barrier 追加项），10 条为计划 §8(e) 分给 IB2 的 contracts 回写义务，收口清单已登记（evidence §5.4 + Brief §e）。
2. DAG `b2-ac-skills.review_status=changes_requested`（OCR finding：tenant_skill_verify.py 反斜杠归一）——已由 `7c76e7cf1` 修复（OCR 建议选项 a：保留字面反斜杠），`review_status` 流转归协调者/复扫，非 T5 范围。
3. 随迁测试计划计数 19 vs 实际 20（`repository/tenant_skill_test.go` 漏列）——框架义务覆盖的多迁差异，evidence §6 登记，无需动作。
4. evidence §1 命令 #8 引写语义（`\|` 单引写 no-op）——已实证并登记（evidence §2.2），基线结论不受影响（28 例 -v 日志为纯竖线语义产出）。
5. 残差接缝（别名/转发/init 注册/占位/parity/shim）全部按 Brief (a) 待 IB2 删除；本节点按 framework:31 保留至装配切换（「legacy 删除前差分必须已通过」已满足，evidence §2.3）。
6. **审查修复轮（2026-09-24，findings R1/R2）**：T5 原报节点 diff「72 文件（11000+/9372−）」系提交本任务 docs commit 前实测（HEAD=`4f8ff1de0`，Brief 尚未落盘），未随终态 HEAD 更新——审查复跑 `git diff --stat 8592f2aac...HEAD`（HEAD=`3df7b97c7`）得 `73 files changed, 11137 insertions(+), 9372 deletions(-)`，本会话已复现同值（`merge-base --is-ancestor` 验 8592f2aac 为 HEAD 祖先）。第 73 文件 = `docs/architecture/passb/briefs/b2-ac-skills.md`（本任务 T5 新建，计划 §1 节点自身产出白名单内；与 finding 描述截断所示 `internal/modules/agentcatalog/service/tenant_skill_verif…` 不符——`tenant_skill_verify.py` 在 72 清单内本就存在，R089）。定性：白名单**差集结论仍为空**（第 73 文件属白名单第 7 项节点产出），但报告数字/枚举对终态不可复现且原枚举自相矛盾（「节点产出 6」含 Brief 而总数 72 不含），已按 R1/R2 修正本文件与调度报告；insertions/deletions 为自指统计（含报告自身行数），文档以 files=73 + 分类清单为稳定判据，各时点输出原文存调度报告。
