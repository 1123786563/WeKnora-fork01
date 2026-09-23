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

## T4（25b.4）— handler 层搬迁 + 残差

> 占位：未执行。

## T5（25b.5）— 差分证据收口 + Integration Brief + 节点门禁

> 占位：未执行。

## 节点收尾核对（T5 定稿前逐条填写）

- [ ] `git diff --name-only "$BASE"...HEAD | sort` vs 计划 §1 白名单差集为空
- [ ] `go build ./...` 0
- [ ] `go test ./internal/modules/agentcatalog/... -count=1` ok
- [ ] `go test ./internal/application/service ./internal/handler ./internal/router -count=1` ok（与 T1 基线同集合）
- [ ] `make check-backend-architecture`（633/23+23/58/16）与 `make verify-module-moves`（16 manifests）0
- [ ] parity 测试通过（计划 §7.2 覆盖）
- [ ] 差分证据落盘（evidence §2–§4）
- [ ] owned_files 逐条核对
- [ ] 未完成项如实列出
