Review complete: 50 finding(s) across 93 selected item(s).

─── apps/desktop/vite.config.ts:68-69 ───
[style · low] 新增的悬空空行：最后一个 alias 条目 `'@weknora/views'` 之后、闭合 `},` 之前多了一个空行，与文件内其余 alias
条目之间不留空行的风格不一致，属于无意义的 diff 噪音，建议删除。

        '@weknora/views': fileURLToPath(new URL('../../packages/views/src/index.ts', import.meta.url)),
- 
      },


─── internal/modules/craftegress/journal.go:230-252 ───
[bug · medium] Resolve 非定局分支的身份复活缺陷:并发同指纹双请求(AllocateIfNotParked 注释明确以此场景为威胁模型)下,请求1 复用 parked 身份 A
并成功获得 definitive resolve(unpark D→A),请求2 携同一身份 A 被网关 BeginBinding 冲突拒绝收到
409+ACTIVITY_UNRESOLVED(definitive=false)。当请求2 的 unknown Resolve 后落盘时,`!ok` 分支无条件把已在网关侧定局的 A 重新 park
回 digest 槽位——磁盘同样留下 A 的新 unresolved 尾巴。此后该指纹的每次重试都会复用已被网关绑定且定局的 A,网关永远返回 409,适配器永远保持 parked,形成永久 409
死循环,只能人工清理 journal 才能恢复;同时磁盘出现同 digest 的 A(resolved)+A(unresolved)双记录,重启 replay 按随机 map 迭代序取
states[A] 最后一条,50% 概率复活 A。修复建议:unknown Resolve 在内存中该 digest 已无 parked(或 parked 为其他 ID)时,不应盲目回填复活——网关
409 本身就表明该身份已绑定,应落盘为独立的对账标记状态而非回写 unresolved 索引;或至少在回填前校验该 attemptID 落盘的最新状态确为 unresolved。



─── internal/modules/craftegress/journal.go:266-269 ───
[bug · medium] 记录上限只在 AllocateIfNotParked 生效,appendLocked/Resolve 均不检查
maxCraftEgressJournalRecords:到达 cap 后新铸造确实 fail-closed(503),但已 parked 的指纹对持续故障网关的重试循环每轮都会追加一条
unknown Resolve 记录,日志在触顶后仍以客户端重试速率无限增长,违背该常量注释声称的"有界增长、超限 fail-closed"意图,长期运行可耗尽宿主磁盘,ordinals
索引亦只增不减加重 replay 内存。建议 appendLocked 内同样强制 j.records 上限(或对同 attemptID 的重复 unknown Resolve 合并/限频)。



─── internal/modules/craftegress/adapter.go:194-194 ───
[performance · low] 两个可避免的开销:(1) `strings.NewReader(string(body))` 将最多 16MB 的 []byte 额外拷贝为 string
才构造 Reader,直接 bytes.NewReader(body) 可零拷贝;(2) 同一请求体的 sha256 摘要在 mint 前计算一次,在响应处理阶段(readErr
分支与成功分支)又各重算一次,大报文高频转发下是三倍哈希开销——可在 ServeHTTP 开头计算一次 digest 复用。



─── cmd/craft-egress-adapter/main.go:31-32 ───
[other · medium] 集成缺口(前轮 OCR 已记录、本轮确认仍未闭合):全仓检索显示 CRAFT_EGRESS_* 环境变量与 craft-egress-adapter 二进制在
docker/craft(Dockerfile、runtime-config.json)及 internal/container(OpenCode provider 端点指向)中均无引用,生产代码中
craftegress 包的唯一消费者就是本命令。mint-before-send 协议在接线完成前整体不可达。ledger
文档(docs/plans/2026-09-23-craft-107-ledger.md)已将镜像烘焙与端点装配声明为 T20 待办——如实上报供集成工程师路由,合并前请确认 T20
与本变更集的落地顺序约束。



─── cmd/craft-egress-adapter/main.go:92-96 ───
[bug · low] ListenAndServe 失败(端口占用、权限等启动即失败场景)仅 log.Printf 后落入正常 shutdown 路径,main 以退出码 0
结束。容器编排与守护进程无法将其识别为失败:K8s onFailure 重启策略不会重启退出码 0 的进程,告警系统也会静默。建议在 shutdown 完成后以非零码退出(如 log.Fatalf 于
Shutdown 之后不可行则用 os.Exit(1) 标记 serveFailed)。



─── docker/craft/runtime-config.json:36-36 ───
[bug · critical] build_program 摘要双权威分歧：此处 build_program_sha256=02ad19f4… 与
docker/craft/web/toolchain.lock.json 的 build_program.sha256=900ae637… 不一致，但两份文件声明了同一个
toolchain_digest（7bc5aab…）。而 toolchain_digest 的推导公式（build.py 的 toolchain_digest() 与 Go 侧
CraftWebWebToolchainDigest 镜像实现）显式包含 build_sha——两个不同的 build sha 不可能推导出同一个 toolchain_digest，故至少一处 pin
为陈旧值。佐证：docs/testing/craft/t04/offline-build-output/build-log.json 是真实离线运行产物，其
toolchain_digest=7bc5aab 由经 lock 校验过的字节推导而来；而本轮 OCR 修复（EVENT_ATTR 接线、悬空标签哨兵、控制字符视图等）确实修改了
build.py，更新时漏掉了其中一侧（或两侧的 digest 均未重算）。后果是确定性的：Dockerfile 新增的构建期交叉断言 test "$rc_build" = "$lock_build"
无条件失败，镜像构建直接中断；若当前 build.py 实际字节与 lock 不符，则更早的 sha256sum -c / build.py --selftest / Go 测试
TestCraftWebToolchainPinsMatchShippedFiles 也会失败。修复：重新计算当前 build.py 的真实 sha256，同步更新
toolchain.lock.json 的 build_program.sha256、runtime-config.json 的 build_program_sha256，并重算两份文件中的
toolchain_digest，使三处推导（Python、Go、Dockerfile 断言）全部一致。

-     "build_program_sha256": "02ad19f473105039377006df8c29de4b95eb4bc175ffdf79b335c095b097787a",
+     "build_program_sha256": "<sha256sum docker/craft/web/build.py 的真实值，与 toolchain.lock.json 的 build_program.sha256 一致>",
+     "toolchain_digest": "<由更新后的 build sha 重算，与 toolchain.lock.json 同步>",


─── internal/modules/execution/sandbox/docker_normal_exec.go:200-203 ───
[bug · medium] CreateAttachedExec 中 ExecInspect(第 200 行)复用了 ExecCreate 已消耗的同一个 rpcCtx 剩余
deadline,而本文件 StartAttachedExecOnce 的 preflight inspect 注释明确将这种预算共享定性为已修复缺陷:"sharing the create
budget let a slow daemon leave only 残余 deadline for this cheap call, failing the whole creation
while the daemon keeps the already-created (never-started, lazy) exec behind"。同样的缺陷模式在 create
路径仍然存在:远程/慢速 daemon 下 ExecCreate 耗尽大部分 rpcTimeout 后,这个廉价的身份校验 inspect 在残余 deadline 内失败,导致:(1)
CreateAttachedExec 整体报错,整个 normal exec 创建失败;(2) daemon 侧残留一个已创建、永不启动的惰性 exec(直到容器销毁才清理),同容器内可累积。建议为该
inspect 单独建立 rpcTimeout 预算,与 preflight inspect 的修复方式对齐。

- 	inspected, err := c.api.ExecInspect(rpcCtx, receipt.ExecID, client.ExecInspectOptions{})
+ 	inspectCtx, inspectCancel := context.WithTimeout(ctx, c.rpcTimeout)
+ 	defer inspectCancel()
+ 	inspected, err := c.api.ExecInspect(inspectCtx, receipt.ExecID, client.ExecInspectOptions{})
  	if err != nil {
  		return DockerNormalExecReceipt{}, dockerError("NormalExecCreateInspect", err)
  	}


─── internal/modules/execution/sandbox/docker_restricted_exec.go:137-139 ───
[maintainability · low] ObserveOutputlessExec 在 ExecInspect 返回的 ID/ContainerID 与 receipt 不一致时静默返回
(unknown, nil):无错误、无日志。同包 normal exec 路径(inspectRaw)对完全相同的场景返回 ErrDockerNormalExecIdentityMismatch
错误。后果:调用方 CraftDockerRestrictedExec.Observe 对 err==nil 且 unknown 状态既不进入 claimed-never-started
对账分支,也永不收敛终态——一个身份错位的 receipt(如 claim 阶段记录错位)在有 running 证据时会无限挂起 Unknown,且错位完全不可观测。建议与 normal exec
对齐返回显式错误(或至少 Warn 日志),保持权威观察面的可观测性一致。

  	if inspected.ID != receipt.ExecID || inspected.ContainerID != receipt.ContainerID {
- 		return unknown, nil
+ 		return unknown, fmt.Errorf("restricted Docker exec inspect identity mismatch: got exec %s container %s, want exec %s container %s",
+ 			inspected.ID, inspected.ContainerID, receipt.ExecID, receipt.ContainerID)
  	}


