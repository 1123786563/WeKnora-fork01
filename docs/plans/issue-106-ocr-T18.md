Review complete: 2 finding(s) across 3 selected item(s).

─── internal/application/service/plugin_install_service.go:1892-1898 ───
[bug · low] rowsByTool 存在性检查与 SetPolicy 写入之间存在 check-then-act 竞态窗口：该检查基于
resolveInstallationPolicyStore 快照读（ListByService），无事务/锁保护。若并发请求（如管理员 A 对同一工具显式 PUT {"enabled":
false} 的行恰好在此窗口落库），本方 requireApproval-only 补丁已将 enabledPatch 物化为非 nil（=snapshotTool.ReadOnly），经
UpsertPolicy 的 ON CONFLICT DoUpdates 会覆盖并发行的 enabled——违反本函数注释声明的「已存在行在 requireApproval-only 补丁下保持其
verdict」契约。读工具场景：并发管理员的显式 enabled=false 会被静默回置为 true（治理意图丢失）；写工具场景为 fail-closed
但仍是丢失更新。窗口极窄且仅限管理面并发，建议：物化默认改用「insert-if-absent」（如先以 ON CONFLICT DO NOTHING 落默认行、再应用
requireApproval-only 补丁），或在注释中显式接受并记录该竞态语义。



─── internal/handler/plugin.go:863-865 ───
[documentation · low] 本次为 ErrInstallationServiceMissing 新增的 409 映射会使 PUT
/plugins/installations/{id}/tools/{tool_name}/policy 返回 409 Conflict，但该端点的 Swagger 契约（@Failure 列表，约
445-447 行）仍只声明 400/404/500，未补充 409。生成的 OpenAPI 文档将缺失这一状态码，API 消费方无法预期「无可物化服务（需卸载重装恢复）」的冲突语义。建议在该端点
godoc 中同步补充 @Failure 409。


