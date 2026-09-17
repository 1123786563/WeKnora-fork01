# W37 任务报告 — 完整验收门禁与分阶段交付报告

- Implementer：W37 implementer（delivery profile 终验）。BASE：`f0f39fa0`（派发时 HEAD）。
- 分支/工作树：`codex/react-vue-parity-align` @ `/Users/wuyongjun/trea/WeKnora-fork01/.worktrees/react-multiclient`（共享 worktree，与并发 W28 进程共存）。
- 状态：DONE_WITH_CONCERNS（concerns 为并发进程文件混杂与本机环境边界，见 §8）。

## 1. 变更清单

主任务（验收门禁与报告）：

- Create `scripts/mobile-workbench/check-acceptance.mjs` — `missingEvidence` + `validateEvidenceRecord` + `validateAcceptance` + `collectGateFailures` + CLI main（exit 0 仅当所有 delivered/pending profile 的 required kinds 均有合法 pass 行）。
- Create `scripts/mobile-workbench/check-acceptance.test.mjs` — 14 用例（分册原文用例逐字保留 + 校验规则全覆盖）。
- Create `docs/evidence/mobile-workbench/acceptance.json` — 36 行证据、7 profiles（见 §4）。
- Create `docs/evidence/mobile-workbench/release-report.md` — 已交付/未交付/blocked-env/已知风险/回退命令 + 六条可追踪链（见 §5）。

Carry-forward（W36 review Important，三处接线，均 TDD）：

- ① `packages/domain/src/mobile/index.ts` re-export `compatibility.ts`（`protocolMode`/`clientGate`/`CLIENT_PROTOCOL_VERSION` 上包面）+ 新增 `index.test.ts`。
- ② `/system/capabilities` 携带协议上下界：
  - `internal/handler/deployment_capabilities.go`：`DefaultProtocolMinimum=2`/`DefaultProtocolMaximum=3` 常量（与 TS `SERVER_PROTOCOL_WINDOW` 对齐）、`DeploymentCapabilitiesData.ProtocolMinimum/ProtocolMaximum`（wire `protocol_minimum`/`protocol_maximum`）、`WithProtocolWindow`（非法窗口保留最后有效值）、`GetDeploymentCapabilities` 零值兜底；
  - `internal/config/config.go`：`WorkbenchConfig.ProtocolMinimum/Maximum`（yaml + `WEKNORA_WORKBENCH_PROTOCOL_MINIMUM/_MAXIMUM` env 覆盖，unparseable 不改窗口）、`Config.ProtocolWindow()` nil-safe 访问器；
  - `internal/router/deployment_capabilities.go`：把 config 解析的窗口绑进快照。
- ③ 移动侧消费：
  - Create `apps/mobile/sources/weknora/platform/protocol-gate.ts` — `serverCapabilitiesRequest()`（GET `/api/v1/system/capabilities`）、`createProtocolGate(fetcher)`（握手→`clientGate(CLIENT_PROTOCOL_VERSION, data)`，失败/未知 schema 一律 fail-closed `unknown_schema`）、`parseWorkbenchWireSnapshot`（snake_case→camelCase 显式映射，闭合 W36 review Minor-2 fail-open 陷阱）；
  - `view-model.ts`：`ConversationProtocolGate` 端口 + `createProductConversationViewModel` 的 `protocolGate` 输入；cancel/steer 在**网络调用前** fail-closed 拒绝（`PROTOCOL_UPGRADE_REQUIRED`）；`capabilities.canCancel/canSteer` 随 verdict 同步；`protocolNotice` 升级说明（三态文案）；`createProductExecutionApi` 增加 `capabilities(signal)`（同一 auth-session transport）；`ExecutionApi` 接口可选方法（测试 double 不受影响）；
  - `ConversationScreen.tsx`：`protocol-compatibility-notice` 提示行（安全登录面保留）；
  - `session/[id].tsx`：生产组装点（bearer + executionApi 存在时建 gate，挂载时握手一次，abort on unmount）。

## 2. RED/GREEN 证据（命令 + 退出码）

