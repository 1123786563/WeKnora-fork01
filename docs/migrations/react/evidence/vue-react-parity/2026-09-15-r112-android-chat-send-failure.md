# r112 Android 聊天发送与失败态验收

- 环境：Android `test36-small`、Release APK、本地后端 `http://10.0.2.2:8080`、认证账号 `parity-test@local.dev`、知识库 `Parity KB Demo`。
- 在 ChatScreen 输入 `hello` 并点击“发送”，用户消息成功加入当前会话，随后渲染 assistant 失败结果 `Sorry, I am unable to answer this question.`。
- 失败后输入区恢复为可发送状态，页面显示“补充队列/暂无排队指令”和“重试”操作；未出现崩溃或卡死。
- 设备截图：`/tmp/android-chat-sent.png`。

## 证据边界

本项关闭 Android 发送触发、用户消息落地、失败响应渲染和输入恢复门禁。由于当前租户没有可用的成功聊天模型，完整 token 流、停止、审批、附件、引用及成功响应仍开放。
