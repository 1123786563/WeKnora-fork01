# CFT-S04-T025 证据

日期：2026-09-18 · worktree `.worktrees/craft-cft` · commit 见本任务提交

## 交付物

- `internal/craft/document_render_pin_test.go`（新增，2 tests）：文档生成链三断言命名化 pin。
- `docs/craft/artifact-document-contract.md`（新增）：任务卡建议的文档类型契约（交付物/生成链/引用策略/manifest/不变性/不承诺）。
- 实现零改动：生成链为 D01 既有（契约规则 + manifest 门 + python-docx skill + OOXML 读回）。

## 验收断言对照

- 中文/表格/引用在实际 OOXML 中可读 ✓（新增 `RenderPinsChineseTableCitations`：真实 python-docx 产物（testdata fixture）打开为 OOXML——中文标题/7 行中文客户表/双 kc_ 引用正文+来源计数 ≥2；既有 `TestPythonDocxFixtureMatchesMarkdown` 全要素对照 Markdown）
- 转换失败不发布 ✓（既有 `TestDocumentExportFailures` + `TestDocumentEntryCheckJudgesTheDOCX`（md-only → entry failed）+ T019 admission pin（缺 manifest 零上传零发布））
- 修改后旧 DOCX hash 不变 ✓（新增 `RenderBothDeliverablesImmutable`：**md+docx 双交付物**内容寻址身份分离 + v1 manifest 在 v2 发布后仍合法；浏览器级下载 SHA 由 T026 执行）

## 命令与退出码

| 命令 | 退出码 | 结果 |
|---|---|---|
| `go test ./internal/craft -run TestCraftDocumentRender -count=1` | 0 | 2 PASS |
| `go test -count=1 ./internal/craft/` | 0 | ok（0.5s 全包含 D01 套件） |

## 未验证事项

- 浏览器级文档闭环（生成→修改→查看→历史下载）在 T026；Gate 仍只开 web（document 类型浏览器验收过了才放开）。

## 回退

revert 本提交（pin 测试 + 契约文档，纯增量）。
