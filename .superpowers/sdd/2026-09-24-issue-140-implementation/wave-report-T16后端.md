# Wave 6 报告：T16 后端——同版 PDF/DOCX 生成与验证（Issue #158，implement）

- BASE `4b09af298` → HEAD `bd13bef1e`（`feat(career): render and verify same-version pdf docx`，单本地提交，未 push）
- Worktree：`/Users/wuyongjun/.codex/worktrees/issue-140-t16-rendering/WeKnora-fork01`（detached）
- 详细报告：worktree 内 `.superpowers/sdd/2026-09-24-issue-140-t16-rendering/task-1-report.md`

## 摘要

对 T15 不可变 material version 同一结构化正文渲染 **PDF+DOCX 双格式**（纯 Go stdlib 自写渲染器，零新依赖，go.mod 未动），落 `career_material_exports`（staged → submittable / failed / revoked 状态机），两文件记录同一 content digest 与版本绑定；渲染后立即用**独立解析路径**验证（PDF：对象/页树/ToUnicode CMap/文本坐标+内容；DOCX：zip 完整性+CRC+OOXML 结构要件+内容），**双验通过才 submittable**，单格式失败保持 staged+错误、绝不全量发布成功一半；下载走 T04 同构 HMAC 短时效授权（`wk-career-export-v1`，TTL≤15min，绑定 owner/资源，兑付时重查撤销态→立即失效；旧版本导出永远可再取）；requestID replay/conflict + expectedRevision CAS + tenant/owner 隔离全 house 语义。

## RED → GREEN

- **RED**（先写 10 个命名测试）：career 包 `build failed`（ExportReceipt/SetExportStorage/PublishMaterialHandler 等未定义）；database `000202_career_material_exports.up.sql 不存在`；router `5 条导出路由未注册`。
- **GREEN**：`go test ./internal/modules/career/ -run 'TestPublishMaterial|TestCareerMaterialExport' -count=1` → ok（10 命名测试全绿）。

## 验证命令（全部实跑，均通过）

| 命令 | 结果 |
| --- | --- |
| `gofmt -w internal/modules/career internal/router` | 完成，`gofmt -l` 无残留 |
| `go test ./internal/modules/career/... -count=1` | ok 12.839s（终态复跑 13.908s） |
| `go test ./internal/database/... -count=1` | ok 33.387s |
| `go test ./internal/router/... -count=1` | ok 4.279s |
| `go test ./tools/architectureguard/... -count=1` | ok 1.074s（实测 606/675，常量按精确值更新 601→606/670→675） |
| `git diff --check` | 通过（无输出） |

## 独立工具复核（测试外，真实渲染样本）

- **PDF**：python3 独立解析器（对象扫描/页树/CMap/Tm 坐标/文本提取，与 Go 代码无共享路径）→ `pages: 1`，提取文本含全部 CJK+ASCII 正文与两条 claim，`PDF-INDEPENDENT-CHECK: OK`。
- **DOCX**：`unzip -t` 全 entry CRC OK（No errors detected）；5 个 XML part `xmllint --noout` 全良构；`xmllint --xpath` 确认 `w:sectPr`=1、全部 `w:t` 正文文本、Heading1/ListParagraph 样式与 document/styles 的 Content-Types Override。

## 提交与文件

- Commit：`bd13bef1e feat(career): render and verify same-version pdf docx`
- create：`internal/modules/career/rendering.go`、`rendering_test.go`、`migrations/versioned/000202_career_material_exports.{up,down}.sql`、`migrations/sqlite/000123_career_material_exports.{up,down}.sql`（migrations/ 被 .gitignore，按 T15 先例 `git add -f`）
- modify：`office.go`、`handler.go`、`handler_test.go`、`routes_career.go`、`routes_career_test.go`、`career_migration_test.go`（+命名迁移测试；既有 latest 断言 122→123×11）、`architectureguard/discovery_test.go`
- 路由合同（冻结，供 Web 下载 E2E 子任务）：`POST .../materials/:id/exports`、`GET .../materials/:id/exports`、`POST .../exports/:exportId/signed-url`、`GET .../exports/:exportId/download?format&expires&signature`、`DELETE .../exports/:exportId`

## 自查与遗留

- 下载兑付为认证 owner+签名授权双检（Career 路由组在全局 Auth 之后；挂根路由超出本任务文件所有权，如需 credential-free 请主控裁决）。
- PDF 字体未嵌入（ToUnicode 保证文本可提取；显示用替代字体）；环境无 pdftotext，独立复核用 python3 自写解析器。
- 渲染先落存储后单事务写终态；崩溃可能留无行的孤儿对象，可重试覆盖，不影响一致性。
- 未运行 golangci-lint（简报清单未要求）。
