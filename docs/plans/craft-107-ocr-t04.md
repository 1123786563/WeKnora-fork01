Review complete: 21 finding(s) across 14 selected item(s).

─── docs/testing/craft/t04/offline-build-output/assets/craft-web.css:1-3 ───
[test · low] 快照与固定工具链之间缺少机器校验（覆盖本目录 css/js/index.html/build-log.json 四件套）：它们是 docker/craft/web
产物的人工拷贝，当前与 deps/lock 逐字一致，但全仓库没有任何测试引用
offline-build-output——TestCraftWebToolchainPinsMatchShippedFiles 只覆盖 docker/craft/web 侧
lock↔runtime-config↔文件字节三方一致，scripts/check-craft-release.py 校验的是 release-evidence.json。今后模板或 deps
升级（template_sha256/依赖 sha256/toolchain_digest 变化）而快照未同步时，T04 的"固定证据"会静默失真并误导排查（本次 README 与快照 digest
不一致正是这类失真的实例）。建议在 CI 中按 toolchain.lock.json 的 sha256 校验快照资产与 deps 逐字一致，或由脚本自动再生本快照。



─── docs/testing/craft/t04/offline-build-output/assets/craft-web.js:1-1 ───
[maintainability · medium] 版本登记漂移：本文件（及 docker/craft/web/deps/craft-web.js）头部自声明 v1.1.0，但固定登记表
docker/craft/web/toolchain.lock.json 中 craft-web.js 的 version 仍为 "1.0.0"（对应同一 sha256）。build.py 的
load_pin 与 Go 侧 LoadCraftWebToolchainPin 都只校验 sha256、从不比对依赖 version 字段，因此该漂移不会被任何环节发现。t04 fixture
的定位是"历史版本证据固定"，登记失真会削弱其审计价值——事实上漂移已在发生：docs/testing/craft/t04/README.md 引用的 toolchain_digest 仍是旧值
9459a7f4…，与本 fixture 的 b121a3be… 不一致。建议：① 将 lock 中 craft-web.js 的 version 对齐为 1.1.0；② 增加同步校验断言本目录
assets 与 docker/craft/web/deps 逐字节一致，使漂移可被测试捕获。

