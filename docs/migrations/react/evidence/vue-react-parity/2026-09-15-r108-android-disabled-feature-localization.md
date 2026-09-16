# r108 Android Wiki 禁用能力错误本地化验收

- 在 Android `test36-small` 上连接本地后端并保持认证态，进入 `Parity KB Demo` → `Wiki`。
- 后端返回 Wiki feature disabled（错误码 1000）时，页面显示本地化文案“此知识库未启用 Wiki 功能”，不再泄漏英文原始错误串。
- Release 增量构建成功，移动端全量测试 190/190、typecheck 通过。
- 设备截图：`/tmp/android-wiki-fixed.png`。
