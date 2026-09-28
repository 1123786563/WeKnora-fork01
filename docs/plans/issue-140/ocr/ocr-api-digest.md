Review failed (cancelled): 1 finding(s); 0 of 1 selected item(s) failed.

─── packages/api-client/src/career.ts:938-939 ───
[bug · medium] 非草稿分支的触发条件与后端 Go
契约的实际序列化形态存在边界冲突：`PreparationSnapshotRef`（internal/modules/career/preparation.go:87-91）三个字段均无
`omitempty`，零值会序列化为 `""` 而非省略键；且 `preparationFailureReceipt` 重建 failed/in-flight 回执时从不设置
`OpportunityID`（恒为 `""`），持久列 `snapshot_id`/`snapshot_sha256` 为 `not null; default:''`。于是对
`{"opportunityId":"","snapshotId":"","snapshotSha256":""}` 这种快照未持久化的行（如迁移回填的旧行），新守卫因 `"" !==
undefined` 触发且 `sha256Hex.test('')` 为 false 而整体拒绝，旧代码可正常解码为空 snapshot——这会让 receipt/list
接口对该行整体不可读。建议把空白字符串视同缺失来判定"是否携带来源"（同时保留对畸形摘要的拒绝），并与后端确认空串行是否可能出现；另外测试套件已覆盖
`{snapshotSha256:'garbage'}` 的拒绝路径，但缺少针对该 Go 零值序列化形态的用例，建议一并补上以钉住语义。

-  if (!draft && (snapshotRecord.opportunityId !== undefined || snapshotRecord.snapshotId !== undefined || snapshotRecord.snapshotSha256 !== undefined)
+  const carriedSource = (v: unknown) => v !== undefined && v !== null && v !== ''
+  if (!draft && (carriedSource(snapshotRecord.opportunityId) || carriedSource(snapshotRecord.snapshotId) || carriedSource(snapshotRecord.snapshotSha256))
    && (typeof snapshotRecord.snapshotSha256 !== 'string' || !sha256Hex.test(snapshotRecord.snapshotSha256))) throw new TypeError('invalid preparation sources')


LLM retry report summary: 1 of 12 requests affected -- 1 request cancelled

Core review (1 request):
- packages/api-client/src/career.ts: cancelled

Per-attempt detail: --format json (retry_report).
