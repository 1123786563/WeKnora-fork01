# Pass A Batch A2 Integration Evidence（IA2）

集成线：`backend-mod-passa`，基线 `03032f890`（batch a1 终版）。

## 1. 集成顺序与提交

| 模块 | worker 分支终版 | merge 提交 | 备注 |
|---|---|---|---|
| A5 execution | 9a05764a0 | 6201b1004 | 3 包/142 文件 |
| A6 airesource | 30382d5f4 | dfe493309 | 12 包/188 文件；container.go 单行 LocalImageResolver 翻转为 A6 分支内已裁定豁免 |
| A7 agentcatalog | 2766e215d | a53492981 | 零搬迁（manifest 空集为真空）；集成工作量零 |
| A8 system+policy | eb78d141a | dfd343dc5 | system 零搬迁（F0 背书）；policy 5 包/25 文件 |

屏障提交：
- `0bb9b354f` composition 切换：container.go 9 条 import 翻转（browserskill/execution/mcp/embedding/limiter/ollama/sandbox/storageallowlist/web_search——最后一条为首次遗漏后补翻）、channels/im/service.go 2 条翻转、删除 batch-a2 全部 12 个别名目录（含 `internal/infrastructure/web_search` 孤儿别名）
- `4c8dbb204` guard 例外：batch-a2 新暴露 6 条预存耦合（精确路径，B-airesource×1 / B-channels×3 / B-execution×1，加 IA1 的 4 条共 10 条在册）
- `67af0b743` 冻结能力入口：`docs/architecture/frozen-entrypoints-batch-a2.md`（execution/airesource/agentcatalog/policy 共 253 符号/19 包，供 A9–A14 消费；冻结规则 + 10 条在册耦合）
- lint 修复（gofmt/gofumpt 若干、web_search alias 删除）随 0bb9b354f 一并入账

## 2. 合并冲突（A 系 import 修复相交，均按"两侧新路径并集"解决）

- A6×A5（7 文件）：agent_capabilities.go、agent_service_test.go、craft_delegate_test.go、session.go、system_setting.go、tenant_skill_install.go、tenant_skill_install_test.go
- A8×A6（9 文件）：extract.go、knowledge_auto_tag.go、knowledge_auto_tag_test.go、knowledge_faq_import.go、knowledge_process.go、embed_channel.go、knowledgebase.go、session/artifact_download.go、session/qa.go

全部为 import 块并集，零逻辑取舍。

## 3. 门禁结果

- `go build ./...` exit 0
- `go run ./tools/architectureguard`：OK（0 violations；literal=564 + apiKeyRoute=69 = 633 routes；redis 23 / lite 23；hooks 58；modules 16 —— 与 F0 基线零漂移）
- `go run ./tools/modulemove verify --all`：OK（16 manifests）
- `golangci-lint run --new-from-rev=03032f890 ./...`：0 issues（含 gofumpt 修复 2 个测试文件、删除 web_search 孤儿别名包后 revive 全净）
- `go test ./internal/... -count=1 -timeout=25m`：见 §5（后台运行，日志 /tmp/ia2-full-test-2.log）

## 4. 遗留与 Pass B 指向

- guard 在册耦合 10 条：B-appconnector×4、B-airesource×1、B-channels×3、B-execution×1、（channels→policy ratelimit/ipclass 已含于 B-channels×3）
- `chat.LocalImageResolver` 经旧路径读取会得到死拷贝（别名已随本轮删除，风险随之消失）
- system 的 housekeeping hook 实现文件归 knowledge（B-knowledge 与 B-system 协调点，A8 已记）

## 5. 批次结论

- `go test ./internal/... -count=1 -timeout=25m`（提交态 67af0b743，2026-09-21）：**119 ok / 0 FAIL / EXIT=0** —— 完全干净，优于 F0 基线（F0 run1=118 ok + 2 个后续注册的 unstable 本轮均未触发）。

A5–A8 全部按 Integration Brief 接入完成；Execution、AI Resource、Agent Catalog、Policy 能力入口已冻结（67af0b743）；全部门禁不劣于基线，批次放行 Parallel Group A3（A9–A11）。

