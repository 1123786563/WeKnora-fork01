# 移动传输按持久语义与实时媒体分层

移动端使用 REST 提交命令和取得权威 Snapshot，使用带游标的 SSE 接收 Task 与 Run 的持久事件，使用 WebSocket 或 WebRTC 承载实时语音等双向低延迟媒体，并把 APNs/FCM 推送仅视为重新同步的提示。我们借鉴 Happy 与 Paseo 的序号、补洞、慢消费者隔离和能力协商经验，但不复制其与本机 Daemon 或 Machine 身份绑定的统一长连接协议。
