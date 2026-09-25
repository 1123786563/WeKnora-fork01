# T14 OCR 报告（第 2 次运行）

范围说明：ask 给定线性范围 eeb1ee2db^..24965f731 仍横跨 36+ 个非 T14 已审提交（同第 1 轮核实结论，lane 交错不可行）。T14 实际提交 3 个：eeb1ee2db、7b223e731（此两段第 1 轮已审，6 条 findings 已由 24965f731 修复并经本轮增量覆盖验证）、24965f731（修复提交，HEAD）。本轮审查第 1 轮之后的 T14 增量段 --from 7b223e731 --to 24965f731（含全部修复 diff，3 文件被审）。

## 本轮：--from 7b223e731 --to 24965f731

Review complete: 2 finding(s) across 3 selected item(s).

─── internal/container/container.go:329-333 ───
[bug · medium] 新注释声称绑定存储是 "every writer and reader (resolver, preview no-egress checker, craft lifecycle)" 的共享单例，但事实不符：`newCraftLifecycleService`（internal/container/craft_lifecycle.go:53）仍自行调用 `selectSessionBindingStore(rdb, false)` 构建独立实例，并未注入本单例。Lite 模式（无 Redis）下该 store 是进程内存 map，lifecycle 的读取/清理路径（回收 sweep、stale 标记、WithLifecycleLock 串行化）运行在另一张 map 上，对 resolver 写入的绑定完全不可见——这正是本次提交为 preview checker 修复的那类 bug，对 craft lifecycle 依然存在（绑定永不被回收/失效，锁不与 resolver 互斥）。建议：将 `newCraftLifecycleService` 的 `rdb *redis.Client` 参数改为注入 `sandbox.SessionSandboxBindingStore` 单例（或至少修正此处注释，避免误导后续维护者认为 lifecycle 已共享）。

─── internal/application/service/craft_preview.go:275-282 ───
[bug · low] IPv6 字面量 host 归一化不对称：`net.SplitHostPort("[::1]:443")` 返回去括号的 `::1`，直接返回 `h` 会丢掉裸 IPv6 host 在 URL/Host 头中必需的方括号。当预览 origin 配置为 IPv6 字面量（如 `https://[::1]`）且请求 Host 带显式默认端口（`[::1]:443`）时，比较 `::1` vs `[::1]` 不匹配 → 共享 /p 路由 404；反向（配置带端口、请求裸地址）同样失败。剥离端口后若 host 含 `:` 应重新加括号。

## 附：第 1 轮历史段（--commit eeb1ee2db 与 --from c799642d0 --to 7b223e731，6 条 findings 已修复于 24965f731）

第 1 轮共 6 条：craft_preview.go:111-115 [performance·medium] 每请求新建 docker client、:116 [bug·medium] ContainerInspect 无超时、:229-230 [bug·low] AcceptsPreviewHost 未归一化默认端口、:130-133 [maintainability·low] "<nil>" 错误尾、:394-402 [performance·low] allowlist 后置、container.go:2586-2590 [bug·medium] Lite 模式 binding store 不互通。全部由 24965f731 修复（per-host 共享 client、inspect 30s 超时、Lite store 单例化、端口归一、去 <nil>、allowlist 前移），原文见 git 历史版本的本文件。
