# Craft 04 文档、表格与演示稿 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 逐类提供可修改、可验证、可预览和下载的办公作品。

**Architecture:** 每类用版本化Skill与受控生成工具产生真实文件，沿W01版本发布和W02隔离预览。每种类型独立feature gate，不因为一个类型通过就开放全部Office格式。

**Tech Stack:** Go/Gin/GORM、现有 Sandbox/对象存储、tRPC-Agent-Go/OpenCode、React/assistant-ui、TypeScript/pnpm；具体依赖沿专项锁定版本。

**Spec:** [产品方案](../specs/2026-09-10-onyx-craft-product-proposal.md)；必读 [总计划及共享契约](2026-09-10-craft-product-implementation.md)。

## Global Constraints

- “首版同一工作区串行修改。”
- “主 Run、ToolCall 和恢复调度复用现有 tRPC 恢复方案。”
- “关闭或刷新页面只影响订阅，不自动重新提交任务。”
- “历史版本引用保持原始内容，不能静默重定向到最新文件。”
- “基础权限与取消安全是每个可运行版本的前提”。
- “本方案不设定套餐、价格或充值规则，商业权威保持在现有专项方案。”
- 本文所有步骤为待执行计划。继承总计划全部门禁、错误类型、身份/版本约束，不把测试替身结果当成上线证据。

---

## 文件结构与执行约定

文件所有权在各任务 Files 中列出；接口定义只在对应责任模块维护。按任务依赖执行，修改共享入口前核对其他任务变更。Go 示例的测试文件使用被测包及 `testing`，生产代码按代码块使用的标准库导入；外部 craft 类型从 `github.com/Tencent/WeKnora/internal/craft` 导入。TypeScript 测试使用 `node:test` 和 `node:assert/strict`；UI 浏览器测试另用 Playwright。最小代码展示核心规则，后续集成动作和验收表同属必做内容，不能只实现纯函数就勾选整个任务。

### Task D01: 报告文档与 DOCX 导出

**依赖与用户结果：** 依赖C01/C05/C06、W06；基于企业资料生成文档，改写后保留旧版本。

**Files:**
- Create: `internal/craft/document.go`
- Test: `internal/craft/document_test.go`
- Create: `skills/craft-document/SKILL.md`
- Create: `skills/craft-document/manifest.json`
- Create: `docker/craft/document-requirements.lock`
- Create: `packages/views/src/craft/document.tsx`
- Create: `apps/web/e2e/craft-document.spec.ts`

**Interfaces:**
- Consumes：Version/Check/File、C01 source manifest、W01不可变发布。
- Produces：`DocumentManifest { MarkdownRef,DOCXRef string; Headings []string; CitationIDs []string }`；`ValidateDocument(DocumentManifest) error`。Markdown是可继续修改的源，DOCX是同次生成导出，二者都入Version.Files，kind=document。

- [ ] **Step 1：先写失败测试。**

```go
func TestDocumentNeedsSourceAndExport(t *testing.T) {
    if ValidateDocument(DocumentManifest{MarkdownRef:"resource://md"})==nil {t.Fatal("missing export")}
    if err:=ValidateDocument(DocumentManifest{MarkdownRef:"resource://md",DOCXRef:"resource://docx",Headings:[]string{"概要"}});err!=nil {t.Fatal(err)}
}
```

- [ ] **Step 2：运行 RED。**

```bash
go test ./internal/craft -run TestDocumentNeeds -count=1
```

预期：新增接口不存在或目标行为断言失败。依赖/网络失败不算 RED；修复测试环境后再确认。

- [ ] **Step 3：实现核心规则。**

```go
type DocumentManifest struct { MarkdownRef,DOCXRef string; Headings,CitationIDs []string }
func ValidateDocument(m DocumentManifest) error {
    if !strings.HasPrefix(m.MarkdownRef,"resource://") || !strings.HasPrefix(m.DOCXRef,"resource://") || len(m.Headings)==0 {return ErrInvalidInput}
    return nil
}
```

- [ ] **Step 4：** 文档Skill明确生成output/report.md、report.docx、manifest.json；引用用C01 citation ID，禁止臆造来源。最小文档包含标题、摘要、章节、表格和来源；生成器使用镜像内Python/python-docx，执行任务时固定发行版本和hash到lock并构建镜像，不由Agent临时pip install。

- [ ] **Step 5：** 生成器测试打开真实DOCX的OOXML包并读取段落/表格，核对Markdown章节、数字、引用映射；检查缺失字体、图片、乱码、空文件、损坏ZIP。文档视图用现有安全Markdown组件禁原始HTML；DOCX下载以附件方式，禁止浏览器直接执行嵌入内容。PDF不在此任务承诺范围。

- [ ] **Step 6：** 浏览器使用两份知识资料生成客户介绍，确认3个章节及2个可点引用；第二轮新增FAQ并保持原数字，再打开旧版DOCX核对原始SHA未变。文件解析失败或内容不一致时Check failed且不展示导出已成功。

