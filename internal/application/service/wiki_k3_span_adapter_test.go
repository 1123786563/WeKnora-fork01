// Pass B（23-knowledge-wikifaq）W2 span 适配器 typed-nil 回归测试（OCR R1）。
//
// 生产装配路径：wikiK3SpanAdapter{inner: 宿主 spanTracker}。宿主
// LookupStage（查无记录 knowledge_span_tracker.go:590 / list 失败 :568）与
// BeginSubSpan（parent==nil :406-408 / Upsert 失败 :434-437）返回 nil *Span；
// 适配器装箱进 wiki.Span 前必须归一，否则 wiki 侧判空控制流失真。
package service

import (
	"context"
	"testing"

	"github.com/Tencent/WeKnora/internal/modules/knowledge/wiki"
	"github.com/Tencent/WeKnora/internal/types"
)

// typedNilHostTracker 覆盖两条生产 nil 返回路径：LookupStage 未命中与
// BeginSubSpan 失败。其余方法经 nil 接口嵌入（本测试不触达）。
type typedNilHostTracker struct {
	SpanTracker // nil 接口嵌入：未覆盖方法被调用即 panic，测试即失败
}

func (typedNilHostTracker) LookupStage(_ context.Context, _ string, _ int, _ string) *Span {
	return nil // 模拟宿主查无记录 / list 失败（:568/:590）
}

func (typedNilHostTracker) BeginSubSpan(_ context.Context, _ *Span, _, _ string, _ types.JSONMap) *Span {
	return nil // 模拟宿主 parent==nil / Upsert 失败（:406-408/:434-437）
}

// liveHostTracker 覆盖非 nil 往返（防归一化误伤真实句柄）。
type liveHostTracker struct {
	typedNilHostTracker
	beginCallsWithNilParent int
}

func (l *liveHostTracker) LookupStage(_ context.Context, _ string, _ int, _ string) *Span {
	return &Span{}
}

func (l *liveHostTracker) BeginSubSpan(_ context.Context, parent *Span, _, _ string, _ types.JSONMap) *Span {
	if parent == nil {
		l.beginCallsWithNilParent++
	}
	return &Span{}
}

func TestWikiK3SpanAdapterTypedNilPassesThrough(t *testing.T) {
	a := wikiK3SpanAdapter{inner: typedNilHostTracker{}}
	ctx := context.Background()

	if got := a.LookupStage(ctx, "kid", 1, "postprocess"); got != nil {
		t.Fatalf("适配器 LookupStage 装箱宿主 typed-nil 后应为 nil 句柄, got %#v", got)
	}
	if got := a.BeginSubSpan(ctx, nil, "postprocess.wiki", types.SpanKindSubSpan, nil); got != nil {
		t.Fatalf("适配器 BeginSubSpan 装箱宿主 typed-nil 后应为 nil 句柄, got %#v", got)
	}

	// 判空控制流同构断言（beginWikiSubspan 形态）。
	beginWikiSubspan := func(parent *wiki.Span) *wiki.Span {
		if parent == nil {
			return nil
		}
		return parent
	}
	if got := beginWikiSubspan(a.LookupStage(ctx, "kid", 1, "postprocess")); got != nil {
		t.Fatalf("生产装配路径 typed-nil 未穿透判空分支, got %#v", got)
	}
}

func TestWikiK3SpanAdapterRoundTripsLiveSpans(t *testing.T) {
	inner := &liveHostTracker{}
	a := wikiK3SpanAdapter{inner: inner}
	ctx := context.Background()

	stage := a.LookupStage(ctx, "kid", 1, "postprocess")
	if stage == nil {
		t.Fatal("宿主非 nil 句柄被误归一")
	}
	sub := a.BeginSubSpan(ctx, stage, "postprocess.wiki", types.SpanKindSubSpan, types.JSONMap{})
	if sub == nil {
		t.Fatal("宿主非 nil 子句柄被误归一")
	}
	// wiki 句柄 → 宿主 *Span 解包往返。
	if host := a.hostSpan(sub); host == nil {
		t.Fatal("非 nil 句柄解包得 nil 宿主 *Span")
	}
	// nil 句柄解包为 nil 宿主（EndSpan/FailSpan/SkipSpan 的 nil 语义）。
	if host := a.hostSpan(nil); host != nil {
		t.Fatalf("nil 句柄解包应为 nil, got %#v", host)
	}
	// 链路上 nil 父句柄应原样传给宿主（宿主自身 nil-safe 分支处理）。
	a.BeginSubSpan(ctx, nil, "orphan", types.SpanKindSubSpan, nil)
	if inner.beginCallsWithNilParent != 1 {
		t.Fatalf("nil 父句柄应原样穿透给宿主 BeginSubSpan, 计数 = %d", inner.beginCallsWithNilParent)
	}
}
