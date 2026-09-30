Review complete: 12 finding(s) across 6 selected item(s).

─── internal/modules/craft/input_code.go:286-287 ───
[security · high] 解释器分支只审查 offset 之后的操作数，而 InterpreterPrefix 消费掉的前缀参数（尤其是 env 的 VAR=value
赋值项）不被任何层筛查：这些赋值既不在 rest 中，也不会出现在 req.Environment（适配器 ReviewNormalExec 只把请求的 environment map 填入
Environment，argv 中的赋值不会合并进去），而命令命中解释器后又跳过了非解释器分支的全操作数筛查。具体绕过：`env
BASH_ENV=/workspace/inputs/<sha>/x.sh bash /workspace/gen.sh`、`env PYTHONSTARTUP=<inputs>/x.py
python3 gen.py`、`env PYTHONPATH=<inputs>/<sha> python3 -m ...` —— bash/python 启动时 source/import
上传材料并执行，2b 层注释声称防护的 BASH_ENV/PYTHONSTARTUP 场景在 argv 形式下完全失效，Review 返回 Allowed 并打上
AuditKindGeneratedExecute。建议：解释器分支对 req.Command[:offset] 的包装器前缀一并用 shellTokens+canonical
筛查（shellTokens 的 envAssignment 剥离会让 BASH_ENV=<path> 的路径重新可见），命中即 deny("input_target")。

  	} else if interpreter, offset := InterpreterPrefix(req.Command); interpreter {
  		rest := req.Command[offset:]
+ 		// The wrapper-consumed prefix (env VAR=value assignments, timeout
+ 		// operands) reaches the runtime exactly like Environment entries
+ 		// (BASH_ENV, PYTHONSTARTUP, PYTHONPATH, ...), so screen it too.
+ 		for _, arg := range req.Command[:offset] {
+ 			for _, token := range shellTokens(arg) {
+ 				if abs := p.canonical(req.WorkingDir, token); p.withinInputs(abs) {
+ 					return p.deny("input_target", abs, "")
+ 				}
+ 			}
+ 		}


─── internal/modules/craft/input_code.go:265-271 ───
[security · high] 2b 层对 Environment 值的筛查复用 shellTokens，只按空白/shell
分隔符切分，不切分冒号：路径列表型变量（PYTHONPATH、NODE_PATH、PERL5LIB、RUBYLIB、PATH）会作为单个 token 参与判定，而 withinInputs 要求
token 以 inputsTree+"/" 开头，因此只要把 inputs 目录放在列表的非首位（如
`PYTHONPATH=/usr/lib/python3:/workspace/inputs/<sha>`）即可通过筛查（首位形式反而会被前缀匹配拒绝）。该请求的 Environment 由
agent 侧完全可控（staged request 的 environment 字段原样透传到容器 exec），随后 `python3 gen.py` 即可 `import analyze`
执行上传模块字节，违背“启动钩子不得指向上传材料”的设计意图。建议：对每个 token 再按 ':' 拆分后逐段做 canonical/withinInputs
筛查（拆分只会增加被筛查的路径元素，不会丢失）。

  	for _, value := range req.Environment {
  		for _, token := range shellTokens(value) {
- 			if abs := p.canonical(req.WorkingDir, token); p.withinInputs(abs) {
+ 			// Path-list variables (PYTHONPATH, NODE_PATH, PERL5LIB, RUBYLIB,
+ 			// PATH) carry colon-separated entries: screen each element.
+ 			for _, entry := range strings.Split(token, ":") {
+ 				if abs := p.canonical(req.WorkingDir, entry); p.withinInputs(abs) {
- 				return p.deny("input_target", abs, "")
+ 					return p.deny("input_target", abs, "")
+ 				}
  			}
  		}
  	}


─── internal/modules/craft/input_code.go:302-305 ───
[security · high] 解释器分支把所有以 '-' 开头的操作数当作纯标志位整块跳过，且子 token
筛查只对含空白的操作数生效，导致携带代码载荷的标志位完全不被审查：解释器支持连写与独立两种形式——`python3
-cexec(open('/workspace/inputs/<sha>/analyze.py').read())`（连写，无空白，路径嵌在 token 中部永远不可能成为独立 token）与
`node -e 'require("/workspace/inputs/<sha>/x.js")'`（独立 -e，载荷无空白）。二者在 Review 中均被跳过或筛查不中而返回
Allowed+AuditKindGeneratedExecute；适配器侧的兜底（shellCommandFlagIndex 精确匹配 "-c" 后转 Shell 形式 fail-closed
拒绝）只覆盖独立的 -c，连写 -c 与一切 -e 均漏过。这与注释中对 shell 表达式的 fail-closed 理由（词法不可审查即拒绝）应同等适用于 -c/-e 代码载荷。建议：解释器分支对
-c/-e 前缀（含连写载荷）的操作数按不可审查直接 deny，与 Shell 分支保持一致。

  		for _, arg := range rest {
  			if !scriptSeen && strings.HasPrefix(arg, "-") {
+ 				// Code-carrying flags (-c<code>, -e<code>, attached or as a
+ 				// following operand) embed an unreviewable program, exactly
+ 				// like a shell expression: deny fail-closed.
+ 				if payload := strings.TrimPrefix(arg, "-"); len(payload) > 1 && (payload[0] == 'c' || payload[0] == 'e') {
+ 					return p.deny("interpreter_input", arg, "")
+ 				}
  				continue
  			}


─── internal/modules/craft/input_code.go:110-113 ───
[bug · medium] Target 与 Digest 可能同时为空，Refusal() 会渲染出 "(matched )" 的残缺文案，deny() 的 Err 同样输出 "matched
"。这不是罕见路径：适配器 ReviewNormalExec 把 -c 形式转为 Shell 请求时从不填 TargetPath/ResolvedTargetPath（TargetSHA256
也仅来自 stdin 字节），因此所有 shell 表达式的 fail-closed 拒绝都会走 canonical(WorkingDir, "")=="" → deny("shell_input",
"", "")，而这是该执行面最常见的拒绝路径，成员可见的拒绝信息却无法辨识被拒目标。建议在两者均为空时回退到稳定文案（如 "unreviewable shell
expression"）或（截断后的）CommandText。

  	target := d.Target
  	if target == "" {
  		target = d.Digest
+ 	}
+ 	if target == "" {
+ 		target = "unreviewable shell expression"
  	}


─── internal/modules/craft/input_code.go:336-344 ───
[security · critical] readOnlyCommands 豁免名单中包含能执行操作数的工具，导致上传材料可被执行：`rg` 在名单内，但 ripgrep 的
`--pre`/`--pre=COMMAND` 选项会为每个被搜索文件以子进程执行该 COMMAND。请求 `rg --pre /workspace/inputs/<sha>/x.sh --
/workspace/notes.txt` 走到本分支时，因 `readOnlyCommands["rg"]` 为 true，全部操作数筛查被跳过，决策返回
Allowed+craft.generated.execute，容器内即执行了上传字节（`--pre 'sh /workspace/inputs/<sha>/x.sh'`
形式连执行位都不需要）。当前唯一接线的适配器 ReviewNormalExec 不解析文件目标摘要（服务端无法访问容器文件系统），字节身份层也不生效，所以这是该执行面上的完整绕过。同类向量还有
less/more 的 `+!cmd` 初始命令串（非 secure 模式下允许执行 shell 命令）。建议：豁免仅对纯读取工具成立——为豁免工具增加执行型选项守卫（rg 的
`--pre`/`--pre=`、less/more 的 `+`/`-+`/`+!` 命令串出现时撤回豁免、照常筛查全部操作数），或直接将 rg/less/more 移出
readOnlyCommands（纯读取场景可由 cat/grep 覆盖）。

- 		if !readOnlyCommands[path.Base(req.Command[0])] {
+ 		base := path.Base(req.Command[0])
+ 		// 豁免仅对纯读取操作数成立：出现执行型选项（rg 的 --pre/--pre=、
+ 		// less/more 的 + 命令串）时撤回豁免，全部操作数照常遏制筛查。
+ 		if !readOnlyCommands[base] || readOnlyToolCarriesExecutingOption(base, req.Command) {
  			for _, arg := range req.Command[1:] {
  				for _, token := range shellTokens(arg) {
  					if abs := p.canonical(req.WorkingDir, token); p.withinInputs(abs) {
  						return p.deny("input_target", abs, "")
  					}
  				}
  			}
  		}


─── internal/application/service/craft_execution_policy.go:158-167 ───
[security · high] shellCommandFlagIndex 只做精确 "-c" 匹配，存在两个相反方向的缺口：

1) 绕过（安全）：组合短旗标如 `bash -lc "…"` / `sh -ic "…"` 中 "-lc" ≠ "-c"，不触发 Shell 归一化，落入
InputExecutionPolicy.Review 的解释器 operand 词法筛查——而 Evidence contract 自己声明 "lexical screening of shell
forms is bypassable by construction"，这正是 shell 表达式必须 fail-closed 的原因。具体逃逸：`bash -lc "eval $(echo
<base64> | base64 -d)"` 的 token 中不含明文路径，词法层放行，实际执行了解码后的 `python3
/workspace/inputs/analyze.py`（上传材料）；同一表达式换成 `bash -c` 则被正确拒绝。语义等价形式获得更弱的审查路径，破坏 T03 门禁的 fail-closed
立场。

2) 误杀（可用性）：扫描整个 rest 而非仅解释器选项前缀，`python3 gen.py -c config.yaml` 这类生成脚本自身参数携带 -c 的合法命令被误判为 shell
表达式而拒绝（拒绝文案还会误导成员说是 shell 形式）。

建议：仅在第一个脚本操作数（首个非 "-" 开头参数）之前的选项前缀内查找，且匹配任何携带 c 的短旗标组合（偏向 fail-closed；注意排除 "--" 长选项与单独 "-" stdin
标记）。

- // shellCommandFlagIndex returns the index of the -c flag in an interpreter
- // operand list, or -1 when absent.
+ // shellCommandFlagIndex returns the index of a shell command flag (-c, or a
+ // combined short flag carrying c like -lc/-ic) inside the interpreter's
+ // option prefix — before the first script operand — or -1 when absent.
+ // A -c after the script operand belongs to the script, not the shell.
  func shellCommandFlagIndex(rest []string) int {
  	for i, arg := range rest {
- 		if arg == "-c" {
+ 		if arg == "-" || !strings.HasPrefix(arg, "-") {
+ 			return -1 // reached the script operand (or stdin marker): stop
+ 		}
+ 		if !strings.HasPrefix(arg, "--") && strings.Contains(arg, "c") {
  			return i
  		}
  	}
  	return -1
  }


─── internal/application/service/craft_execution_policy.go:154-154 ───
[bug · low] writeDenied(ctx, 0, "", …) 以 TenantID=0、ScopeID="" 落审计行：audit_logs
的查询面（AuditLogQuery.List）始终按 tenantID 过滤，这些行写入后不可归属、不可检索，成为零租户孤儿数据；且 AuditLogService.Log 无 dedup（只有
LogDenied 有），一旦该面被调用，每次 Start 都会恒定产生一行无效审计。该拒绝是恒定的（不依赖输入），审计行信息量也仅剩固定 reason。建议：audit
未携带身份时只走日志（logger.Warnf），或在 binding/request 可提取身份前跳过落库；至少应避免写入 TenantID=0 的行。



─── internal/application/service/craft_docker_normal_exec.go:57-60 ───
[maintainability · low] 安全门禁完全依赖装配层主动调用 WithExecutionPolicy（与 craft_docker_restricted_exec.go
同模式），nil 时静默保持旧行为。全库检索确认 NewCraftDelegateExecutionPolicy / NewCraftDockerNormalExecService /
NewCraftDelegation（新三参签名）目前仅测试调用，生产装配点尚不存在——即 T03 门禁当前整体为 no-op 且无任何运行时告警。分阶段合入可以理解，但装配 PR
一旦遗漏注入，无编译错误、无日志可发现。建议：在最终装配处将门禁作为构造参数（或注入时输出一条启动日志），使"安全特性未接线"可被检测；合入装配时需同时覆盖 normal 与 restricted
两个服务及 WithAuditLog。



─── internal/application/service/craft_execution_policy.go:190-195 ───
[maintainability · low] agent_runs 查询失败时裸返回 gorm 错误（含 ErrRecordNotFound）：错误既不属于 craft 哨兵族（上层无法用
errors.Is(craft.Err…) 归类为成员可见拒绝/系统错误），也丢失了是哪个 Run 解析失败的上下文；且当 audit sink 未注入（当前默认状态）时，该 fail-closed
路径无任何日志痕迹。建议包装 %w 并附身份上下文，同时补一条 logger.Warnf，与 ReviewExecution 的决策日志对齐。

  	if err := g.db.WithContext(ctx).Table("agent_runs").
  		Select("owner_id", "session_id", "snapshot").
  		Where("tenant_id = ? AND run_id = ?", tenantID, runID).Take(&row).Error; err != nil {
+ 		logger.Warnf(ctx, "[CraftExecutionPolicy] run identity not resolvable tenant=%d run=%s: %v", tenantID, runID, err)
  		g.writeDenied(ctx, tenantID, runID, "run identity not resolvable")
- 		return err
+ 		return fmt.Errorf("craft: run %d/%s identity not resolvable: %w", tenantID, runID, err)
  	}


─── internal/application/service/craft_execution_policy.go:124-128 ───
[security · critical] 与已确认的 "-lc 组合旗标" 问题（#2）同类但独立：这里对"程序文本类证据"的归一化枚举不完整，导致单条命令即可执行上传材料，绕过 T03
门禁的核心承诺（"词法筛查 by construction 可绕过，故 shell 表达式必须 fail-closed"）：

1) 内联程序旗标不止 -c：`node -e "require('in'+'puts/x.js')"`、`php -r`、`perl -e`、`ruby -e`、`lua -e`、`awk -e`
的程序文本落在 policy 的"首个非旗标 operand"位置，只做 shellTokens 词法筛查——运行期字符串拼接/base64/`require(process.argv[1])`
都能让真实路径永不作为独立 token 出现，上传字节被原地执行（非文档化的 copy-then-execute 残余，无需两步）。

2) run 子命令：`deno run /workspace/inputs/x.ts`、`bun run x.js`——"run" 被当作 script operand
筛查，真正被执行的文件是"后续纯数据 operand"，在 InputExecutionPolicy.Review 中被显式跳过（`!strings.ContainsAny(arg, "
\t\n")` → continue），连字面路径都不筛查，直接放行。

3) `env -i python3 -c "…"`：InterpreterPrefix 不跳过 env 自身旗标（只跳 VAR=value），返回非解释器，连 -c 的 Shell 归一化都不触发。

建议：适配器侧对任何"无法唯一确定 leading script operand"的解释器 argv 形式（-e/-r 等内联旗标、deno/bun run 子命令）一律按本处 -c 的处理方式送
Shell fail-closed 拒绝；对 env 旗标形式要么补齐跳过逻辑要么同样 fail-closed。

- 		// A wrapped shell reading its program from -c is a shell expression:
- 		// send it as Shell/CommandText, which the policy denies without
- 		// adapter-normalized evidence (lexical screening of shell forms is
- 		// bypassable by construction).
+ 		// Fail-closed normalization: any interpreter form whose executed
+ 		// program is not unambiguously a single leading script operand
+ 		// (-c/-e/-r inline program flags, run-style subcommands) is sent
+ 		// as Shell/CommandText, which the policy denies without
+ 		// adapter-normalized evidence.
  		if idx := shellCommandFlagIndex(rest); idx >= 0 && idx+1 < len(rest) {
+ 			execRequest.Shell = true
+ 			execRequest.CommandText = strings.Join(rest[idx+1:], " ")
+ 			execRequest.Command = nil
+ 			execRequest.Environment = nil
+ 		} else if hasInlineProgramFlag(rest) || hasRunSubcommand(rest) {
+ 			execRequest.Shell = true
+ 			execRequest.CommandText = strings.Join(rest, " ")
+ 			execRequest.Command = nil
+ 			execRequest.Environment = nil
+ 		}


─── internal/application/service/craft_execution_policy.go:128-130 ───
[bug · medium] shellCommandFlagIndex 扫描整个 operand 列表而不在首个非旗标 operand（脚本路径）处停止，把"脚本自己的 -c
参数"误判为解释器旗标：`python3 /workspace/rv-x/train.py -c config.json` 会被归一化为
Shell=true（CommandText="config.json"）并在 policy 的 shell_input 分支被 fail-closed 拒绝——这恰好拒绝了拒绝文案自己推荐的"执行
Workspace 生成文件"替代方案，属误伤性可用性缺陷（与已确认 #2 的漏判方向相反，此处是误判方向）。建议只在脚本 operand 之前的旗标区间内查找 -c。

- 		if idx := shellCommandFlagIndex(rest); idx >= 0 && idx+1 < len(rest) {
+ 		flags := rest
+ 		for i, arg := range rest {
+ 			if !strings.HasPrefix(arg, "-") {
+ 				flags = rest[:i] // 脚本 operand 之后的参数属于脚本自身，不属于解释器旗标
+ 				break
+ 			}
+ 		}
+ 		if idx := shellCommandFlagIndex(flags); idx >= 0 && idx+1 < len(flags) {
  			execRequest.Shell = true
  			execRequest.CommandText = strings.Join(rest[idx+1:], " ")


─── internal/application/service/craft_docker_restricted_exec.go:93-100 ───
[security · low] 门禁只挂在 Start 上，而同一服务的 ResumeBound → claimAndStart → StartOutputlessExec
是另一条真实发送路径，完全不做策略审查。对于门禁挂载前（升级窗口或混布集群中未挂门禁的实例）已 Bind 未 Claim 的操作，恢复方会在无任何 T03
审查的情况下把命令发送出去——与本变更自己声明的不变量"an unreviewable command must never be
sent"矛盾（受限请求本就无可审查身份，ReviewOutputlessExec 对新命令一律拒绝，恢复路径却放行）。建议 ResumeBound
也过同一门禁（按其语义应同样拒绝），或至少在注释中显式记录该恢复窗口例外及其理由。

- 	// T03 (#122): uploaded code stays data. Screen the command before the
- 	// durable send is prepared; a denial returns the member-visible refusal
- 	// and nothing is created, bound or sent.
+ // ResumeBound starts only the exact persisted receipt left by a crash after
+ // bind. A consumed claim returns no permission and can never resend.
+ // T03: recovery sends an unreviewable command too — screen it like Start.
+ func (s *CraftDockerRestrictedExec) ResumeBound(ctx context.Context, grantID, activityID string) (CraftDockerOutputlessResult, error) {
  	if s.policy != nil {
- 		if err := s.policy.ReviewOutputlessExec(ctx, binding, request); err != nil {
+ 		if err := s.policy.ReviewOutputlessExec(ctx, CraftCallBinding{}, CraftDockerOutputlessRequest{}); err != nil {
  			return CraftDockerOutputlessResult{}, err
  		}
  	}
+ 	op, durable, err := s.coordinator.ResumeBound(ctx, grantID, activityID)

