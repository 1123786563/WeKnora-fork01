Review complete: 6 finding(s) across 4 selected item(s).

─── internal/container/craft_web_build_review.go:190-200 ───
[bug · high] nil 接收者检查顺序颠倒：`g.hostInputsRel` 字段读取与 `g.warnInputsRootOnce.Do(...)` 都在 `if g == nil`
防御分支之前执行。以 nil gate 调用 Review 时第一行字段访问即空指针 panic，第 198 行精心准备的 fail-closed 拒绝分支（"gate is not
assembled"）成为永远不可达的死代码。作者明确预留了 nil 防御（且同仓约定一致：CraftDelegateExecutionPolicy.ReviewNormalExec 第 114
行、本文件 g.policy 检查都是先判 nil），T20 中央装配按注释设计会「附加同一 gate 实例」——装配失败返回 nil gate 的场景正是该防御的目标场景，届时请求路径崩溃而非返回
ErrForbidden 拒绝。请把 nil 检查移到方法最前。

+ 	if g == nil {
+ 		return fmt.Errorf("%w: craft web build command gate is not assembled", craft.ErrForbidden)
+ 	}
  	if g.hostInputsRel == "" {
  		// The material-overlap layer is unwired (no WithCraftWebHostInputsRoot
  		// at assembly): say so once per process instead of failing open in
  		// silence — the operator can then wire the host root.
  		g.warnInputsRootOnce.Do(func() {
  			logger.Warnf(ctx, "[CraftWebBuildGate] host inputs root NOT configured: the staged-material overlap layer is inactive (pass WithCraftWebHostInputsRoot at assembly)")
  		})
- 	}
- 	if g == nil {
- 		return fmt.Errorf("%w: craft web build command gate is not assembled", craft.ErrForbidden)
  	}


─── internal/application/service/craft_execution_policy.go:130-139 ───
[security · medium] 证据优先级分支在「同时携带显式 TargetSHA256 与 stdin 字节」的请求形态下，静默关闭了 stdin
通道自身的字节身份校验，而非仅仅"不覆盖"。模块策略第 2 层（input_code.go:259-267）是拒绝证据：stdin 摘要命中 manifest 即拒绝。改动前任何
StdinEnabled 且带字节的请求都会做该比对；改动后只要 TargetSHA256 非空且恰好不在 manifest 中（例如解析出的构建程序摘要），stdin 里携带的上传材料字节将完全绕过
identity 层拒绝。当前无生产调用方填充 TargetSHA256，风险取决于 T20
接线面的构造纪律，但该接口是导出的中央审查点，防御不应依赖未来调用方不同时填两字段。建议：优先裁决可以保留（stdin 摘要不覆盖显式摘要），但 stdin
摘要应作为独立证据继续送审（例如两份摘要都探测、任一命中 manifest 即拒绝），或直接拒绝「显式摘要 + 启用 stdin 且有字节」的组合形态，保持 fail-closed。

  	// Evidence priority (central ruling): an explicitly carried
  	// TargetSHA256 (a server-resolvable face walked the host target) WINS —
  	// the stdin digest never silently overwrites it.
  	if request.TargetSHA256 == "" && request.StdinEnabled && len(request.Stdin) > 0 {
  		// Byte identity for the stdin channel: the stdin bytes ride on the
  		// reviewed request itself, so their digest can be matched against
  		// the admitted manifest exactly like a file target.
  		stdinSum := sha256.Sum256(request.Stdin)
  		execRequest.TargetSHA256 = hex.EncodeToString(stdinSum[:])
+ 	} else if request.TargetSHA256 != "" && request.StdinEnabled && len(request.Stdin) > 0 {
+ 		// Both evidences present: a carried non-manifest digest must not
+ 		// silently disable the stdin channel's own identity screen — probe
+ 		// the stdin digest independently (deny on manifest match) or refuse
+ 		// the combined shape, keeping the layer fail-closed.
+ 		_ = sha256.Sum256(request.Stdin)
  	}


─── internal/container/craft_web_build_review.go:137-143 ───
[security · medium] argv[0] 裸名钉死的防包装器声明可被请求携带的 Environment/PATH 击穿：形状层只保证命令字面上写的是
"python3"，但该名字最终由执行器（Docker exec 在容器内按 exec 环境的 PATH 解析二进制）解析，而 request.Environment 原样透传——T03 第 2b
层（input_code.go:275-289）只把环境值各段与只读 inputs 树比对，指向可写目录（/tmp、/workspace）的 PATH 条目完全放行。于是在可写目录放置同名
python3 包装器 + 请求携带 PATH 覆盖，即可借用本入口执行任意代码，注释中"(/tmp/evil/python3, ./python3) must not borrow this
entry"的保证在本门禁自身控制范围内无法兑现。建议本入口（固定形状面）在 Review 中拒绝覆盖 PATH 的 Environment
条目（或对固定入口白名单化环境），否则应把注释的防借用声明收窄到"字面路径形式"并记录 PATH 解析为 T20 接线时必须由执行侧关闭的残余。

  	// argv[0] is pinned to the EXACT interpreter name (never a path):
  	// a same-named wrapper or symlink at any writable location
  	// (/tmp/evil/python3, ./python3) must not borrow this entry, and
