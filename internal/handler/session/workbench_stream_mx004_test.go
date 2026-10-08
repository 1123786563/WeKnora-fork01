package session

import (
	"bytes"
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/Tencent/WeKnora/internal/workbench"
)

// MX-004 跨语言字节合同：本测试用真实 writer 原语产出 SSE 字节流，
// tests/mobile-v2/probes/mx-004.ts 用真实 TS ExecutionSSEParser 逐字节消费同一文件。
// 禁止任何一侧另造 fixture。

// crlfWriter 把真实输出的 \n 规范化为 \r\n，模拟代理链路对 SSE 的换行改写。
type crlfWriter struct{ inner *bytes.Buffer }

func (w crlfWriter) Write(p []byte) (int, error) {
	replaced := strings.ReplaceAll(string(p), "\n", "\r\n")
	_, err := w.inner.WriteString(replaced)
	return len(p), err
}

func mx004Event(seq int64, eventType string, payload map[string]string) workbench.ExecutionEvent {
	raw, _ := json.Marshal(payload)
	return workbench.ExecutionEvent{
		SchemaVersion: 1,
		RunID:         "run-mx004",
		AttemptID:     "attempt-1",
		Seq:           seq,
		Type:          eventType,
		OccurredAt:    "2026-09-18T08:00:00Z",
		Payload:       raw,
	}
}

// emitMX004Stream 用产品 v2 字节合同写流：心跳注释帧 + 显式 control 帧（无业务 id）
// + 业务帧携带完整 envelope。单事件文件以无尾随空行的截断帧收尾（EOF 路径）。
func emitMX004Stream(t *testing.T, events []workbench.ExecutionEvent, controls int, truncate bool) []byte {
	t.Helper()
	buf := &bytes.Buffer{}
	w := crlfWriter{inner: buf}
	if _, err := w.Write([]byte(": heartbeat\n\n")); err != nil {
		t.Fatalf("heartbeat: %v", err)
	}
	for i := 0; i < controls; i++ {
		if err := writeWorkbenchControlSSE(w, "cursor_expired", "history trimmed; reconnect from snapshot"); err != nil {
			t.Fatalf("control frame: %v", err)
		}
	}
	for index, event := range events {
		if err := writeWorkbenchSSEV2(w, event); err != nil {
			t.Fatalf("event %d: %v", event.Seq, err)
		}
		if index == len(events)-2 && truncate {
			break // 最后一帧故意不写：由接收端 final flush 兜底（EOF 解析路径）
		}
	}
	if truncate {
		// 写出最后事件的 data 但省略结尾空行
		last := events[len(events)-1]
		envelope, err := json.Marshal(last)
		if err != nil {
			t.Fatalf("marshal envelope: %v", err)
		}
		_, _ = w.Write([]byte("id: " + jsonInt(last.Seq) + "\r\nevent: " + last.Type + "\r\ndata: " + string(envelope) + "\r\n"))
	}
	return buf.Bytes()
}

func jsonInt(v int64) string {
	raw, _ := json.Marshal(v)
	return string(raw)
}

func TestMX004EmitSSEBytesSingle(t *testing.T) {
	events := []workbench.ExecutionEvent{
		mx004Event(42, "text.delta", map[string]string{"message_id": "m-42", "text": "已完成检索：知识库与反馈"}),
	}
	bytesOut := emitMX004Stream(t, events, 1, true)
	if err := os.WriteFile("../../../tests/mobile-v2/fixtures/mx-004-stream.bin", bytesOut, 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	if !bytes.Contains(bytesOut, []byte("event: control")) {
		t.Fatal("expected explicit control frame in v2 stream")
	}
}

func TestMX004EmitSSEBytesBulk(t *testing.T) {
	// 300 条业务帧 + 每 60 条插入一个 control 帧 + 心跳：验证 >256 条无丢失且控制帧不占业务 seq。
	var events []workbench.ExecutionEvent
	for seq := int64(1); seq <= 300; seq++ {
		events = append(events, mx004Event(seq, "text.delta", map[string]string{"text": "块"}))
	}
	buf := &bytes.Buffer{}
	w := crlfWriter{inner: buf}
	_, _ = w.Write([]byte(": heartbeat\n\n"))
	controlCount := 0
	for index, event := range events {
		if index > 0 && index%60 == 0 {
			if err := writeWorkbenchControlSSE(w, "heartbeat_ack", "keepalive"); err != nil {
				t.Fatalf("control: %v", err)
			}
			controlCount++
		}
		if err := writeWorkbenchSSEV2(w, event); err != nil {
			t.Fatalf("event %d: %v", event.Seq, err)
		}
	}
	if err := os.WriteFile("../../../tests/mobile-v2/fixtures/mx-004-stream-300.bin", buf.Bytes(), 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	if controlCount != 4 {
		t.Fatalf("expected 4 interleaved control frames, got %d", controlCount)
	}
}
