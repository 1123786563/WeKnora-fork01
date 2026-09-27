# T07 OCR 报告（第 2 次运行）

范围说明：ask 给定线性范围 1c1468641^..cfad7ba71 为 T07 的 5 个连续提交。前 3 个第 1 轮已审（2 条 findings：已存在分支无条件补插证据行 [medium]、VersionEvidenceDigest 未用+注释矛盾 [low]），已由 9d939cbb0 全实修复（仅 adopt 不补插+ErrConflict、EncodeVersionEvidence 单源+注释修正），cfad7ba71 为豁免复核（无代码变更仅入册）。本轮审查第 1 轮之后的增量段 --from 0efa67697 --to cfad7ba71（含全部修复 diff，6 文件中 2 个非测试源码被审）。

## 本轮：--from 0efa67697 --to cfad7ba71

Review complete: 2 finding(s) across 2 selected item(s).

─── internal/application/repository/craft_version.go:263-273 ───
[style · low] 本次重构把 publish 闭包内这一段整体少缩进了一层：evidence 插入块及其闭合、`out = in; return nil` 位于 `if created.RowsAffected == 1` 分支内却与该分支语句同层，其后的采纳路径与闭包收尾也各浅一层。括号计数仍匹配、语义未变（evidence 插入仍在创建分支内），但文件已不符合 gofmt，`gofmt -l` / 格式化 CI 会失败。建议对整个文件跑一次 gofmt 后再提交。【经实跑证实：gofmt -l internal/application/repository/craft_version.go 列出该文件】

─── internal/application/repository/craft_version.go:210-216 ───
[maintainability · low] 此处 `raw, digest, err := craft.EncodeVersionEvidence(*evidence)` 在块内遮蔽了外层由 prepareCraftVersion 返回的 manifest digest（旧代码用 sum 命名无遮蔽），而外层 digest 随后仍用于 row 的 ManifestHash。当前语义正确，但同一函数内两种语义不同的摘要共用一个名字，且叠加缩进错位，后续维护极易取错变量。建议改名为 evidenceDigest 消除遮蔽。

第 1 轮 2 条 findings 的修复经重审确认无回归，新暴露 2 条修复引入的 low 级问题（gofmt 缩进、变量遮蔽命名），均已实跑/抽查证实。