| # | RED（先于实现） | 转录 | GREEN |
| --- | --- | --- | --- |
| 主任务 | `node --test scripts/mobile-workbench/check-acceptance.test.mjs` → `ERR_MODULE_NOT_FOUND .../check-acceptance.mjs`（模块未定义，分册预期） | `/tmp/w37-red-acceptance.txt` | 14/14 pass，`node --check` exit 0 |
| ① | `pnpm exec tsx --test packages/domain/src/mobile/index.test.ts` → `does not provide an export named 'CLIENT_PROTOCOL_VERSION'` | `/tmp/w37-red-domain-index.txt` | 2/2 pass；W36 compatibility 回归 11/11 |
| ② | 三处 `go test` → `undefined: DefaultProtocolMinimum` / `unknown field ProtocolMaximum` / `defaults.ProtocolMinimum undefined`（编译失败即 RED） | 见 §3 | `go test ./internal/handler ./internal/config ./internal/router -count=1` 三包 ok（handler 仅余预存 OIDC 失败，BASE 干净快照复现） |
| ③a | `pnpm exec tsx --test .../protocol-gate.test.ts` → `Cannot find module './protocol-gate.ts'` | `/tmp/w37-red-protocol-gate.txt` | 7/7 pass |
| ③b | `pnpm exec tsx --test .../view-model-protocol.test.ts` → 4 fail（`Missing expected rejection`——gate 门禁不存在，非环境故障） | `/tmp/w37-red-vm-protocol.txt` | 6/6 pass |

回归（GREEN 后）：

- `tsx --test`（view-model 既有 + protocol-gate + view-model-protocol + domain index + W36 compatibility）：37/37。
- `tsx --test`（W12 recovery + W25 upload）：32/32。
- `pnpm --filter @weknora/mobile typecheck`：**对"BASE + 本任务提交集"的隔离快照 exit 0**（临时 worktree + node_modules 链接；主工作树因并发 W28 在途文件报错，见 §8）。
- ConversationScreen vitest（隔离快照）：12/12。
- Go：`go vet` 三包零输出；`go test ./internal/config ./internal/router` 全绿；`./internal/handler` 仅 `TestOIDCMobileStartCallbackExchangeIsOneTime` 失败——**在干净 BASE `f0f39fa0` 快照复现同样失败**（500 vs 200，OIDC state 格式），预存、与本任务无关（转录：worktree /tmp/w37-base-snap，已清理）。

## 3. Carry-forward 三处接线证据

1. **domain re-export**：`packages/domain/src/mobile/index.ts` 第 3-6 行；消费面验证——mobile 以 `@weknora/domain/mobile` 导入（domain package.json `exports["./mobile"]`），`index.test.ts` 从 `./index.ts` 导入三符号全通过。
2. **服务端窗口广播**：wire 字段 `protocol_minimum`/`protocol_maximum` 与 TS `parseServerCapabilityWindow` 逐字段对齐（`deployment_capabilities_protocol_test.go` 断言常量必须等于 2/3，防漂移）；默认常量 + yaml/env 双层覆盖；非法覆盖回退最后有效窗口（handler）与编译默认（config）；router 从 `params.Config.ProtocolWindow()` 绑定（nil-config 走默认）。幂等注意：`GetDeploymentCapabilities` 对手工构造的零值快照兜底广播默认，服务端永不广播不可服务窗口。
3. **移动侧消费**：`protocol-gate.test.ts` 七用例覆盖 full/upgrade_required/server_upgrade_required/unknown_schema/握手失败 fail-closed/snake_case 映射/订阅通知；`view-model-protocol.test.ts` 六用例证明 upgrade_required、unknown_schema、**握手未完成**三种状态下 cancel/steer 零网络调用（executions.command 断言不被触达）、late handshake 重开控制命令、无 gate 时旧行为不变；生产组装在 `session/[id].tsx`（bearer 路径唯一入口，与 W25/W29/W12 组装先例同型）。

## 4. acceptance.json 生成方式与分层声明

- 生成脚本（未提交，一次性）：从两台账（`docs/superpowers/plans/mobile-workbench-progress.md` + `.superpowers/sdd/2026-09-12-mobile-ai-saas-workbench/progress.md`）与在盘独立审查文件收集；每行 pass 的 command/exit_code 转录自对应 review 复跑记录；artifact/review 文件存在性与 sha256 在生成时校验；所有 baseline SHA 经 `git cat-file -e` 验证在候选历史中。
- **36 行证据**：14 pass（W08/W10/W11/W12/W13/W24/W25/W26/W27/W29/W33/W34/W36 + security/database/unit 分层）、blocked-env（native 全层、live remote、W35 真实部署恢复）、not_implemented（W28/W31）、review-fail-fix-pending（W30）、accepted-ledger-only（W01-W06、W14-W16、W18-W22 早期会话审查文件丢失——显式声明且**门禁不计为 pass**）。
- **7 profiles**：core/oidc/remote/resources/voice/governance 全部 `pending`（各有真实缺项）；full_happy `not_in_release`（W32 未实施 + 原 H 清单未闭合，带 reason）。
- **门禁实际运行**：`node scripts/mobile-workbench/check-acceptance.mjs` → **exit 1**，6 条 blocking findings（core: backend_model/ios_native/android_native；oidc: ios_native；remote: remote/recovery；resources: ios_native/android_native；voice: billing/ios_native；governance: recovery）+ 7 条 STAGED REPORT 分层声明。**缺项→非零是正确行为**（转录 `/tmp/w37-gate-run.txt`）。零 evidence-record violation（14 个 pass 行的字段校验全过）。
- **计划编写不创建 pass 记录**：`generated_by` 字段显式声明；W37 自身无 pass 行；未实施任务显式 not_implemented。

