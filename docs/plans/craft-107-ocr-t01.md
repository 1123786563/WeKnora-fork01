Review complete: 3 finding(s) across 5 selected item(s).

─── internal/container/craft_run_capture_wiring.go:149-149 ───
[bug · high] resolveSource 构造的 Run-bound 源在构造时以及每次 ListSessionFiles/ReadSessionFile 都会经过
verifyBinding → RevalidateMaterialHandle → currentBinding（craft_runview_material.go:225-245 →
craft_runview_container_provider.go:869-918）：currentBinding 要求该容器的 binding 存在于本进程内存表
p.bindings（仅在容器启动路径 line 803 写入，进程重启后为空，无任何启动时重建），并执行
EnsurePrivateNetwork/InspectContainer/ProbeRuntime 引擎实时检查。后果：
1) 进程在 terminal commit 与捕获之间崩溃后重启——这正是 Start/Recover 注释明确声称要治愈的场景（"receipts stranded by workers that
died"、"a process that died ... is healed here too"）——所有遗留收据的 resolveSource 将永久失败（"container is not a
verified current provider binding"），每 15 秒重试并告警，永不收敛；终态 Run 不会再走 admission 路径，该 generation 的 binding
无法重建（每个 Run 独立 generation/容器）。
2) 容器被停止/删除后同样永久失败，草稿头永远无法推进。
这与 MaterialHandleForCapture 的契约（"intentionally does not require a running container ... sealed
generation output lives on the host independently of the container
lifecycle"）直接矛盾：句柄发放侧跳过活体检查，但源使用侧每次操作都要求活体。
此外 RecoverPending 按 created_at ASC + limit 选取，永久失败的收据会持续占据名额并队头阻塞后续可恢复收据。
建议：为捕获路径提供不做 live-binding 复验的源——verifyBinding 仅保留 admittedRunForCraftFence（epoch/快照栅栏）+
verifyMaterialHandle（盘上不可变身份字节校验），跳过 currentBinding 的引擎活体检查。

- 	source, err := newRunBoundCraftArtifactSource(ctx, &localCraftRuntime{db: r.db}, task, material, craftLocalOutputDir)
+ 	// 建议：为捕获路径使用仅做盘上身份复验的源构造，例如：
+ 	// source, err := newQuiescentCraftArtifactSource(ctx, &localCraftRuntime{db: r.db}, task, material, craftLocalOutputDir)
+ 	// 其 verifyBinding 仅调用 provider 的盘上身份校验（verifyMaterialHandle 语义）
+ 	// 与 admittedRunForCraftFence，不经 RevalidateMaterialHandle -> currentBinding
+ 	// （内存 binding 表 + running 容器 + ProbeRuntime）。


─── internal/container/agent_runtime.go:53-55 ───
[bug · medium] AfterTerminal 在 worker 执行器协程内同步运行且 context.WithoutCancel 无任何 deadline，同时忽略 fence
参数、全局排空最多 32 条任意收据：每条收据的每次列举/读取都会触发 verifyCurrent（EnsurePrivateNetwork + InspectContainer +
ProbeRuntime 引擎往返）、全量目录遍历、读取与对象上传。一旦引擎/数据库/对象存储卡死，执行该 Run 的 worker 并发槽位（默认 MaxWorkers=4，见
agent_run_worker.go DefaultWorkerConfig）被无限期占用，阻塞其他 Run 的执行；对比之下周期扫描路径（Start 中的 ticker 分支）已用
context.WithTimeout(..., interval) 限定了预算。另外多个 Run 同时终态时各 worker
会并发重复处理同一批收据（状态机幂等安全，但重复的引擎调用与上传会进一步放大阻塞时长）。建议给即时排空附加有限超时预算，并考虑只排空 fence 对应 Run 自己的收据。

  		if craftCapture != nil {
- 			craftCapture.AfterTerminal(context.WithoutCancel(ctx), fence)
+ 			drainCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), craftCaptureDrainBudget)
+ 			craftCapture.AfterTerminal(drainCtx, fence)
+ 			cancel()
  		}


─── internal/container/craft_run_capture_wiring.go:44-49 ───
[bug · medium] 两次遍历各自已在内部为每个文件计算 sha256 摘要并记录 device/inode/mtime/ctime 身份（craft_runtime.go:1806-1809
写入 s.listed），但这里的跨遍历比较只投射 Name/Path/Type/Size，把刚收集到的字节级证据丢弃了：两次遍历之间发生的同尺寸内容改写（in-place
覆盖、先写后截断等）不会被检出，与结构体注释声称的 "agree byte for byte" 不符；RemoteDirEntry.ModTime
字段也未被填充与比较。建议在第一次遍历后快照完整身份表（需在 runBoundCraftArtifactSource 上提供持锁的快照/比较辅助方法，避免绕过其内部 mutex 直接读
s.listed 造成数据竞争），第二次遍历后比较完整身份（digest/mtime/ctime/inode），使静默证明达到注释声称的强度。

- 	for i := range first {
- 		if first[i].Name != second[i].Name || first[i].Path != second[i].Path ||
- 			first[i].Type != second[i].Type || first[i].Size != second[i].Size {
- 			return fmt.Errorf("%w: RunView output entry %q changed during the quiescence proof", craft.ErrBusy, first[i].Path)
+ 	firstIDs := s.snapshotListed() // runBoundCraftArtifactSource 上持锁复制 s.listed（含 digest/mtime/ctime/inode）
+ 	second, err := s.ListSessionFiles(ctx, s.task.Scope.SessionID, s.outputDir)
+ 	if err != nil {
+ 		return fmt.Errorf("quiescence re-list: %w", err)
+ 	}
+ 	if len(first) != len(second) {
+ 		return fmt.Errorf("%w: RunView output entry set changed during the quiescence proof", craft.ErrBusy)
- 		}
+ 	}
+ 	if !s.listedMatches(firstIDs) {
+ 		return fmt.Errorf("%w: RunView output content changed during the quiescence proof", craft.ErrBusy)
  	}
+ 	return nil


LLM retry report summary: 4 of 83 requests affected -- 4 requests recovered after retry

Core review (4 requests):
- internal/container/agent_runtime.go,internal/container/container.go,internal/container/craft_run_capture_wiring.go,internal/container/craft_runtime.go,internal/container/craft_runview_material.go: rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> succeeded
- internal/container/agent_runtime.go,internal/container/container.go,internal/container/craft_run_capture_wiring.go,internal/container/craft_runtime.go,internal/container/craft_runview_material.go: rate limited (HTTP 429) -> succeeded
- internal/container/agent_runtime.go,internal/container/container.go,internal/container/craft_run_capture_wiring.go,internal/container/craft_runtime.go,internal/container/craft_runview_material.go: rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> succeeded
- internal/container/agent_runtime.go,internal/container/container.go,internal/container/craft_run_capture_wiring.go,internal/container/craft_runtime.go,internal/container/craft_runview_material.go: rate limited (HTTP 429) -> rate limited (HTTP 429) -> succeeded

Per-attempt detail: --format json (retry_report).
