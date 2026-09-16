# N031 移动数据源 connector 边界证据

日期：2026-09-13

## 实施内容

- 按 Vue `DataSourceEditorDialog.vue` 的 `connectorDefs` 限定移动创建 picker 的 9 个可用 connector。
- 保留已存在但未知的历史数据源行，避免因 picker 过滤而丢失用户已有配置。
- 未修改 API client、后端契约或已有数据源展示/删除语义。

## 验证

- 移动全量测试：97/97 passed。
- `DataSourcesScreen.test.tsx` DOM 页面 harness：16/16 passed。
- Mobile typecheck：passed。
- `git diff --check`：passed。
- 独立只读复审：PASS，无明确 FAIL。

## 证据边界

该切片只证明 connector picker 边界和未知历史行保留。N031 的同步日志/状态、凭据完整生命周期、资源树细节、其他页面本地化、Vue/React 截图、真实后端以及 iOS/Android 原生验收仍未完成。
