package wiki

import (
	"context"
	"testing"
)

// typed-nil 回归（OCR R1）：宿主 spanTracker.LookupStage / BeginSubSpan 在
// 查无记录、出错、parent==nil 等路径返回 nil *Span；W2 适配器把宿主返回值
// 装箱传入 NewSpan。装箱后 `raw == nil` 不再命中（typed-nil），若不归一，
// wiki 侧 `span == nil` 控制流（beginWikiSubspan 的 parent 判空、批量侧
// WikiSpan 判空跳过）会误判为非空。

type seamsTestHostSpan struct{ id int }

func TestNewSpanNormalizesNilAndTypedNil(t *testing.T) {
	if got := NewSpan(nil); got != nil {
		t.Fatalf("NewSpan(nil) = %v, want nil", got)
	}
	if got := NewSpan((*seamsTestHostSpan)(nil)); got != nil {
		t.Fatalf("NewSpan(typed-nil *seamsTestHostSpan) = %v, want nil（nil 语义必须穿透装箱）", got)
	}
	// 非 nil 指针正常装箱。
	live := &seamsTestHostSpan{id: 7}
	got := NewSpan(live)
	if got == nil {
		t.Fatal("NewSpan(live ptr) 不应为 nil")
	}
	if raw, ok := got.Raw().(*seamsTestHostSpan); !ok || raw != live {
		t.Fatalf("Raw() 往返失真: %#v", got.Raw())
	}
	// Raw 的 nil 接收者安全（hostSpan 解包路径）。
	var nilSpan *Span
	if r := nilSpan.Raw(); r != nil {
		t.Fatalf("nil 接收者 Raw() = %v, want nil", r)
	}
}

// TestSpanWrapperControlFlowParity 锚定 wiki 侧对句柄的判空控制流：nil 句柄
// （含 typed-nil 装箱归一产物）必须与迁移前直传 nil *Span 同样命中跳过分支。
func TestSpanWrapperControlFlowParity(t *testing.T) {
	beginSubspan := func(parent *Span) *Span {
		if parent == nil {
			return nil // 与 beginWikiSubspan 的判空分支同构
		}
		return parent
	}
	if got := beginSubspan(NewSpan((*seamsTestHostSpan)(nil))); got != nil {
		t.Fatalf("typed-nil 装箱句柄应归一为 nil 并命中判空分支, got %#v", got)
	}
	_ = context.Background // 保持 context 导入与生产文件形态一致（防 goimports 漂移）
}
