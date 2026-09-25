Review complete: 2 finding(s) across 5 selected item(s).

─── internal/application/repository/craft_version.go:280-285 ───
[bug · medium] “已存在”分支无条件补插证据行，打破了本变更自身声明的固定语义。注释的推理“absent ⇒ 崩溃提交从未发生（行同事务）”忽略了两个真实可达的先发布路径：(1)
部署升级——本变更合入前用无证据 Publish 发布的历史版本，在升级后收到重放晋升回调（PromoteWebVersion 注释明确承诺 "a replayed identical
callback adopts the already published version"）会走到此分支并被事后补插证据；(2) 同一存储上 CollectForKind 仍走无证据
Publish。补插后这些“晋升时未固定证据”的版本从“读取答 ErrNotFound”变为携带证据，且 PinnedAt 记录的是重放时刻而非原始晋升时刻，与
VersionEvidenceStore 接口及 VersionEvidence 读取方法自述的 "a version that was promoted without evidence
answers ErrNotFound / history is never reconstructed" 不变量直接矛盾。建议：仅在版本行与证据行确认同事务引入（例如行内记录发布代次，或仅允许
adopt 已有证据、对无证据的历史行保持 ErrNotFound）时才允许写入，否则将补插限制为显式运维/迁移动作。



─── internal/modules/craft/version.go:422-427 ───
[maintainability · low] VersionEvidenceDigest 存在两个问题：(1) 生产代码未使用它——CraftVersionStore.publish 内联了相同的
json.Marshal+sha256 计算，两处摘要逻辑并存，未来任一侧引入规范化（字段排序、时间精度截断）即产生分歧；(2) 注释与实现矛盾——digest 覆盖了 PinnedAt，而重放晋升的
PinnedAt 必然不同，因此“Identical facts always derive the identical digest — a replayed promotion adopts
the stored row”并不成立；仓储层的重放采纳实际依赖 sameCraftVersionEvidence（刻意排除 PinnedAt），而非
digest。若后续有人按此注释的承诺把存储端比对改到该函数上，幂等重放会被错误拒绝。建议：仓储层复用该领域函数（或在函数内对 PinnedAt 做明确取舍），并修正注释使其与实际摘要口径一致。


