# Career Web OCR round-4 独立验证

- 基准 worktree：`/Users/wuyongjun/.codex/worktrees/issue-140-integration/WeKnora-fork01`
- 验证 revision：`76df0cee0bf3ae23c14411c345b151ad518077ee`
- 验证范围：`round4-highrisk-analysis.md` 中 Career Web 四个 high findings：ProgressPage 回执 mismatch、SearchPage requestId mismatch、ExportDeletionPage 跨 scope 清理、api-client open/list/changes 解码。
- 方法：只读检查代码及既有测试；未修改实现或测试。此报告为唯一新增验证产物。

## 判定

| Finding | 判定 | 证据 |
|---|---|---|
| `apps/web/src/career/ProgressPage.tsx:163-168` | VALID | `lookupReceipt` 在 try 中调用 `acceptReceipt(next, current)`；该函数在 requestId/applicationId 不匹配时抛 `ReceiptMismatchError`（约 103 行）。catch 只特判 `forbidden`，其余错误都置 `unknown` 并提示继续查询/稍后重试；没有 `ReceiptMismatchError` 分支。确定性协议错误因此进入回执恢复状态。相邻 `runWrite`（约 140 行）有 mismatch 特判。 |
| `apps/web/src/career/SearchPage.tsx:210` | VALID | 查得 `stored.requestId !== attempt.requestId` 时仍设置 `unknown`，保留相同 attempt 并提示重试查询；该状态继续渲染“查询回执”和“原请求编号重试”入口（约 300 行），再次得到 mismatch 会重复同一分支。相邻 `send`（约 162 行）对 mismatch 设置 `invalid_response`、清除 attempt 并回到 idle。 |
| `apps/web/src/career/ExportDeletionPage.tsx:71-75` | VALID | `clearPrivate` 清除导出/删除主体状态，但没有重置 `revision`、`revisionState`、`verifyMessage`、`lastExportRequest`、`deletedAnnounced`。scope abort listener 的 effect 依赖 generation（约 78-81 行），但 `readRevision` 的 callback 依赖只有 `[client, scopeController]`，其 effect 依赖 `[readRevision]`，scope generation 变化不会触发重读。组件由 `CareerPage` 不带 key 持续挂载（来源见 finding 报告）；因此旧请求/删除验证引用和 revision 状态可跨 scope 留存。 |
| `packages/api-client/src/career.ts:1255-1257` | VALID | `createCareerApi` 的 `open`、`list`、`changes` 都把 `request(...)` 直接 `as CareerView/CareerChangeSet` 返回。该文件已有严格 decoder；core decoder 可用，而公共 client 入口 `createCareerApi` 将这些方法暴露给调用方。故此三个 endpoint 的响应在此边界未运行 decoder。 |

## 验证命令与结果

在上述 worktree 执行：

```sh
git rev-parse HEAD
git status --short
sed -n '145,180p' apps/web/src/career/ProgressPage.tsx
sed -n '196,220p' apps/web/src/career/SearchPage.tsx
sed -n '45,105p' apps/web/src/career/ExportDeletionPage.tsx
sed -n '1238,1280p' packages/api-client/src/career.ts
rg -n "receipt|scope|mismatch|revision" apps/web/src/career/ProgressPage.test.tsx apps/web/src/career/SearchPage.test.tsx apps/web/src/career/ExportDeletionPage.test.tsx
node --import tsx --test apps/web/src/career/ProgressPage.test.tsx apps/web/src/career/SearchPage.test.tsx apps/web/src/career/ExportDeletionPage.test.tsx packages/api-client/src/career.test.ts
```

测试命令退出码为 0：83 tests passed，0 failed。既有测试证明正常查询恢复、正常搜索恢复、跨租户拒绝清理、删除后授权复核、生命周期 endpoint 行为均通过；测试没有构造错误 requestId 的回执，也未断言跨 scope 后 revision/导出 refs 的复位，api-client 当前测试亦未断言 `open/list/changes` 拒绝畸形 payload。故测试通过不反驳上述 finding。

## 接受标准状态、限制与风险

四项均为 **VALID / acceptance gap remains**，未发现 false positive。该验证不声称验证了真实浏览器下的焦点、窄屏布局或辅助技术行为；这些发现集中于服务状态流与解码边界，检查范围不涉及视觉变化。测试运行输出有一条 jsdom 的 `Not implemented: navigation to another Document` 提示，但进程仍以成功退出且 83 项全通过；未见其与本次四项相关。

主要风险为确定性错配响应可让用户卡在无法终止的 unknown 恢复循环；scope 切换后旧私有状态/请求标识可能影响新空间删除及授权验证；未解码的响应可能以畸形 revision/view/change set 进入页面状态和后续写入流程。