- 	// path.Base would happily accept all of them.
+ 	// path.Base would happily accept all of them. NOTE: this pins only the
+ 	// LITERAL argv token — the bare name still resolves through the exec
+ 	// environment's PATH, so Review must additionally refuse request-carried
+ 	// PATH overrides (see the Environment screen in Review).
  	if len(command) < 2 || command[0] != "python3" || command[1] != craftWebBuildProgramPath {
  		return false
+ 	}
+ 
+ // 并在 Review 的形状检查后追加：
+ 	for name := range request.Environment {
+ 		if name == "PATH" {
+ 			logger.Warnf(ctx, "[CraftWebBuild] refusing PATH override for run %s: the pinned interpreter must resolve through the image PATH", request.RunID)
+ 			return fmt.Errorf("%w: craft web build command entry fixes the interpreter environment; a request-carried PATH is refused", craft.ErrForbidden)
+ 		}
  	}


─── internal/container/craft_web_build_review.go:257-264 ───
[bug · low] filepath.Rel 失败（典型：toolchainDir 由 CRAFT_WEB_TOOLCHAIN_DIR 配成相对路径，resolvedDir 为相对路径而
hostInputsRel 为绝对路径，二者无法求相对关系）时静默返回 false，使 verifyHostToolchain 的「工具链重叠只读材料树」检查在 hostInputsRel != ""
时无声失效——此时第 190 行的"未配置"告警不会触发（字段非空），运维对层失效毫无感知，恰与该方法"拒绝在沉默中 fail-open"的注释意图相反。建议在
NewCraftWebBuildCommandGate 装配时强制 toolchainDir / host root 为绝对路径（EvalSymlinks 保持相对性），或在 Rel
出错时记日志并按命中重叠处理（fail-closed 拒绝派发）。

  func pathWithinDir(directory, target string) bool {
  	if directory == "" || target == "" {
  		return false
  	}
  	rel, err := filepath.Rel(directory, target)
  	if err != nil {
- 		return false
+ 		// Mixed absolute/relative roots (e.g. a relative toolchain dir env)
+ 		// must not silently disable the overlap layer — treat it as overlap
+ 		// so verifyHostToolchain refuses dispatch (fail closed).
+ 		return true
  	}


─── internal/application/repository/craft_docker_normal_input.go:61-62 ───
[bug · medium] 新增证据字段未纳入 validCraftDockerNormalInput 的形状校验：同文件 validCraftDockerNormalReceipt 对同类的
StdinSHA256 强制 len==64 + hex.DecodeString 校验（:491-497），validCraftDockerNormalInput 对
Command/User/WorkingDir/env 值均做 NUL 筛查，而 TargetSHA256（摘要字段，将进入加密持久身份并在 identity 层作 map 键参与裁决）与
ResolvedTargetPath 完全不校验。畸形 TargetSHA256（非 64-hex、大写 hex、截断值）在 p.byDigest
查找时只会静默未命中，等同"无证据"——结合本轮"显式 TargetSHA256 优先并抑制 stdin 摘要推导"的裁决，一个被错误填充的畸形摘要会无任何信号地关闭 stdin
通道的字节身份审查，且已被 Stage 接受为持久身份（重放 DeepEqual 一致，无法事后纠正）。建议在 validCraftDockerNormalInput 补：TargetSHA256
非空时 64 位小写 hex 校验、两字段 NUL 筛查（既有持久行两字段均为空，收紧无兼容性影响）。

- 	ResolvedTargetPath string `json:"resolved_target_path,omitempty"`
- 	TargetSHA256       string `json:"target_sha256,omitempty"`
+ func validCraftDockerNormalInput(in CraftDockerNormalInputRequest) bool {
+ 	// ... 既有校验之外追加：
+ 	if in.TargetSHA256 != "" {
+ 		if len(in.TargetSHA256) != 64 || strings.ContainsRune(in.TargetSHA256, '\x00') {
+ 			return false
+ 		}
+ 		if _, err := hex.DecodeString(in.TargetSHA256); err != nil {
+ 			return false
+ 		}
+ 	}
+ 	if strings.ContainsRune(in.ResolvedTargetPath, '\x00') {
+ 		return false
+ 	}
+ 	// ...


─── internal/container/craft_web_build_review.go:43-46 ───
[documentation · low] 该 ResolvedTargetPath note 与本轮同一变更集自相矛盾：注释声称"共享 DTO 不携带服务端解析目标字段、完整转发需要由中央装配做
additive DTO 变更"，但本提交已在 repository.CraftDockerNormalInputRequest 落地
ResolvedTargetPath/TargetSHA256（omitempty），且 CraftDelegateExecutionPolicy.ReviewNormalExec
已按中央裁决透传。注释同时声称解析事实"不经 CraftDockerNormalInputRequest 转发"，而 Review 原样把 request 交给
g.policy.ReviewNormalExec——DTO 一旦被派发面填充即会进入模块策略的 symlink/identity 层（以容器 /workspace
为基准解释宿主路径），"不转发"并无代码强制。按此注释接线的 T20 lane 会误以为还需自行扩 DTO 或误信该入口不消费这些字段。建议改写为：DTO 字段已落地并经
ReviewNormalExec 透传；本入口因宿主/容器命名空间差异在层 2 强制宿主事实，派发面不得以容器语义填充这两个字段（或由本入口在转发前清空）。