─── internal/modules/craftegress/adapter.go:237-241 ───
[bug · medium] journal.Resolve 失败被完全静默吞掉且无任何日志(本处 `_ = err` 及 readErr/超限分支的 `_ =
a.journal.Resolve(...)` 两处)。后果链并非注释声称的 "reconciles safely on the next pass":definitive resolve
落盘失败(磁盘满、fsync 错误、Close 竞争)后内存与磁盘均无 resolved 记录,attempt 停留 parked;同指纹重试复用该 id 转发,网关 BeginBinding 因该
activity 已定局返回 ErrConflict → 409 ACTIVITY_UNRESOLVED → 适配器按 unknown 继续停靠,死锁至人工清 journal,而全链路(网关侧有
logger.ErrorWithFields,适配器侧 ServeHTTP 无任何日志)零告警。至少应记录日志暴露 resolve 失败;若能区分 definitive 场景,注释也应修正误导性的
"safely" 断言。

  		if err := a.journal.Resolve(attemptID, digest, resp.StatusCode, definitive); err != nil {
- 			// The response was observed; a failed resolution record leaves the
- 			// attempt reusable, which reconciles safely on the next pass.
- 			_ = err
+ 			// The attempt stays parked on failure; same-fingerprint retries will
+ 			// reuse it and can deadlock on the gateway's 409 — surface it.
+ 			log.Printf("craftegress: journal resolve failed (attempt %s, definitive=%v): %v", attemptID, definitive, err)
  		}


─── internal/application/service/craft_docker_restricted_exec.go:321-323 ───
[maintainability · medium] claimed-never-started 收敛判定依赖对 Docker
守护进程错误消息的子串匹配,而这里拿到的错误其实已经是结构化的:provider 端 ObserveOutputlessExec 的错误经 dockerError 包装为
*sandbox.RemoteError,其 Kind 已由 dockerErrorKind 通过 cerrdefs.IsNotFound
精确分类(docker_engine.go:336-364)。子串匹配有两个方向的风险:守护进程措辞随版本漂移(如 exec 404
措辞变化、经由代理/翻译的错误文本)时收敛静默失效,claimed-never-started 的 exec 永久停留 Unknown;反之任何被包装的错误文本恰好包含该子串时会被误收敛为
Failed 终态。建议改用 errors.As + Kind == sandbox.RemoteErrorKindNotFound 判定,既覆盖 'No such container' 与 'No
such exec instance' 两种现措辞,也免疫措辞漂移。

