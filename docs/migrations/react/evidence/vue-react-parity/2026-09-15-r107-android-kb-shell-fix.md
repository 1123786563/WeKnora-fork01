# r107 Android 知识库壳层修复验收

## 发现与修复

Release 设备复验发现 Expo Router 默认 pathname header 会在业务页顶部显示 `knowledge/index` / `knowledge/[id]`，且文档页计数直接显示 `{count} 项`。本次修复：

- `(app)/_layout.tsx` 关闭 Expo Router 默认 header，由各业务 screen 自己渲染 Vue 对齐 header。
- `KnowledgeDocumentsScreen` 将 `total` 传入 `common.itemCount` 插值。
- 新增文档列表计数测试，移动端定向测试 5/5、typecheck 通过。

## 设备复验

- Release APK 重新 `assembleRelease` 成功（16s 增量构建）。
- `test36-small` 重新安装并启动成功，知识库列表不再显示 `knowledge/index`，真实 KB `Parity KB Demo` 正常加载。
- 进入 KB 文档页后不再显示 `knowledge/[id]`，计数正确显示 `0 项`。
- 截图：`/tmp/android-kb-detail-fixed.png`、`/tmp/android-kb-detail-fixed2.png`。