## 5. release-report.md 要点

已交付（代码级，独立复审）/未交付（W28、W30 修复、W31、W32、full_happy、backend_model）/blocked-env（native 无工程无签名、live Paseo/PG、真实部署注入、早期审查文件丢失）/已知风险（W30 双计费在候选中——charged voice 不得上线；预览票据进程内存储多实例需共享；预存 OIDC/000055 失败）/回退命令（W34 六开关、W37 协议窗口 env、git revert 指引）。六条链均有台账指针（含如实声明：取消后结算链的语音段未闭合）。

## 6. 提交纪律

- **ConversationScreen.tsx hunk 级拆分**：该文件混有并发 W28 进程在途 hunks（resultResources seam、selectRenderer export——非本任务）。处置：构造只含 W37 hunks 的补丁 `git apply --cached`（转录 `/tmp/w37-cs-only.patch`），index 中该文件仅含 protocolNotice 两处改动，W28 hunks 完整保留在工作树供其进程继续。提交后仓库版 ConversationScreen.tsx 不引用任何未提交符号（隔离快照 typecheck 0 + vitest 12/12 证明）。
- 其余文件逐文件 `git add`；绝不 `git add -A`；不触碰 `docs/superpowers/plans/mobile-workbench-progress.md`（协调者单写，建议更新行见 §7）与并发进程文件（ConversationScreen.test.tsx、ProductConversationMessages.tsx 等非我所改，不 add）。
- commit message：`feat(acceptance): evidence gate, staged release report and capability protocol wiring`。

## 7. 建议台账更新行（协调者应用，本任务未改该文件）

W 台账 `docs/superpowers/plans/mobile-workbench-progress.md`：

1. W27 行：`pending` → `accepted-with-blockers`；实现提交补 `98293ea6`；规格/质量审查列：`rereview1 Spec PASS / Quality APPROVED（F1/F2 闭合，17/17+router+vitest 6/6 复跑）`；阻塞列：`真机 WebView/独立 origin/vendor blocked-env；F3/F4/F5 deferred`。
2. W30 行：`pending` → `fixing`；实现提交 `f0f39fa0`；RED/GREEN：`voice 9/9、handler 12/12、router/commercial 绿、tsc 0、dictation 20/20`；审查：`review Spec FAIL / Quality CHANGES_REQUIRED（I-1 双计费/I-2 幂等退化等 4 Important）`；下一步：`修复轮聚焦 I-1+I-4`。
3. W37 行：`pending` → `accepted-with-blockers`（若复审通过）；实现提交（本次 SHA）；RED/GREEN：`acceptance 14/14、domain index 2/2、Go 三包 ok、protocol-gate 7/7、vm-protocol 6/6、回归 37+32、隔离 typecheck 0`；审查列留空待复审；阻塞列：`gate exit 1（缺项如实）＝分阶段正确行为；native/live/billing 未闭合`。

SDD 台账（progress.md，协调者单写）：W27 rereview1 PASS、W30 review FAIL 两条在途结论已在盘（task-W27-rereview1.md / task-W30-review.md），建议补记收口行 + W37 完成行。

## 8. 未覆盖条件 / concerns

1. **并发进程**：共享 worktree 上 W28（ConversationScreen.test.tsx、ProductConversationMessages.ts/x、KnowledgeCitation.test.tsx 等在途未跟踪/修改文件）与 parity 进程同时工作；主工作树 typecheck 报错全部来自其文件，本任务提交集在隔离快照全绿。若 W28 进程在我提交与复审之间改写 ConversationScreen.tsx 同一区域，复审时以提交对象为准。
2. **CLI 边界**：门禁校验字段与指针（文件存在/SHA 在历史/review 存在且非实现者报告/exit_code==0/无 skip），不复跑命令本身——"独立 reviewer 阅读原始证据"的一半由 review_ref 指针承担，另一半仍属人工复审职责（分册原文即如此分层）。
3. `backend_model` kind 全计划无任何任务产出过对应层证据，core profile 因此保持 pending——这是计划覆盖面的真实空洞，非本任务可补。
4. handler 预存 OIDC 失败与 repository 000055 族失败均在干净 BASE 复现，未在本任务修复（超所有权）。
5. 真机 native/live Paseo/真实部署注入维持 blocked-env；协调者收口后需按 release-report 要求重新生成两份证据文件。
