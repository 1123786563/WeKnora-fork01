# Wave 5 — T24 修复轮 1/5 报告（F1/F2/F3，Issue #164 fix-resume）

- 实现者：frontend_implementer（fix-resume）；独立 worktree `/Users/wuyongjun/.codex/worktrees/issue-140-t24-miniprogram/WeKnora-fork01`
- BASE `9ef4ee4c0`（调度员实核 clean）→ HEAD **`4fb558122`**；本地 3 commits，未 push
- 复审 diff 范围：`9ef4ee4c0..4fb558122`
- 简报：`wave-5-t24-fix-brief.md`；现场沿用既有 worktree（无既有未提交 checkpoint，直接在基线上续接）

## 1. 提交清单

| commit | 项 | 内容 |
| --- | --- | --- |
| `10fae9c11` | F1 | fix(miniprogram): make the quota-refusal prompt reachable through the real error chain |
| `4272af5ff` | F2 | fix(miniprogram): give the unknown-outcome 404 reconciliation an operable recovery state |
| `4fb558122` | F3 | fix(miniprogram): re-take evidence shots as distinguishable states |

## 2. 文件清单（全部在本任务所有权内：miniprogram tests/src）

- `apps/miniprogram/src/core/errors.ts`（F1：search_quota_refused 专属文案映射）
- `apps/miniprogram/src/services/career.ts`（F2：isReceiptMissing 谓词）
- `apps/miniprogram/src/career/discovery.tsx`（F1+F2：quotaRefused 判定改真实文案；404 恢复态 Notice+安全重发 Action；对账/重发错误不再静默）
- `apps/miniprogram/tests/career-discovery.test.mjs`（+D4b/D6/D7 三个用例，87→90）
- `apps/miniprogram/tests/live/t24r1-live-driver.cjs`（新，F3 取证驱动 + F1/F2 真实复验自动化；不在 `tests/*.test.mjs` glob 内，不影响 pnpm test）
- `apps/miniprogram/tests/live/quota-inject-proxy.mjs`（新，F1 合同级注入代理，自动化辅助）

## 3. F1 额度不足专属提示死代码 → 真实可达（简报选项 a）

- 根因：`useAction` 把错误经 `errorMessage()` 成串后才进页面，typed 429 坍缩为通用文案「请求较多，请稍后重试。」，`includes('search_quota_refused')` 永假 → 专属提示死代码。
- 修复：`errorMessage` 增加 `search_quota_refused` →「搜索额度不足：本次搜索未执行，仍可查看既有档案与申请记录。」；页面判定改为识别该真实渲染文案（Notice 降 warning，删除原不可达后缀拼接）。保留而非删除的依据：全局约束「额度耗尽仍可读取既有档案和申请」要求专属可操作状态，且 T11 冻结合同已定义 typed 拒绝。
- RED（真实输出）：D4b `AssertionError actual:false`（shown='请求较多，请稍后重试。'）；D7 同因。GREEN：D4b/D7 通过。
- 真实触发（DevTools，构造法如实声明）：生产 `SearchQuotaGate` 为 passThrough（`office.go:299`，真实账本属 T21），真实服务器无法产出 typed 429 → `quota-inject-proxy` 在小程序信任 origin 的冻结合同边界回放 429 `{"error":{"code":"search_quota_refused"}}`（proxy.log `REFUSED POST /api/v1/career/searches` ×1，上游无此 POST）；客户端全链路真实（构建/transport/ApiError/errorMessage/Notice），经原生「安全重发原搜索」按钮触发。页面真实渲染「搜索额度不足：本次搜索未执行，仍可查看既有档案与申请记录。」（warning），且 intent 保留、既有事实仍可读（记录 PASS）。

## 4. F2 未知对账 404 无恢复 → 可操作恢复态