- 		if !previouslyRunning && (strings.Contains(err.Error(), "No such container") || strings.Contains(err.Error(), "No such exec")) {
+ 		var remoteErr *sandbox.RemoteError
+ 		if !previouslyRunning && errors.As(err, &remoteErr) && remoteErr.Kind == sandbox.RemoteErrorKindNotFound {
  			return sandbox.DockerOutputlessExecObservation{State: sandbox.DockerOutputlessFailed, OutputAvailable: false}, nil
  		}


─── internal/application/service/craft_docker_normal_exec.go:354-360 ───
[documentation · low] 该 claimed-never-started 对账分支在唯一的生产 provider
下不可达:DockerNormalExecClient.ObserveAttachedExec 在 positiveStartEvidence=false 且非 Running 时始终返回
Unknown(docker_normal_exec.go:434-436),绝不返回 Succeeded/Failed,而本分支条件要求 !startEvidence 且 State 不是
Running/Unknown——恒为假。后果是注释承诺的收敛("replays leave the Unknown limbo instead of parking
forever")实际永远不会发生,claimed-never-started 的 exec 仍会永久 Unknown;同时该分支内还封装了激进假设(伪造 Transport=Complete、强制
output.Unavailable=true),若未来某个 provider 实现真的在无 start evidence 时返回终态,该分支会把一个已启动并产出 sealed 输出的 exec
误报为从未启动、隐藏其可用输出。建议删除该不可达分支并修正注释,或先与 provider 明确扩展 ObserveAttachedExec 的契约再实现收敛。

- 	if observeErr == nil && !startEvidence && process.State != sandbox.DockerNormalExecProcessRunning && process.State != sandbox.DockerNormalExecProcessUnknown {
- 		// The provider observed a DEFINITIVE not-running state with no
- 		// start evidence: the exec never started (claim consumed, process
- 		// crashed before Start). Report the definitive failed observation —
- 		// replays leave the Unknown limbo instead of parking forever.
- 		output, cursor, _ := s.readOutput(context.WithoutCancel(ctx), normalOutputScope(request, receipt))
- 		output.Unavailable = true // no output row can exist for a never-started exec
+ 	// (删除整个 observeErr == nil && !startEvidence && 终态 的不可达对账分支;)
+ 	// 保留后续终态/未知观察处理,并在注释中如实说明:provider 在无
+ 	// positive start evidence 时对非 Running 状态只返回 Unknown,
+ 	// claimed-never-started 的恢复收敛需要 provider 契约先扩展。


─── internal/application/repository/craft_docker_send_claim.go:203-206 ───
[maintainability · low] normalInputExists 读取的是输入域表 craft_docker_normal_inputs,基础设施错误却用输出域哨兵
dockerOutputDBError(ErrCraftDockerOutputUnavailable)包装。这与 craft_docker_normal_input.go 中
dockerNormalInputDBError 的存在理由及其注释("not the output-domain one) so diagnostics are not misread as
output-store outages")直接相悖:该表故障会被运维误读为输出存储不可用。同文件 load() 对 journal 表错误同样使用
dockerOutputDBError。建议输入域表错误改用 dockerNormalInputDBError,journal 表错误可沿用 craft.ErrConflict 或引入专属哨兵。

  	err := db.WithContext(ctx).Table("craft_docker_normal_inputs").Where("tenant_id = ? AND run_id = ? AND activity_key = ?", key.TenantID, key.RunID, key.ActivityKey).Count(&count).Error
  	if err != nil {
- 		return false, dockerOutputDBError(err)
+ 		return false, dockerNormalInputDBError(err)
  	}


─── internal/application/repository/craft_docker_normal_input.go:94-97 ───
[documentation · low] 描述 Stage 幂等语义的文档注释("Stage creates the encrypted request before any Docker
create call...")被错误地放置在 dockerNormalInputDBError 之上,读者会误以为这是错误包装函数的契约说明,而真正的 Stage 方法反而没有文档。应将这段注释移到
func (r *CraftDockerNormalInputRepository) Stage 的上方,并为 dockerNormalInputDBError 保留其自身的说明。

- // Stage creates the encrypted request before any Docker create call. Replaying
- // an exact canonical request is idempotent; a changed request cannot inherit
- // the operation's admission/hold.
  // dockerNormalInputDBError wraps infrastructure failures of the normal-input
+ // chain with the INPUT-domain unavailable sentinel (not the output-domain
+ // one) so diagnostics are not misread as output-store outages.
+ // (并将 Stage 的幂等语义注释移至 Stage 方法声明上方:)


─── docker/craft/web/build.py:312-312 ───
[other · low] aria-labelledby 的语义被误用为 table→input 的配对数据通道：<table aria-labelledby="{filter_id}">
指向的是筛选输入框，而 craft-web.js 的 bindTable 正是靠 table.getAttribute("aria-labelledby") 找回这个 id。按 WAI-ARIA
语义，aria-labelledby 应引用为元素提供可读名称的元素：这样设置后，屏幕阅读器会把表格的可访问名称解析为输入框的 aria-label（"筛选表格行"），真正的表格标题（紧邻的
<h2>）反而被覆盖忽略，每个表格对辅助技术用户都失去真实名称。建议让配对关系由非 ARIA 属性承载（例如 build.py 在 <input> 上生成
data-craft-filter-for="{table_id}"，craft-web.js 改读该属性；input 上已有的 aria-controls 也可反查），并让 table 的
aria-labelledby 指向 h2 的 id 或直接省略，使筛选绑定与无障碍语义解耦（js 中 bindTable 的注释所述绑定依据也需同步改为新属性）。



─── internal/application/service/craft_execution_policy.go:99-101 ───
[security · critical] 安全门面无生产装配点（CRITICAL）：全仓库检索确认 NewCraftDelegateExecutionPolicy 与其唯一消费接口
WithExecutionPolicy（craft_docker_normal_exec.go:63 / craft_docker_restricted_exec.go:59）仅在测试
craft_execution_policy_wiring_test.go 中被调用，internal/container 的装配（container.go 等）从未构造该门面，也未把
CraftExecutionPolicyGate 挂接到任何 Docker exec 服务。后果：CraftDockerNormalExecService.Execute /
CraftDockerRestrictedExec 在 s.policy == nil 时按注释 "nil keeps the unwired legacy behavior" 直接放行，文件头声明的
"Both seams run BEFORE any create, bind, claim or send" 在任何部署路径上都不会发生——T03「上传材料仅作数据不执行」(#122)
的防线实际未生效，仅存在测试覆盖。修复方向二选一：(1) 在 central assembly（internal/container）中构造 gate 并 WithExecutionPolicy
挂接到两个命令面（WithAuditLog 一并挂接，否则 writeDenied 审计行永不落库）；(2) 若装配属于后续 ticket，则应在消费侧把 nil gate 改为
fail-closed（拒绝执行而不是 legacy 放行），避免当前提交状态形成一个已写完但未布防的安全缝隙。

- // NewCraftDelegateExecutionPolicy validates and assembles the gate adapter.
- // workspaceRoot is the container workspace root (for example "/workspace").
- func NewCraftDelegateExecutionPolicy(delegate *CraftDelegateService, db *gorm.DB, workspaceRoot string) (*CraftDelegateExecutionPolicy, error) {
+ // internal/container 装配示意（确保门面真正进入生产路径）：
+ // gate, err := NewCraftDelegateExecutionPolicy(delegate, db, craftWorkspaceRoot)
+ // if err != nil { ... }
+ // gate = gate.WithAuditLog(audit)
+ // normalExec.WithExecutionPolicy(gate)
+ // restrictedExec.WithExecutionPolicy(gate)
+ // 或者：在 CraftDockerNormalExecService/CraftDockerRestrictedExec 的 Execute 入口
+ // 将 s.policy == nil 视为 craft.ErrForbidden（fail-closed），删除 "unwired legacy behavior" 放行分支。


─── internal/modules/craft/archive.go:333-335 ───
[security · high] pax 扩展头条目（'x'/'g'）绕过了全部资源上限，构成 CPU DoS 缝隙：这两个 case 在 reserveEntry 和 readMember
之外直接 continue，其解压出的字节与条目数不进入任何预算——seen 上限（10×MaxArchiveEntries=200）只覆盖 dir/reg 条目，b.total
与压缩比只累计成员内容字节，pax 记录的 header 块与记录体完全不计数。攻击路径：输入端仅受 MaxInputBytes（20MiB，craft_archive.go:93
已确认）约束，一个全零内容的 tar.gz（gzip 比约 1000:1）可解出数十 GB 的纯 TypeXGlobalHeader 流，迫使单个 goroutine
进行数十秒到分钟级的解压+头解析后才在 finish() 被拒（拒绝发生在整流走完之后，CPU 已消耗完毕）；并发上传可直接耗尽 worker。这同时使
MaxArchiveExtractDuration 注释中"每个展开字节 O(1)、字节总量有界 ⇒ 时长有界"的论证落空——头字节并未被计入任何总量。建议：在 gzip 流外包一层计数
Reader，把所有解压字节（含 tar 头块与 pax 记录体）计入 MaxArchiveExpandedBytes 与压缩比预算，并为任意类型的条目头维护一个总条目计数上限（x/g
头同样计入），使每个解压字节真正 O(1) 且总量封顶。

- 		case tar.TypeXHeader, tar.TypeXGlobalHeader:
- 			// Extended pax records are metadata consumed by the reader.
- 			continue
+ // countingReader 让所有解压字节（含头与 pax 记录体）进入预算
+ type countingReader struct {
+ 	r io.Reader
+ 	n int64
+ }
+ 
+ func (c *countingReader) Read(p []byte) (int, error) {
+ 	n, err := c.r.Read(p)
+ 	c.n += int64(n)
+ 	return n, err
+ }
+ 
+ // 在 extractTarArchive 内每个 header（含 x/g）处：
+ // if err := budget.chargeStream(counted.n); err != nil { return nil, err }
+ // chargeStream 校验 n ≤ MaxArchiveExpandedBytes 且 n ≤ compressed*MaxArchiveCompressionRatio，
+ // 并对任意类型的条目头计入总条目上限。


─── internal/modules/craft/archive.go:39-42 ───
[bug · medium] MaxArchiveExtractDuration 的"约束单次展开墙钟"契约在本文件内没有任何执行点：ExtractArchive 与两个 extractor 均不接收
ctx/deadline，循环中无任何时间检查（code_search 确认该常量在包内无使用点）。唯一生产调用方 craft_archive.go:52 的 context.WithTimeout
只能约束其自身的 DB/文件 IO，ExtractArchive 的 CPU 解压一旦开始便不可取消——30s
死线对解压过程本身不生效，纯靠字节上限间接推导时长（而如上一条所述，该推导当前存在头字节漏洞）。建议为 ExtractArchive 增加 ctx 参数并在 zip/tar 条目循环中周期检查（如每
64 个条目查一次 ctx.Err()），或将常量迁至服务层并修正此处注释使契约与实现一致。

- 	// MaxArchiveExtractDuration bounds the wall clock of one expansion,
- 	// which bounds the CPU an archive can consume: each expanded byte costs
- 	// O(1) work and both byte totals are capped above.
- 	MaxArchiveExtractDuration = 30 * time.Second
+ func ExtractArchive(ctx context.Context, archive []byte) ([]ArchiveMember, error) {
+ 	// 循环内（每 N 个条目）：
+ 	if err := ctx.Err(); err != nil {
+ 		return nil, err
+ 	}


─── internal/modules/craft/export_manifest.go:295-301 ───
[security · medium] 两段冒号防御都只检查首段 SplitN(path, "/", 2)[0]，而内层注释自述的威胁样例
"foo/c:evil"（盘符位于路径中段）恰好落在覆盖之外：其首段 "foo" 不含 ':'，外层条件 strings.Contains(first, ":")
为假，整块跳过，成员被放行打包——与注释承诺（"which Windows still resolves" 应被拒绝）直接矛盾。已核实前置
ValidateArtifactPath（version.go）不拒绝冒号，此处是唯一关卡；打包出的 zip 条目 "foo/c:evil" 在 Windows 解包时可被解析为盘符相对路径或
NTFS ADS（c:evil → 文件 c 的流 evil），属 zip-slip 变体。此外第二块的内层条件 strings.Contains(first, ":")（外层已保证）与
!strings.Contains(first, "/")（first 已按 / 切分，恒真）是重复求值，唯一增量是排除首字符 ':'，进一步暴露逻辑拼凑。建议改为逐段判定盘符形态并简化：

- 		if strings.Contains(member.Path, ":") && strings.Contains(strings.SplitN(member.Path, "/", 2)[0], ":") {
- 			if first := strings.SplitN(member.Path, "/", 2)[0]; strings.Contains(first, ":") && first[0] != ':' && !strings.Contains(first, "/") {
- 				// Any remaining colon in the FIRST segment (foo/c:evil put
- 				// the drive spec mid-path, which Windows still resolves).
- 				return fmt.Errorf("%w: bundle member %q first segment carries a drive separator", ErrInvalidInput, member.Path)
+ // 任意路径段形如 "<盘符>:…"（c:/evil、c:evil、foo/c:evil）均拒绝；首段任何位置的冒号亦拒绝。
+ for _, seg := range strings.Split(member.Path, "/") {
+ 	if len(seg) >= 2 && seg[1] == ':' && isDriveLetter(seg[0]) {
+ 		return fmt.Errorf("%w: bundle member %q uses a drive-letter path", ErrInvalidInput, member.Path)
+ 	}
- 			}
+ }
+ if first := strings.SplitN(member.Path, "/", 2)[0]; strings.Contains(first, ":") {
+ 	return fmt.Errorf("%w: bundle member %q first segment carries a drive separator", ErrInvalidInput, member.Path)
- 		}
+ }


─── internal/modules/craft/export_manifest.go:306-312 ───
[maintainability · low] 保留名检查存在两处死代码/冗余：reserved 的三个键本身全小写，strings.ToLower(TrimRight(path, " ."))
归一化后的 reserved[normalized] 查找已是精确匹配 reserved[member.Path] 的严格超集（大小写不敏感 +
尾部点/空格归一），后者永不触发；reservedFold(member.Path, reserved) 又对同一归一化结果做带 ToLower 的全表循环，与
reserved[normalized] 完全重复。三重检查给读者传递"三者各防一类"的错误信号，实为一个判定。建议只保留 reserved[normalized] 一处，删除
reservedFold 与精确匹配分支。

  		normalized := strings.ToLower(strings.TrimRight(member.Path, " ."))
- 		if reserved[normalized] || reservedFold(member.Path, reserved) {
- 			return fmt.Errorf("%w: bundle member %q collides with a fixed bundle document", ErrInvalidInput, member.Path)
- 		}
- 		if reserved[member.Path] {
+ 		if reserved[normalized] {
  			return fmt.Errorf("%w: bundle member %q collides with a fixed bundle document", ErrInvalidInput, member.Path)
  		}


─── internal/modules/craft/citation.go:172-177 ───
[bug · low] 用 decoder.More() 检测尾随内容无法覆盖所有形态：encoding/json 的 More() 在下一非空白字节为 ']' 或 '}' 时返回 false，因此
`{...}}`、`{...]` 这类带杂散闭合括号的清单会被静默接受，与注释"Trailing content after the first JSON document is
refused"的严格契约不符。这是不可信模型输出的拒收边界，shape 校验虽兜底但契约应当自洽。建议改为再 Decode 一次并要求精确的 io.EOF：

- 	if decoder.More() {
- 		// Trailing content after the first JSON document ({...}garbage, or a
- 		// second shadow document) is refused: the manifest is untrusted
- 		// model output and strictness here is part of that contract.
+ 	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
  		return WebCitationManifest{}, fmt.Errorf("%w: web citation manifest has trailing data", ErrInvalidInput)
  	}
+ // 需补 "io"（及 errors.Is 所需的 errors）导入。


─── internal/modules/craft/share.go:64-64 ───
[maintainability · low] TenantID==0 的来源被默认判为非受限（fail-open），且与同包判定语义相反：export_manifest.go:153 对同一事实用
source.TenantID != tenantID（0 值会被 fail-closed 标记
restricted），而此处在同意路径上取开放方向。已核实当前唯一生产写入路径（craft_knowledge.go:278 的记录构造，其上游
craft_knowledge_publisher.go:515 的清单校验拒绝 source.TenantID==0）使持久化记录中 0
值不可达，故今日不可触发；但若未来新增记录写入点遗漏该不变量，受限派生作品将在无 Owner 同意的情况下直接获得共享授权（同意绕过是最坏方向）。建议与 export 侧对齐——去掉 != 0
守卫（0 ≠ Scope.TenantID 自然判 restricted，fail-closed），或在判定前显式校验来源记录完整性。

- 		if source.TenantID != 0 && source.TenantID != record.Scope.TenantID {
+ 		// 与 BuildExportManifest 的 fail-closed 语义对齐：0 值（不可达但不允许静默放行）
+ 		// 亦视为非本租户来源。
+ 		if source.TenantID != record.Scope.TenantID {


─── internal/application/service/craft_artifacts.go:378-378 ───
[maintainability · low] publishWithEvidence 直接调用 time.Now().UTC() 生成被固定（pin）到不可变版本上的证据时间戳，而
CraftArtifactService 没有注入时钟字段。同批的 CraftShareService/CraftExportService 均通过 cfg.Now 注入时钟（且 T12 OCR
修复正是把 auditExport 的 time.Now() 改为 s.now()
以获得确定性测试），此处的偏离使该持久化证据时间戳无法在测试中固定，且同一提升事件的证据时间与审计时间戳可能取自不同时钟读取。建议按同一纪律在 CraftArtifactConfig 中注入 Now
func() time.Time（缺省 time.Now）。

- 		evidence, err := craft.PinVersionEvidence(v.ID, record, time.Now().UTC())
+ // CraftArtifactConfig 增加: Now func() time.Time（withDefaults 缺省 time.Now）
+ 		evidence, err := craft.PinVersionEvidence(v.ID, record, s.config.Now().UTC())


─── internal/application/repository/craft_docker_send_claim.go:239-242 ───
[maintainability · medium] load 读取的是 craft_charge_start_journal(发送认领/计费日志表),基础设施错误却被
dockerOutputDBError 包装为 ErrCraftDockerOutputUnavailable("durable Docker output is
unavailable")。这与已确认的 normalInputExists
问题同类但位置不同:bindDockerExecReceipt/claimDockerExecSend/RecordDockerExecEventPair 在 CAS 落空后的兜底诊断都经由
load,一次 journal 表故障(如 PG 连接抖动)会被运维误读为输出存储不可用,而实际输出链路完全健康。修复已确认项时请一并为本函数改用发送域的不可用哨兵(或直接以 %w
包装原始错误),避免只改 normalInputExists 留下同样的误诊点。

  	if errors.Is(err, gorm.ErrRecordNotFound) {
  		return craftDockerSendClaimRow{}, craftDockerSendConflict("operation not found")
  	}
- 	return row, dockerOutputDBError(err)
+ 	if err != nil {
+ 		return craftDockerSendClaimRow{}, fmt.Errorf("craft docker send claim store unavailable: %w", err)
+ 	}
+ 	return row, nil


─── internal/application/service/craft_docker_restricted_exec.go:132-136 ───
[maintainability · low] ResumeBound 用零值 CraftCallBinding{}/CraftDockerOutputlessRequest{}
调用策略门,但被恢复的持久化命令并不在这两个参数里——这次"筛查"实际没有筛查任何东西,当前之所以 fail-closed,仅仅因为唯一实现
CraftDelegateExecutionPolicy.ReviewOutputlessExec 无条件拒绝、忽略入参。一旦后续有人按接口注释("screens one restricted
(outputless) exec")实现真正检查 request 的门,零值请求(空
Command/空身份)既可能被当作"无可拒绝项"放行,也可能被当作无效输入拒绝——前者会静默反转本注释承诺的"an unreviewable command is never sent, from
either path"。建议改为显式的恢复路径筛查契约(例如按 grantID/activityKey 的 durable 身份做 fail-closed
审查的方法),让安全性不依赖于"门恰好忽略参数"这一巧合。



─── internal/modules/craft/archive.go:261-266 ───
[performance · medium] zip 路径的全部条目上限（MaxArchiveEntries=20、seen 集 200）只在迭代 reader.File 时才生效，而
zip.NewReader 在循环开始前就把整个中央目录一次性解析并物化为 []zip.File。攻击路径：输入端仅受 MaxInputBytes（20MiB）约束，一个由最小尺寸目录记录（46
字节定长 + 空文件名）填满的 zip 可携带约 45 万条条目，在第 201 次 reserveEntry 触顶报错之前，NewReader 已为每条记录分配一个 File+FileHeader（约
170-200B 堆内存），瞬时分配约 80-100MB、约为输入体积的 4-5 倍，随后整个请求才被拒绝——零有效工作换取与文档化的 MaxArchiveExpandedBytes（100MiB
"the extraction's memory ceiling"）同量级的额外内存。这与 reserveEntry 注释中 "A malicious central directory still
cannot name hundreds of thousands of entries" 的断言不符：该断言只对 seen
集合计数成立，对解析期已物化的条目切片不成立；并发恶意请求可线性放大该瞬时分配。tar 路径无此问题（流式逐条解析）。建议：archive/zip 的 NewReader API
无法在物化前限制条目数，可改为自行用 ReaderAt 流式步进中央目录记录、先计数触顶即拒（超过 10×MaxArchiveEntries
直接报错）再逐条解析；或至少在文档中显式承认并界定这一有界放大，修正 reserveEntry 的注释断言。



─── internal/modules/craft/input_code.go:353-359 ───
[security · high] 「run」子命令位移跳过会绕过脚本操作数筛查。`arg == "run" && !pendingRun` 直接 continue，未对该操作数做
canonical/withinInputs 筛查，且对所有解释器（不止 deno/bun）生效。可达路径：成员上传名为 `run` 的无扩展名文件（ValidateInputName 允许，落盘于
inputs/<sha256>/run），Agent 以 WorkingDir 指向 inputs 树内目录（如 /workspace/inputs/<sha256>）staged 一个 normal
exec，Command=["python3","run"]（bash/sh/node 同理）——生产端唯一门禁 CraftDelegateExecutionPolicy 不填
ResolvedTargetPath/TargetSHA256，各词法层均不命中：前缀筛查只覆盖 Command[:offset]=["python3"]，rest=["run"] 被本分支跳过，末尾
TargetPath 防御检查为空，最终判定 Allowed，解释器直接读取并执行上传字节（无需可执行位）。建议跳过前对 "run" 本身做树内筛查——deno/bun 的合法 `run`
子命令在正常 cwd 下解析在树外，不受影响。

  				// run subcommands (deno run x.ts, bun run x.js) shift the
- 				// executed file one operand later; remember and keep
- 				// treating the remainder as the option/operand region.
+ 				// executed file one operand later — but a "run" that itself
+ 				// resolves inside the read-only tree is an uploaded script,
+ 				// not a subcommand: screen it before skipping.
  				if arg == "run" && !pendingRun {
+ 					if abs := p.canonical(req.WorkingDir, arg); p.withinInputs(abs) {
+ 						return p.deny("interpreter_input", abs, "")
+ 					}
  					pendingRun = true
  					continue
  				}


─── internal/modules/craft/input_code.go:293-297 ───
[security · medium] Shell 分支的词法筛查未对 flag 附值与冒号分隔段做拆解，与 argv 分支本轮已显式封堵的走私面（flagValueCandidates、env
值冒号分段）不一致：带证据放行的 shell 形态中，`tool --file=inputs/x.sh` 的 token "--file=inputs/x.sh" 经 canonical 后因
"--file=" 前缀不落树内；`PATH=/usr/lib:inputs x gen.sh` 剥离 VAR= 后的整 token "/usr/lib:inputs"
也不落树内——两者均逃过筛查。当前生产门禁不构造 Shell 请求（CraftDockerNormalInputRequest 为纯 argv，`sh -c` 以 argv 到达且被 -c
程序文本规则拒绝），故为潜伏缺口；但 ResolvedTargetPath/TargetSHA256 证据缝正是为未来具备文件系统观测力的 adapter
预留的，且该证据只锚定主目标，复合表达式中次级命令仅靠本词法层兜底，一旦接线即成活口。建议对每个 token 追加 flagValueCandidates
与冒号分段筛查（过度生成只会增加拒绝，与既有各层同一安全方向）。

  		for _, token := range shellTokens(req.CommandText) {
  			if abs := p.canonical(req.WorkingDir, token); p.withinInputs(abs) {
+ 				return p.deny("shell_input", abs, "")
+ 			}
+ 			// Attached flag values and colon-separated path segments ride
+ 			// the same token: screen them exactly like the argv branches.
+ 			if strings.HasPrefix(token, "-") && token != "-" {
+ 				for _, value := range flagValueCandidates(token) {
+ 					if abs := p.canonical(req.WorkingDir, value); p.withinInputs(abs) {
+ 						return p.deny("shell_input", abs, "")
+ 					}
+ 				}
+ 			}
+ 			for _, segment := range strings.Split(token, ":") {
+ 				if abs := p.canonical(req.WorkingDir, segment); p.withinInputs(abs) {
- 				return p.deny("shell_input", abs, "")
+ 					return p.deny("shell_input", abs, "")
+ 				}
  			}
  		}


─── internal/application/repository/craft_version.go:329-339 ───
[bug · low] sameCraftVersionEvidence 用 != 直接比较含 time.Time 的字段（顶层 AcquiredAt 及 Sources
逐元素结构体比较）。time.Time 的 == 对 Location 表示敏感：非零偏移的 RFC3339 字符串两次独立反序列化会得到各自新分配的 FixedZone
指针，同一时刻也判不等。当前所有写入点均经 .UTC() 归一（craft_knowledge.go 的 s.now().UTC()），JSON 序列化为 "Z"
形式、往返表示稳定，重放收养测试可正常通过——但该不变量仅为约定，任何未来写入点（或注入时钟）漏掉 .UTC()，相同事实的重放就会误报
ErrConflict，破坏注释承诺的幂等收养语义且难以排查。属潜伏缺陷，建议改用 time.Time.Equal 并对 Sources 逐字段比较。

  	if a.VersionID != b.VersionID || a.RunID != b.RunID ||
  		a.RequestDigest != b.RequestDigest || a.PackageDigest != b.PackageDigest ||
- 		a.AcquiredAt != b.AcquiredAt || a.Empty != b.Empty || a.Truncated != b.Truncated ||
+ 		!a.AcquiredAt.Equal(b.AcquiredAt) || a.Empty != b.Empty || a.Truncated != b.Truncated ||
  		len(a.Sources) != len(b.Sources) {
  		return false
  	}
  	for i := range a.Sources {
- 		if a.Sources[i] != b.Sources[i] {
+ 		x, y := a.Sources[i], b.Sources[i]
+ 		if x.ID != y.ID || x.Ref != y.Ref || x.Digest != y.Digest || x.TenantID != y.TenantID ||
+ 			x.ExcerptBytes != y.ExcerptBytes || !x.AcquiredAt.Equal(y.AcquiredAt) {
  			return false
  		}
  	}


─── internal/modules/craft/input_code.go:550-553 ───
[maintainability · low] wrapperCommands 收录了 nohup，但 wrapperValueFlags 缺其条目，依赖 nil map 读取恒为 false
才获得与空表（setsid）等价的 fail-closed 行为。无功能缺陷，仅一致性/可读性：显式补 "nohup": {} 可避免后续维护者误判为遗漏并补入错误符号表。

  	"xargs":   {"-I": true, "-D": true, "-E": true, "-n": true, "-P": true, "-s": true},
  	"stdbuf":  {"-i": true, "-o": true, "-e": true},
  	"setsid":  {},
+ 	"nohup":   {},
  }


─── internal/application/repository/craft_run_capture.go:323-325 ───
[bug · high] 新鲜度门禁存在状态覆盖缺口，并发重做防护在设计目标场景下即失效：gate 只对 state='capturing' 生效，而即时 drain（AfterTerminal →
RecoverRun，15 秒预算，同步运行在 worker executor goroutine）从 receipt 仍是 pending 时就开始工作——resolveSource +
VerifyCraftCaptureQuiescent（两次全树 sha256 遍历）+ stageCapture（全量读文件）全部发生在 BeginCapture 第一次写
state/updated_at 之前，大树耗时轻松超过数秒。ticker（15 秒间隔）的 recoverPending 对 pending 态无条件放行（本 gate 明确豁免
pending），因此 drain 存活的 15 秒内几乎必然有一个 tick 拾取同一 pending receipt 并发驱动：两路 BeginCapture 因 digest
相同而幂等共存（craft_run_capture.go:269-277 不拒绝重入），两路各上传全量物理对象（ObjectRef 各自新生成），一路 Seal 成功后另一路因
sameCaptureFiles 比较 craft.File 全字段（含 Ref，craft_run_capture.go:586）而返回
ErrConflict，败者上传的对象全部成为孤儿资源行——这正是 craftCaptureDrainWindow 注释声称 gate 已防止的后果（"each redo uploads fresh
physical objects; the loser's uploads leak"），但 gate 未覆盖耗时最长的 pending 前处理窗口。draft 不会双推进（Advance CAS +
MarkAdvanced 幂等），但该泄漏在常态路径（几乎每个 Run 终态一次）发生。另外两点同源失配：(a) 30 秒窗口远小于周期扫描 5
分钟预算（craftRunCaptureScanBudget）内单 receipt 的处理时长，上传超过 30 秒后其他执行流（多副本 ticker）即可重入同一 capturing
receipt；(b) RecoverPendingForRun（drain 侧）完全没有 gate，终态事件重放/重试时两个 drain 也会并发驱动同一 receipt。建议：任何执行流在进入
quiescence/staging 之前先做一次 claim 写（例如把 BeginCapture 的状态推进提前为"领取"语义，pending→capturing 并刷新
updated_at），让 freshness gate 对 pending 同样按 updated_at 窗口生效；对长上传在上传循环内周期性心跳续租
updated_at，使窗口只需大于心跳间隔而非总处理时长；RecoverPendingForRun 复用同一 gate。

  		return tx.Where("state IN ('pending','capturing','sealed','blocked')").
- 			Where("state != 'capturing' OR updated_at IS NULL OR updated_at < ?", time.Now().Add(-craftCaptureDrainWindow)).
+ 			Where("updated_at IS NULL OR updated_at < ?", time.Now().Add(-craftCaptureDrainWindow)). // gate 也覆盖 pending：claim 写必须在 quiescence/staging 之前完成
  			Order("created_at ASC").Limit(limit).Find(&rows).Error


─── internal/application/service/craft_access.go:178-183 ───
[performance · medium] 去重 Count 的谓词组合 (tenant_id, actor_user_id, action, scope_id, target_id,
outcome, created_at) 在现有 audit_logs 索引中无匹配:000044/init 仅有 (tenant_id, action)、单列 (actor_user_id)、单列
(created_at) 等,且本批变更未附任何迁移。该查询虽可如 LogDenied 的 CountSinceForDedup 先例一样借用 (tenant_id, action)
索引后过滤剩余列,但先例的去重第三维 request_path 刻意采用路由模板(audit_log.go:92-98 明确注释防 UUID 遍历灌表),基数有限;此处去重键含 scope_id(每
Task 一键)+ target_id,攻击者横扫租户内大量真实 Task 可按 (任务数×5)/分钟持续落行并膨胀 (tenant_id, 'craft.access_denied')
索引分区(迁移注释表明保留期可达 90
天),此后每次拒绝都在同步授权路径(CheckTaskAccess→auditTaskDenial)上对该分区做无索引列的堆过滤扫描,防刷目标被自身查询成本放大削弱。建议在 versioned
迁移(+sqlite init 同步)为去重键补建复合索引,如 (tenant_id, actor_user_id, action, scope_id, target_id, outcome,
created_at) 或最简 (tenant_id, actor_user_id, action, created_at)。

- 		since := time.Now().Add(-craftDenyDedupWindow)
- 	var recent int64
- 	if err := s.db.WithContext(ctx).Model(&craftAccessAudit{}).
- 		Where("tenant_id = ? AND actor_user_id = ? AND action = ? AND scope_id = ? AND target_id = ? AND outcome = ? AND created_at > ?",
- 			scope.TenantID, actor, "craft.access_denied", scope.SessionID, string(action), "denied", since).
- 		Count(&recent).Error; err == nil && recent > 0 {
+ -- migrations/versioned/0XXXXX_craft_deny_dedup_index.up.sql
+ -- Powers the craft.access_denied sliding-window dedup count
+ -- (tenant, actor, action, scope, target, outcome, created_at) without
+ -- scanning the whole (tenant_id, action) partition on every denial.
+ CREATE INDEX IF NOT EXISTS idx_audit_logs_craft_deny_dedup
+     ON audit_logs (tenant_id, actor_user_id, action, scope_id, target_id, outcome, created_at)
+     WHERE action = 'craft.access_denied';
+ -- (mirror in migrations/sqlite/000000_init.up.sql without the WHERE clause if partial indexes are unsuitable)


─── internal/application/service/craft_web_screen.go:97-101 ───
[security · high] 模板壳字面量豁免缺少对被引用成员的服务端摘要钉扎，构成整体绕过：`<script src="assets/craft-web.js"></script>`
被精确剥除后，index.html 无需携带任何 script 形状即可通过筛查；但暂存路径（craft_artifacts.go 的 stageAndUpload 仅调用
craftScreenWebMemberIsHTML 对 html/htm/svg/xhtml/xht 成员筛查）对 `.js` 成员既不筛查也不校验摘要，且全库生产代码没有任何对暂存
assets/craft-web.js 与 toolchain.lock.json 摘要（5b930dd9…）的比对（toolchain.lock.json 仅在沙箱工具链内生效）。输出树是
Agent 可写的——这正是本服务端筛查存在的理由。利用链：Agent 植入任意 output/assets/craft-web.js + 含该精确字面量的 index.html → HTML
经豁免通过 → JS 未筛暂存 → 预览按 manifest 以 text/javascript 透传 → CSP 为 script-src
'self'（PreviewCSP）放行执行；connect-src 'none' 挡不住 location/window.open 导航。这正是该屏注释声称封堵的
"script/navigation/egress shapes"，round-3 修复目标被架空。建议：剥除该字面量时必须验证暂存的 assets/craft-web.js 成员 sha256
等于钉扎摘要（不匹配则拒绝整轮），或改为预览服务端注入壳而不从输出树收纳 assets/craft-web.js。

- var craftScreenTemplateShellLiterals = []string{
- 	`<meta charset="utf-8">`,
- 	`<meta name="viewport" content="width=device-width, initial-scale=1">`,
- 	`<script src="assets/craft-web.js"></script>`,
+ // craftScreenPinnedCraftWebJS is the toolchain.lock.json sha256 of
+ // assets/craft-web.js. The template shell exemption may only stand on
+ // bytes the server itself verified — the output tree is Agent-writable.
+ const craftScreenPinnedCraftWebJS = "5b930dd96b33bf707653ed6c1b5dc294cb8a790ee7ba7b6adfd13d65a0130064"
+ 
+ func craftScreenWebHTMLMember(rel string, data []byte, stagedDigests map[string]string) error {
+ 	page := string(data)
+ 	for _, literal := range craftScreenTemplateShellLiterals {
+ 		page = strings.ReplaceAll(page, literal, "")
+ 	}
+ 	if d, ok := stagedDigests["assets/craft-web.js"]; ok && d != craftScreenPinnedCraftWebJS {
+ 		return fmt.Errorf("%w: web member %q ships a non-pinned assets/craft-web.js", craft.ErrInvalidInput, rel)
+ 	}
+ 	// ...existing variant denylist unchanged
  }


─── internal/application/service/craft_preview.go:453-455 ───
[performance · low] Open 兑换路径直接调用 requireNoEgress，绕过了 requireFreshPreviewDoors 的 doorCache 播种：紧随 302
之后浏览器请求入口文件，lookup → requireFreshPreviewDoors 会再次全量执行 no-egress 门（绑定加载 + 配置解析 + 实时 Docker inspect，慢
daemon 下单次最长 30s），同一页面加载的关键路径串联两次完整检查。这正是注释所述该缓存要解决（"pins workers when the daemon is slow"）的场景在
redemption→entry 首跳上的复现。建议 Open 改用 requireFreshPreviewDoors（既做检查又播种 2 秒缓存），使紧随其后的入口文件请求直接命中。

- 		if err := s.requireNoEgress(ctx, grant.scope); err != nil {
+ 		if err := s.requireFreshPreviewDoors(ctx, grant.scope); err != nil {
  			return PreviewOpen{}, err
  		}


─── internal/modules/craft/input_code.go:408-410 ───
[security · critical] scriptSeen 之后无空白字符的操作数（以及选项 token，含 `--file=inputs/x`
附值）被整体跳过，但多家已收录解释器会执行『靠后的选项/操作数』里的程序：`awk -f /workspace/gen.awk -f /workspace/inputs/<sha>/x.awk
data`（awk 按序拼接并执行所有 -f 程序）、`php -S 127.0.0.1:8080 /workspace/inputs/<sha>/x.php`（router
脚本每请求执行）、`php -B 'code' -F /workspace/inputs/<sha>/x.php data`。这些 argv 在当前逐层筛查下全部干净：第一个 -f 被
continue、gen.awk/127.0.0.1:8080 占据 script 位、后续 `-f` 与 inputs 路径落入『pure data operand』跳过分支（附值形态连
flagValueCandidates 也不再走）→ 判 Allowed，上传字节被执行。与已确认的 run
位移问题（#2）不同，这是『首脚本之后』区域的系统性缺口。建议：对多程序解释器族的程序文件选项（awk/gawk/mawk 的 -f/--file、php 的 -B/-F/-R/-E 及 -S
router 形态）在任何位置 fail-closed（同 -c/-e/-r/-m 的处理），或至少对 scriptSeen 之后的选项 token
附值与随后的操作数恢复树内筛查；纯数据操作数可保留，但需以解释器族建模区分『数据』与『待执行程序』。

- 			if !strings.ContainsAny(arg, " \t\n") {
- 				continue // pure data operand: the script reads it, no execution
+ 		// 在 rest 循环前捕获解释器名，程序文件选项在任何位置 fail-closed：
+ 		interpBase := path.Base(req.Command[offset-1])
+ 		for _, arg := range rest {
+ 			if isProgramFileFlag(interpBase, arg) { // awk 族 -f/--file、php -B/-F/-R/-E、php -S
+ 				return p.deny("interpreter_input", arg, "")
+ 			}
+ 			// ...原有逻辑；scriptSeen 之后的选项附值同样过 flagValueCandidates 筛查
- 			}
+ 		}


─── internal/modules/craft/input_code.go:326-333 ───
[security · high] 注释声称已筛查 `env --split-string=BASH_ENV=<inputs>/x.sh`，但 flagValueCandidates 只取第一个
'=' 之后的整段——得到 `BASH_ENV=inputs/<sha>/x.sh`——VAR= 前缀仍在，canonical 将其按相对路径拼到 WorkingDir（如
/workspace/BASH_ENV=inputs/.../x.sh），永不落入树内。因此 `env --split-string=BASH_ENV=inputs/<sha>/x.sh bash
gen.sh` 通过全部分层：env 扫描把 `--xxx=...` 整体跳过、前缀筛查两段候选（整 token 与含 VAR= 的附值）均解析不进树、rest=[gen.sh] 干净 →
Allowed，而 bash 非交互启动时按 BASH_ENV source 上传脚本。对照：分离形态 `-S BASH_ENV=inputs/x.sh`
因该值操作数位于前缀内、shellTokens 的 envAssignment 剥离后会被拦下；仅附着形态逃逸。修复：对候选值同样应用 envAssignment 剥离（及冒号分段）后再
canonical，即可同时覆盖相对与绝对路径两种写法。

- 		// Wrapper long options with attached values (env
- 		// --split-string=BASH_ENV=<inputs>/x.sh) are startup hooks exactly
- 		// like assignments: the =-attached value is screened too.
  		for _, value := range flagValueCandidates(prefix) {
- 			if abs := p.canonical(req.WorkingDir, value); p.withinInputs(abs) {
+ 			value = strings.TrimPrefix(value, envAssignment(value))
+ 			for _, segment := range strings.Split(value, ":") {
+ 				if abs := p.canonical(req.WorkingDir, segment); p.withinInputs(abs) {
- 				return p.deny("input_target", abs, "")
+ 					return p.deny("input_target", abs, "")
+ 				}
  			}
  		}


─── internal/modules/craft/input_code.go:275-283 ───
[security · high] 环境值筛查只做 token 化 + 冒号分段，未对『flag 附着值』形态拆解，而注释点名要封的 NODE_OPTIONS --require / RUBYOPT
/ PERL5OPT 恰好都以附着形态注入：Environment={"NODE_OPTIONS": "--require=inputs/<sha>/x.js"} + Command=[node,
gen.js]（node 启动即 require 上传脚本）、RUBYOPT="-rinputs/<sha>/x.rb" + [ruby, gen.rb]、PERL5OPT="-Iinputs"（配合
-M 从 inputs 加载并执行模块）。token `--require=inputs/...` / `-rinputs/...` 经 canonical 分别得
/workspace/--require=... 与 /workspace/-rinputs/...，均不入树 → Allowed。注意与已确认 #3（Shell 分支
CommandText）不同：这是 Environment 分支，且位于现行生产可达路径上——ReviewNormalExec 把 request.Environment
原样传入（validCraftDockerNormalInput 仅做尺寸/字符校验，无变量名清洗）。建议本循环对每个 token 追加 flagValueCandidates 候选并同样做
envAssignment 剥离 + 冒号分段后筛查。

  	for _, value := range req.Environment {
  		for _, token := range shellTokens(value) {
- 			for _, segment := range strings.Split(token, ":") {
+ 			for _, c := range append([]string{token}, flagValueCandidates(token)...) {
+ 				c = strings.TrimPrefix(c, envAssignment(c))
+ 				for _, segment := range strings.Split(c, ":") {
- 				if abs := p.canonical(req.WorkingDir, segment); p.withinInputs(abs) {
+ 					if abs := p.canonical(req.WorkingDir, segment); p.withinInputs(abs) {
- 					return p.deny("input_target", abs, "")
+ 						return p.deny("input_target", abs, "")
+ 					}
  				}
  			}
  		}
  	}


─── internal/modules/craft/input_code.go:463-466 ───
[security · high] xargs 被建模为透明 wrapper，但它与 timeout/nice 不同：它在运行时用 stdin 内容构造被包裹命令的操作数。`xargs -I{}
bash {}` 的 argv=[xargs,-I{},bash,{}] 审查全程干净——`{}` canonical 后是 /workspace/{} 不在树内，InterpreterPrefix
扫描把 `-I{}` 按附着值吞掉、bash 定位为解释器、rest=[{}] 通过；而执行时每行 stdin（如 /workspace/inputs/<sha>/x.sh）被替换进 {} 由
bash 执行。stdin 摘要层也无济于事：TargetSHA256 匹配的是 stdin 字节（此处为路径列表文本）与输入文件内容，永不相等；适配器的『解释器从 stdin 读程序』拦截因
rest 含非 flag 操作数 {}（hasScriptOperand=true）同样不触发。这与代码已显式 fail-closed 的 find -exec 属同一『运行时转发』类。建议与
-exec/-execdir 同等处理：命令位上的 xargs（含 wrapper 链后）直接拒绝（拒绝文案同 exec_forward），或至少在包裹命令尾出现 {} 占位符时拒绝。

- var wrapperCommands = map[string]bool{
- 	"timeout": true, "env": true, "nohup": true, "xargs": true,
- 	"nice": true, "stdbuf": true, "setsid": true, "time": true,
+ // hasExecForwardFlag 同时拒绝命令位上的 xargs：其被包裹命令的操作数在
+ // 运行时由 stdin 填充（{} 替换或追加），替换后的 argv 从未被本门审查。
+ func hasExecForwardFlag(command []string) bool {
+ 	for i, arg := range command {
+ 		if arg == "-exec" || arg == "-execdir" || strings.HasPrefix(arg, "-exec=") || strings.HasPrefix(arg, "-execdir=") {
+ 			return true
+ 		}
+ 		if i == 0 || wrapperCommands[path.Base(command[i-1])] {
+ 			if path.Base(arg) == "xargs" {
+ 				return true
+ 			}
+ 		}
+ 	}
+ 	return false
  }


─── internal/modules/craft/input_code.go:781-786 ───
[bug · low] 短选项组按『包含 c/e/r/m 任一字母』判为程序文本，误伤以路径为值的合法启动 flag：`java -jar /workspace/app.jar`（jar 含
r）、`java -cp /workspace/app.jar Main`（cp 含 c）、`pwsh -File /workspace/gen.ps1`（File 含
e）都被拒——而这些恰是执行『Workspace 内生成文件』的规范形态，拒绝文案还建议『改为执行生成的文件』，自相矛盾。java/pwsh 均在 interpreterCommands
显式收录，说明生成物执行是被设想的用途。建议对这些已知的路径型 flag 显式放行（其值继续走 flagValueCandidates 附着值筛查），或在文档/拒绝文案中明确 java/pwsh
以该形态启动不受支持，避免给出无法遵循的替代建议。

  func carriesProgramTextFlag(arg string) bool {
+ 	// 路径取值的启动 flag（java -jar/-cp、pwsh -File）不是程序文本：
+ 	// 放行 flag 本身，其值由附着值/操作数筛查继续把关。
+ 	switch arg {
+ 	case "-jar", "-cp", "-classpath", "-File":
+ 		return false
+ 	}
  	for _, r := range arg[1:] {
  		switch r {
  		case 'c', 'e', 'r', 'm':
  			return true
  		}


─── internal/handler/craft_model_gateway.go:601-605 ───
[bug · high] 同一分支内对同一 attempt 连续两次矛盾定局：第 591 行已 Resolve(CraftChargeStartStarted)（注释明确 headers
已返回即物理调用已启动，应确定性定局 Started），第 604 行又 Resolve(CraftChargeStartUnknown)。resolveCraftChargeStart 只更新
state='intent' 的行（RowsAffected!=1 即返回 ErrConflict）：正常路径下第一次已将 journal 置为
started，第二次必然失败——每次上游响应体读取失败都会产生一条误导性的 "charge start intent outcome changed concurrently"
错误日志，淹没真实对账信号；而若第一次 Resolve(Started) 因瞬时 DB 错误失败（journal 仍为 intent），第二次 Resolve(Unknown)
反而会成功，把本应确定性 Started 的活动改写为 unknown——按本分支自身注释，Unknown 会 park 活动 ID 并使 Run 脱离 lease 恢复，与客户端收到的
UPSTREAM_ERROR（"the activity started and failed"）及 egress adapter 依据该机器码的 parked/resolved 判定相互矛盾。第二个
Resolve(Unknown) 块应为残留代码，直接删除。

  		if len(respBody) > craftMaxForwardBody {
  			failure = errors.Join(failure, errors.New("upstream response body exceeds maximum size"))
  		}
- 		resolveErr := attempt.Resolve(c.Request.Context(), service.CraftChargeStartUnknown)
- 		if resolveErr != nil {
+ 		// Started 已在上方确定性定局（headers 返回即物理调用启动），
+ 		// 不得再次 Resolve：二次 Unknown 要么必然 ErrConflict（误导性日志），
+ 		// 要么在首次写入瞬时失败时把 Started 改写为 unknown 并 park 活动。


─── internal/handler/craft_model_gateway.go:662-668 ───
[bug · high] initiationExpired 分支无条件把 err==nil 改写为 context.DeadlineExceeded，使 Forward 侧的过滤条件
`initiationExpired := initiationExpiredRaw && forwardErr != nil` 成为死代码（initiationExpiredRaw=true 时
forwardErr 永远非 nil）。当 g.forward 已成功返回响应头（err==nil、resp 有效，物理调用已启动）而 startCtx 恰在 Do 返回后、Err()
检查前到期（30s initiation 窗口边界，或客户端断连导致父级请求 ctx 取消）、且 AfterFunc 回调因 responseHeadersReturned=true
未执行取消时：成功响应被伪造成 DeadlineExceeded——closeGatewayResponse 丢弃响应、recordCall(nil usage)、Resolve(Unknown)
park 活动 ID 并使 Run 脱离 lease 恢复、向客户端返回 502 ACTIVITY_UNRESOLVED。这与 Forward 中 "err==nil proves the
cancellation never took effect: a response that raced the deadline callback must not be rewritten
into a phantom DeadlineExceeded" 的注释意图直接矛盾，也与 resp.Body==nil 分支 "headers 返回即 Started"
的定局纪律不一致。修复：仅当取消真实生效（err != nil 或 resp == nil）时才改写并 cancelRequest；err==nil 且 resp 有效时保持原样返回，交由
Forward 既有过滤路由到正常读体路径（Resolve Started）。

  	if initiationExpired {
+ 		if err == nil && resp != nil {
+ 			// 响应头已返回（取消未生效，物理调用已启动）：保持 err==nil，
+ 			// 让 Forward 的 initiationExpiredRaw && forwardErr != nil 过滤
+ 			// 将该响应路由到正常读体路径（Resolve Started）。
+ 			// 此处也不得 cancelRequest——body 读取绑定在 requestCtx 上。
+ 			return resp, nil, true, true, cancelRequest
+ 		}
  		cancelRequest()
  		if err == nil {
  			err = context.DeadlineExceeded
  		}
  		return resp, err, true, true, cancelRequest
  	}


─── internal/application/service/craft_budget.go:401-409 ───
[bug · high] [bug·high] definitely_unstarted 的“clean slate 重启”路径实际必然失败：此处只删除了 journal 行，上一次尝试留下的同
CallKey（“activity/<id>”，且 CallKey 是 craft_budget_calls 主键 tenant_id+call_key 的一部分）的
CraftBudgetCallRow 仍然存在。后续 tx.Create(&call) 会主键冲突，被 prepareCraftChargeStartWithProtocol 的
isUniqueViolation 分支当作序列争用重试 16 次后以 ErrConflict（“charge start call sequence contention”）收场——与注释宣称的
“clean slate” 相反，活动永远无法重启。即使补删 call 行还有第二重障碍：该 Key 的 commercial 预留已被 MarkReservationDispatchedInTx
置为 dispatched，ReserveInTx 的幂等重放仅接受 held 态（budget_reservation.go tryReserve 返回
ErrReservationKeyConflict），而 mapReserveDenial 不映射该错误。这与 gateway handler
依赖的语义直接矛盾（craft_model_gateway.go “UPSTREAM_ERROR ... keeps the adapter from parking an id whose
retry is provably safe”，适配器被期待用同一 activity id 重试），且现有测试未覆盖该重启路径。建议：在删除 journal 行的同一事务中一并清理旧 call
行并将对应预留释放/复用（或让 Resolve(DefinitelyNotStarted) 时同步回滚 call 行与预留），并为该路径补充测试。



─── internal/application/service/craft_budget.go:1296-1299 ───
[security · high] [security·high] authorizeBudgetActor 查询 tenant_members 与 sessions 时均未过滤软删行
deleted_at IS NULL，授权语义与全仓惯例不一致：(1) tenantMemberRepository.SoftDelete（tenant_member.go）通过 GORM 软删只填
deleted_at、不修改 status，因此被移除的租户 admin/owner 仍保留 role='admin'/'owner' 且 status='active' 的软删行，能通过此检查执行
ExtendAndResume/BudgetPause 授权——成员资格撤回后仍保留计费扩展权限，属于授权旁路；(2) Take 无排序，同一 (tenant,user)
存在“软删旧行+重新加入新行”两代成员资格时任取一行，可能导致合法的重新加入 admin 被旧行干扰误拒；(3) sessions 查询同样缺 deleted_at IS NULL（对照
agent_run.go:353、craft_access.go:72 的既有过滤）。建议按 activeMemberID（craft_access.go:82-94）的纪律补充过滤并以确定序读取。



─── internal/application/service/craft_budget.go:837-841 ───
[bug · medium] [bug·medium] AuthorizeSandbox 的序列分配循环把 Create
的任意错误都当作并发争用重试：非唯一冲突类失败（连接中断、写失败、约束外错误等）会空转重试 16 次并丢弃原始错误，最终被误报为泛化的 "sandbox call sequence
contention"，掩盖真实故障原因（且无退避）。同包 prepareCraftChargeStartWithProtocol 的同类循环先用 isUniqueViolation 判别再
continue，此处应保持一致：仅唯一冲突才重试，其余错误立即返回。



─── internal/application/service/craft_budget.go:680-682 ───
[other · low] [迁移缺口·low] Admit 将商业 Task 预算从 runID 重键为 SessionID（EnsureTaskBudget(scope.SessionID) +
AttachChildRun(runID, sessionID)），但没有任何迁移/回填处理旧版本创建的 runID 键预算行：旧行 RootRunID=''，attachChildRunTx 会因
existing.RootRunID != root 返回 ErrTaskBudgetRootConflict，使这些 Run 的每次 Admit 永久失败（requireRunChargeable
同样因 RootRunID 不匹配拒绝计费）；同时旧行上已花费的额度不会迁移到新的 sessionID 键行（新行以全新 TaskLimit 起算），存在存量任务额度重复计费的窗口。若 craft
预算链路尚未在任何持久环境落库则影响有限，否则需要一条 backfill 迁移（将 runID 键行改键/挂 RootRunID=sessionID 并保留 spent/held）。



─── internal/application/service/craft_preview.go:578-578 ───
[performance · medium] doorCache 的时间戳在昂贵的 no-egress 检查之前采集（第 561 行 now :=
s.config.Now()），检查成功后却以这个陈旧时间戳写入条目（at: now）。no-egress 门包含绑定存储读取、租户配置加载与实时 Docker
inspect（craftPreviewInspectTimeout 上限 30s），而 TTL 仅 2s：一旦门的执行耗时 ≥ TTL——这正是该缓存注释自述要解决的"daemon is slow
pins workers"场景——条目在插入时即已过期，后续几十个静态资产请求各自串行重跑完整门，去重机制恰在其设计目标场景下确定性失效（仅当守护进程本来就快、检查 < 2s
完成时缓存才生效，而这恰是最不需要它的场景）。修复：在 requireNoEgress 成功返回之后再取时间戳写入。附带建议：浏览器资产请求并行到达（通常 6
连接），并发未命中时各自穿透执行完整检查，缺少并发合并（singleflight/重复检查丢弃），可一并考虑。

- 	s.doorCache[scope] = craftPreviewDoorResult{ok: true, at: now}
+ 	fresh := s.config.Now()
+ 	s.mu.Lock()
+ 	if len(s.doorCache) > 1024 {
+ 		s.doorCache = make(map[craft.Scope]craftPreviewDoorResult)
+ 	}
+ 	s.doorCache[scope] = craftPreviewDoorResult{ok: true, at: fresh}
+ 	s.mu.Unlock()


─── packages/views/src/craft/usage.tsx:182-185 ───
[maintainability · low] 导出的 CraftBudgetPauseNoticeProps.strings 声明为完整 CraftUsageStrings（约 20+
字段），但组件实参实际接受的是四个 pause 字段的 Pick。接口与实现签名漂移：调用方若按导出的 Props 类型构造只含 pause 文案的 strings 对象会被 TS
误报缺字段，而运行时本可接受。建议把 Pick 提取为命名类型并让 Props 与组件签名共用，消除漂移。

+ export type CraftBudgetPauseStrings = Pick<CraftUsageStrings, 'pauseTitle' | 'pauseCanExtend' | 'pauseContactOwner' | 'pauseRequestExtension'>;
+ export interface CraftBudgetPauseNoticeProps {
+   pause: NonNullable<CraftRunView['budget_pause']>;
+   /** Projected by the server's current Task owner/billing-admin check. */
    canExtend: boolean;
    onRequestExtension?: (runId: string) => void;
-   strings?: CraftUsageStrings;
+   strings?: CraftBudgetPauseStrings;
  }


─── packages/views/src/craft/sources.tsx:167-169 ───
[performance · low] citedIds Set 每次渲染重建，且 citations map 内对 props.sources 逐条
find（O(n×m)）。同级面板对派生投影的既有惯例是 useMemo（document.tsx 甚至专门 memo 了 citations，workbench.tsx 多处同理）。建议用
useMemo 一次构建 citationId→source 的 Map 与 citedIds，渲染内改用 Map.get，既符合本包惯例也避免引用清单增长后的重复线性扫描。

-   const citedIds = new Set<string>(
-     (props.citations ?? []).filter(isCraftCitationFact).map((entry) => entry.citationId),
-   );
+   // import { useMemo } from 'react';  // 顶部补充导入
+   const { citedIds, sourceById } = useMemo(() => {
+     const sourceById = new Map(props.sources.map((source) => [source.citationId, source] as const));
+     const citedIds = new Set<string>();
+     for (const entry of props.citations ?? []) {
+       if (isCraftCitationFact(entry)) citedIds.add(entry.citationId);
+     }
+     return { citedIds, sourceById };
+   }, [props.sources, props.citations]);
+   // 渲染分支内：const row = sourceById.get(entry.citationId);


─── internal/application/service/craft_budget.go:1265-1272 ───
[bug · low] [bug·low] BudgetPause 的 agent_runs 查询未将 gorm.ErrRecordNotFound 映射为 craft.ErrNotFound：该
(tenant, session_id, run_id) 作用域查询在 SessionID 与 Run 实际归属不一致时（ExtendAndResume 中 grant 仅按 tenant+run
取得，随后才经此查询校验 session 归属）必然返回记录不存在，此处却把裸 gorm 错误穿透端口边界。本文件其余各处 Take/First（grant
查询、requireRunChargeable、authorizeBudgetActor）均映射为 craft.ErrNotFound，此处不一致会导致上层 errors.Is(err,
craft.ErrNotFound) 判定失效（误报 500 而非 404）。注释只论证了"数据库故障需传播"，未覆盖"记录不存在即域结论：该 session 下无此暂停"。

  	err := s.db.WithContext(ctx).Table("agent_runs").Select("status, wait_reason").
  		Where("tenant_id = ? AND session_id = ? AND run_id = ?", scope.TenantID, scope.SessionID, runID).Take(&run).Error
+ 	if errors.Is(err, gorm.ErrRecordNotFound) {
+ 		// No such run under this session is the domain conclusion "no such
+ 		// pause"; only real database failures propagate for caller retry.
+ 		return craft.BudgetPause{}, craft.ErrNotFound
+ 	}
  	if err != nil {
- 		// A database failure is not the domain conclusion "no such pause":
- 		// propagate it so callers can retry instead of telling the user the
- 		// pause never existed.
  		return craft.BudgetPause{}, err
  	}


─── packages/views/src/craft/craft.css:169-171 ───
[maintainability · low] `.wk-craft-citations`（sources.tsx:244 引用清单容器 div 的类名）在全仓库任何 CSS
文件中仍无对应规则（全局搜索确认，本轮只补了 citation-list/-fact/-inference 及两个 label 复合选择器，容器类被遗漏）。结果是该区块的 h4 标题保留浏览器默认
margin，与兄弟面板的模式不一致——share 面板显式写了 `.wk-craft-share h4 { margin: 0 }` 并由容器 grid gap 控制间距。建议为容器补一条与
`.wk-craft-share` 一致的规则（如 `display: grid; gap: 0.6rem;` + `h4 { margin: 0 }`，或至少给标题 margin
归零），否则该类名成为悬空引用，后续维护者无法得知容器间距是否有意交给浏览器默认值。

  /* T14 round-3: the sources panel's citation list matches the sibling panels'
     concentrated styling (was browser defaults). */
+ .wk-craft-citations { display: grid; gap: 0.6rem; min-width: 0; }
+ .wk-craft-citations h4 { margin: 0; }
  .wk-craft-citation-list { display: grid; gap: 0.4rem; list-style: none; margin: 0; padding: 0; }


LLM retry report summary: 3 of 658 requests affected -- 3 requests recovered after retry

Core review (3 requests):
- internal/application/repository/craft_workspace.go,internal/application/service/craft_delegate.go,internal/application/service/craft_inputs.go,internal/application/service/craft_session.go,internal/application/service/craft_source_open.go,internal/application/service/craft_workspace.go,internal/application/service/session.go,internal/handler/session/craft.go: rate limited (HTTP 429) -> rate limited (HTTP 429) -> succeeded
- internal/application/service/craft_archive.go,internal/application/service/craft_artifacts.go,internal/application/service/craft_citations.go,internal/application/service/craft_export.go,internal/application/service/craft_share.go,internal/container/craft_export_wiring.go,internal/container/craft_share_wiring.go,internal/handler/session/artifact_download.go,internal/handler/session/craft_share.go: rate limited (HTTP 429) -> succeeded
- internal/modules/craft/archive.go,internal/modules/craft/citation.go,internal/modules/craft/export_manifest.go,internal/modules/craft/share.go: rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> succeeded

Per-attempt detail: --format json (retry_report).
