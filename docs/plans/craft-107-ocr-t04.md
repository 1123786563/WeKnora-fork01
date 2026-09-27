Review complete: 8 finding(s) across 4 selected item(s).

─── internal/container/craft_runtime.go:190-197 ───
[bug · high] 死代码 + 静默失败：NewCraftDelegateExecutionPolicy 的构造函数（craft_execution_policy.go:102 `if
delegate == nil || db == nil { return nil, ...ErrInvalidInput }`）无条件拒绝 nil delegate，此处第一个参数却是字面量
nil——perr 恒非 nil，整个 gate 装配块永远不执行，"gate assembled" 日志永不出现，webBuildGate 恒为 nil，本轮核心交付"中央装配接线"实际未生效。同时
perr != nil 分支完全无日志（与账本 :1031
"装配成功/失败均落日志"的意图相悖），安全装配失败不可观测。测试侧（craft_web_build_review_test.go:68）用的是
service.NewCraftDelegateService(repository.NewCraftStore(db), nil) 构造的真实 delegate。本函数签名已有 store
craft.Store 参数，应据此构造 delegate service 并补上 perr 失败分支的 Warn 日志。

- 		if delegatePolicy, perr := service.NewCraftDelegateExecutionPolicy(nil, db, workDir); perr == nil {
+ 		delegate := service.NewCraftDelegateService(store, nil)
+ 		if delegatePolicy, perr := service.NewCraftDelegateExecutionPolicy(delegate, db, workDir); perr == nil {
  			if gate, gerr := NewCraftWebBuildCommandGate(delegatePolicy, pin, webToolchainDir); gerr == nil {
  				webBuildGate = gate
  				logger.Infof(context.Background(), "[CraftRuntime] T04 web build command gate assembled (toolchain %s)", webToolchainDir)
  			} else {
  				logger.Warnf(context.Background(), "[CraftRuntime] T04 web build command gate NOT assembled: %v (build commands stay fail-closed)", gerr)
  			}
+ 		} else {
+ 			logger.Warnf(context.Background(), "[CraftRuntime] T04 execution policy NOT assembled: %v (web build commands stay fail-closed)", perr)
  		}


─── internal/container/craft_web_build_review.go:136-138 ───
[security · medium] 第一层"固定命令形状"校验对 argv[0] 只做 basename 匹配：/tmp/evil/python3、./python3 等任意以 python3
结尾的可执行路径都能通过，且下游 T03 模块策略的 InterpreterPrefixStatus 同样按 path.Base
识别解释器（input_code.go:629-631），三层审查都会把该包装器当作真 python3——与文件头"the entry cannot be borrowed to dispatch
arbitrary commands"的声明不符（可写目录下的同名包装器/symlink 即可借道）。SKILL.md:99 的固定命令用的是裸 "python3"，建议钉死 command[0]
== "python3"（或镜像内解释器绝对路径）。另外 --input/--output/--runtime-digest 的取值在本层完全未校验（下游模块策略仅对 inputs
树做包含性拒绝），若要维持"固定形状"声明请一并约束或在注释中明确该纵深依赖。

- 	if len(command) < 2 || path.Base(command[0]) != "python3" || command[1] != craftWebBuildProgramPath {
+ 	if len(command) < 2 || command[0] != "python3" || command[1] != craftWebBuildProgramPath {
  		return false
  	}


─── internal/container/craft_web_build_review.go:43-46 ───
[documentation · medium] 此段注释与本轮同批改动直接矛盾：craft_docker_normal_input.go 已为
CraftDockerNormalInputRequest 落地 ResolvedTargetPath/TargetSHA256 additive 字段，且
CraftDelegateExecutionPolicy.ReviewNormalExec 已完成透传（账本 :1031 本轮提交）。"that shared DTO carries no
server-resolved target fields" 与 "requires an additive DTO change" 现在均不成立。T20 实现者若依据此过时注释判断 DTO
无转发通道，可能漏填字段或另建平行机制。建议改写为反映已落地的 additive 字段与中央裁决，并保留"本入口仍以容器工作区为根的 namespace 不匹配为由在第 2
层就地裁决宿主事实"的设计说明。



─── internal/application/service/craft_execution_policy.go:130-136 ───
[bug · medium] stdin 摘要推导无条件覆盖已透传的 execRequest.TargetSHA256：当请求同时携带 TargetSHA256 与 stdin
字节时，文件目标的字节身份审查（input_identity 层）被静默丢弃，只审 stdin 摘要——与账本 :1031 中央裁决"TargetSHA256 非空优先于 stdin
摘要"正好相反。模块策略的 InputExecutionRequest 仅支持单一 TargetSHA256，两个证据源并存时必须显式裁决优先级（或 fail-closed
拒绝），否则服务端可解析面填入的 target digest 会被 stdin 通道悄悄冲掉。建议 TargetSHA256 非空时跳过 stdin 摘要推导。

- 	if request.StdinEnabled && len(request.Stdin) > 0 {
- 		// Byte identity for the stdin channel: the stdin bytes ride on the
- 		// reviewed request itself, so their digest can be matched against
- 		// the admitted manifest exactly like a file target.
+ 	if request.StdinEnabled && len(request.Stdin) > 0 && execRequest.TargetSHA256 == "" {
+ 		// Byte identity for the stdin channel: per the central ruling, an
+ 		// explicitly supplied TargetSHA256 takes precedence and must not be
+ 		// silently overwritten by the stdin digest.
  		stdinSum := sha256.Sum256(request.Stdin)
  		execRequest.TargetSHA256 = hex.EncodeToString(stdinSum[:])
  	}


─── internal/container/craft_web_build_review.go:103-106 ───
[performance · low] 装配路径上 LoadCraftWebToolchainPin 被连续执行两次：craft_runtime.go:177
刚加载并全量校验过（template.html + 全部 deps + build.py 逐文件重算 sha256），构造器内又对同一目录完整重载重算一遍再与传入 pin
比对——结果必然一致（除非亚秒级 TOCTOU）。一次性启动开销、目录小，属低优先级；可让构造器复用调用方刚加载的 pin 或提供跳过重载的选项。注意 verifyHostToolchain 中
per-Review 的重验是设计内（防审查期漂移），不应省略。



─── internal/application/repository/craft_docker_normal_input.go:61-62 ───
[bug · critical] 新增字段未纳入 canonical JSON 尺寸计算器，携带新证据的请求将无法落库：craftDockerNormalCanonicalJSONSize（本文件
:332-381）的计算在 output_policy 后即以 +1 收尾闭合 `}`，而 Stage() :141 强制 `int64(len(plain)) != canonicalSize`
即返回 ErrCraftDockerNormalInputCorrupt（"canonical size calculator disagrees with
json.Marshal"），decryptCraftDockerNormalInput :293 对持久化请求做对称复验。两个字段为 omitempty，双空时确实不变；但本次改动的目的恰是让
server-resolvable 面"填入 resolved target 与 digest"——任一字段非空后，json.Marshal 会多出
,"resolved_target_path":... / ,"target_sha256":... 段，Stage 必然长度失配并以误导性的"integrity/decryption
check"错误拒绝，该请求永远无法进入 durable 加密身份链（Execute→prepareOrRecover→PrepareNormal→Stage 全链阻断）。需在计算器中按
omitempty 语义补齐两个字段的长度（注意把现有挂在 output_policy 上的 +1 收尾改为统一闭合括号），并补一条字段非空走 Stage 的用例（现有
craft_docker_normal_input_test.go 的 mutate 列表未覆盖新字段）。

- 	ResolvedTargetPath string `json:"resolved_target_path,omitempty"`
- 	TargetSHA256       string `json:"target_sha256,omitempty"`
+ // craftDockerNormalCanonicalJSONSize 内、output_policy 计入之后追加：
+ 	if in.ResolvedTargetPath != "" {
+ 		size += int64(len(`,"resolved_target_path":`)) + craftDockerJSONQuotedStringSize(in.ResolvedTargetPath)
+ 	}
+ 	if in.TargetSHA256 != "" {
+ 		size += int64(len(`,"target_sha256":`)) + craftDockerJSONQuotedStringSize(in.TargetSHA256)
+ 	}
+ 	size++ // 收尾 }（从 output_policy 行的 +1 移出）


─── internal/container/craft_runtime.go:190-190 ───
[bug · high] 第三个参数 workspaceRoot 语义错误：NewCraftDelegateExecutionPolicy
的契约（craft_execution_policy.go:100）与全部既有调用点（wiring/review 测试均传
"/workspace"；craft_inputs.go:47-48、craft_delegate.go:329 明确容器工作区根是 /workspace）要求的是容器工作区根，而此处传入的
workDir 是 CRAFT_OPENCODE_WORK_DIR 指向的宿主 serve 工作目录（其下是 ws/<sid>/... 会话目录树）。该值经 g.workspaceRoot →
MaterialPolicy → NewInputExecutionPolicy 推导只读输入树 <workspaceRoot>/inputs 并用于 canonical
相对路径归一，错根后被审命令中的 /workspace/inputs/... 词法包含检查全部失配——即使修复了同行的 nil-delegate 问题（见已确认发现），这个 gate 实例的整套
T03 词法筛查仍被静默错键（fail-open）。应传容器工作区根（"/workspace" 或对应常量）。

- 		if delegatePolicy, perr := service.NewCraftDelegateExecutionPolicy(nil, db, workDir); perr == nil {
+ 		if delegatePolicy, perr := service.NewCraftDelegateExecutionPolicy(delegate, db, "/workspace"); perr == nil {
+ 			// workspaceRoot 必须是容器工作区根（MaterialPolicy 以 <root>/inputs
+ 			// 推导只读树），workDir 是宿主 serve 目录，二者不可混用。


─── internal/container/craft_web_build_review.go:235-235 ───
[security · medium] 第二层"resolved toolchain 不得落入 staged 只读材料树"的防线在唯一生产装配点被静默旁路：该检查以 g.hostInputsRel
!= "" 为守卫，而 hostInputsRel 仅由 WithCraftWebHostInputsRoot 填充——全仓唯一生产调用点 craft_runtime.go 的
NewCraftWebBuildCommandGate 未传该 option（仅测试 craft_web_build_review_test.go:233
传入），因此本文件头注释宣称的三层防线在生产部署中实际只有两层生效，且无任何日志提示该层缺席（fail-open 默认）。与已确认的 nil-delegate
死块问题独立：即便装配块修复，此层仍不会生效。建议在 craft_runtime.go 装配时传入宿主材料根（或让 gate 在 hostInputsRel 为空时记录一次性
Warn，明确该层未布防）。

- 	if g.hostInputsRel != "" && (pathWithinDir(g.hostInputsRel, resolvedDir) || pathWithinDir(g.hostInputsRel, resolvedBuild)) {
+ 	if g.hostInputsRel == "" {
+ 		logger.Warnf(ctx, "[CraftWebBuild] host inputs root not configured; toolchain/inputs overlap check is INERT for run %s", request.RunID)
+ 	} else if pathWithinDir(g.hostInputsRel, resolvedDir) || pathWithinDir(g.hostInputsRel, resolvedBuild) {
+ 		logger.Warnf(ctx, "[CraftWebBuild] resolved toolchain overlaps the staged read-only inputs tree for run %s: %s", request.RunID, resolvedDir)
+ 		return fmt.Errorf("%w: craft web toolchain resolves inside the staged inputs tree for run %s", craft.ErrConflict, request.RunID)
+ 	}


LLM retry report summary: 6 of 26 requests affected -- 6 requests recovered after retry

Core review (5 requests):
- internal/application/repository/craft_docker_normal_input.go,internal/application/service/craft_execution_policy.go,internal/container/craft_runtime.go,internal/container/craft_web_build_review.go: rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> succeeded
- internal/application/repository/craft_docker_normal_input.go,internal/application/service/craft_execution_policy.go,internal/container/craft_runtime.go,internal/container/craft_web_build_review.go: rate limited (HTTP 429) -> succeeded
- internal/application/repository/craft_docker_normal_input.go,internal/application/service/craft_execution_policy.go,internal/container/craft_runtime.go,internal/container/craft_web_build_review.go: rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> succeeded
- internal/application/repository/craft_docker_normal_input.go,internal/application/service/craft_execution_policy.go,internal/container/craft_runtime.go,internal/container/craft_web_build_review.go: rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> succeeded
- internal/application/repository/craft_docker_normal_input.go,internal/application/service/craft_execution_policy.go,internal/container/craft_runtime.go,internal/container/craft_web_build_review.go: rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> succeeded

File grouping (1 request):
- __grouping__: rate limited (HTTP 429) -> rate limited (HTTP 429) -> succeeded

Per-attempt detail: --format json (retry_report).