- [ ] **Step 7：** 更新kind gate document只在生成/修改/预览/导出四项通过后启用；新增工具镜像摘要、字体许可证、文档例子和验证命令进入Skill manifest。

- [ ] **Step 8：运行 GREEN 与验收。**

```bash
go test ./internal/craft -run TestDocumentNeeds -count=1
```

预期 PASS。还必须逐项确认：

- 执行Go规则测试、镜像内DOCX解析测试、`craft-document.spec.ts`真实浏览器测试。
- 引用、中文字体与长表格均有样本；只声称已验证DOCX和Markdown，不泛化任意文档格式。

- [ ] **Step 9：独立提交。** 先检查暂存 diff 仅包含本任务；实现过程中新增的必要文件须明确加入同一提交清单。

```bash
git add internal/craft/document.go internal/craft/document_test.go skills/craft-document/SKILL.md skills/craft-document/manifest.json docker/craft/document-requirements.lock packages/views/src/craft/document.tsx apps/web/e2e/craft-document.spec.ts
git diff --cached --check
git commit -m "feat(craft): d01 报告文档与 DOCX 导出"
```

### Task D02: 数据表格与 XLSX 导出

**依赖与用户结果：** 依赖W06/C05；上传CSV生成表格/图表，修改公式或汇总后可检查结果。

**Files:**
- Create: `internal/craft/spreadsheet.go`
- Test: `internal/craft/spreadsheet_test.go`
- Create: `skills/craft-spreadsheet/SKILL.md`
- Create: `skills/craft-spreadsheet/manifest.json`
- Create: `docker/craft/spreadsheet-requirements.lock`
- Create: `packages/views/src/craft/spreadsheet.tsx`
- Create: `apps/web/e2e/craft-spreadsheet.spec.ts`

**Interfaces:**
- Consumes：W01 Version、R03 CSV材料、C05历史恢复。
- Produces：`SheetSummary { Name string; Rows,Columns int; FormulaErrors []string; Recalculated bool }`；`SheetReady(SheetSummary) bool`。XLSX为可编辑成果；预览由服务端/受控转换器输出经过验证的JSON/静态HTML，不能浏览器解析任意宏。

- [ ] **Step 1：先写失败测试。**

```go
func TestSheetRequiresRecalculation(t *testing.T) {
    s:=SheetSummary{Name:"Sales",Rows:2,Columns:3}
    if SheetReady(s){t.Fatal("unevaluated formulas accepted")}
    s.Recalculated=true
    if !SheetReady(s){t.Fatal("valid sheet rejected")}
    s.FormulaErrors=[]string{"#REF!"}
    if SheetReady(s){t.Fatal("formula errors ignored")}
}
```

- [ ] **Step 2：运行 RED。**

```bash
go test ./internal/craft -run TestSheetRequires -count=1
```

预期：新增接口不存在或目标行为断言失败。依赖/网络失败不算 RED；修复测试环境后再确认。

- [ ] **Step 3：实现核心规则。**

```go
type SheetSummary struct { Name string; Rows,Columns int; FormulaErrors []string; Recalculated bool }
func SheetReady(s SheetSummary) bool {
    return s.Name!="" && s.Rows>0 && s.Columns>0 && s.Recalculated && len(s.FormulaErrors)==0
}
```

- [ ] **Step 4：** Skill生成output/report.xlsx、preview.json和manifest.json，记录sheet名/维度/计算总额/检查；openpyxl负责创建和解析，镜像内锁定LibreOffice headless负责实际重算；单靠写入公式不能标recalculated。公式仅来自允许模板或显式目标，不把输入CSV中以=,+,-,@开头的外部文本直接执行成公式。

- [ ] **Step 5：** 默认上限5个sheet、每sheet100000行/100列、源20MiB、预览最多1000行分页；不承诺在线Excel编辑器。禁止宏、外链、DDE和联网公式；转换在沙箱时间/内存限制内执行，超时写failed。预览单元格按纯文本转义，数值/日期/货币类型保留。

- [ ] **Step 6：** fixture含100/200两行销售，公式SUM=300；第二轮改季度汇总应仍为300，另加一行50应为350。真实解析XLSX公式与重算缓存值，验证日期、中文sheet、空值、千位分隔符和恶意公式输入。

- [ ] **Step 7：** 浏览器显示sheet切换/行数/总额/下载；第二轮修改后旧XLSX bytes仍一致。仅当重算、预览与文件解析通过才启用spreadsheet kind。

- [ ] **Step 8：运行 GREEN 与验收。**

```bash
go test ./internal/craft -run TestSheetRequires -count=1
```

预期 PASS。还必须逐项确认：

- 实际重算值350和预览350一致；#REF!/损坏ZIP/超限输入都有失败状态。
- 记录工具/字体版本与sandbox开销；不声称支持任意Excel宏和插件。

