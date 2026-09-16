# r109 Android 图谱禁用能力错误本地化验收

- `test36-small` 连接本地后端并保持认证态，进入 `Parity KB Demo` → `图谱`。
- 后端返回 Wiki feature disabled（错误码 1000）时，图谱错误卡显示“此知识库未启用知识图谱功能”，保留“重试”操作，不泄漏英文原始错误串。
- 移动端全量测试 190/190、typecheck、Android Release 增量构建通过。
- 设备截图：`/tmp/android-graph-fixed.png`。
