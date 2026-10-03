# F08 设计勘察报告（T04/T19/T20 链头）— 2026-10-03

基线：`chore/f08-design` @ `1d06ef0f3`（只读勘察，工作树干净）。对照契约：
`docs/plans/2026-09-28-craft-107-f08-execution-receipt-design.md`、`2026-09-29-craft-107-ocr-build-receipt-ruling.md`、`2026-09-28-craft-107-f08-receipt-repository-report.md`（均在 /tmp/wk-f08）。

## 假设裁定：基本收敛（converged with one policy decision open）

2026-09-28 记录的五条"上游身份与计费接口缺失"中，四条已在 main 上由 WB-GRAPH / T09 / T19 / T03 轮次补齐或给出机制；F08 剩余工作是**派发调用方接线**，不是新基础设施。唯一仍开放的是一条**策略命名决定**（build 活动的 CraftCallBinding 取值），按设计文档 §"bounded interface proposal" 仍需预算 owner 拍板。

## 1. 逐 seam 对照（file:line 均为 /tmp/wk-f08 @ 1d06ef0f3）

| F08 要求的上游接口 | 现状 | 证据 |
| --- | --- | --- |
| 身份冻结源（准入即冻结） | ✅ 已有 | `BuildDurableRunSnapshot`：internal/application/service/agent_run_graph.go:150；`AdmissionGraphFreezer` seam：internal/modules/workbench/service/workbench/admission.go:83,245；craft 侧持久 Run 读取 + RunView store 进生产装配：internal/container/container.go:3089,3133（`assembleCraftRunViewProduction`，effect authority 强制、fail-closed） |
| 确定性 server-only activity key | ✅ 机制已有 | `admittedRunViewKey`：internal/container/craft_runview_runtime.go:146；带请求摘要的持久 effect claim `BeginEffectWithDigest`：internal/modules/craft/run_view_effect.go:63 + internal/container/craft_runview_runtime.go:630-646（claim 校验 Task/generation/kind/digest，replay 不可重发） |
| 预算授权 seam（build 活动的官方绑定） | ✅ 结构已有，缺命名常量 | Docker 专用持久 charge-start 协议分支 `prepareCraftDockerChargeStart`（`craftDockerSendProtocol`）：internal/application/service/craft_budget.go:370-375；charge-start 强制 TaskBudgetRow Run 映射（T09 durable budget 已并入）：craft_budget.go:410-418；发送协调器 `CraftDockerSendCoordinator.Prepare/ResumeBound/Observe`（replay 冲突检测、不再"选最新 calls 行"）：internal/application/service/craft_docker_send_coordinator.go:106,133,253,312,347；`AuthorizeBinding` 接受任一验证过的绑定并按序持久化：craft_budget.go:746。**缺**：build 固定活动的绑定取值（DelegationID/ModelID/Funding）无命名定义——策略决定，非代码缺口 |
| Provider 派发面（T04 固定命令 + T19 normal-exec） | ✅ 已有，无生产调用方 | `CraftDockerNormalExecService.Execute`（内嵌 T03 fail-closed、staged 输入、幂等 replay、provider receipt、持久输出 sink）：internal/application/service/craft_docker_normal_exec.go:137-230；T03 双面生产装配 `newCraftDockerExecCommandFaces`（同一 policy + 审计）：internal/container/craft_exec_policy_wiring.go:52,88；T04 命令闸门 `CraftWebBuildCommandGate.Review` + 全局注册：internal/container/craft_web_build_review.go:227 + craft_runtime.go:219（注释明示"future T20 dispatch face"即此注册口） |
| Provider 签发 handle 解析器 | ⚠️ 配料齐全，缺薄封装 | 生产 RunView 装配已验证准入容器（`matchesAdmittedObservation`/`matchesAdmittedProbe` 含 DockerID/pins 校验）：internal/container/craft_runview_runtime.go:607-620；`Resolve` 只回 `CraftRunViewRuntimeHandle{View, Directory}`，**不含** `sandbox.RemoteSandboxHandle`：craft_runview_runtime.go:85-90；Docker handle 形态已存在 `dockerSandboxHandle`：internal/modules/execution/sandbox/docker_remote_client.go:213-223，但无从准入容器铸造的路径。**缺**：一个小 resolver（解析→验证 live 观测→铸造 Provider=SandboxTypeDocker/ID=observed DockerID 的 handle） |
| 回执持久化 | ✅ 已在 main | `CraftWebBuildReceiptRepository.RecordTerminal/Read`：internal/application/repository/craft_web_build_receipt.go；迁移已并入且重编号 sqlite 000189（原 worktree 预留 000138/000217 已被集成轮吸收）。**未**装配进 container（构造器无生产调用方） |
| 证据读取翻转 | ❌ 仍 fail-closed | `craftWebBuildEvidenceSource` 仍传 nil receipt：internal/container/craft_web_build.go:373（`CraftWebBuildEvidence(log, pin, nil)`）；契约在 craft_web_build.go:294-328（nil observedExitCode → `not_run`）；装配点 craft_runtime.go:186,265 |
| Promotion 绑定 | ❌ 缺 | `PromoteWebVersion`：internal/application/service/craft_artifacts.go:240 无回执重读；post-terminal promoteter 只管 capture 回执：internal/container/craft_run_capture_promotion.go:79-129；可用绑定机制即上表 `BeginEffectWithDigest` |
| 派发调用方 | ❌ 缺（核心剩余切片） | `Execute` 与 `RegisteredCraftWebBuildCommandGate` 均无非测试调用方（rg 全仓确认）；T19 S3 身份传播路由仍 default-off（craft_exec_policy_wiring.go:4-9 注释），且 F08 设计要求的派发点本就是 Run/delegation 边界而非 S3 路由 |

