# CFT-S05-T035 证据

日期：2026-09-18 · worktree `.worktrees/craft-cft` · commit 见本任务提交

## 交付物（本轮执行的完整安全总回归）

1. **Mimosa 完整深度扫描**（此前 hook 一直 scanner_enobufs，本轮通过 MCP 正式作业跑完）：
   - jobId `scan-job-mu6v34qk-6fc0c8dcc1e9d62c`，深度 deep，66s 完成，seal `sha256:cfd1ded2…c52144`
   - **219 项发现（179 high / 40 medium）+ 依赖离线通告（1144 包扫描、7 包命中 13 条 advisory）**
   - 证据边界 `static_only_no_runtime_execution`（静态发现，非运行时验证；verdictEffect=none）
2. 六个承载安全语义的 Go 包全量回归（-count=1，非零收集）。
3. 安全 e2e（craft-report spec：恶意 fixture/跨租户/viewer/隔离/幂等）复跑。

## 发现分析（关键结论）

- **本轮 CFT 触碰的全部文件 0 发现**（executor/client/preview/knowledge/session/version store/craft 前端三包/routes/e2e harness——程序化比对 findings 与本轮文件清单）。
- 219 项发现全部落在**本轮未触碰的存量区域**：Vue 前端（frontend/src 42 硬编码凭据+污点）、apps 存量（38 硬编码凭据，主要是 mobile/desktop 测试 fixture）、docreader 路径穿越、internal 存量弱加密/SSRF 入口、**docs/design 原型包的 4 项（含 2 处 app.js 的确认式 XSS 提示）**——原型是静态设计资源（README 已声明"不部署为生产站点"），不进入生产构建。
- 结论：**无一项发现要求回退本轮交付**；存量项属仓库整体安全债（跨轮次治理范畴），按任务纪律不顺带修复无关领域——已在 RESUME/最终报告如实列出移交。

## 验收断言对照（本轮实跑的安全/故障注入矩阵）

- 跨租户文件/票据/快照全部拒绝 ✓（e2e 04 双模式 + Go：preview CrossTenantNotFound/版本 scope 守卫/快照 NotFound）
- 来源 prompt injection 不扩大工具范围 ✓（T017 SourceGuard 注入文档纯数据结构白名单 + e2e 03 恶意 fixture CSP/沙箱阻断）
- 旧 fence 结果不能发布 ✓（T016 GuardStaleFenceCannotPublish：plan"lease lost"/prepare 冲突/SaveResult 拒绝/零版本）
- 取消批准竞态与读超时不重复执行 ✓（T023 StopStatus 矩阵+410+SIGKILL 双窗；T015 盲重发修复+POST 恰 1 次）
- 任何 unknown 未被解释为成功 ✓（T015/T021 unknown 不落库不终结；T032 不发 finished 事件；T018 unknown 计数非 0）

## 命令与退出码

| 命令 | 退出码 | 结果 |
|---|---|---|
| Mimosa `security_scan_start(depth=deep, focusFiles=本轮 10 个热点文件)` | completed | 219 findings + 依赖通告；seal 入证据 |
| `go test -count=1 ./internal/agent/opencode/ ./internal/application/repository/ ./internal/application/service/ ./internal/craft/ ./internal/handler/session/ ./internal/container/` | 0 | ok ×6（25.5s/54.5s/70.0s/3.6s/19.5s/6.9s） |
| `bash e2e/craft-stack.sh up mock && run mock e2e/craft-report.spec.ts` | 0 | **6 passed (22.1s)** |

## 未验证事项

- Mimosa 是静态扫描（其自身声明 evidence_boundary=static_only）：运行时安全由本轮 e2e/Go 注入矩阵承载（上表）。
- 存量 219 项的安全债治理不属于本任务（顺带修复违反无关 diff 纪律）——移交清单见最终报告。

## 回退

无代码变更（本任务为总回归执行 + 证据归档）。