- 根因：404 落进共享 `recoverBusy` 且 error 从未渲染（静默失败）；`retryPendingSearch`（同 requestId 幂等重发）从未接 UI。
- 修复：`isReceiptMissing(error)`（404/not_found）+ 页面恢复态：「尚未找到该请求的回执：搜索请求可能未送达服务器。可安全重发（同一请求 ID，服务端不会重复执行），或稍后再用原请求对账。」+「安全重发原搜索」（retryPendingSearch）；「用原请求对账」保留为重试对账；对账/重发错误 Notice 渲染不再静默。
- RED：D6 `TypeError: career.isReceiptMissing is not a function`。GREEN：D6 通过（404 可区分、intent 不被静默丢弃、安全重发复用原 requestId、成功后清除 intent）。
- 真实复验（构造法如实声明）：intent 写入不存在的 requestId（`t24r1-missing-mugsli19`）→「用原请求对账」→ 服务器真实 404 `{"code":"not_found"}`（server.log WARNING 2026-09-25 18:02:55）→ 恢复态渲染（截图 + 视觉复核）→「安全重发原搜索」→ 同 requestId 真实 POST 200 → 结果渲染、intent 清除（全 PASS）。

## 5. F3 三张字节级相同截图 → 可区分重取证

- 根因实测（探针 probe-frame/probe-f2）：截图忠实反映视口像素，字节级相同=可视像素相同——Wave 4 三同图（md5 `199cf63c…`）根因是状态增量渲染在折叠线以下、取证未把关键元素滚入视口；另 `page.$$('text')` 单拍快照偶发不完整会误判。
- 修复（只动 tests/ 与自动化辅助，业务逻辑零改动）：`verifiedShot` 三重纪律——截图前多拍文本断言（snapshotMatching）+ 关键元素滚入视口 + `.wk-button` 扫描断言可操作按钮；截图后逐张 sha256 比对，撞图换视口偏移重拍（仍撞即失败）；收尾两两哈希不等断言 + manifest.json。
- 结果：**14/14 PASS；6 张截图 6 个不同 sha256**（F2-recovered 首拍与 C-search-reconciled 撞图被守卫捕获，换偏移重拍成功——守卫真实生效的现场证据）。

| 截图 | sha256(16) | 证明 |
| --- | --- | --- |
| B-confirm-visible | 1a04fd000e0eccea | 逐项确认生效：意向=远程办公 已确认 |
| C-unknown-search | b082cd8374ee64f0 | 未知结果挂起态 + 用原请求对账入口 |
| C-search-reconciled | 7dc3ff907235772b | 对账完成态：no_vetted_sources + 覆盖来源（0） |
| F2-receipt-missing | 9afe52d71d604575 | 404 恢复态：尚未找到回执 + 安全重发原搜索 |
| F2-recovered | 6982744d0b5a54c5 | 安全重发闭环：结果渲染、intent 清除 |
| F1-quota-refused | 121d054456fd0c6a | 专属提示可达：搜索额度不足…仍可查看既有档案与申请记录 |

F1/F2 两张关键截图另经模型视觉复核，确认提示文案与按钮真实在画面内。证据目录：`/tmp/wk-t24r1-57811-Zh5P/`（shots/ + manifest.json + server.log/proxy.log/driver.log）。

## 6. 验证命令与真实输出（全跑）

```
pnpm --filter @weknora/miniprogram test       → ℹ tests 90 / pass 90 / fail 0 / skipped 0
pnpm --filter @weknora/miniprogram typecheck  → 13 errors（全部 account/pages.tsx CommercialSummary 基线，零新增）
pnpm --filter @weknora/miniprogram build:weapp → 成功（dist/career/* 产出，新恢复态/专属提示文案经 unicode 转义在 bundle 内核验）
git diff --check                               → 干净（DIFF_CHECK_CLEAN）
```

真实 DevTools（端口 57811，Lite@57812 + 注入代理@57811 + cli auto 9423 + 一次性用户 t24r1c）：14/14 PASS（明细见上）。

## 7. 自查与遗留

- 自查：F1/F2 均先 RED 后 GREEN（证据见第 3/4 节）；F3 未改任何业务逻辑；三个分项 commit；只动简报所有权文件；未 push；无子 agent。
- 遗留（如实）：真机验证 blocked（无设备，Wave 4 同）；t-button GUI 自动化触达限制不变（F1 经原生按钮路径触发）；F1 服务器侧真实配额拒绝待 T21 账本落地后端到端复验（本轮证据边界=冻结合同注入 + 客户端全链路真实，已在代理脚本头部与本文声明）。
- 清理：DevTools/代理/Lite 服务器已停（57811/57812 空闲）；一次性 token/密钥已删；日志 grep 凭证 = 0；一次性账号仅存临时 SQLite（bcrypt），评审后可整目录删除。
