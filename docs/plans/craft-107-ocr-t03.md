Review complete: 14 finding(s) across 6 selected item(s).

─── internal/modules/craft/input_code.go:250-254 ───
[security · critical] 非解释器分支只检查 Command[0]，Command[1:] 的所有 operand 一律不做 inputs
树包含检查。结合生产接线点（craft_execution_policy.go 的 ReviewNormalExec 只回填
Command+WorkingDir，TargetSHA256/ResolvedTargetPath
恒为空，字节身份层与符号链接层从不触发），本分支成为唯一生效防线，而以下命令全部放行上传脚本：`timeout 10 python3
/workspace/inputs/<sha>/x.py`、`env VAR=1 python3 ...`（interpreterPrefix 只识别 `env <interp>`，不跳过
VAR=x）、`nohup python3 ...`、`xargs python3 ...`、`python3.11 ...`（interpreterCommands 为精确 basename
匹配，无版本号/变体形态）。这直接击穿文件头声明的核心承诺（"no interpreter, shell, copy, link or alias path may execute the
uploaded bytes"）。建议对 Command[1:] 每个 operand 都做包含检查——这同时顺带拦截 cp 复制上传字节到可写区（copy 也在威胁列表内）；若需保留
cat/grep 等读取型命令，可用显式只读白名单而非依赖 argv[0] 形态匹配的默认放行。

  	} else if len(req.Command) > 0 {
- 		if abs := p.canonical(req.WorkingDir, req.Command[0]); p.withinInputs(abs) {
- 			// Direct execution of the uploaded path itself.
+ 		for _, arg := range req.Command {
+ 			if abs := p.canonical(req.WorkingDir, arg); p.withinInputs(abs) {
+ 				// Direct execution of the uploaded path itself, or any
+ 				// wrapper (timeout/env VAR=x/nohup/xargs/versioned
+ 				// interpreters) forwarding it as the operand to run.
- 			return p.deny("input_target", abs, "")
+ 				return p.deny("input_target", abs, "")
+ 			}
  		}


─── internal/modules/craft/input_code.go:243-249 ───
[security · high] 解释器分支对 `bash -c "python3 /workspace/inputs/<sha>/x.py"` 失效：内层命令作为单个含空白的 argv
元素传入，canonical 结果是 "/workspace/python3 /workspace/inputs/..."（空格保留在路径中），永远不命中 inputs 前缀而放行；现有测试只覆盖了
Shell=true 的 CommandText 形态，未覆盖 argv 形式的 -c 内嵌命令。建议解释器 operand 含空白/引号时按空白再切分逐 token 复检（与 shellTokens
同策略）。

  	} else if interpreter, offset := p.interpreterPrefix(req); interpreter {
  		// Any input-tree argument feeds the interpreter as the script to run.
  		for _, arg := range req.Command[offset:] {
- 			if abs := p.canonical(req.WorkingDir, arg); p.withinInputs(abs) {
+ 			for _, token := range strings.Fields(trimQuotes(arg)) {
+ 				if abs := p.canonical(req.WorkingDir, token); p.withinInputs(abs) {
- 				return p.deny("interpreter_input", abs, "")
+ 					return p.deny("interpreter_input", abs, "")
+ 				}
  			}
  		}


─── internal/modules/craft/input_code.go:255-257 ───
[security · high] 注释声称"digest 规则会在复制字节后续执行时拦截"，但生产适配层（ReviewNormalExec）从不回填
TargetSHA256/ResolvedTargetPath，第 1 层（符号链接）与第 2 层（字节身份）恒不触发，该承诺在唯一接线配置下不成立：先 `cp
/workspace/inputs/<sha>/x.py /workspace/app/main.py` 再 `python3 /workspace/app/main.py`，以及 `ln -s
/workspace/inputs/<sha>/x.py /workspace/run.sh && /workspace/run.sh`，全部检查通过，上传字节被原样执行——而 copy 与 link
正是文件头明确列出的防御对象（测试中的 copy/symlink 用例均依赖手工回填字段，掩盖了该缺口）。建议：要求适配层在 send
前于容器内探测主目标摘要并回填（完成设计中的第三层），本模块也应把"适配层未提供摘要且未解析链接"视为降级证据并在契约中显式声明，而非由注释给出实际不成立的保证。



─── internal/modules/craft/input_code.go:237-242 ───
[security · medium] shellTokens 纯词法切分可被间接引用绕过：`f=/workspace/inputs/<sha>/x.py; python3 "$f"`
中路径仅出现在赋值 token `f=...` 内（canonical 后不命中前缀）；`python3 "in""puts/x.py"` 与 `python3 in\puts/x.py` 经真实
shell 展开后指向 inputs，但策略看到的 token 分别为 `in""puts/x.py`、`in\puts/x.py`，均放行。同时末尾 TargetPath 兜底检查带 `&&
!req.Shell`，Shell=true 的请求即使适配层已填 TargetPath 也不复核。当前生产路径尚无 Shell=true 的生产者（受限执行一律
fail-closed），但该面一旦接线即为零成本绕过。建议与 ReviewOutputlessExec 的 fail-closed 立场对齐：shell
表达式在无法获得适配层归一化/摘要证据时按不可审查直接拒绝；至少应移除 `!req.Shell` 限制并扩展 token 归一（剥离赋值前缀、处理反斜杠转义与相邻引号拼接）。



─── internal/modules/craft/input_code.go:61-63 ───
[bug · low] 带 UTF-8 BOM 前缀（\xEF\xBB\xBF#!）的脚本不会被识别为 shebang，来源标签会被低估为 data。仅影响 ClassifyInputCode
的标签与后续审计口径，不影响执行拦截；建议检测前跳过 BOM。

  func hasShebang(content []byte) bool {
+ 	content = bytes.TrimPrefix(content, []byte("\xef\xbb\xbf"))
  	return len(content) >= 2 && content[0] == '#' && content[1] == '!'
  }


─── internal/application/service/craft_execution_policy.go:66-70 ───
[security · high] 门禁可被 shell 包装命令单条绕过：适配器只填 Command/WorkingDir，从不设置 Shell/CommandText。对 `Command:
["sh", "-c", "python /workspace/inputs/uploaded.py"]`，策略走 argv 分支（interpreterPrefix 命中 sh），对
Command[1:] 逐元素做 canonical——整个表达式作为单个参数被拼成 `/workspace/python
/workspace/inputs/uploaded.py`（空格不拆分），不在 inputs 树内 → 放行。而 craftDockerNormalProviderRequest 以
argv（Command[0]+Args）直发容器，sh -c 会真实执行上传脚本，T03「上传代码仅作数据」被绕过。策略模块专门设计的 Shell=true + shellTokens 路径（拆分
&& ; | 后逐 token 判定）完全没有被此适配器使用。建议识别 argv[0] basename 为 shell（sh/bash/zsh/dash/ksh/csh/tcsh/ash，含 env
包装）且带 -c 时，把 -c 后的表达式作为 CommandText 并置 Shell=true 送审。

  	execRequest := craft.InputExecutionRequest{
  		Command:    append([]string(nil), request.Command...),
  		WorkingDir: request.WorkingDir,
+ 	}
+ 	// A shell wrapper must be reviewed as the shell expression it will run.
+ 	if expr, ok := craft.ShellExpressionOf(request.Command); ok {
+ 		execRequest.Shell, execRequest.CommandText = true, expr
  	}
  	return g.review(ctx, request.TenantID, request.RunID, execRequest)


─── internal/application/service/craft_execution_policy.go:66-70 ───
[security · medium] request.Environment 完全不参与审查：`Environment: {"BASH_ENV":
"/workspace/inputs/init.sh"}` + `Command: ["bash"]` 会放行（interpreterPrefix 命中后 Command[1:]
为空，无参数可查），而 Environment 会被 craftDockerNormalProviderRequest 原样透传进容器，bash/POSIX sh 启动时 source
BASH_ENV，等于执行上传材料。PYTHONSTARTUP、ENV、NODE_OPTIONS（--require）、RUBYOPT、PERL5OPT 同理。建议至少对 Environment
的值做与命令 token 相同的 inputs 树词法判定，或拒绝上述启动钩子键指向 inputs 树。

  	execRequest := craft.InputExecutionRequest{
  		Command:    append([]string(nil), request.Command...),
  		WorkingDir: request.WorkingDir,
+ 	}
+ 	if hook := environmentTouchingInputs(request.Environment, g.workspaceRoot); hook != "" {
+ 		return fmt.Errorf("%w: environment %s points at uploaded input material", craft.ErrForbidden, hook)
  	}
  	return g.review(ctx, request.TenantID, request.RunID, execRequest)


─── internal/application/service/craft_execution_policy.go:35-41 ───
[security · medium] 注释宣称防线是 "lexical canonicalization plus canonical byte identity"，但适配器构造的
InputExecutionRequest 从不填 TargetSHA256（服务器读不到容器内字节，也无从计算），策略的 byDigest
字节同一性分支在本面永远不会触发；ResolvedTargetPath 同样恒空。因此本面实际只剩词法路径包含一条防线：先 `cp /workspace/inputs/x.py
/tmp/x.py`（或 `ln -s`，两步各自都会放行——cp/ln 非 interpreter，argv[0] 不在 inputs
树内）再执行拷贝/软链路径，即可让解释器执行上传字节。策略侧注释 "the digest rule above catches the copied bytes when they later
execute" 在此适配器上不成立。建议：修正本注释以反映实际防线范围（仅词法），并考虑由能观测容器文件系统的一侧（如执行回执/挂载清单）补 digest
证据，否则应把该两步绕过明确记为已知残余风险，供后续 ticket 收紧。



─── internal/application/service/craft_execution_policy.go:86-88 ───
[security · medium] 作为 T03 安全控件，本 gate 的审查身份完全取自被审查请求自身（self-attested TenantID/RunID），未与 Execute 链上的
grantID/activityID/binding 或鉴权 scope 做任何交叉校验；coordinator.Stage 的 run.SessionID == request.TaskID
校验发生在 gate 之后，且同样基于请求内嵌身份。若上游调用链允许指定同租户的任意 run_id，指向一个输入清单为空的合法 Run 即可把字节同一性集合清零（路径判定仍在，但 digest
维度失效），叠加词法判定的绕过面。当前虽无生产调用点，但接线前应收紧契约：gate 改用从 grant/binding 服务端解析出的 tenant/run，或至少复用 Stage 的
session/task 一致性校验后再审查。



─── internal/application/service/craft_execution_policy.go:82-82 ───
[security · low] 恒拒路径（本处）以及 review() 中的身份缺失、agent_runs 查询失败、快照解析失败、无 workspace seed 等 fail-closed
拒绝，都不会产生任何 craft.input.execute_denied 持久审计事件——writeAuditRow 只覆盖 MaterialPolicy.ReviewExecution
产生的策略决策。T03 要求拒绝事件写入 audit_rows；这些路径只留下日志行，审计证据在受限面与故障路径上不完整。建议让 gate 持有 audit sink（或复用
CraftDelegateService 注入的 audit），对恒拒与 fail-closed 拒绝补记 outcome=denied 的行（不含载荷，仅身份）。



─── internal/application/service/craft_execution_policy.go:50-50 ───
[maintainability · low] 装配确认：NewCraftDelegateExecutionPolicy、两个命令面的 WithExecutionPolicy、以及
NewCraftDelegation 的 audit 参数目前在生产代码中均无调用点（仅测试使用）。本 PR 合并后，T03 门禁在所有生产路径上都不会生效（两个命令面保持 nil legacy
行为），材料审计事件也只会停留在日志行（audit 静默降级为 nil 且无任何信号）。若中心装配由后续 ticket 完成，请确认其存在并覆盖：gate 注入两个 exec
面、AuditLogService 实参传入 NewCraftDelegation——否则安全控件只存在于测试中。



─── internal/modules/craft/input_code.go:274-276 ───
[security · high] interpreterCommands 用 basename 精确等值匹配识别解释器，带版本后缀的解释器名完全不被识别：`python3.11
/workspace/inputs/<sha>/x.py`（或 `php8.2`、`ruby3.1`、`lua5.4`、`tclsh8.6`、Debian 默认 awk 实现 `mawk`，这些在
Debian/Ubuntu 基础镜像中是真实存在的二进制，python3 只是指向它们的符号链接）中 argv[0] 的 basename 是
"python3.11"，不在表内，于是落入非解释器分支，只检查 Command[0]（解释器自身，不在 inputs 树内）而放行，上传脚本被直接执行。这与已确认的 timeout/env
包装绕过根因不同：这里 argv[0] 本身就是解释器，只是精确匹配漏识别。建议匹配时容忍版本后缀（同时补充 scala/groovy/elixir/runghc 等缺失项，或改为前缀模式匹配）：

- 	if interpreterCommands[path.Base(req.Command[0])] {
+ // interpreterBase strips a version suffix so python3.11/php8.2/lua5.4
+ // still resolve to their base interpreter name.
+ func interpreterBase(name string) string {
+ 	base := path.Base(name)
+ 	if interpreterCommands[base] {
+ 		return base
+ 	}
+ 	return strings.TrimRight(base, "0123456789.")
+ }
+ 
+ func (p *InputExecutionPolicy) interpreterPrefix(req InputExecutionRequest) (bool, int) {
+ 	if len(req.Command) == 0 {
+ 		return false, 0
+ 	}
+ 	if interpreterCommands[interpreterBase(req.Command[0])] {
  		return true, 1
+ 	}
+ 	if path.Base(req.Command[0]) == "env" && len(req.Command) > 1 && interpreterCommands[interpreterBase(req.Command[1])] {
+ 		return true, 2
+ 	}
+ 	return false, 0
- 	}
+ }


─── internal/modules/craft/input_code.go:245-249 ───
[bug · medium] 解释器分支把 offset 之后的所有 operand 都当作"要执行的脚本"检查，会误杀本策略明确允许的合法用法：`python3
/workspace/app/main.py /workspace/inputs/<sha>/data.csv` 中 main.py 是可写区生成代码，data.csv 只是作为 argv
传入的上传数据（本文件自述读取 input 是允许的：AuditKindInputRead "read, cited, parsed — never executed"，非解释器分支注释也允许
cp/mv/cat 命名 input 材料），但该命令因 data.csv 命中 inputs 树被拒，且 Refusal 给出的替代方案（"generate the code in the
writable Workspace and execute the generated file"）正是它已经在做的事，Agent
会陷入无解的拒绝死胡同，只能改写脚本硬编码路径。解释器实际只执行第一个非选项 operand，后续 operand 是脚本的数据。建议只对脚本 operand（第一个非 `-` 前缀
operand）做 inputs 树判定，其余 operand 按数据读取记录审计；注意个别带值选项（如 `python3 -W ignore`）的值会被误认为脚本
operand，必要时用小的选项值表或在 InputExecutionRequest 上显式标注脚本 operand。

+ 		// Only the first non-flag operand is the script the interpreter
+ 		// executes; later operands are argv data passed to that script.
  		for _, arg := range req.Command[offset:] {
+ 			if strings.HasPrefix(arg, "-") {
+ 				continue // flags; option values need a per-interpreter table
+ 			}
  			if abs := p.canonical(req.WorkingDir, arg); p.withinInputs(abs) {
  				return p.deny("interpreter_input", abs, "")
  			}
+ 			break // script operand found outside the input tree
  		}


─── internal/application/service/craft_execution_policy.go:66-70 ───
[security · high] 与已确认的 Environment 缺口同族但独立的通道：request.Stdin / StdinEnabled 完全不参与审查，而 stdin
会被原样送进容器——craftDockerNormalProviderRequest 设 Stdin: string(request.Stdin)，StartAttachedExecOnce 透传
staged.Request.Stdin。单条请求即可绕过 T03：`Command: ["python3", "-"]`（或裸 `["bash"]`/`["sh"]`/`["node"]`）+
`StdinEnabled: true` + `Stdin` = 上传的 .py/.sh 字节。argv 分支中 "-" 或空参数列表经 canonical 后永远不在 <root>/inputs 内
→ 放行并审计为 craft.generated.execute，随后解释器从 stdin 执行上传材料字节。与容器内 symlink（服务器无法求值）不同，这条通道服务端完全可查：准入输入的
SHA256 就在 review() 加载的 manifest 里，stdin 字节就在被审查的请求上。建议至少：StdinEnabled 且 len(Stdin)>0 时计算
sha256(request.Stdin) 并与准入输入摘要比对（可直接复用 policy.Review 的 TargetSHA256
字节同一性分支语义——它表达的正是"将要执行的字节"），并拒绝解释器从 stdin 读程序的 argv 形态（"-"、"/dev/stdin"、StdinEnabled 下的裸解释器）。

  	execRequest := craft.InputExecutionRequest{
  		Command:    append([]string(nil), request.Command...),
  		WorkingDir: request.WorkingDir,
+ 	}
+ 	// T03: stdin 是可被解释器执行的载荷通道，必须与 Command 一起审查。
+ 	if request.StdinEnabled && len(request.Stdin) > 0 {
+ 		sum := sha256.Sum256(request.Stdin)
+ 		execRequest.TargetSHA256 = hex.EncodeToString(sum[:]) // "将要执行的字节"的摘要，命中准入输入即拒
  	}
  	return g.review(ctx, request.TenantID, request.RunID, execRequest)

