Review partially complete: 13 finding(s); 7 of 27 selected item(s) failed.

─── packages/api-client/src/craft/index.ts:392-396 ───
[bug · medium] parseCraftBudgetExtensionAction 对辅助字段采取抛 INVALID_RESPONSE 策略：一旦 extension_action
形状不合法，整个 budgetPause() 会失败，而 routes.tsx 的 catch 会将任何异常无差别折叠为 viewed:false（无数值的 contact-owner
拒绝视图），导致本已合法的 {limit, used} 数值展示被连带丢弃。现实触发面：extra_credits 来自服务端配置 policy.TaskLimit（Go int64），若部署配置超过
Number.isSafeInteger 上限（2^53 ≈ 9.007e15），每次 GET pause 视图都会整体失败。建议增强字段解析失败时降级为
null（仅隐藏续期入口、保留数值视图），不要让辅助字段失败连带核心字段整体失败。

+       let extensionAction: CraftBudgetExtensionAction | null = null;
+       try {
+         extensionAction = parseCraftBudgetExtensionAction(body['extension_action']);
+       } catch {
+         // 增强字段降级：保留核心 {limit, used} 数值视图，仅隐藏续期入口。
+         extensionAction = null;
+       }
        return {
          pause: parseCraftBudgetPause(body),
          canExtend,
-         extensionAction: parseCraftBudgetExtensionAction(body['extension_action']),
+         extensionAction,
        };


─── packages/api-client/src/craft/index.ts:38-42 ───
[maintainability · low] CraftBudgetPauseView.canExtend 已成死字段：全仓（apps/web/、packages/）对 .canExtend
零消费，routes.tsx 改用 extensionAction !== null 派生入口。当前服务端确实保证 canExtend=true ⇔ extension_action 非
nil（craft_budget_pause.go:104-108 在 canExtend 时必填 pause.ExtensionAction，且 craft_budget.go:1383 恒返回非
nil action），但这一跨端同步不变式在客户端既无消费也无注释说明。建议：要么从返回结构中移除 canExtend（消除误导性 API
表面），要么在消费端用它做一致性校验/注释标注不变式，避免后续调用者沿用过时信号或两端契约漂移时无从发现。

  export interface CraftBudgetPauseView {
    pause: CraftBudgetPause;
-   canExtend: boolean;
    extensionAction: CraftBudgetExtensionAction | null;
  }