## 2. 精确缺失切片

1. **T20 build 派发编排器**（核心）：准入 RunView 解析 → live 容器观测验证 → 铸造已验证 Docker handle → effect claim（新 kind + staged request digest）→ 构造 T04 固定请求 → `Gate.Review` → `faces.Normal.Execute` → 映射结果 → `RecordTerminal`；触发点在两处证据装配（craft_runtime.go:186,265）之前、delegation 成功之后；同时把 receipt 仓库装配进 container。
2. **Build 绑定命名**（策略）：固定活动的 `CraftCallBinding{DelegationID, ModelID, Funding}` 常量 + 预算 owner 签认（设计文档禁止 T20 自造计费身份；`AuthorizeBinding`/coordinator 机制本身已就绪）。
3. **证据读取翻转 + promotion 栅栏**：evidence source 按 Task/Run/activity/request 身份解析回执传入 `CraftWebBuildEvidence`；`CollectCandidate` 封印 manifest 身份；`PromoteWebVersion` 在栅栏内重读回执比对 candidate Run + manifest digest。

## 3. 任务分解（实现就绪）

**T-1 验证 handle 解析器 + T20 派发编排器**（核心，可先行除绑定常量外全部落地）
- 新文件：`internal/container/craft_web_build_dispatch.go`（+ `_test.go`）；小改 `internal/container/craft_runtime.go`（触发点接线、receipt 仓库装配）。若需在 runview_runtime.go 暴露已验证观测，走设计文档的 bounded interface proposal，不改 owner 文件语义。
- 测试：fake provider（复用 `craft_exec_policy_wiring_test.go` 的 fake 模式）跑通 resolve→review→execute→RecordTerminal 全链；绑定常量先以注入占位，等 T-0 裁定后替换。

**T-2 证据翻转 + promotion 绑定**
- 改：`internal/container/craft_web_build.go`（evidence source 注入 receipt 读取，log 降为诊断）、`internal/application/service/craft_artifacts.go`（PromoteWebVersion 栅栏内重读）、`internal/container/craft_run_capture_promotion.go`/collector（CollectCandidate 封印 manifest digest）。
- 测试翻转：`craft_web_build_test.go`、`craft_web_build_ocr_regression_test.go` 的 fail-closed 用例保留并新增 trusted-receipt 正路径；新增异 Run/异 manifest/输出变更后 promotion 拒绝用例。

**T-3（前置，半天）build 绑定策略裁定**：向预算 owner 提 bounded proposal（建议 DelegationID=`web-build`、ModelID=固定 toolchain pin 标识、Funding=沿用 AuthorizeSandbox 同源），产出常量 + 签认记录。阻塞 T-1 的最终形态，不阻塞其骨架。

**T-4 旅程测试 + 迁移链核验**：按设计 §T20.5 九类旅程用例（成功/非零/unknown/截断/异 Task/请求不匹配/输出突变/重启恢复/绑定回执才可 promotion）；核验 000189 迁移在完整链上 up/down/up（集成轮已重编号，无需新迁移）。

## 4. 风险

- **策略决定仍是硬门**：绑定常量未经 owner 签认前，T-1 只能以注入 seam 落地（设计文档明令禁止自造计费身份）。
- **Handle 铸造边界**：必须经"持久 RunView 验证 + live 观测"后由 server 侧铸造，严禁从 `CraftRunView.Runtime.ContainerID` 直接构造（ruling 明令的捷径禁区）；建议 resolver 放 provider/engine 侧并由观测校验背书，以贴合"provider-issued"语义。
- **Effect kind 扩表**：新增 `RunViewEffectWebBuild` kind 需过 effect store 校验白名单（internal/modules/craft/run_view_effect.go），属加法变更。
- **迁移重编号已发生**：worktree 报告的 000138/000217 槽位在 main 已是 000189；勿再引用旧编号，无新迁移工作。
- **S3 路由无关**：成员驱动的 sandbox exec 路由仍 default-off 且非本任务；F08 派发是 server-owned Run 边界，勿混线。

## 5. 勘探方法备注

全程只读：`git -C /tmp/wk-f08` + rg/读文件；主检出未触碰。一处 rg `-r` 误用导致符号名显示为 `n`，已用无 `-r` 复查确认 receipt 仓库构造器确无生产调用方。

---

## 实施计划（/loop 第 14 循环定稿）

> REQUIRED SUB-SKILL: superpowers:subagent-driven-development

**Goal:** 补齐 F08 生产 dispatch 切片，解除 T04(#123)/T19(#138) 验收根阻塞，恢复 web 版本晋升链。

**任务序（串行）**：
- **T-3 绑定策略 bounded proposal**（先行）：`CraftCallBinding{DelegationID,ModelID,Funding}` 取值方案（fail-closed 默认 + 文档留 owner 签认位）。
- **T-1 派发编排器**：新 `internal/modules/craft/craft_web_build_dispatch.go`——RunView 解析→live 容器观测验证→**服务端铸造** `RemoteSandboxHandle`（禁 `Runtime.ContainerID` 捷径，ruling 约束）→Gate.Review→Execute→RecordTerminal；TDD（not_run 翻转为 verified 的用例先行）。
- **T-2 证据翻转+promotion 绑定**：`craft_web_build.go:373` evidence 供体接 dispatch 回执；`PromoteWebVersion`（craft_artifacts.go:240）重读回执。
- **T-4 旅程测试**：九类旅程 + 000189 迁移 up/down/up 核验。

**Global Constraints**：handle 只经验证后铸造；新 effect kind 过白名单；receipt 幂等/首终态不可覆写契约不动；TDD 每任务；提交 `(CRAFT-F08)`；worktree 隔离（主树在途）。
