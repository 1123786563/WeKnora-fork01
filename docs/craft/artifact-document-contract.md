# Craft 文档类型契约（DOCX）

任务：CFT-S04-T025 · 基线：`.worktrees/craft-cft`（本文件为任务卡建议的契约文档落点）

## 交付物与路径

一轮文档交付同时包含**两个**文件（缺一不是文档轮）：

- `report.md` — 可编辑源（后续轮修改的对象）
- `report.docx` — **同轮**由该源导出的 OOXML（python-docx，沙箱内 craft-document skill）

## 生成链

1. 主 Agent 委派 → 沙箱内 skill 撰写 Markdown（标题/摘要/章节/表格/来源）
2. python-docx 从**同一源**渲染 DOCX（同轮导出，禁止跨轮复用）
3. skill 做真实 **OOXML 读回**校验：打开包、对照 Markdown 的章节/数字/引用
4. 服务端 admission（craftValidateStagedManifest）：manifest.json 四门
   generate/modify/preview/export 全过 + 双文件在册才发布；任一失败零发布（T019 pin）
5. entry check 判**真 DOCX**（report.docx 不在 → failed，不以扩展名通过）

## 引用策略

- 引用只能是 C01 知识引用 `kc_` 前缀；虚构来源不过门
- 引用须出现在正文与来源节（读回计数 ≥2）

## manifest.json 契约（核心字段）

`markdown_ref` / `docx_ref` / `headings` / `citation_ids` + 四门 checks +
`markdown` / `docx` 路径（校验 docx 必须是 `report.docx`）。

## 不变性

v2 发布后 v1 的 md/docx 的 SHA、来源关系不变（内容寻址 + 不可变发布链；
`TestCraftDocumentRenderBothDeliverablesImmutable` 显式钉死双交付物组合）。

## 不承诺

PDF 不在承诺内；在线编辑器不在范围（只读预览 + 下载）。

## 证据

- 真实 python-docx 产物 fixture：`internal/craft/testdata/d01_python_docx_report.docx` + `d01_report.md`
  （中文标题/7 行中文表格/双 kc_ 引用；OOXML 读回对照测试 `TestPythonDocxFixtureMatchesMarkdown` + 本轮命名化 pin `TestCraftDocumentRenderPinsChineseTableCitations`）
- 导出失败不发布：`TestDocumentExportFailures` + `TestDocumentEntryCheckJudgesTheDOCX` + T019 admission pin
- 浏览器级（生成→修改→查看→历史下载）：T026 执行
