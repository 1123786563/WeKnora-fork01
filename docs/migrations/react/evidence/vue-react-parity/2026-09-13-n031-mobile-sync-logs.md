# N031 移动数据源同步日志证据

对照 Vue `frontend/src/views/knowledge/settings/DataSourceSyncLogs.vue`，移动端日志抽屉现在使用服务端 `limit/offset` 读取，支持 50 条分页、加载更多、重复请求保护、汇总统计、单条展开计数与关闭后状态重置。

验证：DataSources DOM harness 19/19；mobile typecheck pass；`git diff --check` pass。

状态文案、日期分组、持续时间、错误码本地化及 Vue/React 截图、真实后端、Wails、iOS/Android 运行证据仍未闭合；本切片不改变 N031 的 `implementing` 状态。