- /* craft-web.js v1.1.0 — provisioned offline runtime of the Craft web
+ # docker/craft/web/toolchain.lock.json 中对齐版本
+     {
+       "name": "craft-web.js",
+       "version": "1.1.0",
+       "sha256": "3812625d71f5c10c26d7d9c21fa5d88d024969636c75060db65b6297b2ad055a"
+     }


─── docs/testing/craft/t04/offline-build-output/assets/craft-web.js:31-31 ───
[maintainability · medium] 配对依赖隐式跨文件结构约定：已核实 build.py render_table 当前确实按 <h2><input><table>
顺序紧邻输出，但此处用 previousElementSibling 位置推断配对，任一侧后续改动（如在 input 与 table
间插入包装节点、调整模板）都会让筛选控件静默失效——不匹配时直接跳过、无任何告警，且仓库中没有渲染输出+JS 行为的联动测试（build.py --selftest
只校验文案存在），该回归不可察觉。建议改为显式配对：build.py 为每张表输出 id 并在 input 上写 aria-controls（或 data-craft-filter-for），JS
按引用查找；至少应在表格无配对输入时输出 console.warn 使漂移可见。

-       const input = table.previousElementSibling;
+     // build.py 渲染 <input class="craft-filter" aria-controls="tN"> + <table id="tN" class="craft-table">
+     for (const input of document.querySelectorAll("input.craft-filter")) {
+       const targetId = input.getAttribute("aria-controls");
+       const table = targetId === null ? null : document.getElementById(targetId);
+       if (table !== null && table.classList.contains("craft-table")) {
+         input.addEventListener("input", function () { filterTable(input, table); });
+       } else {
+         console.warn("craft-web: filter input has no paired craft-table", input);
+       }
+     }


─── docs/testing/craft/t04/offline-build-output/assets/craft-web.js:12-15 ───
[performance · low] 每次 input 事件都对全部行重新读取 row.textContent 并做 trim/toLowerCase 规整，单次按键成本为
O(行数×文本长度)。build.py 渲染的表格是静态内容（运行时无增删行），可在绑定事件时预计算每行 haystack 缓存，输入时仅做 indexOf 与属性切换。

+   function bindFilter(input, table) {
+     const entries = [];
+     for (const row of table.querySelectorAll("tbody tr")) {
+       entries.push({ row: row, text: normalize(row.textContent) });
+     }
+     input.addEventListener("input", function () {
-     const needle = normalize(input.value);
+       const needle = normalize(input.value);
-     const rows = table.querySelectorAll("tbody tr");
-     for (const row of rows) {
-       const haystack = normalize(row.textContent);
+       for (const entry of entries) {
+         const hit = needle === "" || entry.text.indexOf(needle) !== -1;
+         if (hit) {
+           entry.row.removeAttribute("data-craft-filter-hide");
+         } else {
+           entry.row.setAttribute("data-craft-filter-hide", "1");
+         }
+       }
+     });
+   }


─── docs/testing/craft/t04/offline-build-output/assets/craft-web.js:32-32 ───
[maintainability · low] previousElementSibling 只会返回 Element 或 null，而 Element.classList 恒有定义，因此 input
非空时 input.classList !== undefined 恒为真，属冗余防御条件，略微降低可读性；简化为判空 + contains 即可。

-       if (input !== null && input.classList !== undefined && input.classList.contains("craft-filter")) {
+       if (input !== null && input.classList.contains("craft-filter")) {


─── docs/testing/craft/t04/offline-build-output/assets/craft-web.js:1-3 ───
[documentation · medium] 证据链自相矛盾（区别于已确认的 lock version 字段漂移）：本快照的 craft-web.js 携带 v1.1.0 字节，快照内
build-log.json 与现行 toolchain.lock.json 的 toolchain_digest 均为 b121a3be…，说明快照是按当前工具链重新生成的产物；但
docs/testing/craft/t04/README.md（本次更新未改动）第 43 行仍声称引用的是 offline-build-output/build-log.json
的"未截断原文"，其中 toolchain_digest 却是
9459a7f4…（旧工具链的构建结果）。同一份"历史版本证据固定"材料出现两个互斥摘要，审计者无法判断哪次构建真实发生。建议二者择一对齐：要么更新 README 引文（及
docker-offline-build.log）为重跑后的真实日志，要么让快照保留被记录那次原始运行的产物。



─── internal/container/craft_runtime.go:150-150 ───
[bug · high] build 证据读取与被采集的 Run 无任何绑定，且在 run-bound 生产路线上读的是另一棵树。事实链：(1) 该包装后的 evidence 同时注入了 legacy
服务（line 156）和 runViewArtifacts（line 169），而 CollectCandidate 会对每次 run-bound 候选采集调用
s.evidence（craft_artifacts.go:250-253）；(2) run-bound 制品由 runBoundCraftArtifactSource 从 Run 私有的
verified generation 输出读取（openOutputRoot → openVerifiedRunViewInputs + Openat("output")），而日志读取器
craftSessionBuildLogReader(source, outputDir) 经 localCraftArtifactSource 解析的是共享的
workDir/<env-outputDir> 指针——R4 之后该指针已无生产维护方（pointWorkspaceOutput 仅剩测试引用），两条路径指向不同目录。后果二选一：该路径下没有
build-log.json 时，T04 构建事实静默保持 not_run（build.py 实际运行过却永不被观测，新特性在生产路线失效）；或该路径残留旧 delegation
的日志时——pin（toolchain/template）是部署级常量必然通过校验，日志又不携带 Run ID——会把**别的 delegation 的 exit status 折进当前 Run 的
BuildChecks**，而构建检查正是新版本能否成为默认的门槛。另外 runViewArtifacts 固定 craftLocalOutputDir 而读取器跟随 env
outputDir，非默认 CRAFT_OPENCODE_OUTPUT_DIR 下偏移进一步扩大。同装配的 preview EvidenceSource 显式按 WorkspaceID+RunID
匹配以防御同类歧义，此处应同等处理。

- 		evidence = craftWebBuildEvidenceSource(evidence, craftSessionBuildLogReader(source, outputDir), pin)
+ 	// 方案一：仅为 legacy 会话级路由包装证据，run-bound 候选服务保持 nil evidence（与 capture 协调器一致）：
+ 	if toolchainDir := strings.TrimSpace(os.Getenv(craftWebToolchainDirEnv)); toolchainDir != "" {
+ 		pin, pinErr := LoadCraftWebToolchainPin(toolchainDir)
+ 		if pinErr != nil {
+ 			return nil, fmt.Errorf("craft web toolchain pin %s: %w", toolchainDir, pinErr)
+ 		}
+ 		legacyEvidence := craftWebBuildEvidenceSource(evidence, craftSessionBuildLogReader(source, outputDir), pin)
+ 		artifacts := service.NewCraftArtifactServiceWithCandidates(source, files, versions,
+ 			repository.NewCraftCandidateStore(db), legacyEvidence,
+ 			service.CraftArtifactConfig{Kind: craft.KindWeb, OutputDir: outputDir})
+ 		_ = artifacts
+ 	}
+ 	// 方案二（更完整）：让 build-log 经由该次采集实际使用的 Run-bound source 的 listed-file 契约读取（CollectCandidate 已持有该 source），使日志与制品来自同一 verified 目录。


─── internal/container/craft_web_build.go:216-219 ───
[bug · medium] 尾随内容检查逻辑写反了：`dec.Decode(&trailing)` 只有在尾随内容恰好是合法 JSON 时才返回 nil（触发拒绝），而形如 `{...}垃圾尾巴`
的输入会返回 SyntaxError（err != nil），检查直接放行——畸形日志/锁文件被当作合法内容接受，与"严格契约"注释相反。正确写法是要求流恰好耗尽（返回
io.EOF）。build.py 正常写入的尾随换行符会得到 io.EOF，不受影响。LoadCraftWebToolchainPin 中存在完全相同的问题。

  	var trailing any
- 	if err := dec.Decode(&trailing); err == nil {
+ 	if err := dec.Decode(&trailing); !errors.Is(err, io.EOF) {
  		return CraftWebBuildLog{}, fmt.Errorf("%w: craft web build log carries trailing content", craft.ErrInvalidInput)
  	}
+ // （需补充 "io" import；io.EOF 之外的任何错误——包括语法错误——都意味着存在尾随内容，应拒绝）


─── internal/container/craft_web_build.go:120-123 ───
[bug · medium] 与 ParseCraftWebBuildLog 相同的尾随内容检查反转问题：`{...}garbage` 形式的 lock 文件会通过校验被信任。lock
来自部署目录，截断/损坏的 lock 若带非法尾随内容不应被接受；应改为 `err != io.EOF` 即拒绝。

  	var trailing any
- 	if err := dec.Decode(&trailing); err == nil {
+ 	if err := dec.Decode(&trailing); !errors.Is(err, io.EOF) {
  		return CraftWebToolchainPin{}, fmt.Errorf("%w: craft web toolchain lock carries trailing content", craft.ErrInvalidInput)
  	}


─── internal/container/craft_web_build.go:232-234 ───
[bug · medium] `ExitCode` 为值类型 int，日志缺失 `exit_code` 字段（或写 null）时解码为 0，恰好通过 0..255 校验并被
CraftWebBuildEvidence 折叠为 `BuildRan:true, BuildExitCode:0`（CheckBuild=passed）。build-log.json 位于
agent 可写的 output 目录、本就是严格解析要防御的不可信输入，"字段缺失被解释为构建成功"与文件头"绝不伪造 build 事实"的设计目标直接矛盾。应改用 *int 并显式校验存在性。

- 	if log.ExitCode < 0 || log.ExitCode > 255 {
- 		return CraftWebBuildLog{}, fmt.Errorf("%w: craft web build log exit code %d is impossible", craft.ErrInvalidInput, log.ExitCode)
+ 	// 结构体字段改为: ExitCode *int `json:"exit_code"`
+ 	if log.ExitCode == nil || *log.ExitCode < 0 || *log.ExitCode > 255 {
+ 		return CraftWebBuildLog{}, fmt.Errorf("%w: craft web build log exit code is missing or impossible", craft.ErrInvalidInput)
  	}
+ // （CraftWebBuildEvidence 随之使用 *log.ExitCode）


─── internal/container/craft_web_build.go:289-289 ───
[security · medium] 构建证据完整性依赖沙箱自报：build-log.json 写在 agent 可写的 /workspace/output（Dockerfile 中 craft
属主唯一可写树），而伪造日志所需的全部 pin 值（toolchain_digest、template sha256/version）在镜像内
/opt/craft/web/toolchain.lock.json 以 0555 世界可读。被委派的 agent 不运行 build.py 也能手工写出 digest 全对、exit_code=0
的日志并自行编写 index.html/assets，Go 端与 pin 的比对只能证明日志作者读得到 lock（沙箱本身授予该能力），不能证明构建发生过。这会绕过 build.py
的内容拒绝清单（外部 URL、事件属性、script/embed 标签等仅在建库时检查），使 CheckBuild=passed
与"固定离线工具链产物"的保证落空。建议由平台侧带外记录委派构建的真实退出码（如装配层在委派包装中捕获 build.py 的执行结果，而非信任 output 目录内的自报文件），或将构建日志的写入移出
agent 可写路径。



─── internal/container/craft_web_build.go:313-317 ───
[bug · low] 对 readLog 的任何 error（包括最常见的 os.ReadFile 缺失文件错误
fs.ErrNotExist——localCraftArtifactSource.ReadSessionFile 原样返回 os 错误）都执行 Warnf，与紧邻注释"missing file …
stays silent"相反。每个未产出构建日志的 run（非 web 作品、构建前的采集轮次）都会产生一条告警，长期形成日志噪音并稀释真正的篡改告警。应排除 not-exist。

- 		if err != nil {
+ 		if err != nil && !errors.Is(err, fs.ErrNotExist) {
  			// Missing file is the common "no build ran" case and stays
  			// silent; a read failure beyond that is worth a trace.
  			logger.Warnf(ctx, "[CraftWebBuild] build log read failed for run %s: %v", task.Fence.RunID, err)
  		}
+ // （需补充 "io/fs" import）


─── internal/container/craft_web_build.go:124-126 ───
[maintainability · low] lock 的 `schema` 字段被解码后从未校验取值，而 build log 侧却检查了 `log.Schema !=
1`——两侧严格性不对称。未来 lock 格式升版（schema 2）时旧代码会静默按 v1 语义解析。建议补上 schema 检查（build.py 的 load_pin 同样缺失，可一并补齐）。

+ 	if lock.Schema != 1 {
+ 		return CraftWebToolchainPin{}, fmt.Errorf("%w: craft web toolchain lock schema %d is unsupported", craft.ErrInvalidInput, lock.Schema)
+ 	}
  	if lock.Name != craftWebToolchainName {
  		return CraftWebToolchainPin{}, fmt.Errorf("%w: craft web toolchain lock names %q", craft.ErrInvalidInput, lock.Name)
  	}


─── internal/container/craft_web_build.go:60-67 ───
[maintainability · low] 与 Python 端的排序口径存在潜在分歧：build.py 对完整 "name:sha" 字节串做 sorted()，Go 端只对 name
排序。当某个依赖名是另一个的前缀且下一字符 < ':'（ASCII 58，如数字、'-'，例如 "craft-web" 与 "craft-web-2.css"）时两种排序结果不同，导出的 digest
会不一致——当前固定依赖集 {craft-web.css, craft-web.js} 无前缀关系所以暂无影响，但该导出函数的文档承诺"byte-for-byte
mirror"，未来新增依赖时会以装配失败/日志被拒的形式静默踩坑。建议直接镜像 Python 口径：拼好 "name:sha" 后整体排序。

- 	for i, name := range names {
+ 	pairs := make([]string, 0, len(dependencies))
+ 	for name, sha := range dependencies {
+ 		pairs = append(pairs, name+":"+sha)
+ 	}
+ 	sort.Strings(pairs)
+ 	for i, pair := range pairs {
  		if i > 0 {
  			payload.WriteByte(0)
  		}
- 		payload.WriteString(name)
- 		payload.WriteString(":")
- 		payload.WriteString(dependencies[name])
+ 		payload.WriteString(pair)
  	}


─── internal/container/craft_web_build.go:276-278 ───
[bug · medium] runtime_digest 只做了非空校验，从未与部署自身的 runtime digest 比对，与文件头及本函数的"foreign log
拒绝"契约不符。newCraftRuntimeExecutor 已经算出 runtimeDigest :=
craftRuntimeDigestFromEnv()（craftOpenCodeRuntimeDigestEnv 的语义即"盖进 provisioned workspace 的 runtime
身份"），build.py 也把它写进日志作为身份字段，但 CraftWebToolchainPin 不携带该值、CraftWebBuildEvidence 只比对
toolchain/template 两项——一份由不同 runtime（如旧部署残留、或共享树中另一 serve 实例）产生、其余 pin
全同的日志会被原样折叠为当前部署的构建证据。既然注释承诺"three independent derivations must agree"，runtime 身份也应参与比对。

+ type CraftWebToolchainPin struct {
+ 	ToolchainDigest string
+ 	TemplateVersion string
+ 	TemplateSHA256  string
+ 	RuntimeDigest   string // 由装配处以 craftRuntimeDigestFromEnv() 填入
+ }
+ 
+ func CraftWebBuildEvidence(log CraftWebBuildLog, pin CraftWebToolchainPin) (craft.ArtifactEvidence, error) {
+ 	if log.RuntimeDigest != pin.RuntimeDigest {
+ 		return craft.ArtifactEvidence{}, fmt.Errorf("%w: build log names runtime %s, deployment pins %s", craft.ErrConflict, log.RuntimeDigest, pin.RuntimeDigest)
+ 	}
  	if log.ToolchainDigest != pin.ToolchainDigest {
  		return craft.ArtifactEvidence{}, fmt.Errorf("%w: build log names toolchain %s, deployment pins %s", craft.ErrConflict, log.ToolchainDigest, pin.ToolchainDigest)
  	}
+ 	// ...(其余不变)


─── docker/craft/Dockerfile:255-259 ───
[maintainability · medium] 交叉断言不完整：runtime-config.json 的 web_toolchain 与 toolchain.lock.json 双权威只比对了
toolchain_digest / template_sha256 / build_program_sha256 三个标量，漏掉了 dependencies 数组里逐文件钉住的
craft-web.css / craft-web.js sha256（以及 template_version）。toolchain_digest 是标量相等：升级时若 lock 更新且
runtime-config 的三个标量同步更新、但 dependencies 数组忘改，镜像仍构建通过，发布出 digest 与依赖哈希自相矛盾的 runtime
身份文件——这正是上方注释（"fails the image instead of shipping a runtime identity file with stale toolchain
metadata"）承诺要拦截的情况。建议补上依赖集比对。

      rc_build="$(python3 -c 'import json;print(json.load(open("/etc/craft/runtime-config.json"))["web_toolchain"]["build_program_sha256"])')" && \
      lock_build="$(python3 -c 'import json;print(json.load(open("/opt/craft/web/toolchain.lock.json"))["build_program"]["sha256"])')" && \
+     rc_deps="$(python3 -c 'import json;print(sorted((d["name"],d["sha256"]) for d in json.load(open("/etc/craft/runtime-config.json"))["web_toolchain"]["dependencies"]))')" && \
+     lock_deps="$(python3 -c 'import json;print(sorted((d["name"],d["sha256"]) for d in json.load(open("/opt/craft/web/toolchain.lock.json"))["dependencies"]))')" && \
      test "$rc_digest" = "$lock_digest" && \
      test "$rc_template" = "$lock_template" && \
-     test "$rc_build" = "$lock_build"
+     test "$rc_build" = "$lock_build" && \
+     test "$rc_deps" = "$lock_deps"


─── docker/craft/web/build.py:259-269 ───
[security · high] render_html 的六条黑名单正则全部匹配原始文本，可被 HTML 字符实体编码绕过：片段 `<a
href="&#106;avascript:alert(1)">` 六条规则全部放行（`a` 不在 EMBED_TAG_RE 标签集内；`&` 开头的值不匹配
ABSOLUTE_REF_RE；ACTIVE_DATA_RE 只匹配字面 `javascript:`），而浏览器在解析 URL
前先解码字符实体，点击即在预览源上执行脚本。`javascript&colon;` 命名实体同理；CSS 侧 `\75 rl(` 转义可绕过
CSS_FETCH_RE。已确认服务层无缓解：internal/handler/artifact_preview.go 的 ArtifactPreviewCSP 与
internal/application/service/craft_preview.go 的 PreviewCSP() 均为 `script-src ... 'unsafe-inline'`，允许
javascript: URI 导航执行。staged 片段是不可信 agent 产出且此处原样拼入页面（不转义），这是存储型 XSS，直接违背注释中"拒绝任何形态的脚本执行"的设计目标。建议至少对
html_mod.unescape 后的形态（解码到不动点）同样执行全部检查；更稳妥的做法是改为白名单解析/净化。

+     # Browsers decode character references before URL parsing, so the
+     # denylist must also see the decoded form (to a fixed point):
+     # "&#106;avascript:" / "javascript&colon;" are "javascript:" to the
+     # browser but invisible to a literal regex. CSS backslash escapes
+     # (\75 rl) remain out of scope — a whitelist sanitizer is the robust fix.
+     variants = [fragment]
+     decoded = html_mod.unescape(fragment)
+     while decoded not in variants:
+         variants.append(decoded)
+         decoded = html_mod.unescape(decoded)
+     for candidate in variants:
-     for pattern, why in (
+         for pattern, why in (
-         (EXTERNAL_URL_RE, "external URL"),
+             (EXTERNAL_URL_RE, "external URL"),
-         (ABSOLUTE_REF_RE, "absolute or scheme reference"),
+             (ABSOLUTE_REF_RE, "absolute or scheme reference"),
-         (ACTIVE_DATA_RE, "active data/javascript URI"),
+             (ACTIVE_DATA_RE, "active data/javascript URI"),
-         (CSS_FETCH_RE, "css url()/@import fetch"),
+             (CSS_FETCH_RE, "css url()/@import fetch"),
-         (EMBED_TAG_RE, "embedding/script/navigation tag"),
+             (EMBED_TAG_RE, "embedding/script/navigation tag"),
-         (EVENT_ATTR_RE, "inline event handler attribute"),
+             (EVENT_ATTR_RE, "inline event handler attribute"),
-     ):
+         ):
-         if pattern.search(fragment):
+             if pattern.search(candidate):
-             raise BuildError(EXIT_CONTENT, "html section {!r} contains a {}: offline local assets only".format(heading, why))
+                 raise BuildError(EXIT_CONTENT, "html section {!r} contains a {}: offline local assets only".format(heading, why))
      return "<h2>{}</h2>{}".format(html_mod.escape(heading), fragment)


─── docker/craft/web/build.py:171-174 ───
[bug · medium] load_pin 对锁字段存在未防护的访问，畸形/敌意锁会以未分类异常崩溃：1) 某个依赖对象缺 "name" 时 `dep.get("name")` 得到
None，sorted() 对 None 与 str 混合排序抛 TypeError；2) 依赖对象缺 "sha256" 时 `dep["sha256"]` 抛 KeyError；3)
`lock["template"]["sha256"]`、`lock["build_program"]["sha256"]`、`lock["toolchain_digest"]` 在键缺失时同样抛
KeyError。这些异常既非 BuildError 也非 OSError，main() 与 __main__ 的兜底均捕获不到——结果是裸 traceback、退出码 1 且不写
build-log.json，违反模块契约"The build log is written for every termination, including failures"。docstring
明确将敌意/损坏的锁视为预期输入，此路径必须优雅拒绝。

-     listed = sorted(dep.get("name") for dep in dependencies_pin if isinstance(dep, dict))
-     if listed != sorted(FIXED_DEPENDENCIES) or len(listed) != len(dependencies_pin):
+     pinned = {}
+     for dep in dependencies_pin:
+         name = dep.get("name") if isinstance(dep, dict) else None
+         sha = dep.get("sha256") if isinstance(dep, dict) else None
+         if not isinstance(name, str) or not isinstance(sha, str):
+             raise BuildError(EXIT_TOOLCHAIN, "toolchain lock dependency entries must carry string name and sha256")
+         pinned[name] = sha
+     listed = sorted(pinned)
+     if listed != sorted(FIXED_DEPENDENCIES):
          raise BuildError(EXIT_TOOLCHAIN, "toolchain lock dependency set must be exactly {}: got {}".format(sorted(FIXED_DEPENDENCIES), listed))
-     pinned = {dep["name"]: dep["sha256"] for dep in dependencies_pin}


─── docker/craft/web/build.py:227-233 ───
[bug · medium] pin_from_lock 是失败路径专用的兜底函数，但自身可崩溃：json.load 返回数组/字符串（lock.get 抛 AttributeError）或
lock["template"] 为非对象（.get("template", {}).get(...) 抛 AttributeError）时，异常不在捕获的 (BuildError, OSError,
ValueError) 内，直接逃逸——兜底函数一崩，失败构建同样丢失 build-log.json 并打印裸 traceback，与"nothing is
fabricated"的健壮性意图相悖。建议补 isinstance 检查并将 AttributeError/TypeError 纳入捕获。

+         empty = {"toolchain_digest": "", "template_version": "", "template_sha256": ""}
+         if not isinstance(lock, dict) or not isinstance(lock.get("template"), dict):
+             return empty
          return {
-             "template_version": str(lock.get("template", {}).get("version", "")),
-             "template_sha256": str(lock.get("template", {}).get("sha256", "")),
+             "template_version": str(lock["template"].get("version", "")),
+             "template_sha256": str(lock["template"].get("sha256", "")),
              "toolchain_digest": str(lock.get("toolchain_digest", "")),
          }
-     except (BuildError, OSError, ValueError):
+     except (BuildError, OSError, ValueError, AttributeError, TypeError):
          return {"toolchain_digest": "", "template_version": "", "template_sha256": ""}


─── docker/craft/web/toolchain.lock.json:15-19 ───
[maintainability · low] 版本元数据与供给文件失真：deps/craft-web.js 文件头自述 "v1.1.0" 且注释描述了行为修复（"later tables used
to be dead controls"），而锁（及镜像它的 docker/craft/runtime-config.json web_toolchain 段）仍记 "version":
"1.0.0"。sha256 在镜像构建期由 Dockerfile 强制校验、夹具 build-log 亦为 exit 0，字节无失配，故不阻断构建；但版本字段失真会误导 build-log
与审计溯源。toolchain_digest 只覆盖 name:sha，改版本号无需重算摘要，也不会破坏 Go 侧 LoadCraftWebToolchainPin 的校验。

      {
        "name": "craft-web.js",
-       "version": "1.0.0",
+       "version": "1.1.0",
        "sha256": "3812625d71f5c10c26d7d9c21fa5d88d024969636c75060db65b6297b2ad055a"
      }


─── docker/craft/web/build.py:380-391 ───
[bug · medium] 发布顺序与失败清理违背自身的"原子发布"承诺：index.html 先于 assets 发布，任一 asset 的 mkdir/move 抛
OSError（如输出目录残留一个名为 assets 的普通文件时 mkdir 抛 FileExistsError，或磁盘满）时，输出目录已留下一个引用缺失
assets/craft-web.css、assets/craft-web.js 的已发布 index.html；且 main 的失败路径不清理 .craft-build-staging，残留的
dot 目录（含未搬走的 asset）会随输出一起被收集/预览——与注释 "a failed build never leaves a half entry" 不符。建议：先发布
assets、最后发布 index.html（entry 作为提交点），并把发布段包进 try/except，失败时回滚已发布文件并 shutil.rmtree(staged,
ignore_errors=True) 后再抛出。

-     # Atomic-ish publication: the entry and assets land in the output only
-     # after every byte rendered, so a failed build never leaves a half entry.
-     staged_entry = safe_path(staged, ENTRY_NAME)
-     final_entry = safe_path(output_root, ENTRY_NAME)
-     final_entry.parent.mkdir(parents=True, exist_ok=True)
-     shutil.move(str(staged_entry), str(final_entry))
+     # Atomic-ish publication: assets land first, the entry last — the entry
+     # is the commit point, so a failure mid-publish never leaves a published
+     # entry referencing missing assets; any failure rolls back and clears
+     # the staging tree.
+     published = []
+     try:
-     for dep_name in FIXED_DEPENDENCIES:
+         for dep_name in FIXED_DEPENDENCIES:
-         staged_dep = safe_path(staged, ASSET_OUTPUT_DIR, dep_name)
+             staged_dep = safe_path(staged, ASSET_OUTPUT_DIR, dep_name)
-         final_dep = safe_path(output_root, ASSET_OUTPUT_DIR, dep_name)
+             final_dep = safe_path(output_root, ASSET_OUTPUT_DIR, dep_name)
-         final_dep.parent.mkdir(parents=True, exist_ok=True)
+             final_dep.parent.mkdir(parents=True, exist_ok=True)
-         shutil.move(str(staged_dep), str(final_dep))
+             shutil.move(str(staged_dep), str(final_dep))
+             published.append(final_dep)
+         final_entry = safe_path(output_root, ENTRY_NAME)
+         final_entry.parent.mkdir(parents=True, exist_ok=True)
+         shutil.move(str(safe_path(staged, ENTRY_NAME)), str(final_entry))
+         published.append(final_entry)
+     except OSError:
+         for path in published:
+             try:
+                 path.unlink()
+             except OSError:
+                 pass
+         raise
+     finally:
-     shutil.rmtree(staged, ignore_errors=True)
+         shutil.rmtree(staged, ignore_errors=True)

