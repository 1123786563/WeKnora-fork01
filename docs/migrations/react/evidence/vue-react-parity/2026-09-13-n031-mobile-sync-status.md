# N031 移动数据源同步状态证据

## 范围

对照 Vue `frontend/src/views/knowledge/settings/DataSourceSettings.vue`，补齐移动端对服务端 `latest_sync_log` 的读取与展示，并在存在 `running` 同步时按 Vue 的 3 秒节奏进行一次静默刷新；组件卸载时清理定时器。

## 代码与契约

- `packages/api-client/src/datasource.ts` 为 `DataSource.latest_sync_log` 及同步日志计数/时间字段补充类型，字段来自 `internal/types/datasource.go` 的 `LatestSyncLog` 与 `docs/swagger.yaml`。
- `apps/mobile/src/features/knowledge/data-sources.ts` 提供 `hasRunningSync`，只对服务端明确返回 `latest_sync_log.status === 'running'` 轮询。
- `apps/mobile/src/features/knowledge/DataSourcesScreen.tsx` 展示最新同步状态、创建数和失败数，并对运行中的同步执行可清理的 3 秒静默刷新。

## 验证

- Mobile full: 99/99
- Resource helper: 8/8
- DataSources DOM harness: 17/17，覆盖 running 状态及结果展示
- Shared typecheck: pass
- Shared datasource-focused tests: pass（test runner 339/339）
- `git diff --check`: pass

## 未闭合项

这只是 N031 的同步状态切片，不代表整个 N031 或总体 Vue/React parity accepted。同步日志抽屉的分页、分组统计、完整本地化，Vue/React 截图、真实后端、Wails、iOS/Android 运行证据仍待完成。
