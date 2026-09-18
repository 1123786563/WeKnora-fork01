# CFT-S04-T029 证据

日期：2026-09-18 · worktree `.worktrees/craft-cft` · commit 见本任务提交

## 交付物

- `internal/craft/slides_render_pin_test.go`（新增，3 tests，`TestCraftSlidesRenderPins*`）：演示稿生成链四断言命名 pin。
- 实现零改动：生成链为 D03 既有（python-pptx 撰稿 → LibreOffice 渲染 → pdftocairo 逐页图 → manifest 门 render/pages/sources）。

## 验收断言对照

- 真实打开页数正确 ✓（`RenderPinsRealPages`：真实 python-pptx 产物以 zip 打开、slideXMLFacts 计数 ≥3；readiness 要求 PageRefs 数 == SlideCount——缺一页即拒）
- 中文/图表/文本无溢出 ✓（既有 `TestSlidesReadyRejections`/OverflowPages 语义 + pin 的 overflow 拒绝分支：OverflowPages 非空即 not-ready——溢出页如实记录而非通过；LibreOffice 渲染 fixture（d03_libreoffice_render.pdf）+ 页图 fixture（d03_pdftocairo_page1.svg）构成真实渲染链证据）
- 失败页检查为 failed 而非 not_run 转 passed ✓（readiness 规则：渲染页缺失 → not ready（等同检查 failed），不存在"缺页但通过"路径；manifest 校验拒绝分支覆盖）
- 修改后历史稿不变 ✓（既有 `TestSlidesSinglePageModificationIsolated`：单页修改其余页逐字不变 + **旧 deck 字节完全一致**——pin 引用执行）
- 渲染链真实性 ✓（`RenderPinsRenderChainFixtures`：三个 D03 fixture 齐备——deck/渲染 PDF/页图）

## 命令与退出码

| 命令 | 退出码 | 结果 |
|---|---|---|
| `go test ./internal/craft -run TestCraftSlidesRenderPins -count=1` | 0 | 3 PASS |
| `go test -count=1 ./internal/craft/` | 0 | ok（0.8s 全包含 D03 套件） |

## 未验证事项

- 浏览器级（生成→修改→逐页查看→历史下载）在 T030 执行。

## 回退

revert 本提交（单测试文件，纯增量）。