─── apps/web/src/features/craft/routes.tsx:1077-1078 ───
[maintainability · low] onRequestExtension 闭包内的 budgetPauseView.extensionAction! 非空断言依赖跨行三元条件不变式（外层
!== null 判断与内层取值分离），可读性与防御性欠佳。建议在回调内先收窄（或在外层提前解构局部常量），消除非空断言。

                  onRequestExtension={budgetPauseView.extensionAction !== null && !budgetExtensionBusy ? (runId) => {
-                   void requestBudgetExtension(runId, budgetPauseView.extensionAction!).catch((error: unknown) => {
+                   const action = budgetPauseView.extensionAction;
+                   if (action === null) return;
+                   void requestBudgetExtension(runId, action).catch((error: unknown) => {


─── apps/web/src/features/craft/routes.tsx:1073-1074 ───
[style · low] 为单个子组件包裹的空 Fragment（<>…</>）属冗余结构：该分支内并无其他兄弟元素或需要组合的表达式，可直接渲染 CraftBudgetPauseNotice。

-             <>
-               <CraftBudgetPauseNotice
+             <CraftBudgetPauseNotice


─── internal/container/craft_web_build.go:373-377 ───
[maintainability · medium] 已核实：当前集成中不存在任何携带服务端观测退出码的生产路径——CraftWebBuildEvidence 仅此一处生产调用且固定传
nil，NewCraftWebBuildReceiptRepository/RecordTerminal 全库仅有测试调用方，而迁移 000217/000138
已在生产库建表（含不可变触发器）。结果是 W01 构建证据永远停留 not_run，新增的回执仓储与表结构在生成为不可达代码（即已知阻塞 F08 生产缝隙，本 PR 未闭合）。fail-closed
语义本身正确，但建议在本 PR 描述/账本中明确记录该惰性状态与接线责任方，避免后续把 craft_web_build_receipts 表当作已有数据源消费。



─── internal/container/craft_web_build_review.go:96-98 ───
[security · low] 构造器校验了 ToolchainDigest/TemplateSHA256 非空，却未校验
pin.RuntimeDigest。当前唯一生产装配点（craft_runtime.go:184，craftRuntimeDigestFromEnv 恒返回非空）保证了非空，但
craftWebBuildCommandShapeOk 第 186 行与 CraftWebBuildEvidence 第 317 行都以 `expectedRuntimeDigest != ""`
为前置条件——未来任何遗漏第 184 行赋值的装配路径都会让运行时身份锚定被静默降级为仅语法检查，而非拒绝装配。建议在构造器一并强制 RuntimeDigest
非空（fail-closed），使空值守卫仅服务于测试。

- 	if strings.TrimSpace(pin.ToolchainDigest) == "" || strings.TrimSpace(pin.TemplateSHA256) == "" {
- 		return nil, fmt.Errorf("%w: craft web command gate requires the loaded toolchain pin", craft.ErrInvalidInput)
+ 	if strings.TrimSpace(pin.ToolchainDigest) == "" || strings.TrimSpace(pin.TemplateSHA256) == "" || strings.TrimSpace(pin.RuntimeDigest) == "" {
+ 		return nil, fmt.Errorf("%w: craft web command gate requires the loaded toolchain pin (including its runtime digest)", craft.ErrInvalidInput)
  	}


─── internal/application/repository/craft_web_build_receipt.go:240-242 ───
[maintainability · low] OutputGeneration 在函数开头已由 craftWebReceiptText(receipt.OutputGeneration, 128)
强制非空，因此 OutputComplete 分支中的 `receipt.OutputGeneration == ""` 恒为假，是不可达的死条件（迁移 DDL 侧的
length(output_generation) > 0 同理是表级冗余防御）。该子条件会误导读者以为合法回执允许空输出代际，建议删去或注释标明为防御性冗余。

- 	if receipt.OutputComplete && (!receipt.TransportComplete || receipt.CandidateManifestSHA256 == "" || receipt.OutputGeneration == "") {
+ 	if receipt.OutputComplete && (!receipt.TransportComplete || receipt.CandidateManifestSHA256 == "") {
  		return ErrCraftWebBuildReceiptInvalid
  	}


─── internal/application/repository/craft_version.go:244-246 ───
[maintainability · medium] 新增的 draft-head 栅栏只证明了 head
快照身份（revision/state/source_run_id/manifest_digest 均与 expectedHead 一致），但从未把被发布版本自身的清单摘要 digest（由
prepareCraftVersion 从 in.Files 计算，写入 craft_versions.manifest_hash）与 expectedHead.ManifestDigest
做比对。也就是说，同一事务只验证了"head 没动"，没有验证"正在发布的文件就是该 head 封存的文件"——内容一致性目前完全依赖调用方（craft_artifacts.go
PromoteWebVersion）在事务外先读 head/candidate 的链路校验。这正是服务层注释里声称该栅栏要拒绝的"stale candidate
的文件被静默发布为最新版本"场景的最后一环；一旦未来出现第二个调用方、或 candidate 生成链路出现 digest/files 不一致的缺陷，store 层无法拦截，且同一
(workspace, run) 可产生内容不同、ID 不同的两个 version。建议在同一事务内补一行 digest 与 expectedHead.ManifestDigest
的相等校验（craft.ManifestDigest 内部按 path 排序，顺序无关，比对有效），使栅栏对内容也是原子的。

  			if expectedHead.WorkspaceID != in.WorkspaceID || expectedHead.State != craft.DraftHeadSelected || expectedHead.SourceRunID != in.RunID {
  				return fmt.Errorf("%w: invalid expected draft head for version %s", craft.ErrConflict, in.ID)
+ 			}
+ 			// The fence must also prove the PUBLISHED manifest IS the sealed
+ 			// head's manifest: digest derives from in.Files while the CAS below
+ 			// pins expectedHead.ManifestDigest — without this comparison,
+ 			// content equality rests entirely on the caller's earlier reads.
+ 			if digest != expectedHead.ManifestDigest {
+ 				return fmt.Errorf("%w: version manifest does not match the fenced draft head for version %s", craft.ErrConflict, in.ID)
  			}


─── packages/api-client/src/index.ts:10-10 ───
[maintainability · low] 新增导出的 CraftBudgetPauseView 的核心成员 pause: CraftBudgetPause 引用了 contracts 的
CraftBudgetPause 类型，但下一行对 @weknora/contracts 的类型转发列表（含 CraftRunView 等其余全部被引用视图类型）未包含
CraftBudgetPause。消费者从包入口导入 CraftBudgetPauseView 后无法在同一入口命名 view.pause 的类型（CraftBudgetPause 仅在
@weknora/contracts 根导出），与该入口既有"被引用类型全部转发"的模式不一致。建议在第 11 行转发列表补上 CraftBudgetPause。

  export type { CraftApi, CraftCreateSessionInput, CraftListSessionsParams, CraftAddInputInput, CraftSubmitRunInput, CraftAccessRole, CraftGrantableAccessRole, CraftAccessMember, CraftInputDecisionAction, CraftInputDecisionAcknowledgement, CraftBudgetExtensionAction, CraftBudgetPauseView } from './craft/index.ts';
+ export type { CraftSessionCreatedView, CraftSessionSummaryView, CraftSessionPageView, CraftRunView, CraftBudgetPause, CraftVersionView, CraftVersionsPageView, CraftWorkspaceView, CraftInputView, CraftPreviewTicketView, CraftRunEventView, CraftEventPayloadView, CraftSessionKind, CraftEventKind } from '@weknora/contracts';


─── internal/application/repository/craft_run_capture_promotion.go:103-105 ───
[maintainability · low] cursor 种子行（id=1）缺失是部署/迁移层故障，而非调用方输入错误；用 craft.ErrInvalidInput
包装会把它归入"输入非法"这一哨兵类（该包中 ErrInvalidInput 用于 malformed caller input，见 archive.go 中与 400
类错误的对照约定）。当前调用方只 Warnf 日志故无功能影响，但一旦上层按哨兵分类（重试 vs 400），该误分类会误导排障与处理策略。建议改用独立的类型化哨兵（如
ErrPromotionCursorUnavailable）或至少不挂 ErrInvalidInput。



─── internal/application/repository/craft_run_capture_promotion.go:285-287 ───
[maintainability · low] RowsAffected != 1 时返回无类型
errors.New：调用方（container.completePromotion）无法区分"收据未被领取"（良性，例如 capture 行被级联删除）与真实写失败，二者都会被归入同一
Warnf。建议返回可被 errors.Is 识别的哨兵（例如包级 ErrPromotionNotClaimed），与文件其余部分的 %w 哨兵风格保持一致。



─── internal/modules/craft/input_code.go:418-421 ───
[security · high] Java 9+ 的官方长形式 `--class-path`（JEP 293，等价于 -cp/-classpath）完全绕过了本次新增的 classpath
逐段筛查，两条路径都放行：(1) 分离式 `java --class-path /tmp:<root>/inputs Main`——值作为首个非旗标操作数走 scriptSeen 分支的整体
canonical（不分 `:`，见 441 行；冒号分段只存在于 277/300/334 行的 env/shell/wrapper 路径），`/tmp:inputs` 不在输入树内即放行，随后
Main 落入 post-script 数据区；(2) 附加式 `java --class-path=lib:inputs Main`——flagValueCandidates 只返回 '='
后的完整值 `lib:inputs`（且 `--` 前缀不做后缀枚举），整体 canonical 后同样放行。运行时 JVM 会对 classpath 按 `:` 分段并把 `inputs`
解析到进程 cwd 下，即可加载并执行（静态初始化器）输入树中上传的 .class/.jar——这正是本次为 -cp/-classpath 补上筛查所要阻止的行为，长形式使其形同虚设。建议把
`--class-path` 纳入分离值状态机、对 `--class-path=` 附加值走 javaClasspathInputEntry（可顺带考虑
`--module-path`，同为可执行材料的路径列表）。

- 				if interpreter == "java" && (arg == "-cp" || arg == "-classpath") {
+ 				if interpreter == "java" && (arg == "-cp" || arg == "-classpath" || arg == "--class-path") {
  					javaClasspathValueNext = true
+ 					continue
+ 				}
+ 				if interpreter == "java" && strings.HasPrefix(arg, "--class-path=") {
+ 					if target := javaClasspathInputEntry(req.WorkingDir, strings.TrimPrefix(arg, "--class-path="), p); target != "" {
+ 						return p.deny("interpreter_input", target, "")
+ 					}
  					continue
  				}


─── internal/modules/craft/input_code.go:939-945 ───
[bug · medium] 新增豁免存在两个确定性缺口（均为 fail-closed 误拒，非绕过）：(1) `-ea` 是精确匹配，而 Java 文档化的启用断言形式还有
`-ea:com.acme...`（冒号附带包名）——carriesProgramTextFlag 对 "-ea:..." 命中 'e' 即拒绝，合法命令仍被当成 program text
误拒，与本豁免的意图（放行常用启动旗标）相悖；(2) 405 行用 interpreterOffset 取解释器名，但该函数与 InterpreterPrefixStatus 不同——它不解析
wrapper 的取值旗标，遇到 wrapper 后的第一个非解释器 token 就返回 0，于是 `timeout 10 java -jar app.jar`、`env X=1 java -cp
lib Main` 里 interpreter 被算成 "timeout"/"env"，豁免与 -cp 值处理全部失效，包装启动的 java/pwsh/bash 仍被整体拒绝。建议为 java 增加
`-ea:` 前缀分支，并让 interpreterOffset 复用 InterpreterPrefixStatus 的扫描结果（或把 offset 作为参数传入），使豁免与 classpath
筛查在 wrapper 场景下行为一致。

  func benignInterpreterLauncherFlag(interpreter, arg string) bool {
  	switch interpreter {
  	case "java":
- 		switch arg {
- 		case "-jar", "-cp", "-classpath", "-ea":
+ 		switch {
+ 		case arg == "-jar", arg == "-cp", arg == "-classpath", arg == "-ea":
+ 			return true
+ 		case strings.HasPrefix(arg, "-ea:"):
  			return true
  		}


LLM retry report summary: 18 of 195 requests affected -- 4 requests failed, 14 requests recovered after retry

Review planning (3 requests):
- apps/web/src/features/craft/routes.tsx,packages/api-client/src/craft/index.ts,packages/api-client/src/index.ts: rate limited (HTTP 429) -> rate limited (HTTP 429) -> succeeded
- docker/craft/runtime-config.json,docker/craft/web/build.py,docker/craft/web/toolchain.lock.json,docs/testing/craft/t04/offline-build-output/build-log.json,internal/application/repository/craft_web_build_receipt.go,internal/container/craft_web_build.go,internal/container/craft_web_build_review.go,internal/modules/craft/web_contracts.go: rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> succeeded
- internal/application/service/craft_budget.go,internal/handler/session/craft_budget_pause.go: rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> succeeded

Core review (15 requests):
- docker/craft/runtime-config.json,docker/craft/web/build.py,docker/craft/web/toolchain.lock.json,docs/testing/craft/t04/offline-build-output/build-log.json,internal/application/repository/craft_web_build_receipt.go,internal/container/craft_web_build.go,internal/container/craft_web_build_review.go,internal/modules/craft/web_contracts.go: rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> failed
- internal/application/repository/craft_run_capture_promotion.go,internal/container/craft_run_capture_promotion.go,internal/container/craft_run_capture_wiring.go,internal/modules/craftegress/adapter.go: rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> failed
- internal/application/repository/craft_stop_intent.go,internal/application/repository/craft_version.go,internal/application/repository/craft_workspace.go: rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> failed
- internal/modules/craft/archive.go,internal/modules/craft/input_code.go,internal/modules/craft/version.go: rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> failed
- apps/web/src/features/craft/routes.tsx,packages/api-client/src/craft/index.ts,packages/api-client/src/index.ts: rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> succeeded
- ... and 10 more

Per-attempt detail: --format json (retry_report).
