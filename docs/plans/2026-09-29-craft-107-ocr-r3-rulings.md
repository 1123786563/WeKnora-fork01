# Craft #107 OCR R3 findings ruling audit

审计范围：只核对 `docs/plans/craft-107-ocr-final-3.md` 中要求复核的 Medium/High findings；审计时间 2026-09-29；工作区为 `codex/craft-107-integration`。未修改应用代码，未重跑 OCR。

## Ruling summary

| Finding | Severity | Ruling | Evidence |
| --- | --- | --- | --- |
| `packages/api-client/src/craft/index.ts:392-396` extension_action 解析导致预算视图整体折叠 | Medium | **Valid** | `budgetPause()` 直接执行 `parseCraftBudgetExtensionAction(body['extension_action'])`，没有降级捕获。`routes.tsx` 的加载错误路径把异常转换为 `viewed: false`，因此辅助字段异常会丢弃同一响应中的 `pause` 数值。 |
| `internal/container/craft_web_build.go:373-377` 缺少生产观测退出码 caller | Medium | **Valid; known F08 blocker** | `CraftWebBuildEvidence(log, pin, nil)` 是当前生产读取路径；该函数在 `observedExitCode == nil` 时立即返回冲突。`NewCraftWebBuildReceiptRepository`/`RecordTerminal` 的调用仅在测试中出现，未发现生产执行协调器把可信退出码接入。结果是生产路径不能把 build log 提升为 `BuildRan` 证据。 |
| `internal/application/repository/craft_version.go:244-246` draft-head 未比较发布 manifest digest | Medium | **Valid** | `digest` 由 `in.Files` 计算并写入 `craft_versions.manifest_hash`；draft-head 栅栏只锁定并校验 workspace/revision/state/source_run_id/`expectedHead.ManifestDigest`，随后直接创建 version，未检查 `digest == expectedHead.ManifestDigest`。服务层调用方的预读不能替代同一事务内的存储层校验。 |
| `internal/modules/craft/input_code.go:418-421` Java `--class-path` 绕过路径筛查 | High | **Valid** | 当前分离值状态机只识别 `-cp`/`-classpath`；附加值的 `flagValueCandidates` 未把 `--class-path=` 的冒号列表逐项交给 `javaClasspathInputEntry`。因此 `java --class-path inputs Main` 和 `java --class-path=lib:inputs Main` 不进入 Java classpath 专用检查。 |
| `internal/modules/craft/input_code.go:939-945` Java `-ea:` 与 wrapper interpreter 检测缺口 | Medium | **Valid** | `benignInterpreterLauncherFlag` 仅精确放行 `-ea`，没有 `-ea:` 前缀；同时调用点通过 `interpreterOffset(req.Command)` 计算解释器，而该函数遇到 wrapper 后返回第一个 wrapper/0，未复用 `InterpreterPrefixStatus` 的扫描结果。因此 `timeout … java`、`env … java` 的解释器相关豁免/检查会失效。该 finding 的 `-ea:` 部分表现为 fail-closed 误拒，wrapper 部分表现为检测失效；两者均为真实契约缺口。 |

## Verified fact vs inference

上述五项均可由当前源代码直接复现，属于 verified fact。对 Java classpath 的执行影响（JVM 按平台分隔符拆分 classpath 并可加载输入树中的 class/jar）是对标准 Java launcher 行为的 inference，但不影响 finding 本身：源码明确没有把该长形式接入既有专用筛查状态机。

## OCR coverage limitation

R3 OCR 报告明确记载只选取 27 个项目进行审查，其中 7 个项目因 HTTP 429 失败；因此该报告不是全仓覆盖证明。以上 ruling 只对列出的五项做源代码核验，不能把 R3 的退出状态或未审查项目推断为通过。尤其 F08 仍保持未闭合，不能因 receipt 表/仓储测试存在而声称已有生产证据接线。
- Java argfile Fix3 closes direct/wrapped/shell-command argfile forms but independent review found a shell-mode environment omission: `req.Environment["JDK_JAVA_OPTIONS"]` is not checked in the `Shell=true` branch. This is another valid HIGH and Fix4 is underway. Do not treat the Java findings as closed until Fix4 independently passes.
- Java policy Fix4 closes the last independent-review HIGH: the shell-detected Java branch now rejects non-empty `req.Environment["JDK_JAVA_OPTIONS"]`; the regression uses a generated target and generated argfile. Fix4 source/package hashes match review, independent review is Spec PASS / quality PASS, and parent focused plus full `TestInputCodePolicy` selectors passed. Java class-path/wrapper `-ea:`, module-path, patch/upgrade-module-path, direct/wrapped/shell argfile and environment findings are therefore repaired and reviewed. Compatibility ruling: generated argfiles and non-empty `JDK_JAVA_OPTIONS` remain intentionally refused because the policy seam cannot inspect their expanded options.
