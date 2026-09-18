# CFT-S03-T019 证据

日期：2026-09-18 · worktree `.worktrees/craft-cft` · commit 见本任务提交

## 交付物

- `internal/application/service/craft_artifact_publish_test.go`（新增，2 tests，`TestCraftArtifactPublish*`）：不可变发布链缺失两环的 pin。
- `craft_artifacts_test.go`：memVersionStore 增加 `failPublish` 事务失败注入。
- 实现零改动：收集/发布链为 W01/D01-D03 既有（hostile 拒绝、manifest admission gate、内容寻址上传、幂等 identity 发布）。

## 验收断言对照

- 路径逃逸/软链接/secret 文件拒绝 ✓（既有 `TestCraftArtifactCollectRejectsHostileOutput`：traversal/.env/symlink/oversize 四类全部 ErrInvalidInput + 零发布——本轮枚举引用，不重复实现）
- 检查失败不发新版本 ✓（新增 `AdmissionFailurePublishesNothing`：document 类型轮次缺 manifest.json → admission 拒绝、**零发布且零上传**（gate 在上传前引爆）；各类型 manifest 验证器另有 document/spreadsheet/slides_test 的 11/15/11 用例）
- 对象上传后事务失败留下可清理 staging 而非公开版本 ✓（新增 `TxnFailureLeavesStagingNotVersion`：failPublish 注入——对象已上传（files.saves=1，内容寻址可回收）、版本零可见、List 空；**同内容重试恢复后正常发布**——staging 对象复用而非残留脏数据）
- v2 发布后 v1 hash 不变 ✓（既有 `PinsImmutableVersions`：两轮发布各自身份隔离；e2e 01 spec 浏览器级 SHA 断言持证）

## 命令与退出码

| 命令 | 退出码 | 结果 |
|---|---|---|
| `go test ./internal/application/service -run TestCraftArtifactPublish -count=1` | 0 | 2 PASS |
| `go test -count=1 ./internal/application/service/` | 0 | ok（33.6s 全包无回归） |

## 回退

revert 本提交（一测试文件新增 + fake 开关增量）。
