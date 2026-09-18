//go:build ignore

// 生成 SSE 真实字节 fixture，供 TS parser 测试消费（G01：Go bytes → TS parser）。
// 写出逻辑逐行复刻本仓库后端：
//   - workbench 流 internal/handler/session/workbench_read.go:318
//     fmt.Fprintf(w, "id: %d\nevent: %s\ndata: %s\n\n", seq, kind, data)
//   - agent_run 流 internal/handler/session/agent_run.go:175-228
//     c.SSEvent(seq字符串, RunEvent) / c.SSEvent("run", ...) / c.SSEvent("keepalive", ...) / c.SSEvent("error", ...)
// 运行: go run tests/sse/gen/main.go > tests/sse/fixtures/go-real-bytes.txt
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
)

func workbenchFrame(buf *bytes.Buffer, seq int64, kind string, raw json.RawMessage) {
	var data bytes.Buffer
	json.Compact(&data, raw)
	fmt.Fprintf(buf, "id: %d\nevent: %s\ndata: %s\n\n", seq, kind, data.Bytes())
}

func ssevent(buf *bytes.Buffer, event string, v any) {
	b, _ := json.Marshal(v)
	fmt.Fprintf(buf, "event: %s\ndata: %s\n\n", event, b)
}

func main() {
	var buf bytes.Buffer

	// agent_run 风格：业务事件（event 名 = seq 数字，data = 完整 RunEvent）
	ssevent(&buf, "1", map[string]any{"seq": 1, "type": "run_started", "payload": map[string]any{"message": "任务已受理"}})
	ssevent(&buf, "2", map[string]any{"seq": 2, "attempt_id": "att_1", "type": "log", "payload": map[string]any{"text": "正在检索知识库"}})
	ssevent(&buf, "3", map[string]any{"seq": 3, "type": "answer_delta", "payload": map[string]any{"delta": "会议纪要：一、产品进度"}})
	// run 快照帧
	ssevent(&buf, "run", map[string]any{"run_id": "run_9", "session_id": "s1", "status": "running", "revision": 4, "seq": 3})
	// keepalive 控制帧
	ssevent(&buf, "keepalive", map[string]any{"seq": 3})
	// error 控制帧
	ssevent(&buf, "error", map[string]any{"code": "cursor_expired", "message": "历史已裁剪", "seq": 3})
	// 事件 data 中含中文 + 转义字符
	ssevent(&buf, "4", map[string]any{"seq": 4, "type": "answer_delta", "payload": map[string]any{"delta": "二、风险与行动项 \"quote\" \n换行"}})

	// workbench 风格：id: seq + event: type + data: payload
	workbenchFrame(&buf, 1, "state", json.RawMessage(`{"phase":"queued"}`))
	workbenchFrame(&buf, 2, "tool_call", json.RawMessage(`{"tool":"search","args":{"q":"客户反馈"}}`))
	workbenchFrame(&buf, 3, "artifact", json.RawMessage(`{"id":"a1","name":"纪要.md"}`))

	os.Stdout.Write(buf.Bytes())
}
