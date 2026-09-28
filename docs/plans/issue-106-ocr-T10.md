Review complete: 1 finding(s) across 1 selected item(s).

─── internal/modules/plugins/plugintest/server.go:281-281 ───
[style · low] 导出方法名 ManifestHandler_mutate 含下划线，不符合 Go 命名惯例（golint/Staticcheck ST1003 会告警）。该 API 将被
T11/T13/T16-T19 多个集成任务引用，导出后不易再改名，建议改用惯用的动词前置命名（如 MutateManifest），与同文件 EnableOAuth/SetTools 风格保持一致。

- func (s *Server) ManifestHandler_mutate(fn func(*types.PluginManifest)) {
+ func (s *Server) MutateManifest(fn func(*types.PluginManifest)) {