- [ ] **Step 9：独立提交。** 先检查暂存 diff 仅包含本任务；实现过程中新增的必要文件须明确加入同一提交清单。

```bash
git add internal/craft/spreadsheet.go internal/craft/spreadsheet_test.go skills/craft-spreadsheet/SKILL.md skills/craft-spreadsheet/manifest.json docker/craft/spreadsheet-requirements.lock packages/views/src/craft/spreadsheet.tsx apps/web/e2e/craft-spreadsheet.spec.ts
git diff --cached --check
git commit -m "feat(craft): d02 数据表格与 XLSX 导出"
```

### Task D03: 演示稿与分页预览

**依赖与用户结果：** 依赖W06/C01/C05；用户生成演示稿、修改指定页并下载PPTX。

**Files:**
- Create: `internal/craft/slides.go`
- Test: `internal/craft/slides_test.go`
- Create: `skills/craft-slides/SKILL.md`
- Create: `skills/craft-slides/manifest.json`
- Create: `docker/craft/slides-requirements.lock`
- Create: `packages/views/src/craft/slides.tsx`
- Create: `apps/web/e2e/craft-slides.spec.ts`

**Interfaces:**
- Consumes：C01 sources、W01 Files、W02隔离预览。
- Produces：`SlideManifest { PPTXRef string; PageRefs []string; SlideCount int; OverflowPages []int }`；`SlidesReady(SlideManifest) bool`。PPTX源文件、渲染PDF和逐页预览均为同一不可变Version。

- [ ] **Step 1：先写失败测试。**

```go
func TestSlidesNeedEveryRenderedPage(t *testing.T) {
    m:=SlideManifest{PPTXRef:"resource://ppt",SlideCount:2,PageRefs:[]string{"resource://p1"}}
    if SlidesReady(m){t.Fatal("missing page accepted")}
    m.PageRefs=append(m.PageRefs,"resource://p2")
    if !SlidesReady(m){t.Fatal("complete preview rejected")}
}
```

- [ ] **Step 2：运行 RED。**

```bash
go test ./internal/craft -run TestSlidesNeed -count=1
```

预期：新增接口不存在或目标行为断言失败。依赖/网络失败不算 RED；修复测试环境后再确认。

- [ ] **Step 3：实现核心规则。**

```go
type SlideManifest struct { PPTXRef string; PageRefs []string; SlideCount int; OverflowPages []int }
func SlidesReady(m SlideManifest) bool {
    if !strings.HasPrefix(m.PPTXRef,"resource://") || m.SlideCount<1 || len(m.PageRefs)!=m.SlideCount || len(m.OverflowPages)>0{return false}
    for _,p:=range m.PageRefs {if !strings.HasPrefix(p,"resource://"){return false}}
    return true
}
```

- [ ] **Step 4：** 镜像锁定python-pptx生成与LibreOffice/PDF渲染工具及中文字体。Skill先生成页大纲，再PPTX，渲染为PDF/页图，检查页数、文本边界、缺图和不可读字号；默认最多30页、单页来源可追溯，不把仅生成PPTX ZIP当作视觉验收。

- [ ] **Step 5：** 引用写讲者备注或末尾来源页，与原知识Source ID关联。slides视图提供缩略图、上一页/下一页、页码、当前版本下载；预览资源受版本授权，键盘方向键可导航，点击修改会把明确页码与当前version传给主Agent。

- [ ] **Step 6：** 浏览器生成5页客户方案，逐页截图检查中文/图表/长标题；要求只修改第3页结论，解析其余页文本/图对象确保未意外变更，重新渲染页数一致。第3页引用真实存在；旧版PPTX/PDF保持原SHA。

- [ ] **Step 7：** 将溢出页面写OverflowPages并反馈主Agent修复；工具无法检测的视觉项标not_run并进入人工验收证据，不伪称全部自动通过。通过此类型独立门禁后启用slides。

- [ ] **Step 8：运行 GREEN 与验收。**

```bash
go test ./internal/craft -run TestSlidesNeed -count=1
```

预期 PASS。还必须逐项确认：

- PPTX实际可打开，5页均有真实渲染预览；损坏字体/缺页/溢出有可见失败。
- 三个类型独立开放；D03失败不回滚已通过的D01/D02，但不得标D全阶段完成。

- [ ] **Step 9：独立提交。** 先检查暂存 diff 仅包含本任务；实现过程中新增的必要文件须明确加入同一提交清单。

```bash
git add internal/craft/slides.go internal/craft/slides_test.go skills/craft-slides/SKILL.md skills/craft-slides/manifest.json docker/craft/slides-requirements.lock packages/views/src/craft/slides.tsx apps/web/e2e/craft-slides.spec.ts
git diff --cached --check
git commit -m "feat(craft): d03 演示稿与分页预览"
```
