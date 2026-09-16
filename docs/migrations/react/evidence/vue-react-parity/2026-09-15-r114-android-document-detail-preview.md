# r114 Android 文档列表、详情与预览状态验收

- 通过认证 API 向 `Parity KB Demo` 上传临时 `android-parity.txt`，返回文档 id `0efc8042-0242-4e4e-859f-78773c2f83b0`，状态 `pending`。
- Android `test36-small` 刷新文档页后显示 `1 项`、文件名 `android-parity.txt` 和 `processing` 状态。
- 点击文档进入详情页，显示返回、文件名、状态、预览、下载并分享、文档 ID、类型和大小。
- 处理中的文档点击预览显示本地化“文档进行中时无法预览。”，没有下载或泄漏文件内容。
- 复验完成后通过认证 API 删除临时文档，租户恢复原始数据。
- 截图：`/tmp/android-doc-list.png`、`/tmp/android-doc-detail.png`、`/tmp/android-doc-preview-processing.png`。
