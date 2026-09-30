Review complete: 4 finding(s) across 5 selected item(s).

─── internal/container/craft_run_capture_wiring.go:204-207 ───
[bug · high] 恢复扫描的整轮预算与扫描间隔共用 interval(15s),且一轮内要顺序处理最多 100 张收据;而即时排空
craftCaptureAfterTerminalBudget 同样是 15s。一次捕获在 Seal 之前的工作量是:5 次
VerifyCraftCaptureQuiescent(每次两次全树走查,每次走查都全文读取并 sha256 所有文件)+ staging 走查 + 全量对象上传 —— 在配置允许的
MaxTotalBytes(默认 200MB)附近,仅走查就要反复读取 GB 级数据,15s 内几乎必然超时。两条路径预算完全相同意味着 drainCraftCaptureAfterTerminal
注释中"the periodic recovery scan retries anything this immediate pass could not
finish"的前提不成立:单张收据一旦需要超过 15s 才能到达 Seal,在两条路径上都会被 deadline 打断、每 15s
从头重做(重复走查与重复上传对象),收据永远无法封存,草稿头永不推进。建议为周期恢复扫描使用独立且明显大于即时预算的 scan budget(或按收据分配预算),而不是复用 interval。

+ const craftRunCaptureScanBudget = 5 * time.Minute
+ ...
  			case <-ticker.C:
- 				scanCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), interval)
+ 				scanCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), craftRunCaptureScanBudget)
  				r.Recover(scanCtx, 100)
  				cancel()


─── internal/container/craft_run_capture_wiring.go:166-168 ───
[performance · medium] AfterTerminal 拿到了刚结束 run 的 fence 却完全忽略,在 worker 执行器槽内同步排空全局 outbox:包括每次都执行
craftCaptureRecoveryInsertSQL 的多表 JOIN INSERT..SELECT(agent_runs × craft_run_views ×
craft_workspaces × craft_workspace_draft_heads),以及最多 32 张任意租户/工作区收据的完整捕获处理。该 executor seam 包装的是所有
graph run(不区分是否 craft run):run 吞吐较高的部署下,每次 run 完成(多个 worker 并发时是多路并发)都会叠加一轮全局恢复扫描,与 15s ticker
重复放大数据库与对象存储负载,并让无关 run 的执行器槽被占用至多 15s。建议至少利用 fence 定向排空本 run 的收据,或把全局恢复完全留给周期 ticker。

- func (r *CraftRunCaptureRunner) AfterTerminal(ctx context.Context, _ runtime.Fence) {
- 	r.Recover(ctx, 32)
- }
+ func (r *CraftRunCaptureRunner) AfterTerminal(ctx context.Context, fence runtime.Fence) {
+ 	r.Recover(ctx, 32) // TODO: 按 fence.RunID 定向排空本 run 的收据,避免每次 run 终态都触发全局恢复扫描


─── internal/container/craft_run_capture_wiring.go:177-180 ───
[maintainability · low] 该惰性分支在每次 AfterTerminal(即每个 graph run 终态后)都会执行:在未配置 RunView 装配的默认部署里,每个 run
完成都会输出一条重复的 "inert" Info 日志,形成持续的日志噪音。建议只在 Start 时输出一次(当前 Start 在 svc == nil 时静默返回,正好可以承载这条说明),或用
sync.Once 限流。

+ func (r *CraftRunCaptureRunner) Start(ctx context.Context) {
+ 	if r == nil {
+ 		return
+ 	}
  	if r.svc == nil {
- 		logger.Infof(ctx, "[CraftRunCapture] inert: %s", r.unavailable)
+ 		logger.Infof(ctx, "[CraftRunCapture] inert: %s", r.Unavailable())
  		return
  	}


─── internal/container/craft_runtime.go:283-290 ───
[bug · medium] Run-bound candidate 暂存的 OutputDir 契约不一致：这里把 Run-bound source 硬编码为
craftLocalOutputDir（"output"，其 ListSessionFiles 严格要求 dir=="output"），但 CollectCandidate →
stageAndUpload 实际传入的是共享 e.artifacts 服务的 config.OutputDir，而该服务在 newCraftRuntimeExecutor 中用
CRAFT_OPENCODE_OUTPUT_DIR 环境变量初始化。一旦部署将该环境变量设为非默认值，每次成功委派的 candidate 暂存都会在第一次 ListSessionFiles
即失败（ErrForbidden: artifact list request differs from the bound RunView），且失败仅记录 Warn（Execute 中
best-effort 吞掉），candidate 将永远无法暂存且无自愈路径。捕获协调器（newCraftRunCaptureRunner）为自己构造了固定 OutputDir:
craftLocalOutputDir 的独立服务，Execute 侧应对齐：为 Run-bound 候选路径使用固定 craftLocalOutputDir 配置的工件服务，或在 RunView
装配启用时拒绝非默认的 CRAFT_OPENCODE_OUTPUT_DIR。

- 	source, err := newRunBoundCraftArtifactSource(ctx, e, task, material, craftLocalOutputDir)
- 	if err != nil {
- 		return err
- 	}
- 	kind := e.sessionKind(ctx, task)
- 	if _, err := e.artifacts.CollectCandidate(ctx, task, kind, source, material.generation); err != nil {
+ 	// Option A: 构造专用于 Run-bound 候选暂存的服务，OutputDir 固定为 craftLocalOutputDir
+ 	// （与 newCraftRunCaptureRunner 的做法一致），避免继承 CRAFT_OPENCODE_OUTPUT_DIR：
+ 	runBoundArtifacts := service.NewCraftArtifactServiceWithCandidates(
+ 		closedCraftArtifactSource{}, e.files, e.versionsRef, e.candidates, nil,
+ 		service.CraftArtifactConfig{Kind: craft.KindWeb, OutputDir: craftLocalOutputDir})
+ 	if _, err := runBoundArtifacts.CollectCandidate(ctx, task, kind, source, material.generation); err != nil {
  		return err
  	}
+ 	// Option B: 在 newCraftRuntimeExecutor 装配时校验 outputDir == craftLocalOutputDir
+ 	// （RunView 路径启用时拒绝非默认 CRAFT_OPENCODE_OUTPUT_DIR），fail-fast 而非运行期静默降级。


LLM retry report summary: 3 of 90 requests affected -- 3 requests recovered after retry

Core review (3 requests):
- internal/container/agent_runtime.go,internal/container/container.go,internal/container/craft_run_capture_wiring.go,internal/container/craft_runtime.go,internal/container/craft_runview_material.go: rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> succeeded
- internal/container/agent_runtime.go,internal/container/container.go,internal/container/craft_run_capture_wiring.go,internal/container/craft_runtime.go,internal/container/craft_runview_material.go: rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> succeeded
- internal/container/agent_runtime.go,internal/container/container.go,internal/container/craft_run_capture_wiring.go,internal/container/craft_runtime.go,internal/container/craft_runview_material.go: rate limited (HTTP 429) -> rate limited (HTTP 429) -> succeeded

Per-attempt detail: --format json (retry_report).
